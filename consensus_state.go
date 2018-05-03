package main

import (
	"log"
	"math"
)

type ConsensusStateToken struct {
	openOrders     map[uint64]Order
	balances       map[string]uint64
	unclaimedFunds map[string]uint64
	//unclaimedFundsLock sync.Mutex
	deletedOrders map[uint64]Order
	//deletedOrdersLock sync.Mutex
	usedTransferIDs map[uint64]bool
}

type ConsensusState struct {
	tokenStates   map[string]*ConsensusStateToken
	createdTokens map[string]TokenInfo
}

func NewConsensusStateToken() *ConsensusStateToken {
	token := ConsensusStateToken{}
	token.openOrders = make(map[uint64]Order)
	token.balances = make(map[string]uint64)
	token.unclaimedFunds = make(map[string]uint64)
	token.deletedOrders = make(map[uint64]Order)
	return &token
}

func NewConsensusState() ConsensusState {
	state := ConsensusState{}
	state.tokenStates = make(map[string]*ConsensusStateToken)
	state.createdTokens = make(map[string]TokenInfo)

	state.tokenStates[MATCH_CHAIN] = NewConsensusStateToken()
	return state
}

func (state *ConsensusState) AddOrder(symbol string, order Order) bool {
	tokenState := state.tokenStates[symbol]
	if tokenState.balances[order.SellerAddress] < order.AmountToSell {
		return false
	}

	_, loaded := tokenState.deletedOrders[order.ID]
	if loaded {
		return false
	}

	_, loaded = tokenState.openOrders[order.ID]
	if loaded {
		return false
	}
	tokenState.openOrders[order.ID] = order
	tokenState.balances[order.SellerAddress] -= order.AmountToSell
	return true
}

func (state *ConsensusState) RollbackOrder(symbol string, order Order) {
	tokenState := state.tokenStates[symbol]
	delete(tokenState.openOrders, order.ID)
	tokenState.balances[order.SellerAddress] += order.AmountToSell
}

func (state *ConsensusState) AddClaimFunds(symbol string, funds ClaimFunds) bool {
	tokenState := state.tokenStates[symbol]
	if tokenState.unclaimedFunds[funds.Address] < funds.Amount {
		return false
	}
	tokenState.unclaimedFunds[funds.Address] -= funds.Amount
	tokenState.balances[funds.Address] += funds.Amount
	return true
}

func (state *ConsensusState) RollbackClaimFunds(symbol string, funds ClaimFunds) {
	tokenState := state.tokenStates[symbol]
	tokenState.unclaimedFunds[funds.Address] += funds.Amount
	tokenState.balances[funds.Address] -= funds.Amount
}

func (state *ConsensusState) AddTransfer(symbol string, transfer Transfer) bool {
	tokenState := state.tokenStates[symbol]
	if _, ok := tokenState.usedTransferIDs[transfer.ID]; ok {
		return false
	}
	if transfer.Amount < 0 {
		return false
	}
	if tokenState.balances[transfer.FromAddress] < transfer.Amount {
		log.Println("Rejecting block due to insufficient funds")
		return false
	}
	tokenState.balances[transfer.FromAddress] -= transfer.Amount
	tokenState.balances[transfer.ToAddress] += transfer.Amount
	tokenState.usedTransferIDs[transfer.ID] = true
	return true
}

func (state *ConsensusState) RollbackTransfer(symbol string, transfer Transfer) {
	tokenState := state.tokenStates[symbol]
	delete(tokenState.usedTransferIDs, transfer.ID)
	tokenState.balances[transfer.FromAddress] += transfer.Amount
	tokenState.balances[transfer.ToAddress] -= transfer.Amount
}

func (state *ConsensusState) AddMatch(match Match) bool {
	//Check if both buy and sell orders are satisfied by match and that orders are open
	//add to unconfirmedSell and Buy Matches
	//remove orders from openOrders
	buyTokenState := state.tokenStates[match.BuySymbol]
	sellTokenState := state.tokenStates[match.SellSymbol]
	buyOrder, ok := buyTokenState.openOrders[match.BuyOrderID]
	if !ok {
		return false
	}
	sellOrder, ok := sellTokenState.openOrders[match.SellOrderID]
	if !ok {
		return false
	}

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
	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] += sellerMinAmountReceived

	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] += match.AmountSold

	sellOrder.AmountToSell -= match.AmountSold
	sellOrder.AmountToBuy -= sellerMinAmountReceived
	buyOrder.AmountToBuy -= match.AmountSold
	buyOrder.AmountToSell -= buyerMaxAmountPaid

	var deletedOrders []Order
	if sellOrder.AmountToBuy <= 0 {
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] += sellOrder.AmountToSell
		}
		deletedOrders = append(deletedOrders, sellOrder)
		sellTokenState.deletedOrders[sellOrder.ID] = sellOrder
		delete(sellTokenState.openOrders, sellOrder.ID)
	} else {
		sellTokenState.openOrders[sellOrder.ID] = sellOrder
	}

	if buyOrder.AmountToBuy <= 0 {
		if buyOrder.AmountToSell > 0 {
			buyTokenState.unclaimedFunds[buyOrder.SellerAddress] += buyOrder.AmountToSell
		}
		deletedOrders = append(deletedOrders, buyOrder)
		buyTokenState.deletedOrders[buyOrder.ID] = buyOrder
		delete(buyTokenState.openOrders, buyOrder.ID)
	} else {
		buyTokenState.openOrders[buyOrder.ID] = buyOrder
	}
	return true
}

func (state *ConsensusState) RollbackMatch(match Match) {
	buyTokenState := state.tokenStates[match.BuySymbol]
	sellTokenState := state.tokenStates[match.SellSymbol]

	// were orders deleted?
	buyOrder, ok := buyTokenState.deletedOrders[match.BuyOrderID]
	if ok {
		if buyOrder.AmountToSell > 0 {
			buyTokenState.unclaimedFunds[buyOrder.SellerAddress] -= buyOrder.AmountToSell
		}
		delete(buyTokenState.deletedOrders, match.BuyOrderID)
	} else {
		buyOrder, _ = buyTokenState.openOrders[match.BuyOrderID]
	}

	sellOrder, ok := sellTokenState.deletedOrders[match.SellOrderID]
	if ok {
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] -= sellOrder.AmountToSell
		}
		delete(sellTokenState.deletedOrders, match.SellOrderID)
	} else {
		sellOrder, _ = sellTokenState.openOrders[match.SellOrderID]
	}

	buyPrice := float64(buyOrder.AmountToBuy) / float64(buyOrder.AmountToSell)
	buyerMaxAmountPaid := uint64(math.Ceil(buyPrice * float64(match.AmountSold)))
	sellPrice := float64(sellOrder.AmountToSell) / float64(sellOrder.AmountToBuy)
	sellerMinAmountReceived := uint64(math.Floor(sellPrice * float64(match.AmountSold)))

	//TODO: minerReward := buyerMaxAmountPaid - sellerMinAmountPaid
	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] -= sellerMinAmountReceived

	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] -= match.AmountSold

	sellOrder.AmountToSell += match.AmountSold
	sellOrder.AmountToBuy += sellerMinAmountReceived
	buyOrder.AmountToBuy += match.AmountSold
	buyOrder.AmountToSell += buyerMaxAmountPaid

	sellTokenState.openOrders[match.SellOrderID] = sellOrder
	buyTokenState.openOrders[match.BuyOrderID] = buyOrder
}

func (state *ConsensusState) AddCancelOrder(cancelOrder CancelOrder) bool {
	tokenState := state.tokenStates[cancelOrder.OrderSymbol]
	order, ok := tokenState.openOrders[cancelOrder.OrderID]
	if !ok {
		return false
	}
	tokenState.unclaimedFunds[order.SellerAddress] += order.AmountToSell
	delete(tokenState.openOrders, cancelOrder.OrderID)
	return true

}

func (state *ConsensusState) RollbackCancelOrder(cancelOrder CancelOrder) {
	tokenState := state.tokenStates[cancelOrder.OrderSymbol]

	deletedOrder, _ := tokenState.deletedOrders[cancelOrder.OrderID]
	delete(tokenState.deletedOrders, cancelOrder.OrderID)

	tokenState.openOrders[cancelOrder.OrderID] = deletedOrder
	tokenState.unclaimedFunds[deletedOrder.SellerAddress] -= deletedOrder.AmountToSell
}

func (state *ConsensusState) AddCreateToken(createToken CreateToken, blockchains *Blockchains) bool {
	_, ok := state.createdTokens[createToken.TokenInfo.Symbol]
	if ok {
		return false
	}

	//create a new entry in tokenStates
	state.tokenStates[createToken.TokenInfo.Symbol] = NewConsensusStateToken()
	state.createdTokens[createToken.TokenInfo.Symbol] = createToken.TokenInfo

	tokenState := state.tokenStates[createToken.TokenInfo.Symbol]

	tokenState.unclaimedFunds[createToken.CreatorAddress] = createToken.TokenInfo.TotalSupply

	//create a new blockchain with proper genesis block
	blockchains.AddTokenChain(createToken)

	return true
}

func (state *ConsensusState) RollbackCreateToken(createToken CreateToken, blockchains *Blockchains) {
	delete(state.tokenStates, createToken.TokenInfo.Symbol)
	delete(state.createdTokens, createToken.TokenInfo.Symbol)
	blockchains.RemoveTokenChain(createToken)
}
