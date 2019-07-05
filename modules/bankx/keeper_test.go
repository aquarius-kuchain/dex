package bankx

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/params"

	"github.com/coinexchain/dex/modules/authx"
	"github.com/coinexchain/dex/modules/msgqueue"
	"github.com/coinexchain/dex/testutil"
	"github.com/coinexchain/dex/types"
)

type fakeAssetStatusKeeper struct{}

func (k fakeAssetStatusKeeper) IsTokenForbidden(ctx sdk.Context, symbol string) bool {
	return false
}
func (k fakeAssetStatusKeeper) IsForbiddenByTokenIssuer(ctx sdk.Context, symbol string, addr sdk.AccAddress) bool {
	return false
}

var myaddr = testutil.ToAccAddress("myaddr")

func defaultContext() (sdk.Context, params.Keeper) {
	cdc := codec.New()
	skey := sdk.NewKVStoreKey("test")
	tkey := sdk.NewTransientStoreKey("transient_test")

	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(skey, sdk.StoreTypeIAVL, db)
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)

	_ = cms.LoadLatestVersion()
	ctx := sdk.NewContext(cms, abci.Header{}, false, log.NewNopLogger())
	paramsKeeper := params.NewKeeper(cdc, skey, tkey)

	return ctx, paramsKeeper
}

func TestParamGetSet(t *testing.T) {
	ctx, paramsKeeper := defaultContext()
	subspace := paramsKeeper.Subspace(DefaultParamspace)
	bkxKepper := NewKeeper(subspace, authx.AccountXKeeper{}, bank.BaseKeeper{}, auth.AccountKeeper{}, auth.FeeCollectionKeeper{}, fakeAssetStatusKeeper{}, msgqueue.NewProducer())

	//expect DefaultActivationFees=1
	defaultParam := DefaultParams()
	require.Equal(t, int64(100000000), defaultParam.ActivationFee)

	//expect SetParam don't panic
	require.NotPanics(t, func() { bkxKepper.SetParam(ctx, defaultParam) }, "bankxKeeper SetParam panics")

	//expect GetParam equals defaultParam
	require.Equal(t, defaultParam, bkxKepper.GetParam(ctx))
}

func givenAccountWith(input testInput, addr sdk.AccAddress, coinsString string) {
	coins, _ := sdk.ParseCoins(coinsString)

	acc := auth.NewBaseAccountWithAddress(addr)
	_ = acc.SetCoins(coins)
	input.ak.SetAccount(input.ctx, &acc)

	accX := authx.AccountX{
		Address: addr,
	}
	input.axk.SetAccountX(input.ctx, accX)
}

func coinsOf(input testInput, addr sdk.AccAddress) string {
	return input.ak.GetAccount(input.ctx, addr).GetCoins().String()
}

func frozenCoinsOf(input testInput, addr sdk.AccAddress) string {
	accX, _ := input.axk.GetAccountX(input.ctx, addr)
	return accX.FrozenCoins.String()
}

func TestFreezeMultiCoins(t *testing.T) {
	input := setupTestInput()

	givenAccountWith(input, myaddr, "1000000000cet,100abc")

	freezeCoins, _ := sdk.ParseCoins("300000000cet, 20abc")
	err := input.bxk.FreezeCoins(input.ctx, myaddr, freezeCoins)

	require.Nil(t, err)
	require.Equal(t, "80abc,700000000cet", coinsOf(input, myaddr))
	require.Equal(t, "20abc,300000000cet", frozenCoinsOf(input, myaddr))

	err = input.bxk.UnFreezeCoins(input.ctx, myaddr, freezeCoins)

	require.Nil(t, err)
	require.Equal(t, "100abc,1000000000cet", coinsOf(input, myaddr))
	require.Equal(t, "", frozenCoinsOf(input, myaddr))
}

func TestFreezeUnFreezeOK(t *testing.T) {
	input := setupTestInput()

	givenAccountWith(input, myaddr, "1000000000cet")

	freezeCoins := types.NewCetCoins(300000000)
	err := input.bxk.FreezeCoins(input.ctx, myaddr, freezeCoins)

	require.Nil(t, err)
	require.Equal(t, "700000000cet", coinsOf(input, myaddr))
	require.Equal(t, "300000000cet", frozenCoinsOf(input, myaddr))

	err = input.bxk.UnFreezeCoins(input.ctx, myaddr, freezeCoins)

	require.Nil(t, err)
	require.Equal(t, "1000000000cet", coinsOf(input, myaddr))
	require.Equal(t, "", frozenCoinsOf(input, myaddr))
}

func TestFreezeUnFreezeInvalidAccount(t *testing.T) {
	input := setupTestInput()

	freezeCoins := types.NewCetCoins(500000000)
	err := input.bxk.FreezeCoins(input.ctx, myaddr, freezeCoins)
	require.Equal(t, sdk.ErrInsufficientCoins("insufficient account funds;  < 500000000cet"), err)

	err = input.bxk.UnFreezeCoins(input.ctx, myaddr, freezeCoins)
	require.Equal(t, sdk.ErrUnknownAddress(fmt.Sprintf("account %s does not exist", myaddr)), err)
}

func TestFreezeUnFreezeInsufficientCoins(t *testing.T) {
	input := setupTestInput()

	givenAccountWith(input, myaddr, "10cet")

	InvalidFreezeCoins := types.NewCetCoins(50)
	err := input.bxk.FreezeCoins(input.ctx, myaddr, InvalidFreezeCoins)
	require.Equal(t, sdk.ErrInsufficientCoins("insufficient account funds; 10cet < 50cet"), err)

	freezeCoins := types.NewCetCoins(5)
	err = input.bxk.FreezeCoins(input.ctx, myaddr, freezeCoins)
	require.Nil(t, err)

	err = input.bxk.UnFreezeCoins(input.ctx, myaddr, InvalidFreezeCoins)
	require.Equal(t, sdk.ErrInsufficientCoins("account has insufficient coins to unfreeze"), err)
}

func TestGetTotalCoins(t *testing.T) {
	input := setupTestInput()
	givenAccountWith(input, myaddr, "100cet, 20bch, 30btc")

	lockedCoins := authx.LockedCoins{
		authx.NewLockedCoin("bch", sdk.NewInt(20), 1000),
		authx.NewLockedCoin("eth", sdk.NewInt(30), 2000),
	}

	frozenCoins := sdk.NewCoins(sdk.Coin{Denom: "btc", Amount: sdk.NewInt(50)},
		sdk.Coin{Denom: "eth", Amount: sdk.NewInt(10)},
	)

	accX := authx.AccountX{
		Address:     myaddr,
		LockedCoins: lockedCoins,
		FrozenCoins: frozenCoins,
	}

	input.axk.SetAccountX(input.ctx, accX)

	expected := sdk.NewCoins(
		sdk.Coin{Denom: "bch", Amount: sdk.NewInt(40)},
		sdk.Coin{Denom: "btc", Amount: sdk.NewInt(80)},
		sdk.Coin{Denom: "cet", Amount: sdk.NewInt(100)},
		sdk.Coin{Denom: "eth", Amount: sdk.NewInt(40)},
	)
	expected = expected.Sort()
	coins := input.bxk.GetTotalCoins(input.ctx, myaddr)

	require.Equal(t, expected, coins)
}

func TestKeeper_TotalAmountOfCoin(t *testing.T) {
	input := setupTestInput()
	amount := input.bxk.TotalAmountOfCoin(input.ctx, "cet")
	require.Equal(t, int64(0), amount.Int64())

	givenAccountWith(input, myaddr, "100cet")

	lockedCoins := authx.LockedCoins{
		authx.NewLockedCoin("cet", sdk.NewInt(100), 1000),
	}
	frozenCoins := sdk.NewCoins(sdk.Coin{Denom: "cet", Amount: sdk.NewInt(100)})

	accX := authx.AccountX{
		Address:     myaddr,
		LockedCoins: lockedCoins,
		FrozenCoins: frozenCoins,
	}
	input.axk.SetAccountX(input.ctx, accX)
	amount = input.bxk.TotalAmountOfCoin(input.ctx, "cet")
	require.Equal(t, int64(300), amount.Int64())
}

func TestKeeper_AddCoins(t *testing.T) {
	input := setupTestInput()
	coins := sdk.NewCoins(
		sdk.Coin{Denom: "aaa", Amount: sdk.NewInt(10)},
		sdk.Coin{Denom: "bbb", Amount: sdk.NewInt(20)},
	)

	coins2 := sdk.NewCoins(
		sdk.Coin{Denom: "aaa", Amount: sdk.NewInt(5)},
		sdk.Coin{Denom: "bbb", Amount: sdk.NewInt(10)},
	)

	err := input.bxk.AddCoins(input.ctx, myaddr, coins)
	require.Equal(t, nil, err)
	err = input.bxk.SubtractCoins(input.ctx, myaddr, coins2)
	require.Equal(t, nil, err)
	cs := input.bxk.GetTotalCoins(input.ctx, myaddr)
	require.Equal(t, coins2, cs)

	coins3 := sdk.NewCoins(
		sdk.Coin{Denom: "aaa", Amount: sdk.NewInt(15)},
		sdk.Coin{Denom: "bbb", Amount: sdk.NewInt(10)},
	)
	err = input.bxk.SubtractCoins(input.ctx, myaddr, coins3)
	require.Error(t, err)
}

func TestKeeper_SendCoins(t *testing.T) {
	input := setupTestInput()
	coins := sdk.NewCoins(
		sdk.Coin{Denom: "aaa", Amount: sdk.NewInt(10)},
	)
	addr2 := testutil.ToAccAddress("addr2")
	_ = input.bxk.AddCoins(input.ctx, myaddr, coins)
	exist := input.bxk.HasCoins(input.ctx, myaddr, coins)
	assert.True(t, exist)
	err := input.bxk.SendCoins(input.ctx, myaddr, addr2, coins)
	require.Equal(t, nil, err)
	cs := input.bxk.GetTotalCoins(input.ctx, addr2)
	require.Equal(t, coins, cs)
}
