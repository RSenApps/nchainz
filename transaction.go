package main

import "log"

func NewTransfer(symbol string, from, to string, amount uint64, blockchains *Blockchains) Transfer {
	//TODO: need to take a lock as otherwise this is a race
	if amount < blockchains.GetBalance(symbol, from) {
		log.Panic("Error Not enough funds")
	}
	return Transfer{
		Amount:      amount,
		FromAddress: from,
		ToAddress:   to,
		Signature:   nil,
	}
}

func NewOrder(sellTokenType string, buyTokenType string, amountToSell uint64, amountToBuy uint64, sellerAddress string, blockchains *Blockchains) Order {
	//TODO: need to take a lock as otherwise this is a race
	if amountToBuy < blockchains.GetBalance(sellTokenType, sellerAddress) {
		log.Panic("Error Not enough funds")
	}

	return Order{
		BuyTokenType:  buyTokenType,
		AmountToSell:  amountToSell,
		AmountToBuy:   amountToBuy,
		SellerAddress: sellerAddress,
		Signature:     nil,
	}
}

func NewCancelMatch(cancelMatchID uint64, orderID uint64, blockchains *Blockchain) CancelMatch {
	//TODO: validate orderid exists and is owned by address
	return CancelMatch{
		CancelMatchID: cancelMatchID,
		OrderID:       orderID,
		Signature:     nil,
	}
}

//TODO: NewMatch (do validation of match)
