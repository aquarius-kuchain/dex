package keepers

import (
	"bytes"
	"fmt"
	"testing"

	sdkstore "github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	abci "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/coinexchain/dex/modules/market/internal/types"
)

// TODO
type storeKeys struct {
	assetCapKey *sdk.KVStoreKey
	authCapKey  *sdk.KVStoreKey
	authxCapKey *sdk.KVStoreKey
	fckCapKey   *sdk.KVStoreKey
	keyParams   *sdk.KVStoreKey
	tkeyParams  *sdk.TransientStoreKey
	marketKey   *sdk.KVStoreKey
	authxKey    *sdk.KVStoreKey
	keyStaking  *sdk.KVStoreKey
	tkeyStaking *sdk.TransientStoreKey
}

func bytes2str(slice []byte) string {
	s := ""
	for _, v := range slice {
		s = s + fmt.Sprintf("%d ", v)
	}
	return s
}

func Test_concatCopyPreAllocate(t *testing.T) {
	res := concatCopyPreAllocate([][]byte{
		{0, 1, 2, 3},
		{4, 5},
		{},
		{6, 7},
	})
	ref := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	if !bytes.Equal(res, ref) {
		t.Errorf("mismatch in concatCopyPreAllocate")
	}
}

func newContextAndMarketKey(chainid string) (sdk.Context, storeKeys) {
	db := dbm.NewMemDB()
	ms := sdkstore.NewCommitMultiStore(db)

	keys := storeKeys{}
	keys.marketKey = sdk.NewKVStoreKey(types.StoreKey)
	keys.keyParams = sdk.NewKVStoreKey(params.StoreKey)
	keys.tkeyParams = sdk.NewTransientStoreKey(params.TStoreKey)
	ms.MountStoreWithDB(keys.keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keys.tkeyParams, sdk.StoreTypeTransient, db)
	ms.MountStoreWithDB(keys.marketKey, sdk.StoreTypeIAVL, db)
	ms.LoadLatestVersion()

	ctx := sdk.NewContext(ms, abci.Header{ChainID: chainid, Height: 1000}, false, log.NewNopLogger())
	return ctx, keys
}

func TestOrderCleanUpDayKeeper(t *testing.T) {
	ctx, keys := newContextAndMarketKey(types.TestNetSubString)
	k := NewOrderCleanUpDayKeeper(keys.marketKey)
	k.SetUnixTime(ctx, 19673122)
	if k.GetUnixTime(ctx) != 19673122 {
		t.Errorf("Error for OrderCleanUpDayKeeper")
	}

	k.SetUnixTime(ctx, -173122)
	if k.GetUnixTime(ctx) != -173122 {
		t.Errorf("Error for OrderCleanUpDayKeeper")
	}

}

func newKeeperForTest(key sdk.StoreKey) OrderKeeper {
	return NewOrderKeeper(key, "cet/usdt", types.ModuleCdc)
}

func newGlobalKeeperForTest(key sdk.StoreKey) GlobalOrderKeeper {
	return NewGlobalOrderKeeper(key, types.ModuleCdc)
}

func simpleAddr(s string) (sdk.AccAddress, error) {
	return sdk.AccAddressFromHex("01234567890123456789012345678901234" + s)
}

func NewTO(sender string, seq uint64, price int64, qua int64, side byte, tif int, h int64) *types.Order {
	addr, _ := simpleAddr(sender)
	decPrice := sdk.NewDec(price).QuoInt(sdk.NewInt(10000))
	freeze := qua
	if side == types.BUY {
		freeze = decPrice.Mul(sdk.NewDec(qua)).RoundInt64()
	}
	return &types.Order{
		Sender:      addr,
		Sequence:    seq,
		TradingPair: "cet/usdt",
		OrderType:   types.LIMIT,
		Price:       decPrice,
		Quantity:    qua,
		Side:        side,
		TimeInForce: tif,
		Height:      h,
		Freeze:      freeze,
		LeftStock:   qua,
	}
}

func sameTO(a, order *types.Order) bool {
	res := bytes.Equal(order.Sender, order.Sender) && order.Sequence == order.Sequence &&
		order.TradingPair == order.TradingPair && order.OrderType == order.OrderType && a.Price.Equal(order.Price) &&
		order.Quantity == order.Quantity && order.Side == order.Side && order.TimeInForce == order.TimeInForce &&
		order.Height == order.Height
	//if !res {
	//	fmt.Printf("seq: %d %d\n", a.Sequence, b.Sequence)
	//	fmt.Printf("symbol: %s %s\n", a.Symbol, b.Symbol)
	//	fmt.Printf("ordertype: %d %d\n", a.OrderType, b.OrderType)
	//	fmt.Printf("price: %s %s\n", a.Price, b.Price)
	//	fmt.Printf("quantity: %d %d\n", a.Quantity, b.Quantity)
	//	fmt.Printf("side: %d %d\n", a.Side, b.Side)
	//	fmt.Printf("tif: %d %d\n", a.TimeInForce, b.TimeInForce)
	//	fmt.Printf("height: %d %d\n", a.Height, b.Height)
	//}
	return res
}

func createTO1() []*types.Order {
	return []*types.Order{
		//sender seq   price quantity       height
		NewTO("00001", 1, 11051, 50, types.BUY, types.GTE, 998),   //0
		NewTO("00002", 2, 11080, 50, types.BUY, types.GTE, 998),   //1 good
		NewTO("00002", 3, 10900, 50, types.BUY, types.GTE, 992),   //2
		NewTO("00003", 2, 11010, 100, types.SELL, types.IOC, 997), //3 good
		NewTO("00004", 4, 11032, 60, types.SELL, types.GTE, 990),  //4
		NewTO("00005", 5, 12039, 120, types.SELL, types.GTE, 996), //5
	}
}

func createTO3() []*types.Order {
	return []*types.Order{
		//sender seq   price quantity       height
		NewTO("00001", 1, 11051, 50, types.BUY, types.GTE, 998),   //0
		NewTO("00002", 2, 11080, 50, types.BUY, types.GTE, 998),   //1
		NewTO("00002", 3, 10900, 50, types.BUY, types.GTE, 992),   //2
		NewTO("00003", 2, 12010, 100, types.SELL, types.IOC, 997), //3
		NewTO("00004", 4, 12032, 60, types.SELL, types.GTE, 990),  //4
		NewTO("00005", 5, 12039, 120, types.SELL, types.GTE, 996), //5
	}
}

func TestOrderBook1(t *testing.T) {
	orders := createTO1()
	ctx, keys := newContextAndMarketKey(types.TestNetSubString)
	keeper := newKeeperForTest(keys.marketKey)
	if keeper.GetSymbol() != "cet/usdt" {
		t.Errorf("Error in GetSymbol")
	}
	gkeeper := newGlobalKeeperForTest(keys.marketKey)
	for _, order := range orders {
		keeper.Add(ctx, order)
		fmt.Printf("AA: %s %d\n", order.OrderID(), order.Height)
	}
	orderseq := []int{5, 0, 3, 4, 1, 2}
	for i, order := range gkeeper.GetAllOrders(ctx) {
		j := orderseq[i]
		if !sameTO(orders[j], order) {
			t.Errorf("Error in GetAllOrders")
		}
		//fmt.Printf("BB: %s %d\n", order.OrderID(), order.Height)
	}
	newOrder := NewTO("00005", 6, 11030, 20, types.SELL, types.GTE, 993)
	if keeper.Remove(ctx, newOrder) == nil {
		t.Errorf("Error in Remove")
	}
	orders1 := keeper.GetOlderThan(ctx, 997)
	//for _, order := range orders1 {
	//	fmt.Printf("11: %s %d\n", order.OrderID(), order.Height)
	//}
	if !(sameTO(orders1[0], orders[5]) && sameTO(orders1[1], orders[2]) && sameTO(orders1[2], orders[4])) {
		t.Errorf("Error in GetOlderThan")
	}
	orders2 := keeper.GetOrdersAtHeight(ctx, 998)
	//for _, order := range orders2 {
	//	fmt.Printf("22: %s %d\n", order.OrderID(), order.Height)
	//}
	if !(sameTO(orders2[0], orders[0]) && sameTO(orders2[1], orders[1])) {
		t.Errorf("Error in GetOrdersAtHeight")
	}
	addr, _ := simpleAddr("00002")
	orderList := gkeeper.GetOrdersFromUser(ctx, addr.String())
	refOrderList := []string{addr.String() + "-3" + "-0", addr.String() + "-2" + "-0"}
	if orderList[0] != refOrderList[1] || orderList[1] != refOrderList[0] {
		t.Errorf("Error in GetOrdersFromUser")
	}
	orderseq = []int{1, 3, 4, 0}
	for _, order := range keeper.GetMatchingCandidates(ctx) {
		//j := orderseq[i]
		if order.OrderID() != order.OrderID() {
			t.Errorf("Error in GetMatchingCandidates")
			//fmt.Printf("orderID %s %s\n", order.OrderID(), order.Price.String())
		}
	}
	for _, order := range orders {
		if gkeeper.QueryOrder(ctx, order.OrderID()) == nil {
			t.Errorf("Can not find added orders!")
			continue
		}
		qorder := gkeeper.QueryOrder(ctx, order.OrderID())
		if !sameTO(order, qorder) {
			t.Errorf("Order's content is changed!")
		}
	}
}

func TestOrderBook2a(t *testing.T) {
	orders := createTO1()
	ctx, keys := newContextAndMarketKey(types.TestNetSubString)
	keeper := newKeeperForTest(keys.marketKey)
	for _, order := range orders {
		if order.Side == types.BUY {
			keeper.Add(ctx, order)
		}
	}
	if len(keeper.GetMatchingCandidates(ctx)) != 0 {
		t.Errorf("Matching result must be nil!")
	}
}

func TestOrderBook2b(t *testing.T) {
	orders := createTO1()
	ctx, keys := newContextAndMarketKey(types.TestNetSubString)
	keeper := newKeeperForTest(keys.marketKey)
	for _, order := range orders {
		if order.Side == types.SELL {
			keeper.Add(ctx, order)
		}
	}
	if len(keeper.GetMatchingCandidates(ctx)) != 0 {
		t.Errorf("Matching result must be nil!")
	}
}

func TestOrderBook3(t *testing.T) {
	orders := createTO3()
	ctx, keys := newContextAndMarketKey(types.TestNetSubString)
	keeper := newKeeperForTest(keys.marketKey)
	for _, order := range orders {
		keeper.Add(ctx, order)
	}
	if len(keeper.GetMatchingCandidates(ctx)) != 0 {
		t.Errorf("Matching result must be nil!")
	}
}
