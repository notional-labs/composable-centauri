package e2e_test

import (
	"fmt"
	"github.com/notional-labs/composable/v6/app"
	"github.com/notional-labs/composable/v6/tests/e2e"
	"testing"
	"time"

	wasmvmtypes "github.com/CosmWasm/wasmvm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"github.com/CosmWasm/wasmd/x/wasm/ibctesting"
)

func TestGovVoteByContract(t *testing.T) {
	// Given a contract with delegation
	// And   a gov proposal
	// When  the contract sends a vote for the proposal
	// Then	 the vote is taken into account

	coord := ibctesting.NewCoordinatorX(t, 1, e2e.DefaultComposableAppFactory)
	chain := coord.GetChain(ibctesting.GetChainID(1))
	contractAddr := e2e.InstantiateReflectContract(t, chain)
	chain.Fund(contractAddr, sdkmath.NewIntFromUint64(1_000_000_000))
	// a contract with a high delegation amount
	delegateMsg := wasmvmtypes.CosmosMsg{
		Staking: &wasmvmtypes.StakingMsg{
			Delegate: &wasmvmtypes.DelegateMsg{
				Validator: sdk.ValAddress(chain.Vals.Validators[0].Address).String(),
				Amount: wasmvmtypes.Coin{
					Denom:  sdk.DefaultBondDenom,
					Amount: "1000000",
				},
			},
		},
	}
	e2e.MustExecViaReflectContract(t, chain, contractAddr, delegateMsg)

	signer := chain.SenderAccount.GetAddress().String()
	app := chain.App.(*app.ComposableApp)
	govKeeper, accountKeeper := app.GovKeeper, app.AccountKeeper
	chain.NextBlock()
	fmt.Println(accountKeeper.GetModuleAccount(chain.GetContext(), distributiontypes.ModuleName).GetAddress().String())
	communityPoolBalance := chain.Balance(accountKeeper.GetModuleAccount(chain.GetContext(), distributiontypes.ModuleName).GetAddress(), sdk.DefaultBondDenom)
	require.False(t, communityPoolBalance.IsZero())

	gParams, err := govKeeper.Params.Get(chain.GetContext())
	require.NoError(t, err)
	initialDeposit := gParams.MinDeposit
	govAcctAddr := govKeeper.GetGovernanceAccount(chain.GetContext()).GetAddress()

	specs := map[string]struct {
		vote    *wasmvmtypes.VoteMsg
		expPass bool
	}{
		"yes": {
			vote: &wasmvmtypes.VoteMsg{
				Vote: wasmvmtypes.Yes,
			},
			expPass: true,
		},
		"no": {
			vote: &wasmvmtypes.VoteMsg{
				Vote: wasmvmtypes.No,
			},
			expPass: false,
		},
		"abstain": {
			vote: &wasmvmtypes.VoteMsg{
				Vote: wasmvmtypes.Abstain,
			},
			expPass: true,
		},
		"no with veto": {
			vote: &wasmvmtypes.VoteMsg{
				Vote: wasmvmtypes.NoWithVeto,
			},
			expPass: false,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// given a unique recipient
			recipientAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
			// and a new proposal
			payloadMsg := &distributiontypes.MsgCommunityPoolSpend{
				Authority: govAcctAddr.String(),
				Recipient: recipientAddr.String(),
				Amount:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.OneInt())),
			}
			msg, err := v1.NewMsgSubmitProposal(
				[]sdk.Msg{payloadMsg},
				initialDeposit,
				signer,
				"",
				"my proposal",
				"testing",
				false,
			)
			require.NoError(t, err)
			rsp, gotErr := chain.SendMsgs(msg)
			require.NoError(t, gotErr)
			var got v1.MsgSubmitProposalResponse
			chain.UnwrapExecTXResult(rsp, &got)

			propID := got.ProposalId

			// with other delegators voted yes
			_, err = chain.SendMsgs(v1.NewMsgVote(chain.SenderAccount.GetAddress(), propID, v1.VoteOption_VOTE_OPTION_YES, ""))
			require.NoError(t, err)

			// when contract votes
			spec.vote.ProposalId = propID
			voteMsg := wasmvmtypes.CosmosMsg{
				Gov: &wasmvmtypes.GovMsg{
					Vote: spec.vote,
				},
			}
			e2e.MustExecViaReflectContract(t, chain, contractAddr, voteMsg)

			// then proposal executed after voting period
			proposal, err := govKeeper.Proposals.Get(chain.GetContext(), propID)
			require.NoError(t, err)
			coord.IncrementTimeBy(proposal.VotingEndTime.Sub(chain.GetContext().BlockTime()) + time.Minute)
			coord.CommitBlock(chain)

			proposal, err = govKeeper.Proposals.Get(chain.GetContext(), propID)

			// and recipient balance updated
			recipientBalance := chain.Balance(recipientAddr, sdk.DefaultBondDenom)
			if !spec.expPass {
				assert.True(t, recipientBalance.IsZero())
				return
			}
			expBalanceAmount := sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.OneInt())
			assert.Equal(t, expBalanceAmount.String(), recipientBalance.String())
		})
	}
}
