package main

type Matcher struct {
	orderbooks map[string]map[string]*Orderbook
	symbols    map[string]bool
	bcs        *Blockchains
	matchCh    chan Match
	updated    bool
}

type MatcherMsg struct {
	Order      Order
	SellSymbol string
	Cancel     bool
}

func StartMatcher(bcs *Blockchains, matchCh chan Match) (matcher *Matcher) {
	orderbooks := make(map[string]map[string]*Orderbook)
	symbols := make(map[string]bool)

	if bcs != nil {
		Log("Starting order matcher")
		matcher = &Matcher{orderbooks, symbols, bcs, nil, false}

	} else {
		Log("Simulating order matcher")
		matcher = &Matcher{orderbooks, symbols, nil, matchCh, false}
	}

	return matcher
}

func (mr *Matcher) AddOrder(order Order, sellSymbol string) {
	buySymbol := order.BuySymbol

	if _, exists := mr.symbols[buySymbol]; !exists {
		mr.addSymbol(buySymbol)
	}
	if _, exists := mr.symbols[sellSymbol]; !exists {
		mr.addSymbol(sellSymbol)
	}

	Log("Adding tx %v to %s", order.ID, GetBookName(buySymbol, sellSymbol))
	orderbook := mr.getOrderbook(buySymbol, sellSymbol)
	orderbook.Add(&order, sellSymbol)

	mr.Match(orderbook)
}

func (mr *Matcher) RemoveOrder(order Order, sellSymbol string) {
	buySymbol := order.BuySymbol
	orderbook := mr.getOrderbook(buySymbol, sellSymbol)
	orderbook.Cancel(&order, sellSymbol)

	mr.Match(orderbook)
}

func (mr *Matcher) Match(orderbook *Orderbook) {
	found, match := orderbook.Match()
	if found {
		Log("Found match on %s/%s: %v/%v", orderbook.QuoteSymbol, orderbook.BaseSymbol, match.BuyOrderID, match.SellOrderID)
		mr.returnMatch(match)

		mr.Match(orderbook)
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
