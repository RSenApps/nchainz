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
	getbalance ADDRESS
		Get the balance for an address
	printaddresses
		Print all adddreses in wallet file

Creating transactions
	order BUY_AMT BUY_SYMBOL SELL_AMT SELL_SYMBOL ADDRESS NODE
		Create an ORDER transaction and send it to the given node
	transfer AMT SYMBOL FROM TO NODE
		Create a TRANSFER transaction and send it to the given node
	cancel SYMBOL ORDER_ID NODE
		Create a CANCEL_ORDER transaction and send it to the given node
	claim AMT SYMBOL ADDRESS NODE
		Create a CLAIM_FUNDS transaction and send it to the given node
	create SYMBOL SUPPLY DECIMALS ADDRESS NODE
		Create a CREATE_TOKEN transaction and send it to the given node

Running a node or miner
	node PORT SEED_IP
		Start up a full node on the given port and connect to the given peer
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
		db := cli.getString(0)
		address := cli.getString(1)
		cli.getBalance(db, address)

	case "printaddresses":
		cli.printAddresses()

	case "order":
		buyAmt := cli.getUint(0)
		buySymbol := cli.getString(1)
		sellAmt := cli.getUint(2)
		sellSymbol := cli.getString(3)
		seller := cli.getString(4)
		serverIp := cli.getString(5)

		client := cli.getClient(serverIp)
		client.Order(buyAmt, buySymbol, sellAmt, sellSymbol, seller)

	case "transfer":
		amt := cli.getUint(0)
		symbol := cli.getString(1)
		from := cli.getString(2)
		to := cli.getString(3)
		serverIp := cli.getString(4)

		client := cli.getClient(serverIp)
		client.Transfer(amt, symbol, from, to)

	case "cancel":
		orderSymbol := cli.getString(0)
		orderId := cli.getUint(1)
		serverIp := cli.getString(2)

		client := cli.getClient(serverIp)
		client.Cancel(orderSymbol, orderId)

	case "claim":
		amt := cli.getUint(0)
		symbol := cli.getString(1)
		address := cli.getString(2)
		serverIp := cli.getString(3)

		client := cli.getClient(serverIp)
		client.Claim(amt, symbol, address)

	case "create":
		symbol := cli.getString(0)
		supply := cli.getUint(1)
		decimals := uint8(cli.getUint(2))
		address := cli.getString(3)
		serverIp := cli.getString(4)

		client := cli.getClient(serverIp)
		client.Create(symbol, supply, decimals, address)

	case "node":
		port := cli.getUint(0)
		seed := cli.getString(1)

		StartNode(port, seed)

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
	i, err := strconv.Atoi(s)

	if err != nil {
		cli.printHelpAndExit()
	}

	return uint64(i)
}

func (cli *CLI) printHelpAndExit() {
	fmt.Print(helpMessage)
	os.Exit(1)
}

func (cli *CLI) getClient(serverIp string) *Client {
	client, err := NewClient(serverIp)

	if err != nil {
		log.Printf(err.Error())
		os.Exit(1)
	}

	return client
}

///////////////////////////////////////////////////////
// CLI commands that really should live somewhere else

func (cli *CLI) getBalance(db string, address string) {
	bcs := CreateNewBlockchains(db + ".db", false)
	result, ok := bcs.GetBalance(NATIVE_CHAIN, address)
	if !ok {
		fmt.Println("Address not found")
	}
	fmt.Printf("Balance: %v\n", result)
}

func (cli *CLI) printChain(db string, symbol string) {
	bcs := CreateNewBlockchains(db + ".db", false)
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
