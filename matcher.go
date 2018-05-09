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
	Log("Adding order %v to %s", order.ID, GetBookName(buySymbol, sellSymbol))

	orderbook := mr.getOrderbook(buySymbol, sellSymbol)
	orderbook.Add(&order, sellSymbol)

	mr.CheckMatch(orderbook)
}

func (mr *Matcher) RemoveOrder(order Order, sellSymbol string) {
	buySymbol := order.BuySymbol
	Log("Removing order %v from %s", order.ID, GetBookName(buySymbol, sellSymbol))

	orderbook := mr.getOrderbook(buySymbol, sellSymbol)
	orderbook.Cancel(&order, sellSymbol)
}

func (mr *Matcher) AddMatch(match Match) {
	Log("Adding match %v/%v to %s", match.BuyOrderID, match.SellOrderID, GetBookName(match.BuySymbol, match.SellSymbol))

	orderbook := mr.getOrderbook(match.BuySymbol, match.SellSymbol)
	orderbook.ApplyMatch(&match)

	mr.CheckMatch(orderbook)
}

// TODO: vanish amt argument
func (mr *Matcher) RemoveMatch(match Match) {
	Log("Removing match %v/%v from %s", match.BuyOrderID, match.SellOrderID, GetBookName(match.BuySymbol, match.SellSymbol))

	orderbook := mr.getOrderbook(match.BuySymbol, match.SellSymbol)
	orderbook.UnapplyMatch(&match)
}

func (mr *Matcher) CheckMatch(orderbook *Orderbook) {
	found, match := orderbook.FindMatch()
	if found {
		Log("Found match on %s/%s: %v/%v", orderbook.QuoteSymbol, orderbook.BaseSymbol, match.BuyOrderID, match.SellOrderID)

		if mr.bcs != nil {
			tx := GenericTransaction{*match, MATCH}
			mr.bcs.AddTransactionToMempool(tx, MATCH_CHAIN, false)
		} else {
			mr.AddMatch(*match)
			mr.matchCh <- *match
		}
	}
}

func (mr *Matcher) SerializeOrderbook(symbol1, symbol2 string) string {
	orderbook := mr.getOrderbook(symbol1, symbol2)
	return orderbook.Serial()
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

func (mr *Matcher) getOrderbook(symbol1, symbol2 string) *Orderbook {
	if _, exists := mr.symbols[symbol1]; !exists {
		mr.addSymbol(symbol1)
	}
	if _, exists := mr.symbols[symbol2]; !exists {
		mr.addSymbol(symbol2)
	}

	base, quote := GetBaseQuote(symbol1, symbol2)
	return mr.orderbooks[base][quote]
}
