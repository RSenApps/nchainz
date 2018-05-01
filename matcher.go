package main

import (
	"time"
)

type Matcher struct {
	orderbooks map[string]map[string]*Orderbook
	symbols    map[string]bool
	orderCh    chan OrderMsg
	matchCh    chan MatchMsg
	updated    bool
}

type OrderMsg struct {
	Order      Order
	SellSymbol string
}

type MatchMsg struct {
	Match  Match
	Spread uint64
}

func NewMatcher(orderCh chan OrderMsg, matchCh chan MatchMsg) *Matcher {
	orderbooks := make(map[string]map[string]*Orderbook)
	symbols := make(map[string]bool)

	matcher := &Matcher{orderbooks, symbols, orderCh, matchCh, false}
	go matcher.matchLoop()
	return matcher
}

func (mr *Matcher) matchLoop() {
	for {
		select {
		case orderMsg := <-mr.orderCh:
			sellSymbol := orderMsg.SellSymbol
			buySymbol := orderMsg.Order.BuySymbol

			if _, exists := mr.symbols[sellSymbol]; !exists {
				mr.addSymbol(sellSymbol)
			}
			if _, exists := mr.symbols[buySymbol]; !exists {
				mr.addSymbol(buySymbol)
			}

			orderbook := mr.getOrderbook(buySymbol, sellSymbol)
			orderbook.Add(&orderMsg.Order, sellSymbol)

			mr.updated = true

		default:
			if mr.updated {
				foundAny := false

			OuterLoop:
				for symbol1 := range mr.symbols {
					for symbol2 := range mr.symbols {
						if symbol1 == symbol2 {
							continue
						}

						orderbook := mr.getOrderbook(symbol1, symbol2)
						foundHere, match, spread := orderbook.Match()

						if foundHere {
							matchMsg := MatchMsg{*match, spread}
							mr.matchCh <- matchMsg

							foundAny = true
							break OuterLoop
						}
					}
				}

				if !foundAny {
					mr.updated = false
				}

			} else {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}
}

func (mr *Matcher) addSymbol(newSymbol string) {
	for oldSymbol := range mr.symbols {
		base, quote := GetBaseQuote(newSymbol, oldSymbol)
		mr.orderbooks[base][quote] = NewOrderbook(base, quote)
	}

	mr.symbols[newSymbol] = true
}

func (mr *Matcher) getOrderbook(symbol1, symbol2 string) *Orderbook {
	base, quote := GetBaseQuote(symbol1, symbol2)
	return mr.orderbooks[base][quote]
}
