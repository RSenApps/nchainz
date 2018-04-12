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
		fmt.Println("Prev hash:", block.PrevBlockHash)
		fmt.Println("Data:", block.Data.(string))
		fmt.Println("Hash:", block.Hash)
		fmt.Println("---------------------------------------")
	}
}
