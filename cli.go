package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

type CLI struct {
}

var helpMessage = `N Chainz: a high performance, decentralized cryptocurrency exchange.

Usage: nchainz COMMAND [ARGS]

The commands are:

Account management
	createwallet
		Create a wallet with a pair of keys
	getbalance ADDRESS SYMBOL
		Get the balance for an address
	printaddresses
		Print all adddreses in wallet file

Creating transactions
	order BUY_AMT BUY_SYMBOL SELL_AMT SELL_SYMBOL ADDRESS
		Create an ORDER transaction
	transfer AMT SYMBOL FROM TO
		Create a TRANSFER transaction
	cancel SYMBOL ORDER_ID
		Create a CANCEL_ORDER transaction
	claim AMT SYMBOL ADDRESS
		Create a CLAIM_FUNDS transaction
	create SYMBOL SUPPLY DECIMALS ADDRESS
		Create a CREATE_TOKEN transaction

Running a node or miner
	node HOSTNAME:PORT
		Start up a full node providing your hostname on the given port
	printchain DB
		Prints all the blocks in the blockchain
`

func (cli *CLI) Run() {
	if len(os.Args) < 2 {
		cli.printHelpAndExit()
	}

	command := os.Args[1]

	switch command {
	case "createwallet":
		cli.createWallet()

	case "getbalance":
		address := cli.getString(0)
		symbol := cli.getString(1)
		client := cli.getClient()
		client.GetBalance(address, symbol)

	case "printaddresses":
		cli.printAddresses()

	case "order":
		buyAmt := cli.getUint(0)
		buySymbol := cli.getString(1)
		sellAmt := cli.getUint(2)
		sellSymbol := cli.getString(3)
		seller := cli.getString(4)

		client := cli.getClient()
		client.Order(buyAmt, buySymbol, sellAmt, sellSymbol, seller)

	case "transfer":
		amt := cli.getUint(0)
		symbol := cli.getString(1)
		from := cli.getString(2)
		to := cli.getString(3)

		client := cli.getClient()
		client.Transfer(amt, symbol, from, to)

	case "cancel":
		orderSymbol := cli.getString(0)
		orderId := cli.getUint(1)

		client := cli.getClient()
		client.Cancel(orderSymbol, orderId)

	case "claim":
		amt := cli.getUint(0)
		symbol := cli.getString(1)
		address := cli.getString(2)

		client := cli.getClient()
		client.Claim(amt, symbol, address)

	case "create":
		symbol := cli.getString(0)
		supply := cli.getUint(1)
		decimals := uint8(cli.getUint(2))
		address := cli.getString(3)

		client := cli.getClient()
		client.Create(symbol, supply, decimals, address)

	case "node":
		socket := cli.getString(0)

		StartNode(socket)

	case "printchain":
		db := cli.getString(0)
		symbol := cli.getString(1)
		cli.printChain(db, symbol)

	default:
		cli.printHelpAndExit()
	}
}

func (cli *CLI) getString(index int) string {
	if len(os.Args)-3 < index {
		cli.printHelpAndExit()
	}

	return os.Args[index+2]
}

func (cli *CLI) getUint(index int) uint64 {
	s := cli.getString(index)
	i, err := strconv.ParseUint(s, 10, 64)

	if err != nil {
		cli.printHelpAndExit()
	}

	return uint64(i)
}

func (cli *CLI) printHelpAndExit() {
	fmt.Print(helpMessage)
	os.Exit(1)
}

func (cli *CLI) getClient() *Client {
	client, err := NewClient()

	if err != nil {
		log.Printf(err.Error())
		os.Exit(1)
	}

	return client
}

///////////////////////////////////////////////////////
// CLI commands that really should live somewhere else

func (cli *CLI) getBalance(db string, address string) {
	bcs := CreateNewBlockchains(db+".db", false)
	result, ok := bcs.GetBalance(NATIVE_CHAIN, address)
	if !ok {
		fmt.Println("Address not found")
	}
	fmt.Printf("Balance: %v\n", result)
}

func (cli *CLI) printChain(db string, symbol string) {
	bcs := CreateNewBlockchains(db+".db", false)
	bc := bcs.chains[symbol]
	bci := bc.Iterator()
	fmt.Println(symbol)
	fmt.Printf("Height: %d tiphash: %x\n", bc.height, bc.tipHash)
	block, err := bci.Prev()
	isGenesis := true
	for err == nil {
		fmt.Printf("Prev Hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		if !isGenesis {
			fmt.Printf("Validated Proof of Work: %s\n", strconv.FormatBool(pow.Validate()))
			isGenesis = false
		}
		fmt.Println("-------------------------------")
		block, err = bci.Prev()
	}
}

func (cli *CLI) printAddresses() {
	ws := NewWalletStore()
	addresses := ws.GetAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}
}

func (cli *CLI) createWallet() {
	ws := NewWalletStore()
	address := ws.AddWallet()
	ws.Persist()

	fmt.Printf("New wallet's address: %s\n", address)
}
