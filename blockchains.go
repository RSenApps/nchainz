package main

import (
	"errors"
)

type Blockchains struct {
	matchChain     *Blockchain
	tokenChains    []*Blockchain
	consensusState ConsensusState
}

func (blockchains *Blockchains) RollbackTokenToHeight(token int, height uint64) {
	if height <= blockchains.tokenChains[token].GetStartHeight() {
		return
	}

	blocksToRemove := blockchains.tokenChains[token].GetStartHeight() - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.tokenChains[token].RemoveLastBlock().(TokenData)
		for _, order := range removedData.Orders {
			blockchains.consensusState.RollbackOrder(token, order)
		}

		for _, cancelOrder := range removedData.CancelOrders {
			blockchains.consensusState.RollbackCancelOrder(token, cancelOrder)
		}

		for _, transactionConfirmed := range removedData.TransactionConfirmed {
			blockchains.consensusState.RollbackTransactionConfirmed(token, transactionConfirmed)
		}

		for _, transfer := range removedData.Transfers {
			blockchains.consensusState.RollbackTransfer(token, transfer)
		}
	}
}

func (blockchains *Blockchains) RollbackMatchToHeight(height uint64) {
	if height <= blockchains.matchChain.GetStartHeight() {
		return
	}

	blocksToRemove := blockchains.matchChain.GetStartHeight() - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.matchChain.RemoveLastBlock().(MatchData)
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

	blockchains.tokenChains[token].AddBlock(tokenData, TOKEN_BLOCK)
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

	blockchains.matchChain.AddBlock(matchData, MATCH_BLOCK)
}

func (blockchains *Blockchains) AddBlock(chainIdx int, blockData BlockData) {
	if chainIdx == 0 {
		blockchains.AddMatchBlock(blockData.(MatchData))
	} else {
		blockchains.AddTokenBlock(chainIdx-1, blockData.(TokenData))
	}
}

func (blockchains *Blockchains) GetBlock(chainIdx int, blockhash []byte) (*Block, error) {
	bc, chainErr := blockchains.getChain(chainIdx)
	if chainErr != nil {
		return nil, chainErr
	}

	block, blockErr := bc.GetBlock(blockhash)
	return block, blockErr
}

func (blockchains *Blockchains) getChain(chainIdx int) (*Blockchain, error) {
	if chainIdx < 0 || chainIdx-1 >= len(blockchains.tokenChains) {
		return nil, errors.New("invalid chain")
	}

	if chainIdx == 0 {
		return blockchains.matchChain, nil
	} else {
		return blockchains.tokenChains[chainIdx-1], nil
	}
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

func CreateNewBlockchains(dbName string) *Blockchains {
	//instantiates state and blockchains
	blockchains := &Blockchains{}
	blockchains.matchChain = NewBlockchain(dbName)
	return blockchains
}

func (blockchains *Blockchains) GetHeightMatching() uint64 {
	return blockchains.matchChain.GetStartHeight()
}

func (blockchains *Blockchains) GetHeightToken(token int) uint64 {
	return blockchains.tokenChains[token].GetStartHeight()
}

func (blockchains *Blockchains) GetHeights() []uint64 {
	heights := make([]uint64, 1)
	heights[0] = blockchains.GetHeightMatching()

	for _, tokenChain := range blockchains.tokenChains {
		heights = append(heights, tokenChain.GetStartHeight())
	}

	return heights
}

func (blockchains *Blockchains) GetBlockhashes() [][][]byte {
	blockhashes := make([][][]byte, 1)
	blockhashes[0] = blockchains.matchChain.GetBlockhashes()

	for _, tokenChain := range blockchains.tokenChains {
		blockhashes = append(blockhashes, tokenChain.GetBlockhashes())
	}

	return blockhashes
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
