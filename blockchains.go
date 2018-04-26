package main

import (
	"errors"
)

const MATCH_CHAIN = "MATCH"

type Blockchains struct {
	chains    map[string]*Blockchain
	consensusState ConsensusState
}

func (blockchains *Blockchains) rollbackTokenToHeight(symbol string, height uint64) {
	if height <= blockchains.chains[symbol].GetStartHeight() {
		return
	}

	blocksToRemove := blockchains.chains[symbol].GetStartHeight() - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.chains[symbol].RemoveLastBlock().(TokenData)
		for _, order := range removedData.Orders {
			blockchains.consensusState.RollbackOrder(symbol, order)
		}

		for _, cancelOrder := range removedData.CancelOrders {
			blockchains.consensusState.RollbackCancelOrder(symbol, cancelOrder)
		}

		for _, transactionConfirmed := range removedData.TransactionConfirmed {
			blockchains.consensusState.RollbackTransactionConfirmed(symbol, transactionConfirmed)
		}

		for _, transfer := range removedData.Transfers {
			blockchains.consensusState.RollbackTransfer(symbol, transfer)
		}
	}
}

func (blockchains *Blockchains) rollbackMatchToHeight(height uint64) {
	if height <= blockchains.chains[MATCH_CHAIN].GetStartHeight() {
		return
	}

	blocksToRemove := blockchains.chains[MATCH_CHAIN].GetStartHeight() - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.chains[MATCH_CHAIN].RemoveLastBlock().(MatchData)
		for _, match := range removedData.Matches {
			blockchains.consensusState.RollbackMatch(match)
		}

		for _, cancelMatch := range removedData.CancelMatches {
			blockchains.consensusState.RollbackCancelMatch(cancelMatch)
		}

		for _, createToken := range removedData.CreateTokens {
			blockchains.consensusState.RollbackCreateToken(createToken)
		}
	}
}

func (blockchains *Blockchains) RollbackToHeight(symbol string, height uint64) {
	if symbol == MATCH_CHAIN {
		blockchains.rollbackMatchToHeight(height)
	} else {
		blockchains.rollbackTokenToHeight(symbol, height)
	}
}

func (blockchains *Blockchains) addTokenData(symbol string, tokenData TokenData) {
	for _, order := range tokenData.Orders {
		blockchains.consensusState.AddOrder(symbol, order)
	}

	for _, cancelOrder := range tokenData.CancelOrders {
		blockchains.consensusState.AddCancelOrder(symbol, cancelOrder)
	}

	for _, transactionConfirmed := range tokenData.TransactionConfirmed {
		blockchains.consensusState.AddTransactionConfirmed(symbol, transactionConfirmed)
	}

	for _, transfer := range tokenData.Transfers {
		blockchains.consensusState.AddTransfer(symbol, transfer)
	}
}

func (blockchains *Blockchains) addMatchData(matchData MatchData) {
	for _, match := range matchData.Matches {
		blockchains.consensusState.AddMatch(match)
	}

	for _, cancelMatch := range matchData.CancelMatches {
		blockchains.consensusState.AddCancelMatch(cancelMatch)
	}

	for _, createToken := range matchData.CreateTokens {
		blockchains.consensusState.AddCreateToken(createToken)
	}
}

func (blockchains *Blockchains) AddBlock(symbol string, block Block) {
	if symbol == MATCH_CHAIN {
		blockchains.addMatchData(block.Data.(MatchData))
	} else {
		blockchains.addTokenData(symbol, block.Data.(TokenData))
	}
	blockchains.chains[symbol].AddBlock(block)
}

func (blockchains *Blockchains) GetOpenOrders(symbol string) []Order {
	return blockchains.consensusState.tokenStates[symbol].openOrders
}

func (blockchains *Blockchains) GetUnconfirmedMatches() map[uint64]bool {
	return blockchains.consensusState.unconfirmedMatchIDs
}

func (blockchains *Blockchains) GetBalance(symbol string, address string) uint64 {
	return blockchains.consensusState.tokenStates[symbol].balances[address]
}

func CreateNewBlockchains(dbName string) *Blockchains {
	//instantiates state and blockchains
	blockchains := &Blockchains{}
	blockchains.chains[MATCH_CHAIN] = NewBlockchain(dbName)
	return blockchains
}

func (blockchains *Blockchains) GetHeight(symbol string) uint64 {
	return blockchains.chains[MATCH_CHAIN].GetStartHeight()
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

func (blockchains *Blockchains) ApplyUpdatesToken(token int, startIndex uint64, updates []byte) {

}

func (blockchains *Blockchains) GetBlock(symbol string, blockhash []byte) (*Block, error) {
	bc, ok := blockchains.chains[symbol]
	if !ok {
		return nil, errors.New("invalid chain")
	}

	block, blockErr := bc.GetBlock(blockhash)
	return block, blockErr
}

func (blockchains *Blockchains) GetHeights() map[string]uint64 {
	heights := make(map[string]uint64)
	for symbol, tokenChain := range blockchains.chains {
		heights[symbol] = tokenChain.GetStartHeight()
	}

	return heights
}

func (blockchains *Blockchains) GetBlockhashes() map[string][][]byte {
	blockhashes := make(map[string][][]byte)

	for symbol, tokenChain := range blockchains.chains {
		blockhashes[symbol] = tokenChain.GetBlockhashes()
	}

	return blockhashes
}
