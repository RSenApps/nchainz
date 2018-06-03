package main

/*import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBasicMatch(t *testing.T) {
	LogRed("Testing: basic match")
	matchCh := make(chan Match, 1000)
	matcher := StartMatcher(nil, matchCh)

	matcher.AddOrder(makeOrder(1, "ETH", 1), "USD")
	matcher.AddOrder(makeOrder(1, "USD", 1), "ETH")
	matcher.FindAllMatches()

	match := <-matchCh
	LogRed("Got match %v", match)

	LogRed("Passed: basic match")
	fmt.Println()
}

func TestLargerBuyCleanSplit(t *testing.T) {
	LogRed("Testing: larger buy clean split")
	matchCh := make(chan Match, 1000)
	matcher := StartMatcher(nil, matchCh)

	matcher.AddOrder(makeOrder(200, "ETH", 100), "USD")
	matcher.AddOrder(makeOrder(4, "USD", 10), "ETH")
	matcher.FindAllMatches()

	match := <-matchCh
	LogRed("Got match %v", match)

	assertEqual(t, match.TransferAmt, 10)

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
	matchCh := make(chan Match, 1000)
	matcher := StartMatcher(nil, matchCh)

	matcher.AddOrder(makeOrder(123, "ETH", 71), "USD")
	matcher.AddOrder(makeOrder(367, "USD", 767), "ETH")
	matcher.FindAllMatches()

	match := <-matchCh
	LogRed("Got match %v", match)

	assertEqual(t, match.TransferAmt, 123)

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
	matchCh := make(chan Match, 1000)
	matcher := StartMatcher(nil, matchCh)

	matcher.AddOrder(makeOrder(1, "ETH", 5), "USD")
	matcher.AddOrder(makeOrder(1, "USD", 5), "ETH")
	matcher.FindAllMatches()

	match := <-matchCh
	LogRed("Got match %v", match)

	assertEqual(t, match.TransferAmt, 1)

	ob := matcher.orderbooks["USD"]["ETH"]
	assertEqual(t, ob.QuoteQueue.Len(), 0)
	assertEqual(t, ob.BaseQueue.Len(), 0)

	LogRed("Passed: vanishing orders")
	fmt.Println()
}

func TestSeveralOrders(t *testing.T) {
	LogRed("Testing: several orders")
	matchCh := make(chan Match, 1000)
	matcher := StartMatcher(nil, matchCh)

	matcher.AddOrder(makeOrder(100, "ETH", 10), "USD")
	matcher.AddOrder(makeOrder(100, "ETH", 20), "USD")
	matcher.AddOrder(makeOrder(100, "ETH", 30), "USD")
	matcher.AddOrder(makeOrder(10, "USD", 100), "ETH")
	matcher.AddOrder(makeOrder(20, "USD", 100), "ETH")
	matcher.AddOrder(makeOrder(30, "USD", 100), "ETH")
	matcher.FindAllMatches()

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
	matchCh := make(chan Match, 1000)
	matcher := StartMatcher(nil, matchCh)

	matcher.AddOrder(makeOrder(10, "ETH", 90), "USD")
	matcher.AddOrder(makeOrder(100, "ETH", 1000), "USD")
	matcher.AddOrder(makeOrder(75, "USD", 25), "ETH")
	matcher.AddOrder(makeOrder(150, "USD", 30), "ETH")
	matcher.AddOrder(makeOrder(440, "USD", 55), "ETH")
	matcher.FindAllMatches()

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

func TestCancellation(t *testing.T) {
	LogRed("Testing: cancellation")
	matchCh := make(chan Match, 1000)
	matcher := StartMatcher(nil, matchCh)

	buyOrder := makeOrder(1, "ETH", 10)
	buySellSymbol := "USD"
	sellOrder := makeOrder(1, "USD", 1)
	sellSellSymbol := "ETH"

	matcher.AddOrder(buyOrder, buySellSymbol)
	matcher.RemoveOrder(buyOrder, buySellSymbol)
	matcher.AddOrder(sellOrder, sellSellSymbol)
	matcher.RemoveOrder(sellOrder, sellSellSymbol)

	matcher.AddOrder(makeOrder(1, "ETH", 1), "USD")
	matcher.AddOrder(makeOrder(1, "USD", 1), "ETH")
	matcher.FindAllMatches()

	match := <-matchCh
	LogRed("Got match %v", match)

	ob := matcher.orderbooks["USD"]["ETH"]
	assertEqual(t, len(matchCh), 0)
	assertEqual(t, ob.QuoteQueue.Len(), 0)
	assertEqual(t, ob.BaseQueue.Len(), 0)

	LogRed("Passed: cancellation")
	fmt.Println()
}

func makeOrder(buyAmt uint64, buySymbol string, sellAmt uint64) Order {
	orderId := rand.Uint64()
	signature := []byte{}
	user := ""

	return Order{orderId, buySymbol, sellAmt, buyAmt, user, signature}
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	// Because "a != b" doesn't work
	if fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) {
		msg := fmt.Sprintf("Assertion failed: %v != %v", a, b)
		t.Fatal(msg)
	}
}*/
