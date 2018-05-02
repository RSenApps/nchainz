package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"math/rand"
	"os"
	"sync"
)

const MATCH_CHAIN = "MATCH"
const NATIVE_CHAIN = "NATIVE"

type Blockchains struct {
	chains          map[string]*Blockchain
	consensusState  ConsensusState
	chainsLock      *sync.RWMutex
	db              *bolt.DB
	mempools        map[string]map[*GenericTransaction]bool // map of sets
	mempoolsLock    *sync.Mutex
	finishedBlockCh chan BlockMsg
	stopMiningCh    chan bool
	miner           *Miner
	recovering      bool
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
		case ORDER:
			state.RollbackOrder(symbol, transaction.(Order))
		case CANCEL_ORDER:
			state.RollbackCancelOrder(transaction.(CancelOrder))
		case CLAIM_FUNDS:
			state.RollbackClaimFunds(symbol, transaction.(ClaimFunds))
		case TRANSFER:
			state.RollbackTransfer(symbol, transaction.(Transfer))
		case MATCH:
			state.RollbackMatch(transaction.(Match))
		case CREATE_TOKEN:
			state.RollbackCreateToken(transaction.(CreateToken))
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

		for _, claimFunds := range removedData.ClaimFunds {
			blockchains.consensusState.RollbackClaimFunds(symbol, claimFunds)
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

		for _, cancelOrder := range removedData.CancelOrders {
			blockchains.consensusState.RollbackCancelOrder(cancelOrder)
		}

		for _, createToken := range removedData.CreateTokens {
			blockchains.consensusState.RollbackCreateToken(createToken)
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

	for _, claimFunds := range tokenData.ClaimFunds {
		if !blockchains.consensusState.AddClaimFunds(symbol, claimFunds) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     claimFunds,
			transactionType: CLAIM_FUNDS,
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

	for _, cancelOrder := range matchData.CancelOrders {
		if !blockchains.consensusState.AddCancelOrder(cancelOrder) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     cancelOrder,
			transactionType: CANCEL_ORDER,
		})
	}

	for _, createToken := range matchData.CreateTokens {
		if !blockchains.consensusState.AddCreateToken(createToken, blockchains) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     createToken,
			transactionType: CREATE_TOKEN,
		})
	}
	return true
}

func (blockchains *Blockchains) AddBlock(symbol string, block Block, takeLock bool) bool {
	return blockchains.AddBlocks(symbol, []Block{block}, takeLock)
}

func (blockchains *Blockchains) AddBlocks(symbol string, blocks []Block, takeLock bool) bool {
	blockchains.stopMiningCh <- true

	if takeLock {
		blockchains.chainsLock.Lock()
		defer blockchains.chainsLock.Unlock()
	}
	blocksAdded := 0
	var uncommitted UncommittedTransactions
	failed := false
	for _, block := range blocks {
		if !bytes.Equal(blockchains.chains[symbol].tipHash, block.PrevBlockHash) {
			log.Printf("prevBlockHash does not match tipHash %v != %v \n", blockchains.chains[symbol].tipHash, block.PrevBlockHash)
			failed = true
			break
		}
		pow := NewProofOfWork(&block)
		if !pow.Validate() {
			log.Println("Proof of work of block is invalid")
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
		uncommitted.undoTransactions(symbol, &blockchains.consensusState)
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
	if !blockchains.recovering { //recovery will replay this block normally
		blockchains.AddBlock(createToken.TokenInfo.Symbol, *NewTokenGenesisBlock(createToken), false)
	}
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
		for symbol, _ := range blockchains.chains {
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
		log.Panic(err)
	}
	blockchains.db = db

	blockchains.finishedBlockCh = make(chan BlockMsg)
	blockchains.miner = NewMiner(blockchains.finishedBlockCh)
	blockchains.stopMiningCh = make(chan bool, 1000)
	blockchains.consensusState = NewConsensusState()
	blockchains.chains = make(map[string]*Blockchain)
	blockchains.chains[MATCH_CHAIN] = NewBlockchain(db, MATCH_CHAIN)
	blockchains.chainsLock = &sync.RWMutex{}
	blockchains.mempoolsLock = &sync.Mutex{}

	blockchains.mempools = make(map[string]map[*GenericTransaction]bool)
	blockchains.mempools[MATCH_CHAIN] = make(map[*GenericTransaction]bool)
	if newDatabase {
		blockchains.recovering = false
		blockchains.AddBlock(MATCH_CHAIN, *NewGenesisBlock(), false)
	} else {
		blockchains.recovering = true
		blockchains.restoreFromDatabase()
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

func (blockchains *Blockchains) ValidateTransaction(tx GenericTransaction, symbol string) bool {
	validated := false
	switch tx.transactionType {
	case MATCH:
		validated = blockchains.consensusState.AddMatch(tx.transaction.(Match))
	case ORDER:
		validated = blockchains.consensusState.AddOrder(symbol, tx.transaction.(Order))
	case TRANSFER:
		validated = blockchains.consensusState.AddTransfer(symbol, tx.transaction.(Transfer))
	case CANCEL_ORDER:
		validated = blockchains.consensusState.AddCancelOrder(tx.transaction.(CancelOrder))
	case CLAIM_FUNDS:
		validated = blockchains.consensusState.AddClaimFunds(symbol, tx.transaction.(ClaimFunds))
	case CREATE_TOKEN:
		validated = blockchains.consensusState.AddCreateToken(tx.transaction.(CreateToken), blockchains)
	}
	return validated
}

func (blockchains *Blockchains) AddTransactionToMempool(tx GenericTransaction, symbol string) {
	blockchains.mempoolsLock.Lock()
	if _, ok := blockchains.mempools[symbol][&tx]; ok {
		return
	}

	fmt.Println("Add tx")

	// Validate transaction
	if blockchains.ValidateTransaction(tx, symbol) {
		fmt.Println("Tx validated")

		// Add transaction to mempool
		blockchains.mempools[symbol][&tx] = true
		blockchains.mempoolsLock.Unlock()

		fmt.Println("Blockchains sending to miner channel")
		fmt.Println("\n\n")
		// Send transaction to miner
		blockchains.miner.minerCh <- MinerMsg{tx, false}
	} else {
		blockchains.mempoolsLock.Unlock()
	}
}

func (blockchains *Blockchains) StartMining() {
	blockchains.mempoolsLock.Lock()
	fmt.Println("Start mining")
	// Pick a random token to start mining
	var tokens []string
	for token := range blockchains.mempools {
		tokens = append(tokens, token)
	}

	if len(tokens) > 0 {
		fmt.Println("Tokens are", tokens)
		chosenToken := tokens[rand.Intn(len(tokens))]
		blockchains.mempoolsLock.Unlock()

		// Send new block message
		fmt.Println("About to send new block")
		switch chosenToken {
		case MATCH_CHAIN:
			fmt.Println("Starting match block")
			blockchains.miner.minerCh <- MinerMsg{NewBlockMsg{MATCH_BLOCK, blockchains.chains[chosenToken].tipHash, chosenToken}, true}
		default:
			fmt.Println("Starting native block")
			blockchains.miner.minerCh <- MinerMsg{NewBlockMsg{TOKEN_BLOCK, blockchains.chains[chosenToken].tipHash, chosenToken}, true}
		}
		fmt.Println("After sending new block")
	} else {
		blockchains.mempoolsLock.Unlock()
	}
}

func (blockchains *Blockchains) ApplyLoop() {
	for {
		select {
		case blockMsg := <-blockchains.finishedBlockCh:
			// When miner finishes, try to add a block
			blockchains.AddBlock(blockMsg.Symbol, blockMsg.Block, true)
		case <-blockchains.stopMiningCh:
			// Stop mining if a new block is added
			// Reevaluate transactions in the mempool
			for token := range blockchains.mempools {
				for tx := range blockchains.mempools[token] {
					if !blockchains.ValidateTransaction(*tx, token) {
						delete(blockchains.mempools[token], tx)
					}
				}
			}
			// Start a new miner
			// blockchains.StartMining() // TODO
		}
	}
}
