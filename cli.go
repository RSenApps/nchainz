package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// For processing cmd line arguments
type CLI struct {
}

//
// Main method to parse and process cmds
//
func (cli *CLI) Run() {
	if len(os.Args) < 2 {
		cli.printHelp()
		os.Exit(1)
	}

	// Use flag package to parse cmd line arguments
	walletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	printAddressesCmd := flag.NewFlagSet("printaddresses", flag.ExitOnError)
	nodeCmd := flag.NewFlagSet("node", flag.ExitOnError)
	createBCCmd := flag.NewFlagSet("createbc", flag.ExitOnError)
	addTXCmd := flag.NewFlagSet("addtx", flag.ExitOnError)

	getBalanceCmd := flag.NewFlagSet("createbc", flag.ExitOnError)
	bcAddress := createBCCmd.String("address", "", "Genesis reward sent to this address.")
	getBalanceAddress := getBalanceCmd.String("address", "", "Address to get balance of")
	switch os.Args[1] {
	case "createwallet":
		err := walletCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "printchain":
		err := printChainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "printaddresses":
		err := printAddressesCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "node":
		err := nodeCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "createbc":
		err := createBCCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "addtx":
		err := addTXCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "getbalance":
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "transfer":
		amt, _ := strconv.Atoi(os.Args[2])
		symbol := os.Args[3]
		from := os.Args[4]
		to := os.Args[5]
		serverIp := os.Args[6]

		client, _ := NewClient(serverIp)
		client.Transfer(uint64(amt), symbol, from, to)
	case "order":
		buyAmt, _ := strconv.Atoi(os.Args[2])
		buySymbol := os.Args[3]
		sellAmt, _ := strconv.Atoi(os.Args[4])
		sellSymbol := os.Args[5]
		seller := os.Args[6]
		serverIp := os.Args[7]

		client, _ := NewClient(serverIp)
		client.Order(uint64(buyAmt), buySymbol, uint64(sellAmt), sellSymbol, seller)
	case "cancel":
		orderSymbol := os.Args[2]
		orderId, _ := strconv.Atoi(os.Args[3])
		serverIp := os.Args[4]

		client, _ := NewClient(serverIp)
		client.Cancel(orderSymbol, uint64(orderId))
	default:
		cli.printHelp()
		os.Exit(1)
	}

	if walletCmd.Parsed() {
		cli.createWallet()
	}

	if printChainCmd.Parsed() {
		cli.printChain()
	}

	if nodeCmd.Parsed() {
		cli.node(os.Args)
	}

	if printAddressesCmd.Parsed() {
		cli.printAddresses()
	}

	if createBCCmd.Parsed() {
		if *bcAddress == "" {
			createBCCmd.Usage()
			os.Exit(1)
		}
		cli.createBC(*bcAddress)
	}

	if addTXCmd.Parsed() {
		cli.addTX()
	}

	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			os.Exit(1)
		}
		cli.getBalance(*getBalanceAddress)
	}
}

func (cli *CLI) printHelp() {
	fmt.Println("===== Help menu =====")
	fmt.Println("go run *.go createwallet                  --- Creates a wallet with a pair of keys")
	fmt.Println("go run *.go createbc -address ADDRESS     --- Creates new blockchain. ADDRESS gets genesis reward")
	fmt.Println("go run *.go printchain                    --- Print all the blocks in the blockchain")
	fmt.Println("go run *.go printaddresses                --- Lists all addresses in walletFile")
	fmt.Println("go run *.go node PORT SEED_IP             --- start up a full node")
	fmt.Println("go run *.go addtx                         --- Add transaction to mempool")
	fmt.Println("go run *.go getbalance -address ADDRESS   --- gets the balance for an address")
	fmt.Println("go run *.go order BUY_AMT BUY_SYMBOL SELL_AMT SELL_SYMBOL ADDRESS NODE")
	fmt.Println("go run *.go transfer BUY_AMT BUY_SYMBOL SELL_AMT SELL_SYMBOL ADDRESS NODE")
	fmt.Println("go run *.go cancel SYMBOL ORDER_ID NODE")
}

func (cli *CLI) getBalance(address string) {
	bcs := CreateNewBlockchains("blockchain.db")
	result, ok := bcs.GetBalance(NATIVE_CHAIN, address)
	if !ok {
		fmt.Println("Address not found")
	}
	fmt.Printf("Balance: %v\n", result)
}

func (cli *CLI) printChain() {
	bcs := CreateNewBlockchains("blockchain.db")
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

func (cli *CLI) node(args []string) {
	// TODO: parse args properly
	port, _ := strconv.Atoi(args[2])
	seed := args[3]

	StartNode(uint(port), seed)
}

func (cli *CLI) createBC(address string) {
	bcs := CreateNewBlockchains("blockchain.db")
	bc := bcs.chains[NATIVE_CHAIN]
	transfer := Transfer{
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
	/*block := NewGenesisBlock()
	pow := NewProofOfWork(block)
	ok, nonce, _ :=pow.Try(99999999999)
	if ok {
		fmt.Printf("Nonce: %v", nonce)
	}*/

	transfer := Transfer{
		Amount:      50,
		FromAddress: "Satoshi",
		ToAddress:   "Negansoft",
		Signature:   nil,
	}

	bcs := CreateNewBlockchains("blockchain.db")
	bcs.AddTransactionToMempool(
		GenericTransaction{transfer, TRANSFER},
		NATIVE_CHAIN,
	)

	for {
		time.Sleep(100 * time.Millisecond)
	}
}
