package main

type ConsensusStateToken struct {
	openOrders []Order
	balances map[string]uint64
}

type ConsensusState struct {
	tokenStates []ConsensusStateToken
	unconfirmedMatchIDs map[uint64]bool
}

type Blockchains struct {
	matchChain Blockchain
	tokenChains []Blockchain
	consensusState ConsensusState
}

func (blockchains *Blockchains) RollbackTokenBlock() {

}

func (blockchains *Blockchains) RollbackMatchBlock() {

}

func (blockchains *Blockchains) AddTokenBlock(block Block) {

}

func (blockchains *Blockchains) AddMatchBlock(block Block) {

}

func (blockchains *Blockchains) GetOpenOrders(token int) []Order {
	return blockchains.consensusState.tokenStates[token].openOrders
}

func (blockchains *Blockchains) GetUnconfirmedMatches() map[uint64]bool {
	return blockchains.consensusState.unconfirmedMatchIDs
}

func (blockchains *Blockchains) GetBalance(token int, address string) uint64 {
	return blockchains.consensusState.tokenStates[token].balances[address]
}