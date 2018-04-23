package main

type Blockchains struct {
	matchChain Blockchain
	tokenChains []Blockchain
	consensusState ConsensusState
}

func (blockchains *Blockchains) RollbackTokenToHeight(token int, height uint64) {

}

func (blockchains *Blockchains) RollbackMatchToHeight(height uint64) {

}

func (blockchains *Blockchains) AddTokenBlock(token int, tokenData TokenData) {
	for _, order := range tokenData.Orders {
		blockchains.consensusState.AddOrder(token, order)
	}

	for _, cancelOrder := range tokenData.CancelOrders {
		blockchains.consensusState.AddCancelOrder(token, cancelOrder)
	}

	for _, transactionConfirmed := range tokenData.TransactionConfirmed {
		blockchains.consensusState.AddTransactionConfirmed(token, transactionConfirmed)
	}

	for _, transfer := range tokenData.Transfers {
		blockchains.consensusState.AddTransfer(token, transfer)
	}

	blockchains.tokenChains[token].AddBlock(tokenData, TOKEN)
}

func (blockchains *Blockchains) AddMatchBlock(matchData MatchData) {
	for _, match := range matchData.Matches {
		blockchains.consensusState.AddMatch(match)
	}

	for _, cancelMatch := range matchData.CancelMatches {
		blockchains.consensusState.AddCancelMatch(cancelMatch)
	}

	for _, createToken := range matchData.CreateTokens {
		blockchains.consensusState.AddCreateToken(createToken)
	}

	blockchains.matchChain.AddBlock(matchData, MATCH)
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
