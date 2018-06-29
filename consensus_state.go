package main

import (
	"log"
	"math"
)

type ConsensusStateToken struct {
	openOrders        map[uint64]Order
	orderUpdatesCount map[uint64]uint32 //count partial matches and cancels to determine when order can be rolled back
	balances          map[[addressLength]byte]uint64
	unclaimedFunds    map[[addressLength]byte]uint64
	//unclaimedFundsLock sync.Mutex
	deletedOrders map[uint64]Order
	//deletedOrdersLock sync.Mutex
	usedTransferIDs  map[uint64]bool
	usedFreezeIDs    map[uint64]bool
	frozenTokenStore map[uint64]map[[addressLength]byte]uint64 // map of block height to address to amount
}

type ConsensusState struct {
	tokenStates   map[string]*ConsensusStateToken
	createdTokens map[string]TokenInfo
	usedMatchIDs  map[uint64]bool
}

func NewConsensusStateToken() *ConsensusStateToken {
	token := ConsensusStateToken{}
	token.openOrders = make(map[uint64]Order)
	token.balances = make(map[[addressLength]byte]uint64)
	token.unclaimedFunds = make(map[[addressLength]byte]uint64)
	token.deletedOrders = make(map[uint64]Order)
	token.usedTransferIDs = make(map[uint64]bool)
	token.usedFreezeIDs = make(map[uint64]bool)
	token.frozenTokenStore = make(map[uint64]map[[addressLength]byte]uint64)
	token.orderUpdatesCount = make(map[uint64]uint32)
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
	tokenState.orderUpdatesCount[order.ID] = 0
	return true
}

func (state *ConsensusState) RollbackUntilRollbackOrderSucceeds(symbol string, order Order, blockchains *Blockchains, takeMempoolLock bool) {
	tokenState := state.tokenStates[symbol]

	//if rolling back order and not in openOrders then it must have been matched (cancels would be rolled back already)
	//Rollback match chain until it works (start with current height to forget unmined)
	if count, ok := tokenState.orderUpdatesCount[order.ID]; count > 0 || !ok {
		if !ok {
			panic("error")
		}
		Log("Rolling back unmined matches as Order %v has %v updates still", order.ID, count)
		blockchains.RollbackToHeight(MATCH_CHAIN, blockchains.chains[MATCH_CHAIN].height, false, takeMempoolLock)
	}

	for tokenState.orderUpdatesCount[order.ID] > 0 {
		Log("Rolling back match chain to height %v as Order %v has %v updates still", blockchains.chains[MATCH_CHAIN].height-1, order.ID, tokenState.orderUpdatesCount[order.ID])
		blockchains.RollbackToHeight(MATCH_CHAIN, blockchains.chains[MATCH_CHAIN].height-1, false, takeMempoolLock)
	}
}

func (state *ConsensusState) RollbackOrder(symbol string, order Order) {
	Log("Order rolled back from consensus state %v", order)
	tokenState := state.tokenStates[symbol]
	delete(tokenState.openOrders, order.ID)
	delete(tokenState.orderUpdatesCount, order.ID)
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

func (state *ConsensusState) AddFreeze(symbol string, freeze Freeze) bool {
	tokenState, symbolExists := state.tokenStates[symbol]
	if !symbolExists {
		Log("Freeze failed as %v chain does not exist", symbol)
		return false
	}

	if _, ok := tokenState.usedFreezeIDs[freeze.ID]; ok {
		Log("Freeze failed as freezeID in usedFreezeIDs")
		return false
	}
	if freeze.Amount < 0 {
		Log("Freeze failed as amount < 0")
		return false
	}
	if tokenState.balances[freeze.FromAddress] < freeze.Amount {
		log.Printf("Freeze failed due to insufficient funds Address: %v Balance: %v AMT: %v \n", freeze.FromAddress, tokenState.balances[freeze.FromAddress], freeze.Amount)
		return false
	}
	tokenState.balances[freeze.FromAddress] -= freeze.Amount
	blockFreezes, ok := tokenState.frozenTokenStore[freeze.UnfreezeBlock]
	if !ok {
		blockFreezes = make(map[[addressLength]byte]uint64)
	}
	blockFreezes[freeze.FromAddress] += freeze.Amount
	tokenState.frozenTokenStore[freeze.UnfreezeBlock] = blockFreezes

	Log("Freeze added to consensus state %v FromBalance %v FrozenBalance %v", freeze, tokenState.balances[freeze.FromAddress], blockFreezes[freeze.FromAddress])
	tokenState.usedFreezeIDs[freeze.ID] = true
	return true
}

func (state *ConsensusState) RollbackFreeze(symbol string, freeze Freeze) {
	tokenState := state.tokenStates[symbol]
	delete(tokenState.usedFreezeIDs, freeze.ID)
	tokenState.balances[freeze.FromAddress] += freeze.Amount
	tokenState.frozenTokenStore[freeze.UnfreezeBlock][freeze.FromAddress] -= freeze.Amount

	Log("Freeze rolled back from consensus state %v FromBalance %v FrozenBalance %v", freeze, tokenState.balances[freeze.FromAddress], tokenState.frozenTokenStore[freeze.UnfreezeBlock][freeze.FromAddress])
}

func (state *ConsensusState) ApplyUnfreezesForBlock(symbol string, height uint64) {
	tokenState, _ := state.tokenStates[symbol]

	blockFreezes, ok := tokenState.frozenTokenStore[height]
	if !ok {
		return
	}
	for address, frozen := range blockFreezes {
		Log("%v Tokens for Address %v became unfrozen at block %v", frozen, address, height)
		tokenState.balances[address] += frozen
	}
}

func (state *ConsensusState) RollbackUnfreezesForBlock(symbol string, height uint64) {
	tokenState, _ := state.tokenStates[symbol]

	blockFreezes, ok := tokenState.frozenTokenStore[height]
	if !ok {
		return
	}
	for address, frozen := range blockFreezes {
		Log("Rolling back unfreeze of %v Tokens for Address %v at block %v", frozen, address, height)
		tokenState.balances[address] -= frozen
	}
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

	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] += match.SellerGain
	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] += match.TransferAmt

	Log("Match added to chain %v unclaimedFunds buy %v sell %v", match, buyTokenState.unclaimedFunds[sellOrder.SellerAddress], sellTokenState.unclaimedFunds[buyOrder.SellerAddress])

	buyOrder.AmountToBuy -= match.TransferAmt
	buyOrder.AmountToSell -= match.BuyerLoss

	sellOrder.AmountToSell -= match.TransferAmt
	sellOrder.AmountToBuy -= match.SellerGain

	buyTokenState.orderUpdatesCount[buyOrder.ID]++
	sellTokenState.orderUpdatesCount[sellOrder.ID]++

	if sellOrder.AmountToBuy == 0 {
		Log("Sell order depleted %v", sellOrder)
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] += sellOrder.AmountToSell
		}
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
	if !ok {
		buyOrder, ok = buyTokenState.openOrders[match.BuyOrderID]
		if !ok {
			panic("error")
		}
	}

	sellOrder, ok := sellTokenState.deletedOrders[match.SellOrderID]
	if !ok {
		sellOrder, ok = sellTokenState.openOrders[match.SellOrderID]
		if !ok {
			panic("error")
		}
	}
	return buyOrder, sellOrder
}

func (state *ConsensusState) RollbackUntilRollbackMatchSucceeds(match Match, blockchains *Blockchains, takeMempoolLock bool) {
	buyTokenState := state.tokenStates[match.BuySymbol]
	sellTokenState := state.tokenStates[match.SellSymbol]

	// were orders deleted?
	buyOrder, sellOrder := state.GetBuySellOrdersForMatch(match)

	//if rolling back match and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	unclaimed, ok := buyTokenState.unclaimedFunds[sellOrder.SellerAddress]
	if !ok {
		unclaimed = 0
	}
	if unclaimed < match.SellerGain {
		Log("Rolling back unmined token %v as rolling back match %v would result in a negative unclaimed funds %v < %v", match.BuySymbol, match, buyTokenState.unclaimedFunds[sellOrder.SellerAddress], match.SellerGain)
		panic("error") //TODO: DEBUG
		blockchains.RollbackToHeight(match.BuySymbol, blockchains.chains[match.BuySymbol].height, false, takeMempoolLock)
	}

	for unclaimed, ok = buyTokenState.unclaimedFunds[sellOrder.SellerAddress]; !ok || unclaimed < match.SellerGain; unclaimed, ok = buyTokenState.unclaimedFunds[sellOrder.SellerAddress] {
		Log("Rolling back token %v to height %v as rolling back match %v would result in a negative unclaimed funds", match.BuySymbol, blockchains.chains[match.SellSymbol].height, match)
		blockchains.RollbackToHeight(match.BuySymbol, blockchains.chains[match.BuySymbol].height-1, false, takeMempoolLock)
	}
	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] -= match.SellerGain

	unclaimed, ok = sellTokenState.unclaimedFunds[buyOrder.SellerAddress]
	if !ok {
		unclaimed = 0
	}
	//if rolling back match and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	if unclaimed < match.TransferAmt {
		Log("Rolling back unmined token %v as rolling back match %v would result in a negative unclaimed funds %v < %v", match.SellSymbol, match, sellTokenState.unclaimedFunds[buyOrder.SellerAddress], match.TransferAmt)
		panic("error") //TODO: DEBUG
		blockchains.RollbackToHeight(match.SellSymbol, blockchains.chains[match.SellSymbol].height, false, takeMempoolLock)
	}

	for unclaimed, ok = sellTokenState.unclaimedFunds[buyOrder.SellerAddress]; !ok || unclaimed < match.TransferAmt; unclaimed, ok = sellTokenState.unclaimedFunds[buyOrder.SellerAddress] {
		Log("Rolling back token %v to height %v as rolling back match %v would result in a negative unclaimed funds", match.SellSymbol, blockchains.chains[match.SellSymbol].height, match)
		blockchains.RollbackToHeight(match.SellSymbol, blockchains.chains[match.SellSymbol].height-1, false, takeMempoolLock)
	}
}

func (state *ConsensusState) RollbackMatch(match Match) {
	Log("Rolling back match from consensus state %v", match)
	buyTokenState := state.tokenStates[match.BuySymbol]
	sellTokenState := state.tokenStates[match.SellSymbol]
	buyOrder, sellOrder := state.GetBuySellOrdersForMatch(match)

	_, ok := buyTokenState.deletedOrders[match.BuyOrderID]
	if ok {
		if buyOrder.AmountToSell > 0 {
			buyTokenState.unclaimedFunds[buyOrder.SellerAddress] -= buyOrder.AmountToSell
		}
		delete(buyTokenState.deletedOrders, match.BuyOrderID)
	}

	_, ok = sellTokenState.deletedOrders[match.SellOrderID]
	if ok {
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] -= sellOrder.AmountToSell
		}
		delete(sellTokenState.deletedOrders, match.SellOrderID)
	}

	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] -= match.TransferAmt

	buyOrder.AmountToBuy += match.TransferAmt
	buyOrder.AmountToSell += match.BuyerLoss
	sellOrder.AmountToBuy += match.SellerGain
	sellOrder.AmountToSell += match.TransferAmt

	sellTokenState.openOrders[match.SellOrderID] = sellOrder
	buyTokenState.openOrders[match.BuyOrderID] = buyOrder

	buyTokenState.orderUpdatesCount[buyOrder.ID]--
	sellTokenState.orderUpdatesCount[sellOrder.ID]--

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
	tokenState.orderUpdatesCount[cancelOrder.OrderID]++
	return true
}

func (state *ConsensusState) GetCancelAddress(cancelOrder CancelOrder) (bool, [addressLength]byte) {
	tokenState, symbolExists := state.tokenStates[cancelOrder.OrderSymbol]
	if !symbolExists {
		Log("Get Cancel Address failed as %v chain does not exist", cancelOrder.OrderSymbol)
		return false, [addressLength]byte{}
	}
	order, ok := tokenState.openOrders[cancelOrder.OrderID]
	if !ok {
		Log("Get Cancel Address failed as order %v is not open", cancelOrder.OrderID)
		return false, [addressLength]byte{}
	}
	Log("Cancel Order added to consensus state %v", cancelOrder)

	return true, order.SellerAddress
}

func (state *ConsensusState) RollbackUntilRollbackCancelOrderSucceeds(cancelOrder CancelOrder, blockchains *Blockchains, takeMempoolLock bool) {
	tokenState := state.tokenStates[cancelOrder.OrderSymbol]
	deletedOrder, _ := tokenState.deletedOrders[cancelOrder.OrderID]

	//if rolling back cancel order and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	if unclaimed, ok := tokenState.unclaimedFunds[deletedOrder.SellerAddress]; !ok || unclaimed < deletedOrder.AmountToSell {
		Log("Rolling back unmined token %v as rolling back cancel order %v would result in a negative unclaimed funds", cancelOrder.OrderSymbol, cancelOrder)
		blockchains.RollbackToHeight(cancelOrder.OrderSymbol, blockchains.chains[cancelOrder.OrderSymbol].height, false, takeMempoolLock)
	}

	for unclaimed, ok := tokenState.unclaimedFunds[deletedOrder.SellerAddress]; !ok || unclaimed < deletedOrder.AmountToSell; unclaimed, ok = tokenState.unclaimedFunds[deletedOrder.SellerAddress] {
		Log("Rolling back token %v to height %v as rolling back cancel order %v would result in a negative unclaimed funds", cancelOrder.OrderSymbol, blockchains.chains[cancelOrder.OrderSymbol].height, cancelOrder)
		blockchains.RollbackToHeight(cancelOrder.OrderSymbol, blockchains.chains[cancelOrder.OrderSymbol].height, false, takeMempoolLock)
	}
}

func (state *ConsensusState) RollbackCancelOrder(cancelOrder CancelOrder) {
	tokenState := state.tokenStates[cancelOrder.OrderSymbol]
	deletedOrder, _ := tokenState.deletedOrders[cancelOrder.OrderID]

	delete(tokenState.deletedOrders, cancelOrder.OrderID)

	tokenState.openOrders[cancelOrder.OrderID] = deletedOrder
	tokenState.orderUpdatesCount[cancelOrder.OrderID]--

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
