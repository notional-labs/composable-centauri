package interchaintest

import (
	"context"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	interchaintestrelayer "github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
)

// TestComposableGaiaIBCTransfer spins up a Composable and Gaia network, initializes an IBC connection between them,
// and sends an ICS20 token transfer from Composable->Gaia and then back from Gaia->Composable.
func TestComposableGaiaIBCTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	// Create chain factory with Composable and Gaia
	numVals := 1
	numFullNodes := 1

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "Centauri",
			ChainConfig:   CentauriConfig,
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
		},
		{
			Name:          "gaia",
			Version:       "v9.1.0",
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
		},
	})

	const (
		path = "ibc-path"
	)

	// Get chains from the chain factory
	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	client, network := interchaintest.DockerSetup(t)

	composable, gaia := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	relayerType, relayerName := ibc.CosmosRly, "relay"

	// Get a relayer instance
	rf := interchaintest.NewBuiltinRelayerFactory(
		relayerType,
		zaptest.NewLogger(t),
		relayer.DockerImage(&DefaultRelayer),
		interchaintestrelayer.StartupFlags("--processor", "events", "--block-history", "100"),
	)

	r := rf.Build(t, client, network)

	ic := interchaintest.NewInterchain().
		AddChain(composable).
		AddChain(gaia).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  composable,
			Chain2:  gaia,
			Relayer: r,
			Path:    path,
		})

	ctx := context.Background()

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create some user accounts on both chains
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), genesisWalletAmount, composable, gaia)

	// Wait a few blocks for relayer to start and for user accounts to be created
	err = testutil.WaitForBlocks(ctx, 5, composable, gaia)
	require.NoError(t, err)

	// Get our Bech32 encoded user addresses
	composableUser, gaiaUser := users[0], users[1]

	composableUserAddr := composableUser.FormattedAddress()
	gaiaUserAddr := gaiaUser.FormattedAddress()

	// Get original account balances
	composableOrigBal, err := composable.GetBalance(ctx, composableUserAddr, composable.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, genesisWalletAmount, composableOrigBal)

	gaiaOrigBal, err := gaia.GetBalance(ctx, gaiaUserAddr, gaia.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, genesisWalletAmount, gaiaOrigBal)

	// Compose an IBC transfer and send from Composable -> Gaia
	var transferAmount = math.NewInt(1_000)
	transfer := ibc.WalletAmount{
		Address: gaiaUserAddr,
		Denom:   composable.Config().Denom,
		Amount:  transferAmount,
	}

	channel, err := ibc.GetTransferChannel(ctx, r, eRep, composable.Config().ChainID, gaia.Config().ChainID)
	require.NoError(t, err)

	composableHeight, err := composable.Height(ctx)
	require.NoError(t, err)

	transferTx, err := composable.SendIBCTransfer(ctx, channel.ChannelID, composableUserAddr, transfer, ibc.TransferOptions{})
	require.NoError(t, err)

	err = r.StartRelayer(ctx, eRep, path)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occurred while stopping the relayer: %s", err)
			}
		},
	)

	// Poll for the ack to know the transfer was successful
	_, err = testutil.PollForAck(ctx, composable, composableHeight, composableHeight+50, transferTx.Packet)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 10, composable)
	require.NoError(t, err)

	// Get the IBC denom for ppica on Gaia
	composableTokenDenom := transfertypes.GetPrefixedDenom(channel.Counterparty.PortID, channel.Counterparty.ChannelID, composable.Config().Denom)
	composableIBCDenom := transfertypes.ParseDenomTrace(composableTokenDenom).IBCDenom()

	// Assert that the funds are no longer present in user acc on Composable and are in the user acc on Gaia
	composableUpdateBal, err := composable.GetBalance(ctx, composableUserAddr, composable.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, composableOrigBal.Sub(transferAmount), composableUpdateBal)

	gaiaUpdateBal, err := gaia.GetBalance(ctx, gaiaUserAddr, composableIBCDenom)
	require.NoError(t, err)
	require.Equal(t, transferAmount, gaiaUpdateBal)

	// Compose an IBC transfer and send from Gaia -> Composable
	transfer = ibc.WalletAmount{
		Address: composableUserAddr,
		Denom:   composableIBCDenom,
		Amount:  transferAmount,
	}

	gaiaHeight, err := gaia.Height(ctx)
	require.NoError(t, err)

	transferTx, err = gaia.SendIBCTransfer(ctx, channel.Counterparty.ChannelID, gaiaUserAddr, transfer, ibc.TransferOptions{})
	require.NoError(t, err)

	// Poll for the ack to know the transfer was successful
	_, err = testutil.PollForAck(ctx, gaia, gaiaHeight, gaiaHeight+25, transferTx.Packet)
	require.NoError(t, err)

	// Assert that the funds are now back on Composable and not on Gaia
	composableUpdateBal, err = composable.GetBalance(ctx, composableUserAddr, composable.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, composableOrigBal, composableUpdateBal)

	gaiaUpdateBal, err = gaia.GetBalance(ctx, gaiaUserAddr, composableIBCDenom)
	require.NoError(t, err)
	require.Equal(t, true, gaiaUpdateBal.IsZero())
}