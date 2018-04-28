package main

import (
	"errors"
)

const MATCH_CHAIN = "MATCH"

type Blockchains struct {
	chains    map[string]*Blockchain
	consensusState ConsensusState
}

type UncommittedTransactions struct {
	transactions []GenericTransaction
}

func (uncommitted *UncommittedTransactions) addTransaction(transaction GenericTransaction) {
	uncommitted.transactions = append(uncommitted.transactions, transaction)
}

func (uncommitted *UncommittedTransactions) undoTransactions(symbol string, state *ConsensusState) {
	for i := len(uncommitted.transactions) - 1; i >= 0; i-- {
		transaction := uncommitted.transactions[i].transaction
		switch uncommitted.transactions[i].transactionType {
		case ORDER: state.RollbackOrder(symbol, transaction.(Order))
		case CANCEL_ORDER: state.RollbackCancelOrder(symbol, transaction.(CancelOrder))
		case TRANSACTION_CONFIRMED: state.RollbackTransactionConfirmed(symbol, transaction.(TransactionConfirmed))
		case TRANSFER: state.RollbackTransfer(symbol, transaction.(Transfer))
		case MATCH: state.RollbackMatch(transaction.(Match))
		case CANCEL_MATCH: state.RollbackCancelMatch(transaction.(CancelMatch))
		case CREATE_TOKEN: state.RollbackCreateToken(transaction.(CreateToken))
		}
	}
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

func (blockchains *Blockchains) addTokenData(symbol string, tokenData TokenData, uncommitted *UncommittedTransactions) bool {
	for _, order := range tokenData.Orders {
		if !blockchains.consensusState.AddOrder(symbol, order) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     order,
			transactionType: ORDER,
		})
	}

	for _, cancelOrder := range tokenData.CancelOrders {
		if !blockchains.consensusState.AddCancelOrder(symbol, cancelOrder) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     cancelOrder,
			transactionType: CANCEL_ORDER,
		})
	}

	for _, transactionConfirmed := range tokenData.TransactionConfirmed {
		if !blockchains.consensusState.AddTransactionConfirmed(symbol, transactionConfirmed) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     transactionConfirmed,
			transactionType: TRANSACTION_CONFIRMED,
		})
	}

	for _, transfer := range tokenData.Transfers {
		if !blockchains.consensusState.AddTransfer(symbol, transfer) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     transfer,
			transactionType: TRANSFER,
		})
	}
	return true
}

func (blockchains *Blockchains) addMatchData(matchData MatchData, uncommitted *UncommittedTransactions) bool {
	for _, match := range matchData.Matches {
		if !blockchains.consensusState.AddMatch(match) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     match,
			transactionType: MATCH,
		})
	}

	for _, cancelMatch := range matchData.CancelMatches {
		if !blockchains.consensusState.AddCancelMatch(cancelMatch) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     cancelMatch,
			transactionType: MATCH,
		})
	}

	for _, createToken := range matchData.CreateTokens {
		if !blockchains.consensusState.AddCreateToken(createToken) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     createToken,
			transactionType: CREATE_TOKEN,
		})
	}
	return true
}

func (blockchains *Blockchains) AddBlock(symbol string, block Block) {
	blockchains.AddBlocks(symbol, []Block{block})
}

func (blockchains *Blockchains) AddBlocks(symbol string, blocks []Block) bool {
	blocksAdded := 0
	var uncommitted UncommittedTransactions
	failed := false
	for _, block := range blocks {
		if symbol == MATCH_CHAIN {
			if !blockchains.addMatchData(block.Data.(MatchData), &uncommitted) {
				failed = true
				break
			}
		} else {
			if !blockchains.addTokenData(symbol, block.Data.(TokenData), &uncommitted) {
				failed = true
				break
			}
		}
		blockchains.chains[symbol].AddBlock(block)
		blocksAdded++
	}
	if failed {
		uncommitted.undoTransactions(symbol, &blockchains.consensusState)
		for i := 0; i < blocksAdded; i++ {
			blockchains.chains[symbol].RemoveLastBlock()
		}
		return false
	}
	return true
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
