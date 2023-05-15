package keeper

import (
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	porttypes "github.com/cosmos/ibc-go/v7/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/notional-labs/banksy/v2/x/transfermiddleware/types"
	routerkeeper "github.com/strangelove-ventures/packet-forward-middleware/v7/router/keeper"
)

type Keeper struct {
	cdc            codec.BinaryCodec
	storeKey       storetypes.StoreKey
	ICS4Wrapper    porttypes.ICS4Wrapper
	bankKeeper     types.BankKeeper
	transferKeeper types.TransferKeeper
	routerKeeper   *routerkeeper.Keeper
	// the address capable of executing a AddParachainIBCTokenInfo and RemoveParachainIBCTokenInfo message. Typically, this
	// should be the x/gov module account.
	authority string
}

// NewKeeper returns a new instance of the x/ibchooks keeper
func NewKeeper(
	storeKey storetypes.StoreKey,
	codec codec.BinaryCodec,
	ics4Wrapper porttypes.ICS4Wrapper,
	transferKeeper types.TransferKeeper,
	bankKeeper types.BankKeeper,
	router *routerkeeper.Keeper,
) Keeper {
	return Keeper{
		storeKey:       storeKey,
		transferKeeper: transferKeeper,
		bankKeeper:     bankKeeper,
		cdc:            codec,
		ICS4Wrapper:    ics4Wrapper,
		routerKeeper:   router,
	}
}

// TODO: testing
// AddParachainIBCTokenInfo add new parachain token information token to chain state.
func (keeper Keeper) AddParachainIBCInfo(ctx sdk.Context, ibcDenom, channelId, nativeDenom string) error {
	if keeper.hasParachainIBCTokenInfo(ctx, nativeDenom) {
		return types.ErrDuplicateParachainIBCTokenInfo
	}

	info := types.ParachainIBCTokenInfo{
		IbcDenom:    ibcDenom,
		ChannelId:   channelId,
		NativeDenom: nativeDenom,
	}

	bz, err := keeper.cdc.Marshal(&info)
	if err != nil {
		return err
	}
	store := ctx.KVStore(keeper.storeKey)
	store.Set(types.GetKeyParachainIBCTokenInfo(nativeDenom), bz)

	// update the IBCdenom-native index
	store.Set(types.GetKeyIBCDenomAndNativeIndex(ibcDenom), []byte(nativeDenom))
	return nil
}

// TODO: testing
// RemoveParachainIBCTokenInfo remove parachain token information from chain state.
func (keeper Keeper) RemoveParachainIBCInfo(ctx sdk.Context, nativeDenom string) error {
	if !keeper.hasParachainIBCTokenInfo(ctx, nativeDenom) {
		return types.ErrDuplicateParachainIBCTokenInfo
	}

	// get the IBCdenom
	IBCDenom := keeper.GetParachainIBCTokenInfo(ctx, nativeDenom).IbcDenom

	store := ctx.KVStore(keeper.storeKey)
	store.Delete(types.GetKeyParachainIBCTokenInfo(nativeDenom))

	// update the IBCdenom-native index
	if !store.Has(types.GetKeyIBCDenomAndNativeIndex(IBCDenom)) {
		panic("broken data in state")
	}

	store.Delete(types.GetKeyIBCDenomAndNativeIndex(IBCDenom))

	return nil
}

// TODO: testing
// GetParachainIBCTokenInfo add new information about parachain token to chain state.
func (keeper Keeper) GetParachainIBCTokenInfo(ctx sdk.Context, nativeDenom string) (info types.ParachainIBCTokenInfo) {
	store := ctx.KVStore(keeper.storeKey)
	bz := store.Get(types.GetKeyParachainIBCTokenInfo(nativeDenom))

	keeper.cdc.Unmarshal(bz, &info)

	return info
}

func (keeper Keeper) GetNativeDenomByIBCDenomSecondaryIndex(ctx sdk.Context, IBCdenom string) string {
	store := ctx.KVStore(keeper.storeKey)
	bz := store.Get(types.GetKeyParachainIBCTokenInfo(IBCdenom))

	return string(bz)
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+exported.ModuleName+"-"+types.ModuleName)
}
