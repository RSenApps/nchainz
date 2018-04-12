package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

func decode(b []byte) string {
	r := bytes.NewBuffer(b)
	d := gob.NewDecoder(r)

	var data string

	if d.Decode(&data) == nil {
		return data
	} else {
		return "ERROR"
	}
}

func main() {
	chain := NewBlockchain()
	chain.AddBlock("Send 1 BNB to Ryan")
	chain.AddBlock("Send 2 BNB to Lizzie")
	chain.AddBlock("Send 3 BNB to Nick")

	for _, block := range chain.blocks {
		fmt.Printf("Prev hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		fmt.Println("---------------------------------------")
	}
}
