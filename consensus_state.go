package main

import (
	"sync"
	"math"
)

type ConsensusStateToken struct {
	openOrders sync.Map //map[uint64]Order
	unconfirmedOrderIDs map[uint64]bool //Matched but no transaction confirmed
	balances map[string]uint64
	unconfirmedBuyMatches sync.Map //map[uint64-matchid]BuyMatchInfo
	unconfirmedSellMatches sync.Map //map[uint64-matchid]SellMatchInfo
	unconfirmedRefunds sync.Map //map[uint64-matchid]Refund
}

type ConsensusState struct {
	tokenStates sync.Map //map[string]*ConsensusStateToken
	createdTokens sync.Map //map[string]TokenInfo
}

type MatchInfo struct{
	Info interface{}
	IsSell bool
}

type BuyMatchInfo struct {
	BuyOrderID uint64
	SellerAddress string
	AmountReceived uint64 //in seller currency
}

type SellMatchInfo struct {
	SellOrderID uint64
	BuyerAddress string
	AmountReceived uint64
}

type Refund struct {
	BuyerAddress string
	Refund uint64
}

func (state *ConsensusState) GetTokenState(symbol string) *ConsensusStateToken {
	result, _ := state.tokenStates.Load(symbol)
	return result.(*ConsensusStateToken)
}

func (state *ConsensusState) AddOrder(symbol string, order Order) bool {
	tokenState := state.GetTokenState(symbol)
	if tokenState.balances[order.SellerAddress] < order.AmountToSell {
		return false
	}
	_, loaded := tokenState.openOrders.LoadOrStore(order.ID, order)
	if loaded {
		return false
	}
	tokenState.balances[order.SellerAddress] -= order.AmountToSell
	return true
}

func (state *ConsensusState) RollbackOrder(symbol string, order Order) {
	tokenState := state.GetTokenState(symbol)
	tokenState.openOrders.Delete(order.ID)
	tokenState.balances[order.SellerAddress] += order.AmountToSell
}

//TODO:
//Refunds an order that has already been cancelled on the match chain
//Check if the order has been cancelled on match chain
//Refund order
//If we delete order here can't rollback so send back transaction for recovery
func (state *ConsensusState) AddCancelOrder(symbol string, cancelOrder CancelOrder) (bool, Order) {
	_, ok := state.unconfirmedCancelMatches.Load(cancelOrder.OrderID)
	if !ok {
		return false, Order{}
	}
	tokenState := state.GetTokenState(symbol)
	order, ok := tokenState.openOrders.Load(cancelOrder.OrderID)
	if !ok {
		return false, Order{}
	}
	tokenState.openOrders.Delete(cancelOrder.OrderID)
	state.unconfirmedCancelMatches.Delete(cancelOrder.OrderID)
	return true, order.(Order)
}

func (state *ConsensusState) RollbackCancelOrder(symbol string, cancelOrder CancelOrder, order Order) {
	tokenState := state.GetTokenState(symbol)
	tokenState.openOrders.Store(order.ID, order)
	state.unconfirmedCancelMatches.Store(cancelOrder.OrderID, cancelOrder)
}

func (state *ConsensusState) addTransactionConfirmedBuy(symbol string, transactionConfirmed TransactionConfirmed) (bool, MatchInfo) {
	tokenState := state.GetTokenState(symbol)
	matchGen, ok := tokenState.unconfirmedBuyMatches.Load(transactionConfirmed.MatchID)
	if !ok {
		return false, MatchInfo{}
	}

	match := matchGen.(BuyMatchInfo)
	// credit the seller address with AmountBought
	_, ok = tokenState.unconfirmedOrderIDs[match.BuyOrderID]
	if !ok {
		return false, MatchInfo{}
	}
	delete(tokenState.unconfirmedOrderIDs, match.BuyOrderID)
	tokenState.unconfirmedBuyMatches.Delete(transactionConfirmed.MatchID)
	tokenState.balances[match.SellerAddress] += match.AmountBought

	return true, MatchInfo{match, false}
}

func (state *ConsensusState) AddTransactionConfirmed(symbol string, transactionConfirmed TransactionConfirmed) (bool, MatchInfo) {
	//Assume sell order
	tokenState := state.GetTokenState(symbol)
	matchGen, ok := tokenState.unconfirmedSellMatches.Load(transactionConfirmed.MatchID)
	if !ok {
		return state.addTransactionConfirmedBuy(symbol, transactionConfirmed)
	}

	match := matchGen.(SellMatchInfo)
	// credit the buyer address with AmountSold
	_, ok = tokenState.unconfirmedOrderIDs[match.SellOrderID]
	if !ok {
		return false, MatchInfo{}
	}
	delete(tokenState.unconfirmedOrderIDs, match.SellOrderID)
	tokenState.unconfirmedSellMatches.Delete(transactionConfirmed.MatchID)
	tokenState.balances[match.BuyerAddress] += match.AmountSold

	return true, MatchInfo{match, true}
}

func (state *ConsensusState) RollbackTransactionConfirmed(symbol string, transactionConfirmed TransactionConfirmed, matchInfo MatchInfo) {
	tokenState := state.GetTokenState(symbol)
	if matchInfo.IsSell {
		sellInfo := matchInfo.Info.(SellMatchInfo)
		tokenState.unconfirmedSellMatches.Store(transactionConfirmed.MatchID, true)
		tokenState := state.GetTokenState(symbol)
		tokenState.balances[sellInfo.BuyerAddress] -= sellInfo.AmountSold
		tokenState.unconfirmedOrderIDs[sellInfo.SellOrderID] = true
	} else {
		buyInfo := matchInfo.Info.(BuyMatchInfo)
		tokenState.unconfirmedBuyMatches.Store(transactionConfirmed.MatchID, true)
		tokenState := state.GetTokenState(symbol)
		tokenState.balances[buyInfo.SellerAddress] -= buyInfo.AmountBought
		tokenState.unconfirmedOrderIDs[buyInfo.BuyOrderID] = true
	}
}

func (state *ConsensusState) AddTransfer(symbol string, transfer Transfer) bool {
	tokenState := state.GetTokenState(symbol)
	if tokenState.balances[transfer.FromAddress] < transfer.Amount {
		return false
	}
	tokenState.balances[transfer.FromAddress] -= transfer.Amount
	tokenState.balances[transfer.ToAddress] += transfer.Amount
	return true
}

func (state *ConsensusState) RollbackTransfer(symbol string, transfer Transfer) {
	tokenState := state.GetTokenState(symbol)
	tokenState.balances[transfer.FromAddress] += transfer.Amount
	tokenState.balances[transfer.ToAddress] -= transfer.Amount
}

func (state *ConsensusState) AddMatch(match Match) bool {
	//Check if both buy and sell orders are satisfied by match and that orders are open
	//add to unconfirmedSell and Buy Matches
	//remove orders from openOrders
	buyTokenState := state.GetTokenState(match.BuySymbol)
	sellTokenState := state.GetTokenState(match.SellSymbol)
	buyOrderGen, ok := buyTokenState.openOrders.Load(match.BuyOrderID)
	if !ok {
		return false
	}
	sellOrderGen, ok := sellTokenState.openOrders.Load(match.SellOrderID)
	if !ok {
		return false
	}
	buyOrder := buyOrderGen.(Order)
	sellOrder := sellOrderGen.(Order)

	if sellOrder.AmountToSell < match.AmountSold {
		return false
	}

	//price = amountBuyCurrency / amountSellCurrency
	buyPrice := float64(buyOrder.AmountToBuy) / float64(buyOrder.AmountToSell)
	buyerMaxAmountPaid := uint64(math.Ceil(buyPrice * float64(match.AmountSold)))
	sellPrice := float64(sellOrder.AmountToSell) / float64(sellOrder.AmountToBuy)
	sellerMinAmountPaid := uint64(math.Floor(sellPrice * float64(match.AmountSold)))
	if buyerMaxAmountPaid < sellerMinAmountPaid {
		return false
	}

	//Trade is between match.AmountSold and buyerMaxAmountPaid
	//TODO: minerReward := buyerMaxAmountPaid - sellerMinAmountPaid
	buyMatch := BuyMatchInfo {
		BuyOrderID:    match.BuyOrderID,
		SellerAddress: sellOrder.SellerAddress,
		AmountReceived:  match.AmountSold,
	}
	sellMatch := SellMatchInfo{
		SellOrderID:  match.SellOrderID,
		BuyerAddress: buyOrder.SellerAddress,
		AmountReceived:   match.AmountSold,
	}
	buyTokenState.unconfirmedBuyMatches.Store(match.MatchID, buyMatch)
	sellTokenState.unconfirmedSellMatches.Store(match.MatchID, sellMatch)

	sellOrder.AmountToSell -= match.AmountSold
	sellOrder.AmountToBuy -= sellerMaxAmountBought
	buyOrder.AmountToBuy -= match.AmountSold
	buyOrder.AmountToSell -= buyerMinAmountBought
	return false
}

func (state *ConsensusState) RollbackMatch(match Match) {

}

func (state *ConsensusState) AddCancelMatch(cancelMatch CancelMatch) bool {
	return false
}

func (state *ConsensusState) RollbackCancelMatch(cancelMatch CancelMatch) {

}

func (state *ConsensusState) AddCreateToken(createToken CreateToken) bool {
	return false
}

func (state *ConsensusState) RollbackCreateToken(createToken CreateToken) {

}
