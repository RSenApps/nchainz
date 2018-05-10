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
	usedMatchIDs  map[uint64]bool
}

func NewConsensusStateToken() *ConsensusStateToken {
	token := ConsensusStateToken{}
	token.openOrders = make(map[uint64]Order)
	token.balances = make(map[string]uint64)
	token.unclaimedFunds = make(map[string]uint64)
	token.deletedOrders = make(map[uint64]Order)
	token.usedTransferIDs = make(map[uint64]bool)
	return &token
}

func NewConsensusState() ConsensusState {
	state := ConsensusState{}
	state.tokenStates = make(map[string]*ConsensusStateToken)
	state.createdTokens = make(map[string]TokenInfo)
	state.usedMatchIDs = make(map[uint64]bool)
	state.tokenStates[MATCH_CHAIN] = NewConsensusStateToken()
	return state
}

func (state *ConsensusState) AddOrder(symbol string, order Order) bool {
	tokenState, symbolExists := state.tokenStates[symbol]
	if !symbolExists {
		Log("Order failed as %v chain does not exist", symbol)
		return false
	}

	if tokenState.balances[order.SellerAddress] < order.AmountToSell {
		Log("Order failed as seller %v has %v, but needs %v", order.SellerAddress, tokenState.balances[order.SellerAddress], order.AmountToSell)
		return false
	}

	_, loaded := tokenState.deletedOrders[order.ID]
	if loaded {
		Log("Order failed as orderID in deletedOrders")
		return false
	}

	_, loaded = tokenState.openOrders[order.ID]
	if loaded {
		Log("Order failed as orderID in openOrders")
		return false
	}
	Log("Order added to consensus state %v", order)
	tokenState.openOrders[order.ID] = order
	tokenState.balances[order.SellerAddress] -= order.AmountToSell
	return true
}

func (state *ConsensusState) RollbackOrder(symbol string, order Order, blockchains *Blockchains) {
	Log("Order rolled back from consensus state %v", order)
	tokenState := state.tokenStates[symbol]

	//if rolling back order and not in openOrders then it must have been matched (cancels would be rolled back already)
	//Rollback match chain until it works (start with current height to forget unmined)
	if _, ok := tokenState.openOrders[order.ID]; !ok {
		Log("Rolling back unmined matches as Order %v does not show up in open orders", order.ID)
		blockchains.RollbackToHeight(MATCH_CHAIN, blockchains.chains[MATCH_CHAIN].height, false)
	}

	for _, ok := tokenState.openOrders[order.ID]; !ok; _, ok = tokenState.openOrders[order.ID] {
		Log("Rolling back match chain to height %v as Order %v does not show up in open orders", blockchains.chains[MATCH_CHAIN].height-1, order.ID)
		blockchains.RollbackToHeight(MATCH_CHAIN, blockchains.chains[MATCH_CHAIN].height-1, false)
	}

	delete(tokenState.openOrders, order.ID)
	tokenState.balances[order.SellerAddress] += order.AmountToSell
}

func (state *ConsensusState) AddClaimFunds(symbol string, funds ClaimFunds) bool {
	tokenState, symbolExists := state.tokenStates[symbol]
	if !symbolExists {
		Log("Claim funds failed as %v chain does not exist", symbol)
		return false
	}

	if tokenState.unclaimedFunds[funds.Address] < funds.Amount {
		Log("Claim funds failed as address %v has %v, but needs %v", funds.Address, tokenState.unclaimedFunds[funds.Address], funds.Amount)
		return false
	}
	Log("Claim funds added to chain %v", funds)
	tokenState.unclaimedFunds[funds.Address] -= funds.Amount
	tokenState.balances[funds.Address] += funds.Amount
	return true
}

func (state *ConsensusState) RollbackClaimFunds(symbol string, funds ClaimFunds) {
	Log("Claim funds rolled back from consensus state %v", funds)
	tokenState := state.tokenStates[symbol]
	tokenState.unclaimedFunds[funds.Address] += funds.Amount
	tokenState.balances[funds.Address] -= funds.Amount
}

func (state *ConsensusState) AddTransfer(symbol string, transfer Transfer) bool {
	tokenState, symbolExists := state.tokenStates[symbol]
	if !symbolExists {
		Log("Transfer failed as %v chain does not exist", symbol)
		return false
	}

	if _, ok := tokenState.usedTransferIDs[transfer.ID]; ok {
		Log("Transfer failed as transferID in usedTransferIDs")
		return false
	}
	if transfer.Amount < 0 {
		Log("Transfer failed as amount < 0")
		return false
	}
	if tokenState.balances[transfer.FromAddress] < transfer.Amount {
		log.Printf("Transfer failed due to insufficient funds Address: %v Balance: %v TransferAMT: %v \n", transfer.FromAddress, tokenState.balances[transfer.FromAddress], transfer.Amount)
		return false
	}
	tokenState.balances[transfer.FromAddress] -= transfer.Amount
	tokenState.balances[transfer.ToAddress] += transfer.Amount
	Log("Transfer added to consensus state %v FromBalance %v ToBalance %v", transfer, tokenState.balances[transfer.FromAddress], tokenState.balances[transfer.ToAddress])
	tokenState.usedTransferIDs[transfer.ID] = true
	return true
}

func (state *ConsensusState) RollbackTransfer(symbol string, transfer Transfer) {
	tokenState := state.tokenStates[symbol]
	delete(tokenState.usedTransferIDs, transfer.ID)
	tokenState.balances[transfer.FromAddress] += transfer.Amount
	tokenState.balances[transfer.ToAddress] -= transfer.Amount

	Log("Transfer rolled back from consensus state %v FromBalance %v ToBalance %v", transfer, tokenState.balances[transfer.FromAddress], tokenState.balances[transfer.ToAddress])
}

func (state *ConsensusState) AddMatch(match Match) bool {
	//Check if both buy and sell orders are satisfied by match and that orders are open
	//add to unconfirmedSell and Buy Matches
	//remove orders from openOrders
	if state.usedMatchIDs[match.MatchID] {
		Log("Duplicate match ignored: %v", match.MatchID)
		return false
	}
	buyTokenState, symbolExists := state.tokenStates[match.BuySymbol]
	if !symbolExists {
		Log("Match failed as %v buy chain does not exist", match.BuySymbol)
		return false
	}
	sellTokenState, symbolExists := state.tokenStates[match.SellSymbol]
	if !symbolExists {
		Log("Match failed as %v sell chain does not exist", match.SellSymbol)
		return false
	}
	buyOrder, ok := buyTokenState.openOrders[match.BuyOrderID]
	if !ok {
		Log("Match failed as Buy Order %v is not in openOrders", match.BuyOrderID)
		return false
	}
	sellOrder, ok := sellTokenState.openOrders[match.SellOrderID]
	if !ok {
		Log("Match failed as Sell Order %v is not in openOrders", match.SellOrderID)
		return false
	}

	if sellOrder.AmountToSell < match.TransferAmt {
		Log("Match failed as Sell Order has %v left, but match is for %v", sellOrder.AmountToSell, match.TransferAmt)
		return false
	}

	buyPrice := float64(buyOrder.AmountToSell) / float64(buyOrder.AmountToBuy)
	sellPrice := float64(sellOrder.AmountToBuy) / float64(sellOrder.AmountToSell)

	var transferAmt, buyerBaseLoss, sellerBaseGain uint64

	if buyOrder.AmountToBuy > sellOrder.AmountToSell {
		transferAmt = sellOrder.AmountToSell
		sellerBaseGain = sellOrder.AmountToBuy
		if buyPrice*float64(transferAmt) > float64(^uint64(0)) {
			Log("Match failed due to buy overflow")
			return false
		}
		buyerBaseLoss = uint64(math.Floor(buyPrice * float64(transferAmt)))

	} else { // buyOrder.AmountToBuy <= sellOrder.AmountToSell
		transferAmt = buyOrder.AmountToBuy
		buyerBaseLoss = buyOrder.AmountToSell
		if sellPrice*float64(transferAmt) > float64(^uint64(0)) {
			Log("Match failed due to sell overflow")
			return false
		}
		sellerBaseGain = uint64(math.Ceil(sellPrice * float64(transferAmt)))
	}

	if match.TransferAmt != transferAmt {
		Log("Match failed as transfer amt %v is incorrect: should be %v", match.TransferAmt, transferAmt)
		return false
	}
	if sellerBaseGain != match.SellerGain {
		Log("Match failed as seller gain %v is incorrect: should be %v", match.SellerGain, sellerBaseGain)
		return false
	}
	if buyerBaseLoss != match.BuyerLoss {
		Log("Match failed as buyer loss %v is incorrect: should be %v", match.BuyerLoss, buyerBaseLoss)
		return false
	}
	if sellerBaseGain > buyerBaseLoss {
		Log("Match failed as Buyer price is too high willing to pay %v, but seller needs at least %v", buyerBaseLoss, sellerBaseGain)
		return false
	}
	if buyerBaseLoss > buyOrder.AmountToSell {
		Log("Match failed as Buy Order has %v left, but match needs %v from buyer", buyOrder.AmountToSell, buyerBaseLoss)
		return false
	}

	Log("Match added to chain %v", match)

	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] += match.SellerGain
	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] += match.TransferAmt

	buyOrder.AmountToBuy -= match.TransferAmt
	buyOrder.AmountToSell -= match.BuyerLoss

	sellOrder.AmountToSell -= match.TransferAmt
	sellOrder.AmountToBuy -= match.SellerGain

	var deletedOrders []Order
	if sellOrder.AmountToBuy == 0 {
		Log("Sell order depleted %v", sellOrder)
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] += sellOrder.AmountToSell
		}
		deletedOrders = append(deletedOrders, sellOrder)
		sellTokenState.deletedOrders[sellOrder.ID] = sellOrder
		delete(sellTokenState.openOrders, sellOrder.ID)
	} else {
		sellTokenState.openOrders[sellOrder.ID] = sellOrder
	}

	if buyOrder.AmountToBuy == 0 {
		Log("Buy order depleted %v", buyOrder)
		if buyOrder.AmountToSell > 0 {
			buyTokenState.unclaimedFunds[buyOrder.SellerAddress] += buyOrder.AmountToSell
		}
		deletedOrders = append(deletedOrders, buyOrder)
		buyTokenState.deletedOrders[buyOrder.ID] = buyOrder
		delete(buyTokenState.openOrders, buyOrder.ID)
	} else {
		buyTokenState.openOrders[buyOrder.ID] = buyOrder
	}

	state.usedMatchIDs[match.MatchID] = true
	return true
}

func (state *ConsensusState) GetBuySellOrdersForMatch(match Match) (Order, Order) {
	buyTokenState := state.tokenStates[match.BuySymbol]
	sellTokenState := state.tokenStates[match.SellSymbol]

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
	return buyOrder, sellOrder
}

func (state *ConsensusState) RollbackMatch(match Match, blockchains *Blockchains) {
	Log("Rolling back match from consensus state %v", match)
	buyTokenState := state.tokenStates[match.BuySymbol]
	sellTokenState := state.tokenStates[match.SellSymbol]

	// were orders deleted?
	buyOrder, sellOrder := state.GetBuySellOrdersForMatch(match)

	//if rolling back match and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	if buyTokenState.unclaimedFunds[sellOrder.SellerAddress] < match.SellerGain {
		Log("Rolling back unmined token %v as rolling back match %v would result in a negative unclaimed funds", match.BuySymbol, match)
		blockchains.RollbackToHeight(match.BuySymbol, blockchains.chains[match.BuySymbol].height, false)
	}

	for buyTokenState.unclaimedFunds[sellOrder.SellerAddress] < match.SellerGain {
		Log("Rolling back token %v to height %v as rolling back match %v would result in a negative unclaimed funds", match.BuySymbol, blockchains.chains[match.SellSymbol].height, match)
		blockchains.RollbackToHeight(match.BuySymbol, blockchains.chains[match.BuySymbol].height-1, false)
	}
	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] -= match.SellerGain

	//if rolling back match and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	if sellTokenState.unclaimedFunds[buyOrder.SellerAddress] < match.TransferAmt {
		Log("Rolling back unmined token %v as rolling back match %v would result in a negative unclaimed funds", match.SellSymbol, match)
		blockchains.RollbackToHeight(match.SellSymbol, blockchains.chains[match.SellSymbol].height, false)
	}

	for sellTokenState.unclaimedFunds[buyOrder.SellerAddress] < match.TransferAmt {
		Log("Rolling back token %v to height %v as rolling back match %v would result in a negative unclaimed funds", match.SellSymbol, blockchains.chains[match.SellSymbol].height, match)
		blockchains.RollbackToHeight(match.SellSymbol, blockchains.chains[match.SellSymbol].height-1, false)
	}
	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] -= match.TransferAmt

	buyOrder.AmountToBuy += match.TransferAmt
	buyOrder.AmountToSell += match.BuyerLoss
	sellOrder.AmountToBuy += match.SellerGain
	sellOrder.AmountToSell += match.TransferAmt

	sellTokenState.openOrders[match.SellOrderID] = sellOrder
	buyTokenState.openOrders[match.BuyOrderID] = buyOrder

	delete(state.usedMatchIDs, match.MatchID)
}

func (state *ConsensusState) AddCancelOrder(cancelOrder CancelOrder) bool {
	tokenState, symbolExists := state.tokenStates[cancelOrder.OrderSymbol]
	if !symbolExists {
		Log("Cancel Order failed as %v chain does not exist", cancelOrder.OrderSymbol)
		return false
	}
	order, ok := tokenState.openOrders[cancelOrder.OrderID]
	if !ok {
		Log("Cancel Order failed as order %v is not open", cancelOrder.OrderID)
		return false
	}
	Log("Cancel Order added to consensus state %v", cancelOrder)

	tokenState.unclaimedFunds[order.SellerAddress] += order.AmountToSell
	delete(tokenState.openOrders, cancelOrder.OrderID)
	return true
}

func (state *ConsensusState) RollbackCancelOrder(cancelOrder CancelOrder, blockchains *Blockchains) {
	tokenState := state.tokenStates[cancelOrder.OrderSymbol]
	deletedOrder, _ := tokenState.deletedOrders[cancelOrder.OrderID]

	delete(tokenState.deletedOrders, cancelOrder.OrderID)

	tokenState.openOrders[cancelOrder.OrderID] = deletedOrder

	//if rolling back cancel order and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	if tokenState.unclaimedFunds[deletedOrder.SellerAddress] < deletedOrder.AmountToSell {
		Log("Rolling back unmined token %v as rolling back cancel order %v would result in a negative unclaimed funds", cancelOrder.OrderSymbol, cancelOrder)
		blockchains.RollbackToHeight(cancelOrder.OrderSymbol, blockchains.chains[cancelOrder.OrderSymbol].height, false)
	}

	for tokenState.unclaimedFunds[deletedOrder.SellerAddress] < deletedOrder.AmountToSell {
		Log("Rolling back token %v to height %v as rolling back cancel order %v would result in a negative unclaimed funds", cancelOrder.OrderSymbol, blockchains.chains[cancelOrder.OrderSymbol].height, cancelOrder)
		blockchains.RollbackToHeight(cancelOrder.OrderSymbol, blockchains.chains[cancelOrder.OrderSymbol].height, false)
	}
	tokenState.unclaimedFunds[deletedOrder.SellerAddress] -= deletedOrder.AmountToSell
}

func (state *ConsensusState) AddCreateToken(createToken CreateToken, blockchains *Blockchains) bool {
	_, ok := state.createdTokens[createToken.TokenInfo.Symbol]
	if ok {
		Log("Create Token failed as symbol %v already exists", createToken.TokenInfo.Symbol)
		return false
	}

	Log("Create token added %v", createToken)

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
	Log("Create token rolled back %v", createToken)
	delete(state.tokenStates, createToken.TokenInfo.Symbol)
	delete(state.createdTokens, createToken.TokenInfo.Symbol)
	blockchains.RemoveTokenChain(createToken)
}
