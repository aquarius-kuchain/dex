package stakingx

import (
	"bytes"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// iterate through the validator set and perform the provided function
func (k Keeper) IterateValidators(ctx sdk.Context, fn func(index int64, validator sdk.Validator) (stop bool)) {
	k.sk.IterateValidators(ctx, fn)
}

// iterate through the bonded validator set and perform the provided function
func (k Keeper) IterateBondedValidatorsByPower(ctx sdk.Context, fn func(index int64, validator sdk.Validator) (stop bool)) {
	k.sk.IterateBondedValidatorsByPower(ctx, fn)
}

// iterate through the active validator set and perform the provided function
func (k Keeper) IterateLastValidators(ctx sdk.Context, fn func(index int64, validator sdk.Validator) (stop bool)) {
	k.sk.IterateLastValidators(ctx, fn)
}

// get the sdk.validator for a particular address
func (k Keeper) Validator(ctx sdk.Context, address sdk.ValAddress) sdk.Validator {
	return k.sk.Validator(ctx, address)
}

// get the sdk.validator for a particular pubkey
func (k Keeper) ValidatorByConsAddr(ctx sdk.Context, addr sdk.ConsAddress) sdk.Validator {
	return k.sk.ValidatorByConsAddr(ctx, addr)
}

// total staking tokens supply which is bonded
func (k Keeper) TotalBondedTokens(ctx sdk.Context) sdk.Int {
	return k.sk.TotalBondedTokens(ctx)
}

// total staking tokens supply bonded and unbonded
func (k Keeper) TotalTokens(ctx sdk.Context) sdk.Int {
	return k.sk.TotalTokens(ctx)
}

// get the delegation for a particular set of delegator and validator addresses
func (k Keeper) Delegation(ctx sdk.Context, addrDel sdk.AccAddress, addrVal sdk.ValAddress) sdk.Delegation {
	return k.sk.Delegation(ctx, addrDel, addrVal)
}

// jail a validator
func (k Keeper) Jail(ctx sdk.Context, consAddr sdk.ConsAddress) {
	k.sk.Jail(ctx, consAddr)
}

// unjail a validator
func (k Keeper) Unjail(ctx sdk.Context, consAddr sdk.ConsAddress) {
	k.sk.Unjail(ctx, consAddr)
}

// Slash a validator for an infraction committed at a known height
// Find the contributing stake at that height and put the specified slashFactor
// of it to CommunityPool, updating unbonding delegations & redelegations appropriately
//
// CONTRACT:
//    slashFactor is non-negative
// CONTRACT:
//    Infraction was committed equal to or less than an unbonding period in the past,
//    so all unbonding delegations and redelegations from that height are stored
// CONTRACT:
//    Slash will not slash unbonded validators (for the above reason)
// CONTRACT:
//    Infraction was committed at the current height or at a past height,
//    not at a height in the future
func (k Keeper) Slash(ctx sdk.Context, consAddr sdk.ConsAddress, infractionHeight int64, power int64, slashFactor sdk.Dec) {
	logger := ctx.Logger().With("module", "x/staking")

	if slashFactor.LT(sdk.ZeroDec()) {
		panic(fmt.Errorf("attempted to slash with a negative slash factor: %v", slashFactor))
	}

	// Amount of slashing = slash slashFactor * power at time of infraction
	amount := sdk.TokensFromTendermintPower(power)
	slashAmountDec := amount.ToDec().Mul(slashFactor)
	slashAmount := slashAmountDec.TruncateInt()

	// ref https://github.com/cosmos/cosmos-sdk/issues/1348

	validator, found := k.sk.GetValidatorByConsAddr(ctx, consAddr)
	if !found {
		// If not found, the validator must have been overslashed and removed - so we don't need to do anything
		// NOTE:  Correctness dependent on invariant that unbonding delegations / redelegations must also have been completely
		//        slashed in this case - which we don't explicitly check, but should be true.
		// Log the slash attempt for future reference (maybe we should tag it too)
		logger.Error(fmt.Sprintf(
			"WARNING: Ignored attempt to slash a nonexistent validator with address %s, we recommend you investigate immediately",
			consAddr))
		return
	}

	// should not be slashing an unbonded validator
	if validator.Status == sdk.Unbonded {
		panic(fmt.Sprintf("should not be slashing unbonded validator: %s", validator.GetOperator()))
	}

	operatorAddress := validator.GetOperator()

	// call the before-modification hook
	k.sk.BeforeValidatorModified(ctx, operatorAddress)

	// Track remaining slash amount for the validator
	// This will decrease when we slash unbondings and
	// redelegations, as that stake has since unbonded
	remainingSlashAmount := slashAmount

	switch {
	case infractionHeight > ctx.BlockHeight():

		// Can't slash infractions in the future
		panic(fmt.Sprintf(
			"impossible attempt to slash future infraction at height %d but we are at height %d",
			infractionHeight, ctx.BlockHeight()))

	case infractionHeight == ctx.BlockHeight():

		// Special-case slash at current height for efficiency - we don't need to look through unbonding delegations or redelegations
		logger.Info(fmt.Sprintf(
			"slashing at current height %d, not scanning unbonding delegations & redelegations",
			infractionHeight))

	case infractionHeight < ctx.BlockHeight():

		// Iterate through unbonding delegations from slashed validator
		unbondingDelegations := k.sk.GetUnbondingDelegationsFromValidator(ctx, operatorAddress)
		for _, unbondingDelegation := range unbondingDelegations {
			amountSlashed := k.slashUnbondingDelegation(ctx, unbondingDelegation, infractionHeight, slashFactor)
			if amountSlashed.IsZero() {
				continue
			}
			remainingSlashAmount = remainingSlashAmount.Sub(amountSlashed)
		}

		// Iterate through redelegations from slashed validator
		redelegations := k.sk.GetRedelegationsFromValidator(ctx, operatorAddress)
		for _, redelegation := range redelegations {
			amountSlashed := k.slashRedelegation(ctx, validator, redelegation, infractionHeight, slashFactor)
			if amountSlashed.IsZero() {
				continue
			}
			remainingSlashAmount = remainingSlashAmount.Sub(amountSlashed)
		}
	}

	// cannot decrease balance below zero
	tokensToAddInt := sdk.MinInt(remainingSlashAmount, validator.Tokens)
	tokensToAddInt = sdk.MaxInt(tokensToAddInt, sdk.ZeroInt()) // defensive.

	// we need to calculate the *effective* slash fraction for distribution
	if validator.Tokens.GT(sdk.ZeroInt()) {
		effectiveFraction := tokensToAddInt.ToDec().QuoRoundUp(validator.Tokens.ToDec())
		// possible if power has changed
		if effectiveFraction.GT(sdk.OneDec()) {
			effectiveFraction = sdk.OneDec()
		}
		// call the before-slashed hook
		k.sk.BeforeValidatorSlashed(ctx, operatorAddress, effectiveFraction)
	}

	// Deduct from validator's bonded tokens and update the validator.
	// The deducted tokens are returned to pool.NotBondedTokens.
	// TODO: Move the token accounting outside of `RemoveValidatorTokens` so it is less confusing
	validator = k.sk.RemoveValidatorTokens(ctx, validator, tokensToAddInt)

	//Add tokens to CommunityPool
	feePool := k.dk.GetFeePool(ctx)
	tokensToAdd := sdk.NewDecCoin(k.sk.BondDenom(ctx), tokensToAddInt)
	feePool.CommunityPool = feePool.CommunityPool.Add(sdk.DecCoins{tokensToAdd})
	k.dk.SetFeePool(ctx, feePool)

	// Log that a slash occurred!
	logger.Info(fmt.Sprintf(
		"validator %s slashed by slash factor of %s; burned %v tokens",
		validator.GetOperator(), slashFactor.String(), tokensToAddInt))

	// TODO Return event(s), blocked on https://github.com/tendermint/tendermint/pull/1803

}

// slash an unbonding delegation and update the pool & CommunityPool
// return the amount that would have been slashed assuming
// the unbonding delegation had enough stake to slash
// (the amount actually slashed may be less if there's
// insufficient stake remaining)
func (k Keeper) slashUnbondingDelegation(ctx sdk.Context, unbondingDelegation types.UnbondingDelegation,
	infractionHeight int64, slashFactor sdk.Dec) (totalSlashAmount sdk.Int) {

	now := ctx.BlockHeader().Time
	totalSlashAmount = sdk.ZeroInt()

	// perform slashing on all entries within the unbonding delegation
	for i, entry := range unbondingDelegation.Entries {

		// If unbonding started before this height, stake didn't contribute to infraction
		if entry.CreationHeight < infractionHeight {
			continue
		}

		if entry.IsMature(now) {
			// Unbonding delegation no longer eligible for slashing, skip it
			continue
		}

		// Calculate slash amount proportional to stake contributing to infraction
		slashAmountDec := slashFactor.MulInt(entry.InitialBalance)
		slashAmount := slashAmountDec.TruncateInt()
		totalSlashAmount = totalSlashAmount.Add(slashAmount)

		// Don't slash more tokens than held
		// Possible since the unbonding delegation may already
		// have been slashed, and slash amounts are calculated
		// according to stake held at time of infraction
		unbondingSlashAmount := sdk.MinInt(slashAmount, entry.Balance)

		// Update unbonding delegation if necessary
		if unbondingSlashAmount.IsZero() {
			continue
		}
		entry.Balance = entry.Balance.Sub(unbondingSlashAmount)
		unbondingDelegation.Entries[i] = entry
		k.sk.SetUnbondingDelegation(ctx, unbondingDelegation)

		//Add slash tokens to communityPool
		feePool := k.dk.GetFeePool(ctx)
		tokensToAdd := sdk.NewDecCoin(k.sk.BondDenom(ctx), unbondingSlashAmount)
		feePool.CommunityPool = feePool.CommunityPool.Add(sdk.DecCoins{tokensToAdd})
		k.dk.SetFeePool(ctx, feePool)

	}

	return totalSlashAmount
}

// slash a redelegation and update the pool & CommunityPool
// return the amount that would have been slashed assuming
// the unbonding delegation had enough stake to slash
// (the amount actually slashed may be less if there's
// insufficient stake remaining)
// nolint: unparam
func (k Keeper) slashRedelegation(ctx sdk.Context, validator types.Validator, redelegation types.Redelegation,
	infractionHeight int64, slashFactor sdk.Dec) (totalSlashAmount sdk.Int) {

	now := ctx.BlockHeader().Time
	totalSlashAmount = sdk.ZeroInt()

	// perform slashing on all entries within the redelegation
	for _, entry := range redelegation.Entries {

		// If redelegation started before this height, stake didn't contribute to infraction
		if entry.CreationHeight < infractionHeight {
			continue
		}

		if entry.IsMature(now) {
			// Redelegation no longer eligible for slashing, skip it
			continue
		}

		// Calculate slash amount proportional to stake contributing to infraction
		slashAmountDec := slashFactor.MulInt(entry.InitialBalance)
		slashAmount := slashAmountDec.TruncateInt()
		totalSlashAmount = totalSlashAmount.Add(slashAmount)

		// Unbond from target validator
		sharesToUnbond := slashFactor.Mul(entry.SharesDst)
		if sharesToUnbond.IsZero() {
			continue
		}
		delegation, found := k.sk.GetDelegation(ctx, redelegation.DelegatorAddress, redelegation.ValidatorDstAddress)
		if !found {
			// If deleted, delegation has zero shares, and we can't unbond any more
			continue
		}
		if sharesToUnbond.GT(delegation.Shares) {
			sharesToUnbond = delegation.Shares
		}

		tokensToBurn, err := k.unbond(ctx, redelegation.DelegatorAddress, redelegation.ValidatorDstAddress, sharesToUnbond)
		if err != nil {
			panic(fmt.Errorf("error unbonding delegator: %v", err))
		}

		//Add slash tokens to communityPool
		feePool := k.dk.GetFeePool(ctx)
		tokensToAdd := sdk.NewDecCoin(k.sk.BondDenom(ctx), tokensToBurn)
		feePool.CommunityPool = feePool.CommunityPool.Add(sdk.DecCoins{tokensToAdd})
		k.dk.SetFeePool(ctx, feePool)
	}

	return totalSlashAmount
}

// unbond a particular delegation and perform associated store operations
func (k Keeper) unbond(ctx sdk.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress,
	shares sdk.Dec) (amount sdk.Int, err sdk.Error) {

	// check if a delegation object exists in the store
	delegation, found := k.sk.GetDelegation(ctx, delAddr, valAddr)
	if !found {
		return amount, types.ErrNoDelegatorForAddress(k.sk.Codespace())
	}

	// call the before-delegation-modified hook
	k.sk.BeforeDelegationSharesModified(ctx, delAddr, valAddr)

	// ensure that we have enough shares to remove
	if delegation.Shares.LT(shares) {
		return amount, types.ErrNotEnoughDelegationShares(k.sk.Codespace(), delegation.Shares.String())
	}

	// get validator
	validator, found := k.sk.GetValidator(ctx, valAddr)
	if !found {
		return amount, types.ErrNoValidatorFound(k.sk.Codespace())
	}

	// subtract shares from delegation
	delegation.Shares = delegation.Shares.Sub(shares)

	isValidatorOperator := bytes.Equal(delegation.DelegatorAddress, validator.OperatorAddress)

	// if the delegation is the operator of the validator and undelegating will decrease the validator's self delegation below their minimum
	// trigger a jail validator
	if isValidatorOperator && !validator.Jailed &&
		validator.TokensFromShares(delegation.Shares).TruncateInt().LT(validator.MinSelfDelegation) {

		k.jailValidator(ctx, validator)
		validator = k.mustGetValidator(ctx, validator.OperatorAddress)
	}

	// remove the delegation
	if delegation.Shares.IsZero() {
		k.sk.RemoveDelegation(ctx, delegation)
	} else {
		k.sk.SetDelegation(ctx, delegation)
		// call the after delegation modification hook
		k.sk.AfterDelegationModified(ctx, delegation.DelegatorAddress, delegation.ValidatorAddress)
	}

	// remove the shares and coins from the validator
	validator, amount = k.sk.RemoveValidatorTokensAndShares(ctx, validator, shares)

	if validator.DelegatorShares.IsZero() && validator.Status == sdk.Unbonded {
		// if not unbonded, we must instead remove validator in EndBlocker once it finishes its unbonding period
		k.sk.RemoveValidator(ctx, validator.OperatorAddress)
	}

	return amount, nil
}

// send a validator to jail
func (k Keeper) jailValidator(ctx sdk.Context, validator types.Validator) {
	if validator.Jailed {
		panic(fmt.Sprintf("cannot jail already jailed validator, validator: %v\n", validator))
	}

	validator.Jailed = true
	k.sk.SetValidator(ctx, validator)
	k.sk.DeleteValidatorByPowerIndex(ctx, validator)
}

func (k Keeper) mustGetValidator(ctx sdk.Context, addr sdk.ValAddress) types.Validator {
	validator, found := k.sk.GetValidator(ctx, addr)
	if !found {
		panic(fmt.Sprintf("validator record not found for address: %X\n", addr))
	}
	return validator
}