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

func (blockchains *Blockchains) RollbackTokenToHeight(token int, height uint64) {

}

func (blockchains *Blockchains) RollbackMatchToHeight(height uint64) {

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

func (blockchains *Blockchains) CreateNew() {
	//instantiates state and blockchains
}

func (blockchains *Blockchains) GetHeightMatching() uint64 {
	return 0
}

func (blockchains *Blockchains) GetHeightToken(token int) uint64 {
	return 0
}

// for use in applying updates to other nodes
func (blockchains *Blockchains) SerializeUpdatesMatching(startIndex uint64) []byte {
	return []byte{}
}

func (blockchains *Blockchains) SerializeUpdatesToken(token int, startIndex uint64) []byte {
	return []byte{}
}

func (blockchains *Blockchains) ApplyUpdatesMatching(startIndex uint64, updates []byte) {

}

func (blockchains *Blockchains) ApplyUpdatesToken(token int, startIndex uint64, updates []byte){

}
