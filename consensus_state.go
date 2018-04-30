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

type MatchRollbackInfo struct {
	OriginalSellOrder Order
	OriginalBuyOrder Order
	SellerUnclaimedChangeBuy uint64
	SellerUnclaimedChangeSell uint64
	BuyerUnclaimedChangeBuy uint64
}

func (state *ConsensusState) AddMatch(match Match) (bool, MatchRollbackInfo) {
	//Check if both buy and sell orders are satisfied by match and that orders are open
	//add to unconfirmedSell and Buy Matches
	//remove orders from openOrders
	buyTokenState := state.GetTokenState(match.BuySymbol)
	sellTokenState := state.GetTokenState(match.SellSymbol)
	buyOrderGen, ok := buyTokenState.openOrders.Load(match.BuyOrderID)
	if !ok {
		return false, MatchRollbackInfo{}
	}
	sellOrderGen, ok := sellTokenState.openOrders.Load(match.SellOrderID)
	if !ok {
		return false, MatchRollbackInfo{}
	}
	buyOrder := buyOrderGen.(Order)
	sellOrder := sellOrderGen.(Order)

	if sellOrder.AmountToSell < match.AmountSold {
		return false, MatchRollbackInfo{}
	}

	//sell currency: AmountFromSellOrder = match.AmountSold; AmountToBuyer = match.AmountSold;
	//buy currency: AmountFromBuyOrder = buyerMaxAmountPaid; AmountToSeller = sellerMinAmountReceived;

	//price = amountBuyCurrency / amountSellCurrency
	buyPrice := float64(buyOrder.AmountToBuy) / float64(buyOrder.AmountToSell)
	buyerMaxAmountPaid := uint64(math.Ceil(buyPrice * float64(match.AmountSold)))
	sellPrice := float64(sellOrder.AmountToSell) / float64(sellOrder.AmountToBuy)
	sellerMinAmountReceived := uint64(math.Floor(sellPrice * float64(match.AmountSold)))
	if buyerMaxAmountPaid < sellerMinAmountReceived || buyerMaxAmountPaid > buyOrder.AmountToSell {
		return false, MatchRollbackInfo{}
	}

	var rollbackInfo MatchRollbackInfo
	rollbackInfo.OriginalBuyOrder = buyOrder
	rollbackInfo.OriginalSellOrder = sellOrder

	//Trade is between match.AmountSold and buyerMaxAmountPaid
	//TODO: minerReward := buyerMaxAmountPaid - sellerMinAmountPaid
	rollbackInfo.SellerUnclaimedChangeBuy = sellerMinAmountReceived
	buyTokenState.unclaimedFundsLock.Lock()
	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] += rollbackInfo.SellerUnclaimedChangeBuy
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
			rollbackInfo.SellerUnclaimedChangeSell = sellOrder.AmountToSell
			sellTokenState.unclaimedFundsLock.Lock()
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] += rollbackInfo.SellerUnclaimedChangeSell
			sellTokenState.unclaimedFundsLock.Unlock()
		}
		deletedOrders = append(deletedOrders, sellOrder)
		sellTokenState.openOrders.Delete(sellOrder.ID)
	} else {
		sellTokenState.openOrders.Store(sellOrder.ID, sellOrder)
	}

	if buyOrder.AmountToBuy <= 0 {
		if buyOrder.AmountToSell > 0 {
			rollbackInfo.BuyerUnclaimedChangeBuy = buyOrder.AmountToSell
			buyTokenState.unclaimedFundsLock.Lock()
			buyTokenState.unclaimedFunds[buyOrder.SellerAddress] += rollbackInfo.BuyerUnclaimedChangeBuy
			buyTokenState.unclaimedFundsLock.Unlock()
		}
		deletedOrders = append(deletedOrders, buyOrder)
		buyTokenState.openOrders.Delete(buyOrder.ID)
	} else {
		buyTokenState.openOrders.Store(buyOrder.ID, buyOrder)
	}
	return true, rollbackInfo
}

func (state *ConsensusState) RollbackMatch(match Match, rollbackInfo MatchRollbackInfo) {
	buyTokenState := state.GetTokenState(match.BuySymbol)
	sellTokenState := state.GetTokenState(match.SellSymbol)
	buyTokenState.openOrders.Store(match.SellOrderID, rollbackInfo.OriginalBuyOrder)
	sellTokenState.openOrders.Store(match.BuyOrderID, rollbackInfo.OriginalSellOrder)

	buyTokenState.unclaimedFundsLock.Lock()
	buyTokenState.unclaimedFunds[rollbackInfo.OriginalSellOrder.SellerAddress] -= rollbackInfo.SellerUnclaimedChangeBuy
	buyTokenState.unclaimedFunds[rollbackInfo.OriginalBuyOrder.SellerAddress] -= rollbackInfo.BuyerUnclaimedChangeBuy
	buyTokenState.unclaimedFundsLock.Unlock()

	sellTokenState.unclaimedFundsLock.Lock()
	sellTokenState.unclaimedFunds[rollbackInfo.OriginalSellOrder.SellerAddress] -= rollbackInfo.SellerUnclaimedChangeSell
	sellTokenState.unclaimedFunds[rollbackInfo.OriginalBuyOrder.SellerAddress] -= match.AmountSold
	sellTokenState.unclaimedFundsLock.Unlock()
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
