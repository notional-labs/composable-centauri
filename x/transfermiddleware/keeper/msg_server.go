package keeper

import (
	"context"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/notional-labs/banksy/v2/x/transfermiddleware/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	Keeper
}

func (ms msgServer) AddParachainIBCTokenInfo(goCtx context.Context, req *types.MsgAddParachainIBCTokenInfo) (*types.MsgAddParachainIBCTokenInfoResponse, error) {
	if ms.authority != req.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	err := ms.Keeper.AddParachainIBCTokenInfo(ctx, req.IbcDenom, req.ChannelId, req.NativeDenom)

	if err != nil {
		return nil, err
	}

	return &types.MsgAddParachainIBCTokenInfoResponse{}, nil
}

func (ms msgServer) RemoveParachainIBCTokenInfo(goCtx context.Context, req *types.MsgRemoveParachainIBCTokenInfo) (*types.MsgRemoveParachainIBCTokenInfoResponse, error) {
	return &types.MsgRemoveParachainIBCTokenInfoResponse{}, nil
}