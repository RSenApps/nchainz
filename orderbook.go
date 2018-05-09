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

func (ob *Orderbook) FindMatch() (found bool, match *Match) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	Log("Checking for matches on %s/%s", ob.QuoteSymbol, ob.BaseSymbol)

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
	return
}

func (ob *Orderbook) ApplyMatch(match *Match) {
	buyOrder, _ := ob.QuoteQueue.GetOrder(match.BuyOrderID)
	sellOrder, _ := ob.BaseQueue.GetOrder(match.SellOrderID)

	if match.TransferAmt < buyOrder.AmountToBuy && match.BuyerLoss < buyOrder.AmountToSell {
		buyOrder.AmountToBuy -= match.TransferAmt
		buyOrder.AmountToSell -= match.BuyerLoss
		ob.QuoteQueue.FixPrice(buyOrder.ID)
	} else {
		ob.QuoteQueue.Remove(buyOrder.ID)
	}

	if match.TransferAmt < sellOrder.AmountToSell && match.SellerGain < sellOrder.AmountToBuy {
		sellOrder.AmountToSell -= match.TransferAmt
		sellOrder.AmountToBuy -= match.SellerGain
		ob.BaseQueue.FixPrice(sellOrder.ID)
	} else {
		ob.BaseQueue.Remove(sellOrder.ID)
	}

	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.QuoteQueue)
	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.BaseQueue)
}

func (ob *Orderbook) UnapplyMatch(match *Match, order1 *Order, order2 *Order) {
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

	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.QuoteQueue)
	Log("%s %v", GetBookName(ob.BaseSymbol, ob.QuoteSymbol), ob.BaseQueue)
}

func (ob *Orderbook) Serial() string {
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
