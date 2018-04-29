package main

import "sync"

type ConsensusStateToken struct {
	openOrders sync.Map //map[uint64]Order
	unconfirmedOrderIDs map[uint64]bool //Matched but no transaction confirmed
	balances map[string]uint64
}

type ConsensusState struct {
	tokenStates sync.Map //map[string]*ConsensusStateToken
	unconfirmedBuyMatches sync.Map //map[uint64]BuyMatchInfo
	unconfirmedSellMatches sync.Map //map[uint64]SellMatchInfo
	createdTokens sync.Map //map[string]TokenInfo
	unconfirmedCancelMatches sync.Map //map[uint64]bool
}

type MatchInfo struct{
	Info interface{}
	IsSell bool
}

type BuyMatchInfo struct {
	BuySymbol string
	BuyOrderID uint64
	SellerAddress string
	AmountBought uint64
}

type SellMatchInfo struct {
	SellSymbol string
	SellOrderID uint64
	BuyerAddress string
	AmountSold uint64
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
	matchGen, ok := state.unconfirmedBuyMatches.Load(transactionConfirmed.MatchID)
	if !ok {
		return false, MatchInfo{}
	}

	match := matchGen.(BuyMatchInfo)
	tokenState := state.GetTokenState(symbol)
	if symbol == match.BuySymbol {
		// credit the seller address with AmountBought
		_, ok := tokenState.unconfirmedOrderIDs[match.BuyOrderID]
		if !ok {
			return false, MatchInfo{}
		}
		delete(tokenState.unconfirmedOrderIDs, match.BuyOrderID)
		state.unconfirmedBuyMatches.Delete(transactionConfirmed.MatchID)
		tokenState.balances[match.SellerAddress] += match.AmountBought
	} else {
		return false, MatchInfo{}
	}

	return true, MatchInfo{match, false}
}

func (state *ConsensusState) AddTransactionConfirmed(symbol string, transactionConfirmed TransactionConfirmed) (bool, MatchInfo) {
	//Assume sell order
	matchGen, ok := state.unconfirmedSellMatches.Load(transactionConfirmed.MatchID)
	if !ok {
		return state.addTransactionConfirmedBuy(symbol, transactionConfirmed)
	}

	match := matchGen.(SellMatchInfo)
	tokenState := state.GetTokenState(symbol)
	if symbol == match.SellSymbol {
		// credit the buyer address with AmountSold
		_, ok := tokenState.unconfirmedOrderIDs[match.SellOrderID]
		if !ok {
			return false, MatchInfo{}
		}
		delete(tokenState.unconfirmedOrderIDs, match.SellOrderID)
		state.unconfirmedSellMatches.Delete(transactionConfirmed.MatchID)
		tokenState.balances[match.BuyerAddress] += match.AmountSold
	} else {
		return state.addTransactionConfirmedBuy(symbol, transactionConfirmed)
	}

	return true, MatchInfo{match, true}
}

func (state *ConsensusState) RollbackTransactionConfirmed(symbol string, transactionConfirmed TransactionConfirmed, matchInfo MatchInfo) {
	if matchInfo.IsSell {
		sellInfo := matchInfo.Info.(SellMatchInfo)
		state.unconfirmedSellMatches.Store(transactionConfirmed.MatchID, true)
		tokenState := state.GetTokenState(symbol)
		tokenState.balances[sellInfo.BuyerAddress] -= sellInfo.AmountSold
		tokenState.unconfirmedOrderIDs[sellInfo.SellOrderID] = true
	} else {
		buyInfo := matchInfo.Info.(BuyMatchInfo)
		state.unconfirmedBuyMatches.Store(transactionConfirmed.MatchID, true)
		tokenState := state.GetTokenState(symbol)
		tokenState.balances[buyInfo.SellerAddress] -= buyInfo.AmountBought
		tokenState.unconfirmedOrderIDs[buyInfo.BuyOrderID] = true
	}
}

func (state *ConsensusState) AddTransfer(symbol string, transfer Transfer) bool {
	return false
}

func (state *ConsensusState) RollbackTransfer(symbol string, transfer Transfer) {

}

func (state *ConsensusState) AddMatch(match Match) bool {
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
