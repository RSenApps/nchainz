package main

import (
	"fmt"
	"strconv"
)

func main() {
	chain := NewBlockchain()
	chain.AddBlock("Send 1 BNB to Ryan")
	chain.AddBlock("Send 2 BNB to Lizzie")
	chain.AddBlock("Send 3 BNB to Nick")

	for _, block := range chain.blocks {
		fmt.Printf("Prev Hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("Validated Proof of Work: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println("---------------------------------------")
	}
}
