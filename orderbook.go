package main

import (
	"math"
	"sync"
)

type Orderbook struct {
	BaseQueue   *OrderQueue
	BaseSymbol  string
	QuoteQueue  *OrderQueue
	QuoteSymbol string
	mu          *sync.Mutex
}

func NewOrderbook(symbol1, symbol2 string) *Orderbook {
	baseSymbol, quoteSymbol := GetBaseQuote(symbol1, symbol2)
	baseQueue := NewOrderQueue(BASE)
	quoteQueue := NewOrderQueue(QUOTE)
	mu := &sync.Mutex{}

	return &Orderbook{baseQueue, baseSymbol, quoteQueue, quoteSymbol, mu}
}

func (ob *Orderbook) Add(order *Order, sellSymbol string) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if sellSymbol == ob.BaseSymbol {
		ob.BaseQueue.Enq(order)
	} else if sellSymbol == ob.QuoteSymbol {
		ob.QuoteQueue.Enq(order)
	}
}

func (ob *Orderbook) Match() (found bool, match *Match, spread uint64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	buyOrder, buyPrice, buyErr := ob.QuoteQueue.Peek()
	sellOrder, sellPrice, sellErr := ob.BaseQueue.Peek()

	if buyErr != nil || sellErr != nil || buyPrice < sellPrice {
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
	match = &Match{0, ob.BaseSymbol, sellOrder.ID, ob.QuoteSymbol, buyOrder.ID, uint64(transferAmt)}
	spread = buyerBaseLoss - sellerBaseGain

	ob.QuoteQueue.Deq()
	if transferAmt < buyOrder.AmountToBuy && buyerBaseLoss < buyOrder.AmountToSell {
		buyOrder.AmountToBuy -= transferAmt
		buyOrder.AmountToSell -= buyerBaseLoss
		ob.QuoteQueue.Enq(buyOrder)
	}

	ob.BaseQueue.Deq()
	if transferAmt < sellOrder.AmountToSell && sellerBaseGain < sellOrder.AmountToBuy {
		sellOrder.AmountToSell -= transferAmt
		sellOrder.AmountToBuy -= sellerBaseGain
		ob.BaseQueue.Enq(sellOrder)
	}

	return
}

func GetBaseQuote(symbol1, symbol2 string) (base, quote string) {
	if symbol1 < symbol2 {
		return symbol1, symbol2
	}
	return symbol2, symbol1
}
