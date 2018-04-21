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

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  addblock -data BLOCK_DATA - add a block to the blockchain")
	fmt.Println("  printchain - print all the blocks of the blockchain")
	fmt.Println("  node - start up a full node")
}

func (cli *CLI) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		os.Exit(1)
	}
}

func (cli *CLI) addBlock(data string) {
	bc := NewBlockchain("blockchain.db")
	defer bc.db.Close()

	bc.AddBlock(data)
	fmt.Println("Success!")
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

func (cli *CLI) node(args []string) {
	// TODO: parse args properly
	port, _ := strconv.Atoi(args[2])
	seed := args[3]

	StartNode(uint(port), seed)
}

// Run parses cmd line arguments and processes cmds
func (cli *CLI) Run() {
	cli.validateArgs()

	// Use flag package to parse cmd line arguments
	addBlockCmd := flag.NewFlagSet("addblock", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	nodeCmd := flag.NewFlagSet("node", flag.ExitOnError)

	addBlockData := addBlockCmd.String("data", "", "Block data")

	switch os.Args[1] {
	case "addblock":
		err := addBlockCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "printchain":
		err := printChainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "node":
		err := nodeCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	default:
		cli.printUsage()
		os.Exit(1)
	}

	if addBlockCmd.Parsed() {
		if *addBlockData == "" {
			addBlockCmd.Usage()
			os.Exit(1)
		}
		cli.addBlock(*addBlockData)
	}

	if printChainCmd.Parsed() {
		cli.printChain()
	}

	if nodeCmd.Parsed() {
		cli.node(os.Args)
	}
}
