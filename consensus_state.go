package main

import (
	"sync"
	"math"
)

type ConsensusStateToken struct {
	openOrders sync.Map //map[uint64]Order
	unconfirmedOrderIDs map[uint64]bool //Matched but no transaction confirmed
	balances map[string]uint64
	unclaimedFunds map[string]uint64
	unclaimedFundsLock sync.Mutex
}

type ConsensusState struct {
	tokenStates sync.Map //map[string]*ConsensusStateToken
	createdTokens sync.Map //map[string]TokenInfo
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

func (state *ConsensusState) AddClaimFunds(symbol string, funds ClaimFunds) bool {
	return false
}

func (state *ConsensusState) RollbackClaimFunds(symbol string, funds ClaimFunds) {

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

	//sell currency: AmountFromSellOrder = match.AmountSold; AmountToBuyer = match.AmountSold;
	//buy currency: AmountFromBuyOrder = buyerMaxAmountPaid; AmountToSeller = sellerMinAmountReceived;

	//price = amountBuyCurrency / amountSellCurrency
	buyPrice := float64(buyOrder.AmountToBuy) / float64(buyOrder.AmountToSell)
	buyerMaxAmountPaid := uint64(math.Ceil(buyPrice * float64(match.AmountSold)))
	sellPrice := float64(sellOrder.AmountToSell) / float64(sellOrder.AmountToBuy)
	sellerMinAmountReceived := uint64(math.Floor(sellPrice * float64(match.AmountSold)))
	if buyerMaxAmountPaid < sellerMinAmountReceived || buyerMaxAmountPaid > buyOrder.AmountToSell {
		return false
	}

	//Trade is between match.AmountSold and buyerMaxAmountPaid
	//TODO: minerReward := buyerMaxAmountPaid - sellerMinAmountPaid
	buyTokenState.unclaimedFundsLock.Lock()
	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] += sellerMinAmountReceived
	buyTokenState.unclaimedFundsLock.Unlock()

	sellTokenState.unclaimedFundsLock.Lock()
	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] += match.AmountSold
	sellTokenState.unclaimedFundsLock.Unlock()


	sellOrder.AmountToSell -= match.AmountSold
	sellOrder.AmountToBuy -= sellerMinAmountReceived
	buyOrder.AmountToBuy -= match.AmountSold
	buyOrder.AmountToSell -= buyerMaxAmountPaid

	if sellOrder.AmountToBuy <= 0 {
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFundsLock.Lock()
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] += sellOrder.AmountToSell
			sellTokenState.unclaimedFundsLock.Unlock()
		}
		sellTokenState.openOrders.Delete(sellOrder.ID)
	} else {
		sellTokenState.openOrders.Store(sellOrder.ID, sellOrder)
	}

	if buyOrder.AmountToBuy <= 0 {
		if buyOrder.AmountToSell > 0 {
			buyTokenState.unclaimedFundsLock.Lock()
			buyTokenState.unclaimedFunds[buyOrder.SellerAddress] += buyOrder.AmountToSell
			buyTokenState.unclaimedFundsLock.Unlock()
		}
		buyTokenState.openOrders.Delete(buyOrder.ID)
	} else {
		buyTokenState.openOrders.Store(buyOrder.ID, buyOrder)
	}
	return true
}

func (state *ConsensusState) RollbackMatch(match Match) {

}

func (state *ConsensusState) AddCancelOrder(cancelOrder CancelOrder) bool {
	return false
}

func (state *ConsensusState) RollbackCancelOrder(cancelOrder CancelOrder) {

}

func (state *ConsensusState) AddCreateToken(createToken CreateToken) bool {
	return false
}

func (state *ConsensusState) RollbackCreateToken(createToken CreateToken) {

}
