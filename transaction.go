package main

import "log"

//import "log"

func NewTransfer(from, to string, amount uint64, bc *Blockchain) Transfer {
	//TODO: need to take a lock as otherwise this is a race
	if amount < bc.GetBalance(from) {
		log.Panic("Error Not enough funds")
	}
	return Transfer{
		Amount:      amount,
		FromAddress: []byte(from),
		ToAddress:   []byte(to),
		Signature:   nil,
	}
}

func NewOrder(buyTokenType TokenType, amountToSell uint64, amountToBuy uint64, sellerAddress string, bc *Blockchain) Order {
	//TODO: need to take a lock as otherwise this is a race
	if amountToBuy < bc.GetBalance(sellerAddress) {
		log.Panic("Error Not enough funds")
	}

	return Order{
		BuyTokenType:  buyTokenType,
		AmountToSell:  amountToSell,
		AmountToBuy:   amountToBuy,
		SellerAddress: []byte(sellerAddress),
		Signature:     nil,
	}
}

func NewCancel(blockIndex uint64, blockOffset uint8, address string, bc *Blockchain) Cancel {
	//TODO: validate block exists and is owned by address
	return Cancel{
		BlockIndex:  blockIndex,
		BlockOffset: blockOffset,
		BlockHash:   nil, //TODO
		Address:     []byte(address),
		Signature:   nil,
	}
}

//TODO: NewMatch (do validation of match