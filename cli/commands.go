package cli

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
		fmt.Printf("Data: %x\n", block.Data)
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
	ws := NewWalletStore(false)
	addresses := ws.GetAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}
}

func (cli *CLI) createWallet() {
	ws := NewWalletStore(false)
	address := ws.AddWallet()
	ws.Persist()

	fmt.Printf("New wallet's address: %s\n", address)
}
