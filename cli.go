package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
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
}

func (cli *CLI) printHelp() {
	fmt.Println("===== Help menu =====")
	fmt.Println("go run *.go createwallet                  --- Creates a wallet with a pair of keys")
	fmt.Println("go run *.go createbc -address ADDRESS     --- Creates new blockchain. ADDRESS gets genesis reward")
	fmt.Println("go run *.go printchain                    --- Print all the blocks in the blockchain")
	fmt.Println("go run *.go printaddresses								 --- Lists all addresses in walletFile")
	fmt.Println("go run *.go node                          --- start up a full node")
}

func (cli *CLI) getBalance(address string) {
	/*bc := NewBlockchain(address)
	defer bc.db.Close()*/
}

func (cli *CLI) printChain() {
	bc := NewBlockchain("blockchain.db")
	defer bc.db.Close()

	bci := bc.Iterator()

	for {
		block, _ := bci.Next()

		fmt.Printf("Prev Hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("Validated Proof of Work: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println("-------------------------------")

		if len(block.PrevBlockHash) == 0 {
			break
		}
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
