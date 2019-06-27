package distributionx

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewHandler(k Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		switch msg := msg.(type) {
		case MsgDonateToCommunityPool:
			return handleMsgDonateToCommunityPool(ctx, k, msg)
		default:
			errMsg := "Unrecognized distributionx Msg type: %s" + msg.Type()
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}

func handleMsgDonateToCommunityPool(ctx sdk.Context, k Keeper, msg MsgDonateToCommunityPool) sdk.Result {

	res := k.bxk.SubtractCoins(ctx, msg.FromAddr, msg.Amount)
	if res != nil {
		return res.Result()
	}

	feePool := k.dk.GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(sdk.NewDecCoins(msg.Amount))
	k.dk.SetFeePool(ctx, feePool)

	return sdk.Result{
		Tags: sdk.NewTags(
			TagKeyDonator, msg.FromAddr.String(),
		),
	}
}