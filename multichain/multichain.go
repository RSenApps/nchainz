package multichain

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"math/rand"
	"os"
	"sync"
)

const MATCH_CHAIN = "MATCH"
const NATIVE_CHAIN = "NATIVE"

type Multichain struct {
	chains         map[string]*Blockchain
	consensusState ConsensusState
	chainsLock     *sync.RWMutex // Lock on consensus state and chains
	db             *bolt.DB
	recovering     bool
}

func CreateNewBlockchains(dbName string, startMining bool) *Multichain {
	//instantiates state and blockchains
	blockchains := &Multichain{}
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

	blockchains.matcher = StartMatcher(blockchains, nil)
	blockchains.consensusState = NewConsensusState()
	blockchains.chains = make(map[string]*Blockchain)
	blockchains.chains[MATCH_CHAIN] = NewBlockchain(db, MATCH_CHAIN)
	blockchains.chainsLock = &sync.RWMutex{}
	blockchains.mempoolsLock = &sync.Mutex{}

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
		go blockchains.StartMining(true)
		go blockchains.ApplyLoop()
	}
	return blockchains
}

////////////////////////////////
// Chain Manipulation

func (blockchains *Multichain) RollbackAndAddBlocks(symbol string, height uint64) {
	//TODO:
}

func (blockchains *Multichain) AddBlock(symbol string, block Block, takeLock bool) bool {
	return blockchains.AddBlocks(symbol, []Block{block}, takeLock)
}

func (blockchains *Multichain) AddBlocks(symbol string, blocks []Block, takeLock bool) bool {
	defer blockchains.matcher.FindAllMatches()

	if takeLock {
		blockchains.chainsLock.Lock()
		defer blockchains.chainsLock.Unlock()
	}

	if _, ok := blockchains.chains[symbol]; !ok {
		Log("AddBlocks failed as symbol not found: %v", symbol)
		return false
	}

	if !blockchains.recovering && takeLock {
		go func() { blockchains.stopMiningCh <- symbol }()
		blockchains.mempoolsLock.Lock()
		Log("AddBlocks undoing transactions for: %v", symbol)
		blockchains.mempoolUncommitted[symbol].undoTransactions(symbol, blockchains, false)
		blockchains.mempoolUncommitted[symbol] = &UncommittedTransactions{}
		blockchains.mempoolsLock.Unlock()
	}

	blocksAdded := 0
	var uncommitted UncommittedTransactions
	failed := false
	for _, block := range blocks {
		if !bytes.Equal(blockchains.chains[symbol].tipHash, block.PrevBlockHash) {
			Log("prevBlockHash does not match tipHash for symbol %v %x != %x \n", symbol, blockchains.chains[symbol].tipHash, block.PrevBlockHash)
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
		uncommitted.undoTransactions(symbol, blockchains, true)
		for i := 0; i < blocksAdded; i++ {
			blockchains.chains[symbol].RemoveLastBlock()
		}
		return false
	}
	Log("AddBlocks added %v blocks to %v chain", blocksAdded, symbol)

	return true
}

func (blockchains *Multichain) RollbackToHeight(symbol string, height uint64, takeLock bool, takeMempoolLock bool) {
	if takeLock {
		defer blockchains.matcher.FindAllMatches()
		blockchains.chainsLock.Lock()
		defer blockchains.chainsLock.Unlock()
	}
	Log("Rolling back %v block to height: %v \n", symbol, height)

	if _, ok := blockchains.chains[symbol]; !ok {
		Log("RollbackToHeight failed as symbol not found: ", symbol)
		return
	}

	if !blockchains.recovering {
		if takeMempoolLock {
			go func() { blockchains.stopMiningCh <- symbol }()
			blockchains.mempoolsLock.Lock()
		}
		Log("RollbackToHeight undoing transactions for: %v", symbol)
		blockchains.mempoolUncommitted[symbol].undoTransactions(symbol, blockchains, false)
		blockchains.mempoolUncommitted[symbol] = &UncommittedTransactions{}
		if takeMempoolLock {
			blockchains.mempoolsLock.Unlock()
		}
	}

	if symbol == MATCH_CHAIN {
		blockchains.rollbackMatchToHeight(height)
	} else {
		blockchains.rollbackTokenToHeight(symbol, height)
	}

}

func (blockchains *Multichain) AddTokenChain(createToken CreateToken) {
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

func (blockchains *Multichain) RemoveTokenChain(createToken CreateToken) {
	Log("Removing token chain")
	blockchains.chains[createToken.TokenInfo.Symbol].DeleteStorage()
	delete(blockchains.chains, createToken.TokenInfo.Symbol)
	delete(blockchains.mempools, createToken.TokenInfo.Symbol)
	delete(blockchains.mempoolUncommitted, createToken.TokenInfo.Symbol)
}

////////////////////////////////
// State Getters

func (blockchains *Multichain) GetBlock(symbol string, blockhash []byte) (*Block, error) {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()

	bc, ok := blockchains.chains[symbol]
	if !ok {
		return nil, errors.New("invalid chain")
	}

	block, blockErr := bc.GetBlock(blockhash)
	return block, blockErr
}

func (blockchains *Multichain) GetHeights() map[string]uint64 {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	heights := make(map[string]uint64)
	for symbol, chain := range blockchains.chains {
		heights[symbol] = chain.height
	}
	return heights
}

func (blockchains *Multichain) GetHeight(symbol string) uint64 {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	return blockchains.chains[symbol].height
}

func (blockchains *Multichain) GetBlockhashes() map[string][][]byte {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	blockhashes := make(map[string][][]byte)
	for symbol, chain := range blockchains.chains {
		blockhashes[symbol] = chain.blockhashes
	}
	return blockhashes
}

func (blockchains *Multichain) GetBalance(symbol string, address [addressLength]byte) (uint64, bool) {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	state, ok := blockchains.consensusState.tokenStates[symbol]
	if !ok {
		Log("GetBalance failed symbol %v does not exist", symbol)
		return 0, false
	}

	balance, ok := state.balances[address]
	if !ok {
		return 0, true
	} else {
		return balance, ok
	}
}

func (blockchains *Multichain) GetUnclaimedBalance(symbol string, address [addressLength]byte) (uint64, bool) {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	state, ok := blockchains.consensusState.tokenStates[symbol]
	if !ok {
		Log("GetBalance failed symbol %v does not exist", symbol)
		return 0, false
	}

	balance, ok := state.unclaimedFunds[address]
	return balance, ok
}

func (blockchains *Multichain) GetOpenOrders(symbol string) map[uint64]Order {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()
	state, _ := blockchains.consensusState.tokenStates[symbol]
	return state.openOrders
}

////////////////////////////////
// Private Implementation

func (blockchains *Multichain) addTokenData(symbol string, tokenData TokenData, uncommitted *UncommittedTransactions) bool {
	for _, claimFunds := range tokenData.ClaimFunds {
		tx := GenericTransaction{
			Transaction:     claimFunds,
			TransactionType: CLAIM_FUNDS,
		}
		if !blockchains.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}

	for _, transfer := range tokenData.Transfers {
		tx := GenericTransaction{
			Transaction:     transfer,
			TransactionType: TRANSFER,
		}
		if !blockchains.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}

	for _, order := range tokenData.Orders {
		tx := GenericTransaction{
			Transaction:     order,
			TransactionType: ORDER,
		}
		if !blockchains.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}
	return true
}

func (blockchains *Multichain) addMatchData(matchData MatchData, uncommitted *UncommittedTransactions) bool {
	for _, createToken := range matchData.CreateTokens {
		tx := GenericTransaction{
			Transaction:     createToken,
			TransactionType: CREATE_TOKEN,
		}
		if !blockchains.addGenericTransaction(MATCH_CHAIN, tx, uncommitted, true) {
			return false
		}
	}

	for _, match := range matchData.Matches {
		tx := GenericTransaction{
			Transaction:     match,
			TransactionType: MATCH,
		}
		if !blockchains.addGenericTransaction(MATCH_CHAIN, tx, uncommitted, true) {
			return false
		}
	}

	for _, cancelOrder := range matchData.CancelOrders {
		tx := GenericTransaction{
			Transaction:     cancelOrder,
			TransactionType: CANCEL_ORDER,
		}
		if !blockchains.addGenericTransaction(MATCH_CHAIN, tx, uncommitted, true) {
			return false
		}
	}

	return true
}

func (blockchains *Multichain) rollbackTokenToHeight(symbol string, height uint64) {
	if height >= blockchains.chains[symbol].height {
		return
	}
	blocksToRemove := blockchains.chains[symbol].height - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.chains[symbol].RemoveLastBlock().(TokenData)
		for j := len(removedData.Orders) - 1; j >= 0; j-- {
			blockchains.consensusState.RollbackUntilRollbackOrderSucceeds(symbol, removedData.Orders[j], blockchains, true)
			blockchains.matcher.RemoveOrder(removedData.Orders[j], symbol)
			blockchains.consensusState.RollbackOrder(symbol, removedData.Orders[j])
		}

		for j := len(removedData.Transfers) - 1; j >= 0; j-- {
			blockchains.consensusState.RollbackTransfer(symbol, removedData.Transfers[j])
		}

		for j := len(removedData.ClaimFunds) - 1; j >= 0; j-- {
			blockchains.consensusState.RollbackClaimFunds(symbol, removedData.ClaimFunds[j])
		}
	}
}

func (blockchains *Multichain) rollbackMatchToHeight(height uint64) {
	if height >= blockchains.chains[MATCH_CHAIN].height {
		return
	}

	blocksToRemove := blockchains.chains[MATCH_CHAIN].height - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.chains[MATCH_CHAIN].RemoveLastBlock().(MatchData)

		for j := len(removedData.CancelOrders) - 1; j >= 0; j-- {
			blockchains.consensusState.RollbackUntilRollbackCancelOrderSucceeds(removedData.CancelOrders[j], blockchains, true)
			blockchains.matcher.RemoveCancelOrder(removedData.CancelOrders[j])
			blockchains.consensusState.RollbackCancelOrder(removedData.CancelOrders[j])
		}

		for j := len(removedData.Matches) - 1; j >= 0; j-- {
			blockchains.consensusState.RollbackUntilRollbackMatchSucceeds(removedData.Matches[j], blockchains, true)
			buyOrder, sellOrder := blockchains.consensusState.GetBuySellOrdersForMatch(removedData.Matches[j])
			blockchains.matcher.RemoveMatch(removedData.Matches[j], buyOrder, sellOrder)
			blockchains.consensusState.RollbackMatch(removedData.Matches[j])
		}

		for j := len(removedData.CreateTokens) - 1; j >= 0; j-- {
			blockchains.consensusState.RollbackCreateToken(removedData.CreateTokens[j], blockchains)
		}
	}
}

//only send mined transactions to matcher
func (blockchains *Multichain) addGenericTransaction(symbol string, transaction GenericTransaction, uncommitted *UncommittedTransactions, mined bool) bool {
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
		blockchains.consensusState.AddCancelOrder(transaction.Transaction.(CancelOrder))
	case CREATE_TOKEN:
		success = blockchains.consensusState.AddCreateToken(transaction.Transaction.(CreateToken), blockchains)
	}

	if !success {
		return false
	}
	uncommitted.addTransaction(transaction)

	if mined {
		switch transaction.TransactionType {
		case ORDER:
			blockchains.matcher.AddOrder(transaction.Transaction.(Order), symbol)
		case MATCH:
			blockchains.matcher.AddMatch(transaction.Transaction.(Match))
		case CANCEL_ORDER:
			blockchains.matcher.AddCancelOrder(transaction.Transaction.(CancelOrder), symbol)
		}
	}

	return true
}

func (blockchains *Multichain) rollbackGenericTransaction(symbol string, transaction GenericTransaction, mined bool) {
	var buyOrder, sellOrder Order
	if mined && transaction.TransactionType == MATCH {
		buyOrder, sellOrder = blockchains.consensusState.GetBuySellOrdersForMatch(transaction.Transaction.(Match))
	}

	switch transaction.TransactionType {
	case ORDER:
		blockchains.consensusState.RollbackUntilRollbackOrderSucceeds(symbol, transaction.Transaction.(Order), blockchains, mined)
		blockchains.consensusState.RollbackOrder(symbol, transaction.Transaction.(Order))
	case CLAIM_FUNDS:
		blockchains.consensusState.RollbackClaimFunds(symbol, transaction.Transaction.(ClaimFunds))
	case TRANSFER:
		blockchains.consensusState.RollbackTransfer(symbol, transaction.Transaction.(Transfer))
	case MATCH:
		blockchains.consensusState.RollbackUntilRollbackMatchSucceeds(transaction.Transaction.(Match), blockchains, mined)
		blockchains.consensusState.RollbackMatch(transaction.Transaction.(Match))
	case CANCEL_ORDER:
		blockchains.consensusState.RollbackUntilRollbackCancelOrderSucceeds(transaction.Transaction.(CancelOrder), blockchains, mined)
		blockchains.consensusState.RollbackCancelOrder(transaction.Transaction.(CancelOrder))
	case CREATE_TOKEN:
		blockchains.consensusState.RollbackCreateToken(transaction.Transaction.(CreateToken), blockchains)
	}

	if mined {
		switch transaction.TransactionType {
		case ORDER:
			blockchains.matcher.RemoveOrder(transaction.Transaction.(Order), symbol)
		case MATCH:
			blockchains.matcher.RemoveMatch(transaction.Transaction.(Match), buyOrder, sellOrder)
		case CANCEL_ORDER:
			blockchains.matcher.RemoveCancelOrder(transaction.Transaction.(CancelOrder))
		}
	}
}

func (blockchains *Multichain) restoreFromDatabase() {
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
				if symbol == MATCH_CHAIN {
					var uncommitted UncommittedTransactions
					if !blockchains.addMatchData(block.Data.(MatchData), &uncommitted) {
						uncommitted.undoTransactions(MATCH_CHAIN, blockchains, true)
						iterator.Undo()
						break
					}
				} else {
					var uncommitted UncommittedTransactions
					if !blockchains.addTokenData(symbol, block.Data.(TokenData), &uncommitted) {
						uncommitted.undoTransactions(symbol, blockchains, true)
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

func (blockchains *Multichain) Cleanup() {
	blockchains.chainsLock.Lock()
	blockchains.mempoolsLock.Lock()

	close(blockchains.miner.minerCh)
	blockchains.db.Close()
}

func (blockchains *Multichain) DumpChains(amt uint64) string {
	blockchains.chainsLock.RLock()
	defer blockchains.chainsLock.RUnlock()

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%v\n", len(blockchains.chains)))

	for symbol, chain := range blockchains.chains {
		chainDump, numBlocks := chain.DumpChain(amt)
		header := fmt.Sprintf("%s %v %v\n", symbol, chain.height, numBlocks)

		buffer.WriteString(header)
		buffer.WriteString(chainDump)
	}

	return buffer.String()
}

type UncommittedTransactions struct {
	transactions []GenericTransaction
}

func (uncommitted *UncommittedTransactions) addTransaction(transaction GenericTransaction) {
	uncommitted.transactions = append(uncommitted.transactions, transaction)
}

func (uncommitted *UncommittedTransactions) undoTransactions(symbol string, blockchains *Multichain, mined bool) {
	for i := len(uncommitted.transactions) - 1; i >= 0; i-- {
		transaction := uncommitted.transactions[i]
		blockchains.rollbackGenericTransaction(symbol, transaction, mined)
	}
}

func (uncommitted *UncommittedTransactions) rollbackLast(symbol string, blockchains *Multichain, mined bool) {
	blockchains.rollbackGenericTransaction(symbol, uncommitted.transactions[len(uncommitted.transactions)-1], mined)
	uncommitted.transactions = uncommitted.transactions[:len(uncommitted.transactions)-1]
}
