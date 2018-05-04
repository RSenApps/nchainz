package main

import (
	"time"
)

type Matcher struct {
	orderbooks map[string]map[string]*Orderbook
	symbols    map[string]bool
	txCh       chan MatcherMsg
	bcs        *Blockchains
	matchCh    chan Match
	updated    bool
}

type MatcherMsg struct {
	Order      Order
	SellSymbol string
	Cancel     bool
}

func StartMatcher(txCh chan MatcherMsg, bcs *Blockchains, matchCh chan Match) (matcher *Matcher) {
	orderbooks := make(map[string]map[string]*Orderbook)
	symbols := make(map[string]bool)

	if bcs != nil {
		Log("Starting order matcher")
		matcher = &Matcher{orderbooks, symbols, txCh, bcs, nil, false}
		go matcher.matchLoop()

	} else {
		Log("Simulating order matcher")
		matcher = &Matcher{orderbooks, symbols, txCh, nil, matchCh, false}
		go matcher.matchLoop()
	}

	return matcher
}

func (mr *Matcher) matchLoop() {
	for {
		select {
		case orderMsg := <-mr.txCh:
			buySymbol := orderMsg.Order.BuySymbol
			sellSymbol := orderMsg.SellSymbol

			if _, exists := mr.symbols[buySymbol]; !exists {
				mr.addSymbol(buySymbol)
			}
			if _, exists := mr.symbols[sellSymbol]; !exists {
				mr.addSymbol(sellSymbol)
			}

			if orderMsg.Cancel {
				Log("Canceling tx %v on %s", orderMsg.Order.ID, GetBookName(buySymbol, sellSymbol))
				orderbook := mr.getOrderbook(buySymbol, sellSymbol)
				orderbook.Cancel(&orderMsg.Order, sellSymbol)

			} else {
				Log("Adding tx %v to %s", orderMsg.Order.ID, GetBookName(buySymbol, sellSymbol))
				orderbook := mr.getOrderbook(buySymbol, sellSymbol)
				orderbook.Add(&orderMsg.Order, sellSymbol)
			}

			mr.updated = true

			Log("Done adding tx %v to %s", orderMsg.Order.ID, GetBookName(buySymbol, sellSymbol))

		default:
			if mr.updated {
				Log("Looking for matches")

				foundAny := false

			OuterLoop:
				for symbol1 := range mr.symbols {
					for symbol2 := range mr.symbols {
						if symbol1 >= symbol2 {
							continue
						}

						orderbook := mr.getOrderbook(symbol1, symbol2)
						foundHere, match := orderbook.Match()

						if foundHere {
							Log("Found match on %s/%s: %v/%v", orderbook.QuoteSymbol, orderbook.BaseSymbol, match.BuyOrderID, match.SellOrderID)

							mr.returnMatch(match)

							foundAny = true
							break OuterLoop
						}
					}
				}

				if !foundAny {
					Log("No matches found")
					mr.updated = false
				}

			} else {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}
}

func (mr *Matcher) addSymbol(newSymbol string) {
	Log("Adding symbol %s to matcher", newSymbol)

	mr.orderbooks[newSymbol] = make(map[string]*Orderbook)

	for oldSymbol := range mr.symbols {
		base, quote := GetBaseQuote(newSymbol, oldSymbol)
		mr.orderbooks[base][quote] = NewOrderbook(base, quote)
	}

	mr.symbols[newSymbol] = true
}

func (mr *Matcher) returnMatch(match *Match) {
	if mr.bcs != nil {
		tx := GenericTransaction{*match, MATCH}
		mr.bcs.AddTransactionToMempool(tx, MATCH_CHAIN)
	} else {
		mr.matchCh <- *match
	}
}

func (mr *Matcher) getOrderbook(symbol1, symbol2 string) *Orderbook {
	base, quote := GetBaseQuote(symbol1, symbol2)
	return mr.orderbooks[base][quote]
}
