package e2e

import (
	"testing"

	wasmibctesting "github.com/CosmWasm/wasmd/x/wasm/ibctesting"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	tmtypes "github.com/cometbft/cometbft/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/notional-labs/composable/v6/app"
)

// DefaultComposableAppFactory instantiates and sets up the default Composable App
func DefaultComposableAppFactory(t *testing.T, valSet *tmtypes.ValidatorSet, genAccs []authtypes.GenesisAccount, chainID string, opts []wasmkeeper.Option, balances ...banktypes.Balance) wasmibctesting.ChainApp {
	t.Helper()
	return app.SetupWithGenesisValSet(t, valSet, genAccs, chainID, opts, balances...)
}
