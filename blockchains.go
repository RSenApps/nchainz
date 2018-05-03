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
	chainsLock         *sync.RWMutex // Lock on consensus state and chains
	db                 *bolt.DB
	mempools           map[string]map[string]GenericTransaction // map of sets (of GenericTransaction ID)
	mempoolUncommitted map[string]*UncommittedTransactions
	mempoolsLock       *sync.Mutex // Lock on mempools
	finishedBlockCh    chan BlockMsg
	stopMiningCh       chan string
	miner              *Miner
	minerChosenToken   string
	recovering         bool
	matcherCh          chan MatcherMsg
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
	if height >= blockchains.chains[symbol].height {
		return
	}
	blocksToRemove := blockchains.chains[symbol].height - height
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
	if height >= blockchains.chains[MATCH_CHAIN].height {
		return
	}

	blocksToRemove := blockchains.chains[MATCH_CHAIN].height - height
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
	Log("Rolling back %v block to height: %v \n", symbol, height)
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

	if success && transaction.TransactionType == ORDER {
		matcherMsg := MatcherMsg{transaction.Transaction.(Order), symbol}
		blockchains.matcherCh <- matcherMsg
	}

	if !success {
		return false
	}

	uncommitted.addTransaction(transaction)
	return true
}

func (blockchains *Blockchains) rollbackGenericTransaction(symbol string, transaction GenericTransaction) {
	switch transaction.TransactionType {
	case ORDER:
		blockchains.consensusState.RollbackOrder(symbol, transaction.Transaction.(Order))
	case CLAIM_FUNDS:
		blockchains.consensusState.RollbackClaimFunds(symbol, transaction.Transaction.(ClaimFunds))
	case TRANSFER:
		blockchains.consensusState.RollbackTransfer(symbol, transaction.Transaction.(Transfer))
	case MATCH:
		blockchains.consensusState.RollbackMatch(transaction.Transaction.(Match))
	case CANCEL_ORDER:
		blockchains.consensusState.RollbackCancelOrder(transaction.Transaction.(CancelOrder))
	case CREATE_TOKEN:
		blockchains.consensusState.RollbackCreateToken(transaction.Transaction.(CreateToken), blockchains)
	}
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
	if takeLock {
		blockchains.chainsLock.Lock()
		defer blockchains.chainsLock.Unlock()
	}

	if _, ok := blockchains.chains[symbol]; !ok {
		Log("AddBlocks failed as symbol not found: ", symbol)
		return false
	}

	if !blockchains.recovering && blockchains.minerChosenToken == symbol {
		go func() { blockchains.stopMiningCh <- symbol }()
		blockchains.mempoolsLock.Lock()
		Log("AddBlocks undoing transactions for: %v", symbol)
		blockchains.mempoolUncommitted[symbol].undoTransactions(symbol, blockchains)
		blockchains.mempoolUncommitted[symbol] = &UncommittedTransactions{}
		blockchains.mempoolsLock.Unlock()
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

		if blockchains.chains[symbol].height > 1 {
			pow := NewProofOfWork(&block)
			if !pow.Validate() {
				Log("Proof of work of block is invalid")
			}
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
		Log("Adding blocks failed, rolling back.")
		uncommitted.undoTransactions(symbol, blockchains)
		for i := 0; i < blocksAdded; i++ {
			blockchains.chains[symbol].RemoveLastBlock()
		}
		return false
	}
	Log("AddBlocks added %v blocks to %v chain", blocksAdded, symbol)

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
	Log("Adding token chain")
	chain := NewBlockchain(blockchains.db, createToken.TokenInfo.Symbol)
	blockchains.chains[createToken.TokenInfo.Symbol] = chain
	blockchains.mempools[createToken.TokenInfo.Symbol] = make(map[string]GenericTransaction)
	blockchains.mempoolUncommitted[createToken.TokenInfo.Symbol] = &UncommittedTransactions{}

	if !blockchains.recovering { //recovery will replay this block normally
		blockchains.AddBlock(createToken.TokenInfo.Symbol, *NewTokenGenesisBlock(createToken), false)
	}
}

func (blockchains *Blockchains) RemoveTokenChain(createToken CreateToken) {
	Log("Removing token chain")
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

func CreateNewBlockchains(dbName string, startMining bool) *Blockchains {
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
	blockchains.mempools = make(map[string]map[string]GenericTransaction)
	blockchains.mempoolUncommitted = make(map[string]*UncommittedTransactions)

	blockchains.mempools[MATCH_CHAIN] = make(map[string]GenericTransaction)
	blockchains.mempoolUncommitted[MATCH_CHAIN] = &UncommittedTransactions{}

	blockchains.consensusState = NewConsensusState()
	blockchains.chains = make(map[string]*Blockchain)
	blockchains.chains[MATCH_CHAIN] = NewBlockchain(db, MATCH_CHAIN)
	blockchains.chainsLock = &sync.RWMutex{}
	blockchains.mempoolsLock = &sync.Mutex{}

	blockchains.matcherCh = make(chan MatcherMsg, 1000)
	StartMatcher(blockchains.matcherCh, blockchains, nil)

	blockchains.mempools[MATCH_CHAIN] = make(map[string]GenericTransaction)
	blockchains.mempoolUncommitted[MATCH_CHAIN] = &UncommittedTransactions{}
	if newDatabase {
		blockchains.recovering = false
		blockchains.AddBlock(MATCH_CHAIN, *NewGenesisBlock(), false)
	} else {
		blockchains.recovering = true
		blockchains.restoreFromDatabase()
		blockchains.recovering = false
	}
	if startMining {
		go blockchains.StartMining()
		go blockchains.ApplyLoop()
	}
	return blockchains
}

func (blockchains *Blockchains) GetHeight(symbol string) uint64 {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	return blockchains.chains[symbol].height
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
		heights[symbol] = chain.height
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
		Log("Failed to add tx to mempool consensus state")
		return false
	}
	blockchains.mempoolsLock.Lock()
	if _, ok := blockchains.mempools[symbol][tx.ID()]; ok {
		Log("Tx already in mempool")
		blockchains.mempoolsLock.Unlock()
		blockchains.chainsLock.Unlock()
		return false
	}

	Log("Tx added to mempool")
	// Add transaction to mempool
	blockchains.mempools[symbol][tx.ID()] = tx

	if blockchains.minerChosenToken == symbol {
		// Send transaction to miner
		blockchains.mempoolsLock.Unlock()
		blockchains.chainsLock.Unlock()
		blockchains.miner.minerCh <- MinerMsg{tx, false}
	} else {
		blockchains.rollbackGenericTransaction(symbol, tx)
		blockchains.mempoolsLock.Unlock()
		blockchains.chainsLock.Unlock()
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
		Log("Starting %v block", blockchains.minerChosenToken)
		blockchains.miner.minerCh <- MinerMsg{NewBlockMsg{TOKEN_BLOCK, blockchains.chains[blockchains.minerChosenToken].tipHash, blockchains.minerChosenToken}, true}
	}

	blockchains.mempoolsLock.Lock()
	var txInPool []GenericTransaction
	for _, tx := range blockchains.mempools[blockchains.minerChosenToken] {
		txInPool = append(txInPool, tx)
	}
	blockchains.mempoolsLock.Unlock()
	Log("%v TX in mempool to revalidate and send", len(txInPool))

	//Send stuff currently in mem pool (re-validate too)
	go func(currentToken string, transactions []GenericTransaction) {
		for _, tx := range transactions {
			blockchains.chainsLock.Lock()
			blockchains.mempoolsLock.Lock()
			if currentToken != blockchains.minerChosenToken {
				blockchains.chainsLock.Unlock()
				blockchains.mempoolsLock.Unlock()
				return
			}

			// Validate transaction
			if !blockchains.addGenericTransaction(blockchains.minerChosenToken, tx, blockchains.mempoolUncommitted[blockchains.minerChosenToken]) {
				delete(blockchains.mempools[blockchains.minerChosenToken], tx.ID())
				Log("TX in mempool failed revalidation")
				blockchains.mempoolsLock.Unlock()
				blockchains.chainsLock.Unlock()
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
			Log("Received block from miner")
			// When miner finishes, try to add a block
			blockchains.chainsLock.Lock()
			//state was applied during validation so just add to chain
			if !bytes.Equal(blockchains.chains[blockMsg.Symbol].tipHash, blockMsg.Block.PrevBlockHash) {
				//block failed so retry
				Log("miner prevBlockHash does not match tipHash %x != %x", blockchains.chains[blockMsg.Symbol].tipHash, blockMsg.Block.PrevBlockHash)

				blockchains.mempoolsLock.Lock()
				blockchains.mempoolUncommitted[blockMsg.Symbol].undoTransactions(blockMsg.Symbol, blockchains)
				blockchains.mempoolUncommitted[blockMsg.Symbol] = &UncommittedTransactions{}
				//WARNING taking both locks always take chain lock first
				blockchains.mempoolsLock.Unlock()
				blockchains.chainsLock.Unlock()

			} else {
				blockchains.chains[blockMsg.Symbol].AddBlock(blockMsg.Block)
				blockchains.chainsLock.Unlock()

				blockchains.mempoolsLock.Lock()
				txInBlock := blockMsg.TxInBlock
				Log("%v tx mined in block and added to chain %v", len(txInBlock), blockMsg.Symbol)

				var newUncommitted []GenericTransaction
				for _, tx := range blockchains.mempoolUncommitted[blockMsg.Symbol].transactions {
					if _, ok := txInBlock[tx.ID()]; !ok {
						blockchains.rollbackGenericTransaction(blockMsg.Symbol, tx)
						newUncommitted = append(newUncommitted, tx)
					} else {
						Log("%tx mined in block and deleted from mempool %v", tx)
						delete(blockchains.mempools[blockMsg.Symbol], tx.ID())
					}
				}

				blockchains.mempoolUncommitted[blockMsg.Symbol].transactions = newUncommitted
				blockchains.mempoolsLock.Unlock()
			}

		case symbol := <-blockchains.stopMiningCh:
			Log("Restarting mining due to new block being added or removed")
			if symbol != blockchains.minerChosenToken {
				continue
			}
		}

		blockchains.StartMining()

	}
}
