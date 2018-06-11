package consensus

import (
	"github.com/rsenapps/nchainz/txs"
	"github.com/rsenapps/nchainz/utils"
	"log"
	"math"
)

type ConsensusStateToken struct {
	openOrders        map[uint64]txs.Order
	orderUpdatesCount map[uint64]uint32 //count partial matches and cancels to determine when order can be rolled back
	balances          map[[utils.AddressLength]byte]uint64
	unclaimedFunds    map[[utils.AddressLength]byte]uint64
	//unclaimedFundsLock sync.Mutex
	deletedOrders map[uint64]txs.Order
	//deletedOrdersLock sync.Mutex
	usedTransferIDs map[uint64]bool
}

type ConsensusState struct {
	tokenStates   map[string]*ConsensusStateToken
	createdTokens map[string]txs.TokenInfo
	usedMatchIDs  map[uint64]bool
}

func NewConsensusStateToken() *ConsensusStateToken {
	token := ConsensusStateToken{}
	token.openOrders = make(map[uint64]txs.Order)
	token.balances = make(map[[utils.AddressLength]byte]uint64)
	token.unclaimedFunds = make(map[[utils.AddressLength]byte]uint64)
	token.deletedOrders = make(map[uint64]txs.Order)
	token.usedTransferIDs = make(map[uint64]bool)
	token.orderUpdatesCount = make(map[uint64]uint32)
	return &token
}

func NewConsensusState() ConsensusState {
	state := ConsensusState{}
	state.tokenStates = make(map[string]*ConsensusStateToken)
	state.createdTokens = make(map[string]txs.TokenInfo)
	state.usedMatchIDs = make(map[uint64]bool)
	state.tokenStates[txs.MATCH_TOKEN] = NewConsensusStateToken()
	return state
}

func (state *ConsensusState) GetBalance(symbol string, address [utils.AddressLength]byte) uint64 {
	tokenState, ok := state.tokenStates[symbol]
	if !ok {
		utils.Log("GetBalance failed symbol %v does not exist", symbol)
		return 0
	}

	balance, ok := tokenState.balances[address]
	if !ok {
		return 0
	} else {
		return balance
	}
}

func (state *ConsensusState) GetUnclaimedBalance(symbol string, address [utils.AddressLength]byte) uint64 {
	tokenState, ok := state.tokenStates[symbol]
	if !ok {
		utils.Log("GetBalance failed symbol %v does not exist", symbol)
		return 0
	}

	balance, ok := tokenState.unclaimedFunds[address]
	return balance
}

func (state *ConsensusState) GetOpenOrders(symbol string) map[uint64]txs.Order {
	tokenState, ok := state.tokenStates[symbol]
	if !ok {
		utils.Log("GetOpenOrders failed symbol %v does not exist", symbol)
		return make(map[uint64]txs.Order)
	}
	return tokenState.openOrders
}

func (state *ConsensusState) GetOrderUpdatesCount(symbol string, orderID uint64) (uint32, bool) {
	tokenState := state.tokenStates[symbol]
	result, ok := tokenState.orderUpdatesCount[orderID]
	return result, ok
}

func (state *ConsensusState) GetDeletedOrder(symbol string, orderID uint64) (txs.Order, bool) {
	tokenState := state.tokenStates[symbol]
	order, ok := tokenState.deletedOrders[orderID]
	return order, ok
}

func (state *ConsensusState) AddOrder(symbol string, order txs.Order) bool {
	tokenState, symbolExists := state.tokenStates[symbol]
	if !symbolExists {
		utils.Log("Order failed as %v chain does not exist", symbol)
		return false
	}

	if tokenState.balances[order.SellerAddress] < order.AmountToSell {
		utils.Log("Order failed as seller %v has %v, but needs %v", order.SellerAddress, tokenState.balances[order.SellerAddress], order.AmountToSell)
		return false
	}

	_, loaded := tokenState.deletedOrders[order.ID]
	if loaded {
		utils.Log("Order failed as orderID in deletedOrders")
		return false
	}

	_, loaded = tokenState.openOrders[order.ID]
	if loaded {
		utils.Log("Order failed as orderID in openOrders")
		return false
	}
	utils.Log("Order added to consensus state %v", order)
	tokenState.openOrders[order.ID] = order
	tokenState.balances[order.SellerAddress] -= order.AmountToSell
	tokenState.orderUpdatesCount[order.ID] = 0
	return true
}

func (state *ConsensusState) RollbackOrder(symbol string, order txs.Order) {
	utils.Log("Order rolled back from consensus state %v", order)
	tokenState := state.tokenStates[symbol]
	delete(tokenState.openOrders, order.ID)
	delete(tokenState.orderUpdatesCount, order.ID)
	tokenState.balances[order.SellerAddress] += order.AmountToSell
}

func (state *ConsensusState) AddClaimFunds(symbol string, funds txs.ClaimFunds) bool {
	tokenState, symbolExists := state.tokenStates[symbol]
	if !symbolExists {
		utils.Log("Claim funds failed as %v chain does not exist", symbol)
		return false
	}

	if tokenState.unclaimedFunds[funds.Address] < funds.Amount {
		utils.Log("Claim funds failed as address %v has %v, but needs %v", funds.Address, tokenState.unclaimedFunds[funds.Address], funds.Amount)
		return false
	}
	utils.Log("Claim funds added to chain %v", funds)
	tokenState.unclaimedFunds[funds.Address] -= funds.Amount
	tokenState.balances[funds.Address] += funds.Amount
	return true
}

func (state *ConsensusState) RollbackClaimFunds(symbol string, funds txs.ClaimFunds) {
	utils.Log("Claim funds rolled back from consensus state %v", funds)
	tokenState := state.tokenStates[symbol]
	tokenState.unclaimedFunds[funds.Address] += funds.Amount
	tokenState.balances[funds.Address] -= funds.Amount
}

func (state *ConsensusState) AddTransfer(symbol string, transfer txs.Transfer) bool {
	tokenState, symbolExists := state.tokenStates[symbol]
	if !symbolExists {
		utils.Log("Transfer failed as %v chain does not exist", symbol)
		return false
	}

	if _, ok := tokenState.usedTransferIDs[transfer.ID]; ok {
		utils.Log("Transfer failed as transferID in usedTransferIDs")
		return false
	}
	if transfer.Amount < 0 {
		utils.Log("Transfer failed as amount < 0")
		return false
	}
	if tokenState.balances[transfer.FromAddress] < transfer.Amount {
		log.Printf("Transfer failed due to insufficient funds Address: %v Balance: %v TransferAMT: %v \n", transfer.FromAddress, tokenState.balances[transfer.FromAddress], transfer.Amount)
		return false
	}
	tokenState.balances[transfer.FromAddress] -= transfer.Amount
	tokenState.balances[transfer.ToAddress] += transfer.Amount
	utils.Log("Transfer added to consensus state %v FromBalance %v ToBalance %v", transfer, tokenState.balances[transfer.FromAddress], tokenState.balances[transfer.ToAddress])
	tokenState.usedTransferIDs[transfer.ID] = true
	return true
}

func (state *ConsensusState) RollbackTransfer(symbol string, transfer txs.Transfer) {
	tokenState := state.tokenStates[symbol]
	delete(tokenState.usedTransferIDs, transfer.ID)
	tokenState.balances[transfer.FromAddress] += transfer.Amount
	tokenState.balances[transfer.ToAddress] -= transfer.Amount

	utils.Log("Transfer rolled back from consensus state %v FromBalance %v ToBalance %v", transfer, tokenState.balances[transfer.FromAddress], tokenState.balances[transfer.ToAddress])
}

func (state *ConsensusState) AddMatch(match txs.Match) bool {
	//Check if both buy and sell orders are satisfied by match and that orders are open
	//add to unconfirmedSell and Buy Matches
	//remove orders from openOrders
	if state.usedMatchIDs[match.MatchID] {
		utils.Log("Duplicate match ignored: %v", match.MatchID)
		return false
	}
	buyTokenState, symbolExists := state.tokenStates[match.BuySymbol]
	if !symbolExists {
		utils.Log("Match failed as %v buy chain does not exist", match.BuySymbol)
		return false
	}
	sellTokenState, symbolExists := state.tokenStates[match.SellSymbol]
	if !symbolExists {
		utils.Log("Match failed as %v sell chain does not exist", match.SellSymbol)
		return false
	}
	buyOrder, ok := buyTokenState.openOrders[match.BuyOrderID]
	if !ok {
		utils.Log("Match failed as Buy Order %v is not in openOrders", match.BuyOrderID)
		return false
	}
	sellOrder, ok := sellTokenState.openOrders[match.SellOrderID]
	if !ok {
		utils.Log("Match failed as Sell Order %v is not in openOrders", match.SellOrderID)
		return false
	}

	if sellOrder.AmountToSell < match.TransferAmt {
		utils.Log("Match failed as Sell Order has %v left, but match is for %v", sellOrder.AmountToSell, match.TransferAmt)
		return false
	}

	buyPrice := float64(buyOrder.AmountToSell) / float64(buyOrder.AmountToBuy)
	sellPrice := float64(sellOrder.AmountToBuy) / float64(sellOrder.AmountToSell)

	var transferAmt, buyerBaseLoss, sellerBaseGain uint64

	if buyOrder.AmountToBuy > sellOrder.AmountToSell {
		transferAmt = sellOrder.AmountToSell
		sellerBaseGain = sellOrder.AmountToBuy
		if buyPrice*float64(transferAmt) > float64(^uint64(0)) {
			utils.Log("Match failed due to buy overflow")
			return false
		}
		buyerBaseLoss = uint64(math.Floor(buyPrice * float64(transferAmt)))

	} else { // buyOrder.AmountToBuy <= sellOrder.AmountToSell
		transferAmt = buyOrder.AmountToBuy
		buyerBaseLoss = buyOrder.AmountToSell
		if sellPrice*float64(transferAmt) > float64(^uint64(0)) {
			utils.Log("Match failed due to sell overflow")
			return false
		}
		sellerBaseGain = uint64(math.Ceil(sellPrice * float64(transferAmt)))
	}

	if match.TransferAmt != transferAmt {
		utils.Log("Match failed as transfer amt %v is incorrect: should be %v", match.TransferAmt, transferAmt)
		return false
	}
	if sellerBaseGain != match.SellerGain {
		utils.Log("Match failed as seller gain %v is incorrect: should be %v", match.SellerGain, sellerBaseGain)
		return false
	}
	if buyerBaseLoss != match.BuyerLoss {
		utils.Log("Match failed as buyer loss %v is incorrect: should be %v", match.BuyerLoss, buyerBaseLoss)
		return false
	}
	if sellerBaseGain > buyerBaseLoss {
		utils.Log("Match failed as Buyer price is too high willing to pay %v, but seller needs at least %v", buyerBaseLoss, sellerBaseGain)
		return false
	}
	if buyerBaseLoss > buyOrder.AmountToSell {
		utils.Log("Match failed as Buy Order has %v left, but match needs %v from buyer", buyOrder.AmountToSell, buyerBaseLoss)
		return false
	}

	buyTokenState.unclaimedFunds[sellOrder.SellerAddress] += match.SellerGain
	sellTokenState.unclaimedFunds[buyOrder.SellerAddress] += match.TransferAmt

	utils.Log("Match added to chain %v unclaimedFunds buy %v sell %v", match, buyTokenState.unclaimedFunds[sellOrder.SellerAddress], sellTokenState.unclaimedFunds[buyOrder.SellerAddress])

	buyOrder.AmountToBuy -= match.TransferAmt
	buyOrder.AmountToSell -= match.BuyerLoss

	sellOrder.AmountToSell -= match.TransferAmt
	sellOrder.AmountToBuy -= match.SellerGain

	buyTokenState.orderUpdatesCount[buyOrder.ID]++
	sellTokenState.orderUpdatesCount[sellOrder.ID]++

	if sellOrder.AmountToBuy == 0 {
		utils.Log("Sell order depleted %v", sellOrder)
		if sellOrder.AmountToSell > 0 {
			sellTokenState.unclaimedFunds[sellOrder.SellerAddress] += sellOrder.AmountToSell
		}
		sellTokenState.deletedOrders[sellOrder.ID] = sellOrder
		delete(sellTokenState.openOrders, sellOrder.ID)
	} else {
		sellTokenState.openOrders[sellOrder.ID] = sellOrder
	}

	if buyOrder.AmountToBuy == 0 {
		utils.Log("Buy order depleted %v", buyOrder)
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

func (state *ConsensusState) GetBuySellOrdersForMatch(match txs.Match) (txs.Order, txs.Order) {
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

func (state *ConsensusState) RollbackMatch(match txs.Match) {
	utils.Log("Rolling back match from consensus state %v", match)
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

func (state *ConsensusState) AddCancelOrder(cancelOrder txs.CancelOrder) bool {
	tokenState, symbolExists := state.tokenStates[cancelOrder.OrderSymbol]
	if !symbolExists {
		utils.Log("Cancel Order failed as %v chain does not exist", cancelOrder.OrderSymbol)
		return false
	}
	order, ok := tokenState.openOrders[cancelOrder.OrderID]
	if !ok {
		utils.Log("Cancel Order failed as order %v is not open", cancelOrder.OrderID)
		return false
	}
	utils.Log("Cancel Order added to consensus state %v", cancelOrder)

	tokenState.unclaimedFunds[order.SellerAddress] += order.AmountToSell
	delete(tokenState.openOrders, cancelOrder.OrderID)
	tokenState.orderUpdatesCount[cancelOrder.OrderID]++
	return true
}

func (state *ConsensusState) GetCancelAddress(cancelOrder txs.CancelOrder) (bool, [utils.AddressLength]byte) {
	tokenState, symbolExists := state.tokenStates[cancelOrder.OrderSymbol]
	if !symbolExists {
		utils.Log("Get Cancel Address failed as %v chain does not exist", cancelOrder.OrderSymbol)
		return false, [utils.AddressLength]byte{}
	}
	order, ok := tokenState.openOrders[cancelOrder.OrderID]
	if !ok {
		utils.Log("Get Cancel Address failed as order %v is not open", cancelOrder.OrderID)
		return false, [utils.AddressLength]byte{}
	}
	utils.Log("Cancel Order added to consensus state %v", cancelOrder)

	return true, order.SellerAddress
}

func (state *ConsensusState) RollbackCancelOrder(cancelOrder txs.CancelOrder) {
	tokenState := state.tokenStates[cancelOrder.OrderSymbol]
	deletedOrder, _ := tokenState.deletedOrders[cancelOrder.OrderID]

	delete(tokenState.deletedOrders, cancelOrder.OrderID)

	tokenState.openOrders[cancelOrder.OrderID] = deletedOrder
	tokenState.orderUpdatesCount[cancelOrder.OrderID]--

	tokenState.unclaimedFunds[deletedOrder.SellerAddress] -= deletedOrder.AmountToSell
}

func (state *ConsensusState) AddCreateToken(createToken txs.CreateToken) bool {
	_, ok := state.createdTokens[createToken.TokenInfo.Symbol]
	if ok {
		utils.Log("Create Token failed as symbol %v already exists", createToken.TokenInfo.Symbol)
		return false
	}

	utils.Log("Create token added %v", createToken)

	//create a new entry in tokenStates
	state.tokenStates[createToken.TokenInfo.Symbol] = NewConsensusStateToken()
	state.createdTokens[createToken.TokenInfo.Symbol] = createToken.TokenInfo

	tokenState := state.tokenStates[createToken.TokenInfo.Symbol]

	tokenState.unclaimedFunds[createToken.CreatorAddress] = createToken.TokenInfo.TotalSupply

	//create a new blockchain with proper genesis block
	//blockchains.AddTokenChain(createToken)

	return true
}

func (state *ConsensusState) RollbackCreateToken(createToken txs.CreateToken) {
	utils.Log("Create token rolled back %v", createToken)
	delete(state.tokenStates, createToken.TokenInfo.Symbol)
	delete(state.createdTokens, createToken.TokenInfo.Symbol)
	//blockchains.RemoveTokenChain(createToken)
}
