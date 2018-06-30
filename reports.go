package main

import (
	"bytes"
	"fmt"
)

// NChainz Execution Tags
// <1> Spec Version Number
// <2> Event
// <3> Execution ID
// <4> Buyer Address
// <5> Seller Address
// <6> Buy Symbol
// <7> Sell Symbol
// <8> Buy Order ID
// <9> Sell Order ID
// <10> Transfer Amount
// <11> Buyer Loss
// <12> Seller Gain

const executionReportVersion = 1

type Execution struct {
	Match         *Match
	BuyerAddress  [addressLength]byte
	SellerAddress [addressLength]byte
}

func LogExecutionReport(e *Execution) {
	var b bytes.Buffer

	b.WriteString(fmt.Sprintf("1=%v\x01", executionReportVersion))
	b.WriteString("2=exec\x01")
	b.WriteString(fmt.Sprintf("3=%v\x01", e.Match.MatchID))
	b.WriteString(fmt.Sprintf("4=%s\x01", KeyToString(e.BuyerAddress)))
	b.WriteString(fmt.Sprintf("5=%s\x01", KeyToString(e.SellerAddress)))
	b.WriteString(fmt.Sprintf("6=%s\x01", e.Match.BuySymbol))
	b.WriteString(fmt.Sprintf("7=%s\x01", e.Match.SellSymbol))
	b.WriteString(fmt.Sprintf("8=%v\x01", e.Match.BuyOrderID))
	b.WriteString(fmt.Sprintf("9=%v\x01", e.Match.SellOrderID))
	b.WriteString(fmt.Sprintf("10=%v\x01", e.Match.TransferAmt))
	b.WriteString(fmt.Sprintf("11=%v\x01", e.Match.BuyerLoss))
	b.WriteString(fmt.Sprintf("12=%v\x01", e.Match.SellerGain))

	report := b.String()
	Log(report)
}
