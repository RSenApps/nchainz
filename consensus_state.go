package main

import (
	"sync"
	"math"
)

type ConsensusStateToken struct {
	openOrders sync.Map //map[uint64]Order
	balances map[string]uint64
	unclaimedFunds map[string]uint64
	unclaimedFundsLock sync.Mutex
	deletedOrders map[uint64]Order
	deletedOrdersLock sync.Mutex
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

	tokenState.deletedOrdersLock.Lock()
	_, loaded := tokenState.deletedOrders[order.ID]
	tokenState.deletedOrdersLock.Unlock()
	if loaded {
		return false
	}

	_, loaded = tokenState.openOrders.LoadOrStore(order.ID, order)
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
	tokenState := state.GetTokenState(symbol)
	tokenState.unclaimedFundsLock.Lock()
	defer tokenState.unclaimedFundsLock.Unlock()
	if tokenState.unclaimedFunds[funds.Address] < funds.Amount {
		return false
	}
	tokenState.unclaimedFunds[funds.Address] -= funds.Amount
	tokenState.balances[funds.Address] += funds.Amount
	return true
}

func (state *ConsensusState) RollbackClaimFunds(symbol string, funds ClaimFunds) {
	tokenState := state.GetTokenState(symbol)
	tokenState.unclaimedFundsLock.Lock()
	defer tokenState.unclaimedFundsLock.Unlock()
	tokenState.unclaimedFunds[funds.Address] += funds.Amount
	tokenState.balances[funds.Address] -= funds.Amount
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

	var deletedOrders []Order
	if sellOrder.AmountToBuy <= 0 {
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFundsLock.Lock()
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] += sellOrder.AmountToSell
			sellTokenState.unclaimedFundsLock.Unlock()
		}
		deletedOrders = append(deletedOrders, sellOrder)
		sellTokenState.deletedOrdersLock.Lock()
		sellTokenState.deletedOrders[sellOrder.ID] = sellOrder
		sellTokenState.deletedOrdersLock.Unlock()
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
		deletedOrders = append(deletedOrders, buyOrder)
		buyTokenState.deletedOrdersLock.Lock()
		buyTokenState.deletedOrders[buyOrder.ID] = buyOrder
		buyTokenState.deletedOrdersLock.Unlock()
		buyTokenState.openOrders.Delete(buyOrder.ID)
	} else {
		buyTokenState.openOrders.Store(buyOrder.ID, buyOrder)
	}
	return true
}

func (state *ConsensusState) RollbackMatch(match Match) {
	buyTokenState := state.GetTokenState(match.BuySymbol)
	sellTokenState := state.GetTokenState(match.SellSymbol)

	// were orders deleted?
	buyTokenState.deletedOrdersLock.Lock()
	buyOrder, ok := buyTokenState.deletedOrders[match.BuyOrderID]
	if ok {
		if buyOrder.AmountToSell > 0 {
			buyTokenState.unclaimedFundsLock.Lock()
			buyTokenState.unclaimedFunds[buyOrder.SellerAddress] -= buyOrder.AmountToSell
			buyTokenState.unclaimedFundsLock.Unlock()
		}
		delete(buyTokenState.deletedOrders, match.BuyOrderID)
	} else {
		buyOrderGen, _ := buyTokenState.openOrders.Load(match.BuyOrderID)
		buyOrder = buyOrderGen.(Order)
	}
	buyTokenState.deletedOrdersLock.Unlock()

	sellTokenState.deletedOrdersLock.Lock()
	sellOrder, ok := sellTokenState.deletedOrders[match.SellOrderID]
	if ok {
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFundsLock.Lock()
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] -= sellOrder.AmountToSell
			sellTokenState.unclaimedFundsLock.Unlock()
		}
		delete(sellTokenState.deletedOrders, match.SellOrderID)
	} else {
		sellOrderGen, _ := sellTokenState.openOrders.Load(match.SellOrderID)
		sellOrder = sellOrderGen.(Order)
	}
	sellTokenState.deletedOrdersLock.Unlock()

	buyPrice := float64(buyOrder.AmountToBuy) / float64(buyOrder.AmountToSell)
	buyerMaxAmountPaid := uint64(math.Ceil(buyPrice * float64(match.AmountSold)))
	sellPrice := float64(sellOrder.AmountToSell) / float64(sellOrder.AmountToBuy)
	sellerMinAmountReceived := uint64(math.Floor(sellPrice * float64(match.AmountSold)))

	//TODO: minerReward := buyerMaxAmountPaid - sellerMinAmountPaid
	buyTokenState.unclaimedFundsLock.Lock()
	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] -= sellerMinAmountReceived
	buyTokenState.unclaimedFundsLock.Unlock()

	sellTokenState.unclaimedFundsLock.Lock()
	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] -= match.AmountSold
	sellTokenState.unclaimedFundsLock.Unlock()

	sellOrder.AmountToSell += match.AmountSold
	sellOrder.AmountToBuy += sellerMinAmountReceived
	buyOrder.AmountToBuy += match.AmountSold
	buyOrder.AmountToSell += buyerMaxAmountPaid

	sellTokenState.openOrders.Store(match.SellOrderID, sellOrder)
	buyTokenState.openOrders.Store(match.BuyOrderID, buyOrder)
}

func (state *ConsensusState) AddCancelOrder(cancelOrder CancelOrder) bool {
	tokenState := state.GetTokenState(cancelOrder.OrderSymbol)
	orderGen, ok := tokenState.openOrders.Load(cancelOrder.OrderID)
	if !ok {
		return false
	}
	order := orderGen.(Order)
	tokenState.unclaimedFundsLock.Lock()
	tokenState.unclaimedFunds[order.SellerAddress] += order.AmountToSell
	tokenState.unclaimedFundsLock.Unlock()
	tokenState.openOrders.Delete(cancelOrder.OrderID)
	return true

}

func (state *ConsensusState) RollbackCancelOrder(cancelOrder CancelOrder) {
	tokenState := state.GetTokenState(cancelOrder.OrderSymbol)

	tokenState.deletedOrdersLock.Lock()
	deletedOrder, _ := tokenState.deletedOrders[cancelOrder.OrderID]
	delete(tokenState.deletedOrders, cancelOrder.OrderID)
	tokenState.deletedOrdersLock.Unlock()

	tokenState.openOrders.Store(cancelOrder.OrderID, deletedOrder)
	tokenState.unclaimedFundsLock.Lock()
	tokenState.unclaimedFunds[deletedOrder.SellerAddress] -= deletedOrder.AmountToSell
	tokenState.unclaimedFundsLock.Unlock()
}

func (state *ConsensusState) AddCreateToken(createToken CreateToken) bool {
	return false
}

func (state *ConsensusState) RollbackCreateToken(createToken CreateToken) {

}
