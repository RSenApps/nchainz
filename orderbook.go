package main

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
)

type Orderbook struct {
	BaseQueue   *OrderQueue
	BaseSymbol  string
	QuoteQueue  *OrderQueue
	QuoteSymbol string
	Dirty       bool
	mu          *sync.RWMutex
}

type MatchDiscovery struct {
	Match      *Match
	QuoteOrder *Order
	BaseOrder  *Order
}

func NewOrderbook(symbol1, symbol2 string) *Orderbook {
	baseSymbol, quoteSymbol := GetBaseQuote(symbol1, symbol2)
	baseQueue := NewOrderQueue(BASE)
	quoteQueue := NewOrderQueue(QUOTE)
	dirty := false
	mu := &sync.RWMutex{}

	return &Orderbook{baseQueue, baseSymbol, quoteQueue, quoteSymbol, dirty, mu}
}

func (ob *Orderbook) Add(order *Order, sellSymbol string) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.Dirty = true

	if sellSymbol == ob.BaseSymbol {
		ob.QuoteQueue.Enq(order)
	} else if sellSymbol == ob.QuoteSymbol {
		ob.BaseQueue.Enq(order)
	}

	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.QuoteQueue)
	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.BaseQueue)
}

func (ob *Orderbook) Cancel(order *Order, sellSymbol string) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if sellSymbol == ob.BaseSymbol {
		ob.QuoteQueue.Remove(order.ID)
	} else if sellSymbol == ob.QuoteSymbol {
		ob.BaseQueue.Remove(order.ID)
	}

	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.QuoteQueue)
	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.BaseQueue)
}

func (ob *Orderbook) ApplyMatch(match *Match) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.Dirty = true

	ob.applyMatchUnlocked(match)
}

func (ob *Orderbook) applyMatchUnlocked(match *Match) {
	buyOrder, _ := ob.QuoteQueue.GetOrder(match.BuyOrderID)
	sellOrder, _ := ob.BaseQueue.GetOrder(match.SellOrderID)

	buyOrder.AmountToBuy -= match.TransferAmt
	buyOrder.AmountToSell -= match.BuyerLoss
	ob.QuoteQueue.FixPrice(buyOrder.ID)

	if buyOrder.AmountToBuy == 0 {
		Log("Removing buy order %v", buyOrder.ID)
		ob.QuoteQueue.Remove(buyOrder.ID)
	}

	sellOrder.AmountToSell -= match.TransferAmt
	sellOrder.AmountToBuy -= match.SellerGain
	ob.BaseQueue.FixPrice(sellOrder.ID)

	if sellOrder.AmountToBuy == 0 {
		Log("Removing sell order %v", sellOrder.ID)
		ob.BaseQueue.Remove(sellOrder.ID)
	}

	Log("Result of applying match buy order %v sell order %v", buyOrder, sellOrder)

	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.QuoteQueue)
	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.BaseQueue)
}

func (ob *Orderbook) UnapplyMatch(match *Match, order1 *Order, order2 *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.Dirty = true

	ob.unapplyMatchUnlocked(match, order1, order2)
}

func (ob *Orderbook) unapplyMatchUnlocked(match *Match, order1 *Order, order2 *Order) {
	var backupBuy, backupSell *Order

	if order1.BuySymbol == ob.QuoteSymbol {
		backupBuy = order1
		backupSell = order2
	} else {
		backupBuy = order2
		backupSell = order1
	}

	buyOrder, buyExists := ob.QuoteQueue.GetOrder(match.BuyOrderID)
	sellOrder, sellExists := ob.BaseQueue.GetOrder(match.SellOrderID)

	if !buyExists {
		buyOrder = backupBuy
		ob.QuoteQueue.Enq(buyOrder)
	}

	if !sellExists {
		sellOrder = backupSell
		ob.BaseQueue.Enq(sellOrder)
	}

	buyOrder.AmountToBuy += match.TransferAmt
	buyOrder.AmountToSell += match.BuyerLoss
	sellOrder.AmountToBuy += match.SellerGain
	sellOrder.AmountToSell += match.TransferAmt

	ob.QuoteQueue.FixPrice(buyOrder.ID)
	ob.BaseQueue.FixPrice(sellOrder.ID)

	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.QuoteQueue)
	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.BaseQueue)
}

func (ob *Orderbook) FindMatch() (found bool, match *Match) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	Log("Checking for matches on %s/%s", ob.QuoteSymbol, ob.BaseSymbol)
	found, match, _, _ = ob.findMatchUnlocked()
	return
}

func (ob *Orderbook) findMatchUnlocked() (found bool, match *Match, quoteOrder *Order, baseOrder *Order) {
	buyOrder, buyPrice, buyErr := ob.QuoteQueue.Peek()
	sellOrder, sellPrice, sellErr := ob.BaseQueue.Peek()

	if buyErr != nil {
		Log("No buy orders on %s/%s", ob.QuoteSymbol, ob.BaseSymbol)
		return
	}
	if sellErr != nil {
		Log("No sell orders on %s/%s", ob.QuoteSymbol, ob.BaseSymbol)
		return
	}

	Log("%f bid (%v), %f ask (%v)", buyPrice, buyOrder.ID, sellPrice, sellOrder.ID)

	if buyPrice < sellPrice {
		return
	}

	var transferAmt, buyerBaseLoss, sellerBaseGain uint64

	if buyOrder.AmountToBuy > sellOrder.AmountToSell {
		transferAmt = sellOrder.AmountToSell
		sellerBaseGain = sellOrder.AmountToBuy
		buyerBaseLoss = uint64(math.Floor(buyPrice * float64(transferAmt)))

	} else { // buyOrder.AmountToBuy <= sellOrder.AmountToSell
		transferAmt = buyOrder.AmountToBuy
		buyerBaseLoss = buyOrder.AmountToSell
		sellerBaseGain = uint64(math.Ceil(sellPrice * float64(transferAmt)))
	}

	if sellerBaseGain > buyerBaseLoss {
		return
	}

	found = true
	id := rand.Uint64()
	match = &Match{id, ob.QuoteSymbol, sellOrder.ID, sellerBaseGain, ob.BaseSymbol, buyOrder.ID, buyerBaseLoss, uint64(transferAmt)}
	quoteOrder = buyOrder
	baseOrder = sellOrder
	return
}

func (ob *Orderbook) FindAllMatches() []*Match {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	discoveries := make([]*MatchDiscovery, 0)
	matches := make([]*Match, 0)

	for {
		found, match, quoteOrder, baseOrder := ob.findMatchUnlocked()

		if found {
			discovery := &MatchDiscovery{match, quoteOrder, baseOrder}
			discoveries = append(discoveries, discovery)
			matches = append(matches, match)

			ob.applyMatchUnlocked(match)

		} else {
			break
		}
	}

	for i := len(discoveries) - 1; i >= 0; i-- {
		discovery := discoveries[i]
		ob.unapplyMatchUnlocked(discovery.Match, discovery.QuoteOrder, discovery.BaseOrder)
	}

	Log("Found %v matches on %s", len(matches), GetBookName(ob.BaseSymbol, ob.QuoteSymbol))
	return matches
}

func (ob *Orderbook) Serial() string {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	_, bid, bidErr := ob.QuoteQueue.Peek()
	_, ask, askErr := ob.BaseQueue.Peek()

	marketPrice := (bid + ask) / 2

	if bidErr != nil {
		marketPrice = ask
	} else if askErr != nil {
		marketPrice = bid
	}

	return fmt.Sprintf("%v\n%v\n%v\n%v\n%v", marketPrice, ob.QuoteSymbol, ob.BaseSymbol, ob.QuoteQueue.Serial(), ob.BaseQueue.Serial())
}

func GetBaseQuote(symbol1, symbol2 string) (base, quote string) {
	if symbol1 > symbol2 {
		return symbol1, symbol2
	}
	return symbol2, symbol1
}

func GetBookName(symbol1, symbol2 string) string {
	if symbol1 < symbol2 {
		return fmt.Sprintf("%v/%v", symbol1, symbol2)
	}
	return fmt.Sprintf("%v/%v", symbol2, symbol1)
}
