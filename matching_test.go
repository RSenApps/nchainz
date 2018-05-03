package main

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBasicMatch(t *testing.T) {
	LogRed("Testing: basic match")
	txCh := make(chan MatcherMsg, 1000)
	matchCh := make(chan Match, 1000)

	go func(txCh chan MatcherMsg) {
		txCh <- makeMsg(1, "ETH", 1, "USD")
		txCh <- makeMsg(1, "USD", 1, "ETH")
	}(txCh)

	StartMatcher(txCh, nil, matchCh)
	match := <-matchCh

	LogRed("Got match %v", match)
	LogRed("Passed: basic match")
	fmt.Println()
}

func TestLargerBuyCleanSplit(t *testing.T) {
	LogRed("Testing: larger buy clean split")
	txCh := make(chan MatcherMsg, 1000)
	matchCh := make(chan Match, 1000)

	go func(txCh chan MatcherMsg) {
		txCh <- makeMsg(200, "ETH", 100, "USD")
		txCh <- makeMsg(4, "USD", 10, "ETH")
	}(txCh)

	matcher := StartMatcher(txCh, nil, matchCh)
	match := <-matchCh
	LogRed("Got match %v", match)

	assertEqual(t, match.AmountSold, 10)

	ob := matcher.orderbooks["USD"]["ETH"]
	assertEqual(t, ob.QuoteQueue.Len(), 1)
	assertEqual(t, ob.QuoteQueue.Items[0].order.AmountToBuy, 190)
	assertEqual(t, ob.QuoteQueue.Items[0].order.AmountToSell, 95)
	assertEqual(t, ob.BaseQueue.Len(), 0)

	LogRed("Passed: larger buy clean split")
	fmt.Println()
}

func TestLargerSellMessySplit(t *testing.T) {
	LogRed("Testing: larger sell messy split")
	txCh := make(chan MatcherMsg, 1000)
	matchCh := make(chan Match, 1000)

	go func(txCh chan MatcherMsg) {
		txCh <- makeMsg(123, "ETH", 71, "USD")
		txCh <- makeMsg(367, "USD", 767, "ETH")
	}(txCh)

	matcher := StartMatcher(txCh, nil, matchCh)
	match := <-matchCh
	LogRed("Got match %v", match)

	assertEqual(t, match.AmountSold, 123)

	ob := matcher.orderbooks["USD"]["ETH"]
	assertEqual(t, ob.QuoteQueue.Len(), 0)
	assertEqual(t, ob.BaseQueue.Len(), 1)
	assertEqual(t, ob.BaseQueue.Items[0].order.AmountToBuy, 308)
	assertEqual(t, ob.BaseQueue.Items[0].order.AmountToSell, 644)

	LogRed("Passed: larger sell messy split")
	fmt.Println()
}

func TestVanishingOrders(t *testing.T) {
	LogRed("Testing: vanishing orders")
	txCh := make(chan MatcherMsg, 1000)
	matchCh := make(chan Match, 1000)

	go func(txCh chan MatcherMsg) {
		txCh <- makeMsg(1, "ETH", 5, "USD")
		txCh <- makeMsg(1, "USD", 5, "ETH")
	}(txCh)

	matcher := StartMatcher(txCh, nil, matchCh)
	match := <-matchCh
	LogRed("Got match %v", match)

	assertEqual(t, match.AmountSold, 1)

	ob := matcher.orderbooks["USD"]["ETH"]
	assertEqual(t, ob.QuoteQueue.Len(), 0)
	assertEqual(t, ob.BaseQueue.Len(), 0)

	LogRed("Passed: vanishing orders")
	fmt.Println()
}

func TestSeveralOrders(t *testing.T) {
	LogRed("Testing: several orders")
	txCh := make(chan MatcherMsg, 1000)
	matchCh := make(chan Match, 1000)

	go func(txCh chan MatcherMsg) {
		txCh <- makeMsg(100, "ETH", 10, "USD")
		txCh <- makeMsg(100, "ETH", 20, "USD")
		txCh <- makeMsg(100, "ETH", 30, "USD")
		txCh <- makeMsg(10, "USD", 100, "ETH")
		txCh <- makeMsg(20, "USD", 100, "ETH")
		txCh <- makeMsg(30, "USD", 100, "ETH")
	}(txCh)

	matcher := StartMatcher(txCh, nil, matchCh)
	match1 := <-matchCh
	LogRed("Got match %v", match1)
	match2 := <-matchCh
	LogRed("Got match %v", match2)

	ob := matcher.orderbooks["USD"]["ETH"]
	assertEqual(t, ob.QuoteQueue.Len(), 1)
	assertEqual(t, ob.QuoteQueue.Items[0].order.AmountToSell, 10)
	assertEqual(t, ob.BaseQueue.Len(), 1)
	assertEqual(t, ob.BaseQueue.Items[0].order.AmountToBuy, 30)

	LogRed("Passed: several orders")
	fmt.Println()
}

func TestMultifill(t *testing.T) {
	LogRed("Testing: multifill")
	txCh := make(chan MatcherMsg, 1000)
	matchCh := make(chan Match, 1000)

	go func(txCh chan MatcherMsg) {
		txCh <- makeMsg(10, "ETH", 90, "USD")
		txCh <- makeMsg(100, "ETH", 1000, "USD")
		txCh <- makeMsg(75, "USD", 25, "ETH")
		txCh <- makeMsg(150, "USD", 30, "ETH")
		txCh <- makeMsg(440, "USD", 55, "ETH")
	}(txCh)

	matcher := StartMatcher(txCh, nil, matchCh)
	match1 := <-matchCh
	LogRed("Got match %v", match1)
	match2 := <-matchCh
	LogRed("Got match %v", match2)
	match3 := <-matchCh
	LogRed("Got match %v", match3)
	match4 := <-matchCh
	LogRed("Got match %v", match4)

	ob := matcher.orderbooks["USD"]["ETH"]
	assertEqual(t, ob.QuoteQueue.Len(), 0)
	assertEqual(t, ob.BaseQueue.Len(), 0)

	LogRed("Passed: multifill")
	fmt.Println()
}

func makeMsg(buyAmt uint64, buySymbol string, sellAmt uint64, sellSymbol string) MatcherMsg {
	orderId := rand.Uint64()
	signature := []byte{}
	user := ""

	order := Order{orderId, buySymbol, sellAmt, buyAmt, user, signature}
	msg := MatcherMsg{order, sellSymbol}
	return msg
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	// Because "a != b" doesn't work
	if fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) {
		msg := fmt.Sprintf("Assertion failed: %v != %v", a, b)
		t.Fatal(msg)
	}
}
