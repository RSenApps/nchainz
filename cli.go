package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
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
	claim AMT ADDRESS
		Create a CLAIM_FUNDS transaction and send it to the given node
	create ... ADDRESS
		Create a CREATE_TOKEN transaction and send it to the given node

Running a node or miner
	node PORT SEED_IP
		Start up a full node on the given port and connect to the given peer

Utilities
	printchain DB
		Prints all the blocks in the blockchain
	addtx
		Add transaction to mempool
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
		cli.getBalance(address)

	case "printaddresses":
		cli.printAddresses()

	case "order":
		buyAmt := cli.getUint(0)
		buySymbol := cli.getString(1)
		sellAmt := cli.getUint(2)
		sellSymbol := cli.getString(3)
		seller := cli.getString(4)
		serverIp := cli.getString(5)

		client, _ := NewClient(serverIp)
		client.Order(buyAmt, buySymbol, sellAmt, sellSymbol, seller)

	case "transfer":
		amt := cli.getUint(0)
		symbol := cli.getString(1)
		from := cli.getString(2)
		to := cli.getString(3)
		serverIp := cli.getString(4)

		client, _ := NewClient(serverIp)
		client.Transfer(amt, symbol, from, to)

	case "cancel":
		orderSymbol := cli.getString(0)
		orderId := cli.getUint(1)
		serverIp := cli.getString(2)

		client, _ := NewClient(serverIp)
		client.Cancel(orderSymbol, orderId)

	case "claim":
		// TODO

	case "createbc":
		address := cli.getString(0)
		cli.createBC(address)

	case "node":
		port := cli.getUint(0)
		seed := cli.getString(1)

		StartNode(port, seed)

	case "printchain":
		db := cli.getString(0)

		cli.printChain(db)

	case "addtx":
		cli.addTX()

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

///////////////////////////////////////////////////////
// CLI commands that really should live somewhere else

func (cli *CLI) getBalance(address string) {
	bcs := CreateNewBlockchains("blockchain.db")
	result, ok := bcs.GetBalance(NATIVE_CHAIN, address)
	if !ok {
		fmt.Println("Address not found")
	}
	fmt.Printf("Balance: %v\n", result)
}

func (cli *CLI) printChain(db string) {
	bcs := CreateNewBlockchains(db + ".db")
	bc := bcs.chains[MATCH_CHAIN]
	defer bc.db.Close()

	bci := bc.Iterator()
	fmt.Println(MATCH_CHAIN)
	fmt.Printf("Height: %d tiphash: %x\n", bc.height, bc.tipHash)
	block, err := bci.Prev()
	for err == nil {

		fmt.Printf("Prev Hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("Validated Proof of Work: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println("-------------------------------")
		block, err = bci.Prev()
	}

	bc = bcs.chains[NATIVE_CHAIN]

	bci = bc.Iterator()
	fmt.Println(NATIVE_CHAIN)
	fmt.Printf("Height: %d tiphash: %x\n", bc.height, bc.tipHash)
	block, err = bci.Prev()
	for err == nil {

		fmt.Printf("Prev Hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("Validated Proof of Work: %s\n", strconv.FormatBool(pow.Validate()))
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

func (cli *CLI) createBC(address string) {
	bcs := CreateNewBlockchains("blockchain.db")
	bc := bcs.chains[NATIVE_CHAIN]
	transfer := Transfer{
		ID:          2,
		Amount:      10,
		FromAddress: "Satoshi",
		ToAddress:   "Negansoft",
		Signature:   nil,
	}
	tokenData := TokenData{
		Orders:     nil,
		ClaimFunds: nil,
		Transfers:  []Transfer{transfer},
	}
	block := NewBlock(tokenData, TOKEN_BLOCK, bc.tipHash)
	bcs.AddBlock(NATIVE_CHAIN, *block, true)
}

func (cli *CLI) addTX() {
	transfer := Transfer{
		ID:          1,
		Amount:      50,
		FromAddress: "Satoshi",
		ToAddress:   "Negansoft",
		Signature:   nil,
	}

	bcs := CreateNewBlockchains("blockchains.db")
	bcs.AddTransactionToMempool(
		GenericTransaction{transfer, TRANSFER},
		NATIVE_CHAIN,
	)

	for {
		time.Sleep(100 * time.Millisecond)
	}
}
