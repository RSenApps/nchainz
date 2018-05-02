package main

import (
	"bytes"
	"errors"
	"github.com/boltdb/bolt"
	"math/rand"
	"os"
	"sync"
)

const MATCH_CHAIN = "MATCH"
const NATIVE_CHAIN = "NATIVE"

type Blockchains struct {
	chains             map[string]*Blockchain
	consensusState     ConsensusState
	chainsLock         *sync.RWMutex
	db                 *bolt.DB
	mempools           map[string]map[*GenericTransaction]bool // map of sets
	mempoolUncommitted map[string]*UncommittedTransactions
	mempoolsLock       *sync.Mutex
	finishedBlockCh    chan BlockMsg
	stopMiningCh       chan string
	miner              *Miner
	minerChosenToken   string
	recovering         bool
}

type UncommittedTransactions struct {
	transactions []GenericTransaction
}

func (uncommitted *UncommittedTransactions) addTransaction(transaction GenericTransaction) {
	uncommitted.transactions = append(uncommitted.transactions, transaction)
}

func (uncommitted *UncommittedTransactions) undoTransactions(symbol string, blockchains *Blockchains) {
	for i := len(uncommitted.transactions) - 1; i >= 0; i-- {
		transaction := uncommitted.transactions[i].Transaction
		switch uncommitted.transactions[i].TransactionType {
		case ORDER:
			blockchains.consensusState.RollbackOrder(symbol, transaction.(Order))
		case CANCEL_ORDER:
			blockchains.consensusState.RollbackCancelOrder(transaction.(CancelOrder))
		case CLAIM_FUNDS:
			blockchains.consensusState.RollbackClaimFunds(symbol, transaction.(ClaimFunds))
		case TRANSFER:
			blockchains.consensusState.RollbackTransfer(symbol, transaction.(Transfer))
		case MATCH:
			blockchains.consensusState.RollbackMatch(transaction.(Match))
		case CREATE_TOKEN:
			blockchains.consensusState.RollbackCreateToken(transaction.(CreateToken), blockchains)
		}
	}
}

func (blockchains *Blockchains) rollbackTokenToHeight(symbol string, height uint64) {
	if height >= blockchains.chains[symbol].GetStartHeight() {
		return
	}
	blocksToRemove := blockchains.chains[symbol].GetStartHeight() - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.chains[symbol].RemoveLastBlock().(TokenData)
		for _, order := range removedData.Orders {
			blockchains.consensusState.RollbackOrder(symbol, order)
		}

		for _, claimFunds := range removedData.ClaimFunds {
			blockchains.consensusState.RollbackClaimFunds(symbol, claimFunds)
		}

		for _, transfer := range removedData.Transfers {
			blockchains.consensusState.RollbackTransfer(symbol, transfer)
		}
	}
}

func (blockchains *Blockchains) rollbackMatchToHeight(height uint64) {
	if height >= blockchains.chains[MATCH_CHAIN].GetStartHeight() {
		return
	}

	blocksToRemove := blockchains.chains[MATCH_CHAIN].GetStartHeight() - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.chains[MATCH_CHAIN].RemoveLastBlock().(MatchData)
		for _, match := range removedData.Matches {
			blockchains.consensusState.RollbackMatch(match)
		}

		for _, cancelOrder := range removedData.CancelOrders {
			blockchains.consensusState.RollbackCancelOrder(cancelOrder)
		}

		for _, createToken := range removedData.CreateTokens {
			blockchains.consensusState.RollbackCreateToken(createToken, blockchains)
		}
	}
}

func (blockchains *Blockchains) RollbackToHeight(symbol string, height uint64) {
	blockchains.chainsLock.Lock()
	defer blockchains.chainsLock.Unlock()
	if symbol == MATCH_CHAIN {
		blockchains.rollbackMatchToHeight(height)
	} else {
		blockchains.rollbackTokenToHeight(symbol, height)
	}
}

func (blockchains *Blockchains) addGenericTransaction(symbol string, transaction GenericTransaction, uncommitted *UncommittedTransactions) bool {
	success := false
	switch transaction.TransactionType {
	case ORDER:
		success = blockchains.consensusState.AddOrder(symbol, transaction.Transaction.(Order))
	case CLAIM_FUNDS:
		success = blockchains.consensusState.AddClaimFunds(symbol, transaction.Transaction.(ClaimFunds))
	case TRANSFER:
		success = blockchains.consensusState.AddTransfer(symbol, transaction.Transaction.(Transfer))
	case MATCH:
		success = blockchains.consensusState.AddMatch(transaction.Transaction.(Match))
	case CANCEL_ORDER:
		success = blockchains.consensusState.AddCancelOrder(transaction.Transaction.(CancelOrder))
	case CREATE_TOKEN:
		success = blockchains.consensusState.AddCreateToken(transaction.Transaction.(CreateToken), blockchains)
	}
	if !success {
		return false
	}
	uncommitted.addTransaction(transaction)
	return true
}

func (blockchains *Blockchains) addTokenData(symbol string, tokenData TokenData, uncommitted *UncommittedTransactions) bool {
	for _, order := range tokenData.Orders {
		tx := GenericTransaction{
			Transaction:     order,
			TransactionType: ORDER,
		}
		if !blockchains.addGenericTransaction(symbol, tx, uncommitted) {
			return false
		}
	}

	for _, claimFunds := range tokenData.ClaimFunds {
		tx := GenericTransaction{
			Transaction:     claimFunds,
			TransactionType: CLAIM_FUNDS,
		}
		if !blockchains.addGenericTransaction(symbol, tx, uncommitted) {
			return false
		}
	}

	for _, transfer := range tokenData.Transfers {
		tx := GenericTransaction{
			Transaction:     transfer,
			TransactionType: TRANSFER,
		}
		if !blockchains.addGenericTransaction(symbol, tx, uncommitted) {
			return false
		}
	}
	return true
}

func (blockchains *Blockchains) addMatchData(matchData MatchData, uncommitted *UncommittedTransactions) bool {
	for _, match := range matchData.Matches {
		tx := GenericTransaction{
			Transaction:     match,
			TransactionType: MATCH,
		}
		if !blockchains.addGenericTransaction(MATCH_CHAIN, tx, uncommitted) {
			return false
		}
	}

	for _, cancelOrder := range matchData.CancelOrders {
		tx := GenericTransaction{
			Transaction:     cancelOrder,
			TransactionType: CANCEL_ORDER,
		}
		if !blockchains.addGenericTransaction(MATCH_CHAIN, tx, uncommitted) {
			return false
		}
	}

	for _, createToken := range matchData.CreateTokens {
		tx := GenericTransaction{
			Transaction:     createToken,
			TransactionType: CREATE_TOKEN,
		}
		if !blockchains.addGenericTransaction(MATCH_CHAIN, tx, uncommitted) {
			return false
		}
	}
	return true
}

func (blockchains *Blockchains) AddBlock(symbol string, block Block, takeLock bool) bool {
	return blockchains.AddBlocks(symbol, []Block{block}, takeLock)
}

func (blockchains *Blockchains) AddBlocks(symbol string, blocks []Block, takeLock bool) bool {
	if !blockchains.recovering && blockchains.minerChosenToken == symbol {
		go func() { blockchains.stopMiningCh <- symbol }()
		blockchains.mempoolsLock.Lock()
		blockchains.mempoolUncommitted[symbol].undoTransactions(symbol, blockchains)
		blockchains.mempoolUncommitted[symbol] = &UncommittedTransactions{}
		blockchains.mempoolsLock.Unlock()
	}

	if takeLock {
		blockchains.chainsLock.Lock()
		defer blockchains.chainsLock.Unlock()
	}

	blocksAdded := 0
	var uncommitted UncommittedTransactions
	failed := false
	for _, block := range blocks {
		if !bytes.Equal(blockchains.chains[symbol].tipHash, block.PrevBlockHash) {
			Log("prevBlockHash does not match tipHash %x != %x \n", blockchains.chains[symbol].tipHash, block.PrevBlockHash)
			failed = true
			break
		}

		//TODO:
		pow := NewProofOfWork(&block)
		if !pow.Validate() {
			Log("Proof of work of block is invalid")
			/*DEBUG failed = true
			break*/
		}
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
		uncommitted.undoTransactions(symbol, blockchains)
		for i := 0; i < blocksAdded; i++ {
			blockchains.chains[symbol].RemoveLastBlock()
		}
		return false
	}
	return true
}

func (blockchains *Blockchains) GetOpenOrders(symbol string) map[uint64]Order {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	state, _ := blockchains.consensusState.tokenStates[symbol]
	return state.openOrders
}

func (blockchains *Blockchains) GetBalance(symbol string, address string) (uint64, bool) {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	state, _ := blockchains.consensusState.tokenStates[symbol]
	balance, ok := state.balances[address]
	return balance, ok
}

func (blockchains *Blockchains) AddTokenChain(createToken CreateToken) {
	//no lock needed
	chain := NewBlockchain(blockchains.db, createToken.TokenInfo.Symbol)
	blockchains.chains[createToken.TokenInfo.Symbol] = chain
	blockchains.mempools[createToken.TokenInfo.Symbol] = make(map[*GenericTransaction]bool)
	blockchains.mempoolUncommitted[createToken.TokenInfo.Symbol] = &UncommittedTransactions{}

	if !blockchains.recovering { //recovery will replay this block normally
		blockchains.AddBlock(createToken.TokenInfo.Symbol, *NewTokenGenesisBlock(createToken), false)
	}
}

func (blockchains *Blockchains) RemoveTokenChain(createToken CreateToken) {
	delete(blockchains.chains, createToken.TokenInfo.Symbol)
	delete(blockchains.mempools, createToken.TokenInfo.Symbol)
	delete(blockchains.mempoolUncommitted, createToken.TokenInfo.Symbol)
}

func (blockchains *Blockchains) restoreFromDatabase() {
	iterators := make(map[string]*BlockchainForwardIterator)
	chainsDone := make(map[string]bool)
	done := false
	for !done {
		for symbol, chain := range blockchains.chains {
			if chainsDone[symbol] {
				continue
			}
			iterator, ok := iterators[symbol]
			if !ok {
				iterator = chain.ForwardIterator()
				chain.height = uint64(len(iterator.hashes))
				iterators[symbol] = iterator
			}
			block, err := iterator.Next()
			for err == nil {
				var uncommitted UncommittedTransactions
				if symbol == MATCH_CHAIN {
					if !blockchains.addMatchData(block.Data.(MatchData), &uncommitted) {
						iterator.Undo()
						break
					}
				} else {
					if !blockchains.addTokenData(symbol, block.Data.(TokenData), &uncommitted) {
						iterator.Undo()
						break
					}
				}
				block, err = iterator.Next()
			}
			if err != nil {
				chainsDone[symbol] = true
			}
		}
		done = true
		for symbol := range blockchains.chains {
			if !chainsDone[symbol] {
				done = false
				break
			}
		}
	}
}

func CreateNewBlockchains(dbName string) *Blockchains {
	//instantiates state and blockchains
	blockchains := &Blockchains{}
	newDatabase := true
	if _, err := os.Stat(dbName); err == nil {
		// path/to/whatever exists
		newDatabase = false
	}
	// Open BoltDB file
	db, err := bolt.Open(dbName, 0600, nil)
	if err != nil {
		LogPanic(err.Error())
	}
	blockchains.db = db

	blockchains.finishedBlockCh = make(chan BlockMsg, 1000)
	blockchains.miner = NewMiner(blockchains.finishedBlockCh)
	blockchains.stopMiningCh = make(chan string, 1000)
	blockchains.consensusState = NewConsensusState()
	blockchains.chains = make(map[string]*Blockchain)
	blockchains.chains[MATCH_CHAIN] = NewBlockchain(db, MATCH_CHAIN)
	blockchains.chainsLock = &sync.RWMutex{}
	blockchains.mempoolsLock = &sync.Mutex{}

	blockchains.mempools = make(map[string]map[*GenericTransaction]bool)
	blockchains.mempoolUncommitted = make(map[string]*UncommittedTransactions)

	blockchains.mempools[MATCH_CHAIN] = make(map[*GenericTransaction]bool)
	blockchains.mempoolUncommitted[MATCH_CHAIN] = &UncommittedTransactions{}
	if newDatabase {
		blockchains.recovering = false
		blockchains.AddBlock(MATCH_CHAIN, *NewGenesisBlock(), false)
	} else {
		blockchains.recovering = true
		blockchains.restoreFromDatabase()
		blockchains.recovering = false
	}

	go blockchains.StartMining()
	go blockchains.ApplyLoop()
	return blockchains
}

func (blockchains *Blockchains) GetHeight(symbol string) uint64 {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	return blockchains.chains[symbol].GetStartHeight()
}

func (blockchains *Blockchains) GetBlock(symbol string, blockhash []byte) (*Block, error) {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()

	bc, ok := blockchains.chains[symbol]
	if !ok {
		return nil, errors.New("invalid chain")
	}

	block, blockErr := bc.GetBlock(blockhash)
	return block, blockErr
}

func (blockchains *Blockchains) GetHeights() map[string]uint64 {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	heights := make(map[string]uint64)
	for symbol, chain := range blockchains.chains {
		heights[symbol] = chain.GetStartHeight()
	}
	return heights
}

func (blockchains *Blockchains) GetBlockhashes() map[string][][]byte {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	blockhashes := make(map[string][][]byte)
	for symbol, chain := range blockchains.chains {
		blockhashes[symbol] = chain.blockhashes
	}
	return blockhashes
}

func (blockchains *Blockchains) AddTransactionToMempool(tx GenericTransaction, symbol string) bool {
	blockchains.chainsLock.Lock()
	// Validate transaction
	if !blockchains.addGenericTransaction(symbol, tx, blockchains.mempoolUncommitted[symbol]) {
		blockchains.chainsLock.Unlock()
		return false
	}
	blockchains.chainsLock.Unlock()

	blockchains.mempoolsLock.Lock()
	if _, ok := blockchains.mempools[symbol][&tx]; ok {
		blockchains.mempoolsLock.Unlock()
		return false
	}

	// Add transaction to mempool
	blockchains.mempools[symbol][&tx] = true
	blockchains.mempoolsLock.Unlock()

	if blockchains.minerChosenToken == symbol {
		// Send transaction to miner
		blockchains.miner.minerCh <- MinerMsg{tx, false}
	}

	return true
}

func (blockchains *Blockchains) StartMining() {
	blockchains.mempoolsLock.Lock()
	Log("Start mining")
	// Pick a random token to start mining
	var tokens []string
	for token := range blockchains.mempools {
		tokens = append(tokens, token)
	}
	blockchains.mempoolsLock.Unlock()

	blockchains.minerChosenToken = tokens[rand.Intn(len(tokens))]

	// Send new block message
	switch blockchains.minerChosenToken {
	case MATCH_CHAIN:
		Log("Starting match block")
		blockchains.miner.minerCh <- MinerMsg{NewBlockMsg{MATCH_BLOCK, blockchains.chains[blockchains.minerChosenToken].tipHash, blockchains.minerChosenToken}, true}
	default:
		Log("Starting native block")
		blockchains.miner.minerCh <- MinerMsg{NewBlockMsg{TOKEN_BLOCK, blockchains.chains[blockchains.minerChosenToken].tipHash, blockchains.minerChosenToken}, true}
	}

	blockchains.mempoolsLock.Lock()
	var txInPool []GenericTransaction
	for tx := range blockchains.mempools[blockchains.minerChosenToken] {
		txInPool = append(txInPool, *tx)
	}
	blockchains.mempoolsLock.Unlock()

	//Send stuff currently in mem pool (re-validate too)
	go func(currentToken string, transactions []GenericTransaction) {
		for _, tx := range txInPool {
			blockchains.mempoolsLock.Lock()
			if currentToken != blockchains.minerChosenToken {
				blockchains.mempoolsLock.Unlock()
				return
			}
			blockchains.mempoolsLock.Unlock()

			blockchains.chainsLock.Lock()
			// Validate transaction
			if !blockchains.addGenericTransaction(blockchains.minerChosenToken, tx, blockchains.mempoolUncommitted[blockchains.minerChosenToken]) {
				blockchains.chainsLock.Unlock()
				blockchains.mempoolsLock.Lock()
				delete(blockchains.mempools[blockchains.minerChosenToken], &tx)
				blockchains.mempoolsLock.Unlock()
				continue
			}
			blockchains.chainsLock.Unlock()

			// Send transaction to miner
			Log("Sending from re-validated mempool")
			blockchains.miner.minerCh <- MinerMsg{tx, false}
		}
	}(blockchains.minerChosenToken, txInPool)
}

func (blockchains *Blockchains) ApplyLoop() {
	for {
		select {
		case blockMsg := <-blockchains.finishedBlockCh:
			// When miner finishes, try to add a block
			blockchains.chainsLock.Lock()
			//state was applied during validation so just add to chain
			if !bytes.Equal(blockchains.chains[blockMsg.Symbol].tipHash, blockMsg.Block.PrevBlockHash) {
				//block failed so retry
				Log("miner prevBlockHash does not match tipHash %x != %x \n", blockchains.chains[blockMsg.Symbol].tipHash, blockMsg.Block.PrevBlockHash)

				blockchains.mempoolsLock.Lock()
				blockchains.mempoolUncommitted[blockMsg.Symbol].undoTransactions(blockMsg.Symbol, blockchains)
				blockchains.mempoolUncommitted[blockMsg.Symbol] = &UncommittedTransactions{}
				//WARNING taking both locks always take chain lock first
				blockchains.chainsLock.Unlock()

			} else {
				blockchains.chains[blockMsg.Symbol].AddBlock(blockMsg.Block)
				blockchains.chainsLock.Unlock()

				blockchains.mempoolsLock.Lock()
				blockchains.mempoolUncommitted[blockMsg.Symbol] = &UncommittedTransactions{}

				//assume that all transactions for token have been added to block so delete mempool
				blockchains.mempools[blockMsg.Symbol] = make(map[*GenericTransaction]bool)
				blockchains.mempoolsLock.Unlock()
			}

		case symbol := <-blockchains.stopMiningCh:
			if symbol != blockchains.minerChosenToken {
				continue
			}
		}

		blockchains.StartMining()

	}
}
