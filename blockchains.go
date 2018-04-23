package main

type Blockchains struct {
	matchChain Blockchain
	tokenChains []Blockchain
}


func (bc *Blockchain) AddTokenBlock() {

}

func (bc *Blockchain) AddMatchBlock() {

}

func (bc *Blockchain) GetBalance(address string) uint64 {
	/*bci := bc.Iterator()

	for {
		block := bci.Next()

	}*/
	return 0
}