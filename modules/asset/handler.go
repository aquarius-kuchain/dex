package asset

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
)

// NewHandler returns a handler for "asset" type messages.
func NewHandler(tk TokenKeeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		switch msg := msg.(type) {
		case MsgIssueToken:
			return handleMsgIssueToken(ctx, tk, msg)
		case MsgTransferOwnership:
			return handleMsgTransferOwnership(ctx, tk, msg)
		case MsgFreezeAddress:
			return handleMsgFreezeAddress(ctx, tk, msg)
		case MsgUnfreezeAddress:
			return handleMsgUnfreezeAddress(ctx, tk, msg)
		case MsgFreezeToken:
			return handleMsgFreezeToken(ctx, tk, msg)
		case MsgUnfreezeToken:
			return handleMsgUnfreezeToken(ctx, tk, msg)
		case MsgBurnToken:
			return handleMsgBurnToken(ctx, tk, msg)
		case MsgMintToken:
			return handleMsgMintToken(ctx, tk, msg)

		default:
			errMsg := "Unrecognized asset Msg type: %s" + msg.Type()
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}

func setCoins(ctx sdk.Context, am auth.AccountKeeper, addr sdk.AccAddress, amt sdk.Coins) sdk.Error {
	if !amt.IsValid() {
		return sdk.ErrInvalidCoins(amt.String())
	}
	acc := am.GetAccount(ctx, addr)
	if acc == nil {
		return sdk.ErrUnknownAddress("no issue address")
	}
	err := acc.SetCoins(amt)
	if err != nil {
		// Handle w/ #870
		panic(err)
	}
	am.SetAccount(ctx, acc)
	return nil
}

func subTokenFee(ctx sdk.Context, tk TokenKeeper, addr sdk.AccAddress, fee sdk.Coins) sdk.Error {

	acc := tk.ak.GetAccount(ctx, addr)
	if acc == nil {
		return sdk.ErrUnknownAddress("no valid address")
	}

	oldCoins := acc.GetCoins()
	spendableCoins := acc.SpendableCoins(ctx.BlockHeader().Time)

	_, hasNeg := spendableCoins.SafeSub(fee)
	if hasNeg {
		return sdk.ErrInsufficientCoins(
			fmt.Sprintf("insufficient account funds; %s < %s", spendableCoins, fee))
	}

	newCoins := oldCoins.Sub(fee) // should not panic as spendable coins was already checked
	if err := setCoins(ctx, tk.ak, addr, newCoins); err != nil {
		return err
	}

	return nil
}
func addTokenCoins(ctx sdk.Context, tk TokenKeeper, addr sdk.AccAddress, amt sdk.Coins) sdk.Error {

	acc := tk.ak.GetAccount(ctx, addr)
	if acc == nil {
		return sdk.ErrUnknownAddress("no valid address")
	}

	if !amt.IsValid() {
		return sdk.ErrInvalidCoins(amt.String())
	}

	oldCoins := acc.GetCoins()
	newCoins := oldCoins.Add(amt)

	if newCoins.IsAnyNegative() {
		return sdk.ErrInsufficientCoins(
			fmt.Sprintf("insufficient account funds; %s < %s", oldCoins, amt),
		)
	}

	err := setCoins(ctx, tk.ak, addr, newCoins)

	return err
}

// handleMsgIssueToken - Handle MsgIssueToken
func handleMsgIssueToken(ctx sdk.Context, tk TokenKeeper, msg MsgIssueToken) sdk.Result {

	issueFee := tk.GetParams(ctx).IssueTokenFee
	if err := subTokenFee(ctx, tk, msg.Owner, issueFee); err != nil {
		return err.Result()
	}
	tk.fck.AddCollectedFees(ctx, issueFee)

	if err := tk.IssueToken(ctx, msg); err != nil {
		return err.Result()
	}

	if err := addTokenCoins(ctx, tk, msg.Owner, NewTokenCoins(msg.Symbol, msg.TotalSupply)); err != nil {
		return err.Result()
	}

	return sdk.Result{}
}

// handleMsgTransferOwnership - Handle MsgTransferOwnership
func handleMsgTransferOwnership(ctx sdk.Context, tk TokenKeeper, msg MsgTransferOwnership) (res sdk.Result) {

	return
}

// handleMsgFreezeAddress - Handle MsgFreezeAddress
func handleMsgFreezeAddress(ctx sdk.Context, tk TokenKeeper, msg MsgFreezeAddress) (res sdk.Result) {

	return
}

// handleMsgUnfreezeAddress - Handle MsgUnfreezeAddress
func handleMsgUnfreezeAddress(ctx sdk.Context, tk TokenKeeper, msg MsgUnfreezeAddress) (res sdk.Result) {

	return
}

// handleMsgFreezeToken - HandleMsgFreezeToken
func handleMsgFreezeToken(ctx sdk.Context, tk TokenKeeper, msg MsgFreezeToken) (res sdk.Result) {

	return
} // handleMsgUnfreezeToken - Handle MsgUnfreezeToken
func handleMsgUnfreezeToken(ctx sdk.Context, tk TokenKeeper, msg MsgUnfreezeToken) (res sdk.Result) {

	return
}

// handleMsgBurnToken - Handle MsgBurnToken
func handleMsgBurnToken(ctx sdk.Context, tk TokenKeeper, msg MsgBurnToken) (res sdk.Result) {

	return
}

// handleMsgMintToken - Handle MsgMintToken
func handleMsgMintToken(ctx sdk.Context, tk TokenKeeper, msg MsgMintToken) (res sdk.Result) {

	return
}