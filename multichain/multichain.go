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

func CreateMultichain(dbName string, startMining bool) *Multichain {
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

func (multichain *Multichain) RollbackAndAddBlocks(symbol string, height uint64) {
	//TODO:
}

func (multichain *Multichain) AddBlock(symbol string, block Block, takeLock bool) bool {
	return multichain.AddBlocks(symbol, []Block{block}, takeLock)
}

func (multichain *Multichain) AddBlocks(symbol string, blocks []Block, takeLock bool) bool {
	defer multichain.matcher.FindAllMatches()

	if takeLock {
		multichain.chainsLock.Lock()
		defer multichain.chainsLock.Unlock()
	}

	if _, ok := multichain.chains[symbol]; !ok {
		Log("AddBlocks failed as symbol not found: %v", symbol)
		return false
	}

	if !multichain.recovering && takeLock {
		go func() { multichain.stopMiningCh <- symbol }()
		multichain.mempoolsLock.Lock()
		Log("AddBlocks undoing transactions for: %v", symbol)
		multichain.mempoolUncommitted[symbol].undoTransactions(symbol, multichain, false)
		multichain.mempoolUncommitted[symbol] = &UncommittedTransactions{}
		multichain.mempoolsLock.Unlock()
	}

	blocksAdded := 0
	var uncommitted UncommittedTransactions
	failed := false
	for _, block := range blocks {
		if !bytes.Equal(multichain.chains[symbol].tipHash, block.PrevBlockHash) {
			Log("prevBlockHash does not match tipHash for symbol %v %x != %x \n", symbol, blockchains.chains[symbol].tipHash, block.PrevBlockHash)
			failed = true
			break
		}

		if multichain.chains[symbol].height > 1 {
			pow := NewProofOfWork(&block)
			if !pow.Validate() {
				Log("Proof of work of block is invalid")
			}
		}
		if symbol == MATCH_CHAIN {
			if !multichain.addMatchData(block.Data.(MatchData), &uncommitted) {
				failed = true
				break
			}
		} else {
			if !multichain.addTokenData(symbol, block.Data.(TokenData), &uncommitted) {
				failed = true
				break
			}
		}
		multichain.chains[symbol].AddBlock(block)
		blocksAdded++
	}
	if failed {
		Log("Adding blocks failed, rolling back.")
		uncommitted.undoTransactions(symbol, multichain, true)
		for i := 0; i < blocksAdded; i++ {
			multichain.chains[symbol].RemoveLastBlock()
		}
		return false
	}
	Log("AddBlocks added %v blocks to %v chain", blocksAdded, symbol)

	return true
}

func (multichain *Multichain) RollbackToHeight(symbol string, height uint64, takeLock bool, takeMempoolLock bool) {
	if takeLock {
		defer multichain.matcher.FindAllMatches()
		multichain.chainsLock.Lock()
		defer multichain.chainsLock.Unlock()
	}
	Log("Rolling back %v block to height: %v \n", symbol, height)

	if _, ok := multichain.chains[symbol]; !ok {
		Log("RollbackToHeight failed as symbol not found: ", symbol)
		return
	}

	if !multichain.recovering {
		if takeMempoolLock {
			go func() { multichain.stopMiningCh <- symbol }()
			multichain.mempoolsLock.Lock()
		}
		Log("RollbackToHeight undoing transactions for: %v", symbol)
		multichain.mempoolUncommitted[symbol].undoTransactions(symbol, multichain, false)
		multichain.mempoolUncommitted[symbol] = &UncommittedTransactions{}
		if takeMempoolLock {
			multichain.mempoolsLock.Unlock()
		}
	}

	if symbol == MATCH_CHAIN {
		multichain.rollbackMatchToHeight(height)
	} else {
		multichain.rollbackTokenToHeight(symbol, height)
	}

}

func (multichain *Multichain) AddTokenChain(createToken CreateToken) {
	//no lock needed
	Log("Adding token chain")
	chain := NewBlockchain(multichain.db, createToken.TokenInfo.Symbol)
	multichain.chains[createToken.TokenInfo.Symbol] = chain
	multichain.mempools[createToken.TokenInfo.Symbol] = make(map[string]GenericTransaction)
	multichain.mempoolUncommitted[createToken.TokenInfo.Symbol] = &UncommittedTransactions{}

	if !multichain.recovering { //recovery will replay this block normally
		multichain.AddBlock(createToken.TokenInfo.Symbol, *NewTokenGenesisBlock(createToken), false)
	}
}

func (multichain *Multichain) RemoveTokenChain(createToken CreateToken) {
	Log("Removing token chain")
	multichain.chains[createToken.TokenInfo.Symbol].DeleteStorage()
	delete(multichain.chains, createToken.TokenInfo.Symbol)
	delete(multichain.mempools, createToken.TokenInfo.Symbol)
	delete(multichain.mempoolUncommitted, createToken.TokenInfo.Symbol)
}

////////////////////////////////
// State Getters

func (multichain *Multichain) GetBlock(symbol string, blockhash []byte) (*Block, error) {
	multichain.chainsLock.RLock()
	defer multichain.chainsLock.RUnlock()

	bc, ok := multichain.chains[symbol]
	if !ok {
		return nil, errors.New("invalid chain")
	}

	block, blockErr := bc.GetBlock(blockhash)
	return block, blockErr
}

func (multichain *Multichain) GetHeights() map[string]uint64 {
	multichain.chainsLock.RLock()
	defer multichain.chainsLock.RUnlock()
	heights := make(map[string]uint64)
	for symbol, chain := range multichain.chains {
		heights[symbol] = chain.height
	}
	return heights
}

func (multichain *Multichain) GetHeight(symbol string) uint64 {
	multichain.chainsLock.RLock()
	defer multichain.chainsLock.RUnlock()
	return multichain.chains[symbol].height
}

func (multichain *Multichain) GetBlockhashes() map[string][][]byte {
	multichain.chainsLock.RLock()
	defer multichain.chainsLock.RUnlock()
	blockhashes := make(map[string][][]byte)
	for symbol, chain := range multichain.chains {
		blockhashes[symbol] = chain.blockhashes
	}
	return blockhashes
}

func (multichain *Multichain) GetBalance(symbol string, address [addressLength]byte) (uint64, bool) {
	multichain.chainsLock.RLock()
	defer multichain.chainsLock.RUnlock()
	state, ok := multichain.consensusState.tokenStates[symbol]
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

func (multichain *Multichain) GetUnclaimedBalance(symbol string, address [addressLength]byte) (uint64, bool) {
	multichain.chainsLock.RLock()
	defer multichain.chainsLock.RUnlock()
	state, ok := multichain.consensusState.tokenStates[symbol]
	if !ok {
		Log("GetBalance failed symbol %v does not exist", symbol)
		return 0, false
	}

	balance, ok := state.unclaimedFunds[address]
	return balance, ok
}

func (multichain *Multichain) GetOpenOrders(symbol string) map[uint64]Order {
	multichain.chainsLock.RLock()
	defer multichain.chainsLock.RUnlock()
	state, _ := multichain.consensusState.tokenStates[symbol]
	return state.openOrders
}

////////////////////////////////
// Private Implementation

func (multichain *Multichain) addTokenData(symbol string, tokenData TokenData, uncommitted *UncommittedTransactions) bool {
	for _, claimFunds := range tokenData.ClaimFunds {
		tx := GenericTransaction{
			Transaction:     claimFunds,
			TransactionType: CLAIM_FUNDS,
		}
		if !multichain.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}

	for _, transfer := range tokenData.Transfers {
		tx := GenericTransaction{
			Transaction:     transfer,
			TransactionType: TRANSFER,
		}
		if !multichain.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}

	for _, order := range tokenData.Orders {
		tx := GenericTransaction{
			Transaction:     order,
			TransactionType: ORDER,
		}
		if !multichain.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}
	return true
}

func (multichain *Multichain) addMatchData(matchData MatchData, uncommitted *UncommittedTransactions) bool {
	for _, createToken := range matchData.CreateTokens {
		tx := GenericTransaction{
			Transaction:     createToken,
			TransactionType: CREATE_TOKEN,
		}
		if !multichain.addGenericTransaction(MATCH_CHAIN, tx, uncommitted, true) {
			return false
		}
	}

	for _, match := range matchData.Matches {
		tx := GenericTransaction{
			Transaction:     match,
			TransactionType: MATCH,
		}
		if !multichain.addGenericTransaction(MATCH_CHAIN, tx, uncommitted, true) {
			return false
		}
	}

	for _, cancelOrder := range matchData.CancelOrders {
		tx := GenericTransaction{
			Transaction:     cancelOrder,
			TransactionType: CANCEL_ORDER,
		}
		if !multichain.addGenericTransaction(MATCH_CHAIN, tx, uncommitted, true) {
			return false
		}
	}

	return true
}

func (multichain *Multichain) rollbackTokenToHeight(symbol string, height uint64) {
	if height >= multichain.chains[symbol].height {
		return
	}
	blocksToRemove := multichain.chains[symbol].height - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := multichain.chains[symbol].RemoveLastBlock().(TokenData)
		for j := len(removedData.Orders) - 1; j >= 0; j-- {
			multichain.consensusState.RollbackUntilRollbackOrderSucceeds(symbol, removedData.Orders[j], multichain, true)
			multichain.matcher.RemoveOrder(removedData.Orders[j], symbol)
			multichain.consensusState.RollbackOrder(symbol, removedData.Orders[j])
		}

		for j := len(removedData.Transfers) - 1; j >= 0; j-- {
			multichain.consensusState.RollbackTransfer(symbol, removedData.Transfers[j])
		}

		for j := len(removedData.ClaimFunds) - 1; j >= 0; j-- {
			multichain.consensusState.RollbackClaimFunds(symbol, removedData.ClaimFunds[j])
		}
	}
}

func (multichain *Multichain) rollbackMatchToHeight(height uint64) {
	if height >= multichain.chains[MATCH_CHAIN].height {
		return
	}

	blocksToRemove := multichain.chains[MATCH_CHAIN].height - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := multichain.chains[MATCH_CHAIN].RemoveLastBlock().(MatchData)

		for j := len(removedData.CancelOrders) - 1; j >= 0; j-- {
			multichain.consensusState.RollbackUntilRollbackCancelOrderSucceeds(removedData.CancelOrders[j], multichain, true)
			multichain.matcher.RemoveCancelOrder(removedData.CancelOrders[j])
			multichain.consensusState.RollbackCancelOrder(removedData.CancelOrders[j])
		}

		for j := len(removedData.Matches) - 1; j >= 0; j-- {
			multichain.consensusState.RollbackUntilRollbackMatchSucceeds(removedData.Matches[j], multichain, true)
			buyOrder, sellOrder := multichain.consensusState.GetBuySellOrdersForMatch(removedData.Matches[j])
			multichain.matcher.RemoveMatch(removedData.Matches[j], buyOrder, sellOrder)
			multichain.consensusState.RollbackMatch(removedData.Matches[j])
		}

		for j := len(removedData.CreateTokens) - 1; j >= 0; j-- {
			multichain.consensusState.RollbackCreateToken(removedData.CreateTokens[j], multichain)
		}
	}
}

//only send mined transactions to matcher
func (multichain *Multichain) addGenericTransaction(symbol string, transaction GenericTransaction, uncommitted *UncommittedTransactions, mined bool) bool {
	success := false

	switch transaction.TransactionType {
	case ORDER:
		success = multichain.consensusState.AddOrder(symbol, transaction.Transaction.(Order))
	case CLAIM_FUNDS:
		success = multichain.consensusState.AddClaimFunds(symbol, transaction.Transaction.(ClaimFunds))
	case TRANSFER:
		success = multichain.consensusState.AddTransfer(symbol, transaction.Transaction.(Transfer))
	case MATCH:
		success = multichain.consensusState.AddMatch(transaction.Transaction.(Match))
	case CANCEL_ORDER:
		multichain.consensusState.AddCancelOrder(transaction.Transaction.(CancelOrder))
	case CREATE_TOKEN:
		success = multichain.consensusState.AddCreateToken(transaction.Transaction.(CreateToken), multichain)
	}

	if !success {
		return false
	}
	uncommitted.addTransaction(transaction)

	if mined {
		switch transaction.TransactionType {
		case ORDER:
			multichain.matcher.AddOrder(transaction.Transaction.(Order), symbol)
		case MATCH:
			multichain.matcher.AddMatch(transaction.Transaction.(Match))
		case CANCEL_ORDER:
			multichain.matcher.AddCancelOrder(transaction.Transaction.(CancelOrder), symbol)
		}
	}

	return true
}

func (multichain *Multichain) rollbackGenericTransaction(symbol string, transaction GenericTransaction, mined bool) {
	var buyOrder, sellOrder Order
	if mined && transaction.TransactionType == MATCH {
		buyOrder, sellOrder = multichain.consensusState.GetBuySellOrdersForMatch(transaction.Transaction.(Match))
	}

	switch transaction.TransactionType {
	case ORDER:
		multichain.consensusState.RollbackUntilRollbackOrderSucceeds(symbol, transaction.Transaction.(Order), multichain, mined)
		multichain.consensusState.RollbackOrder(symbol, transaction.Transaction.(Order))
	case CLAIM_FUNDS:
		multichain.consensusState.RollbackClaimFunds(symbol, transaction.Transaction.(ClaimFunds))
	case TRANSFER:
		multichain.consensusState.RollbackTransfer(symbol, transaction.Transaction.(Transfer))
	case MATCH:
		multichain.consensusState.RollbackUntilRollbackMatchSucceeds(transaction.Transaction.(Match), multichain, mined)
		multichain.consensusState.RollbackMatch(transaction.Transaction.(Match))
	case CANCEL_ORDER:
		multichain.consensusState.RollbackUntilRollbackCancelOrderSucceeds(transaction.Transaction.(CancelOrder), multichain, mined)
		multichain.consensusState.RollbackCancelOrder(transaction.Transaction.(CancelOrder))
	case CREATE_TOKEN:
		multichain.consensusState.RollbackCreateToken(transaction.Transaction.(CreateToken), multichain)
	}

	if mined {
		switch transaction.TransactionType {
		case ORDER:
			multichain.matcher.RemoveOrder(transaction.Transaction.(Order), symbol)
		case MATCH:
			multichain.matcher.RemoveMatch(transaction.Transaction.(Match), buyOrder, sellOrder)
		case CANCEL_ORDER:
			multichain.matcher.RemoveCancelOrder(transaction.Transaction.(CancelOrder))
		}
	}
}

func (multichain *Multichain) restoreFromDatabase() {
	iterators := make(map[string]*BlockchainForwardIterator)
	chainsDone := make(map[string]bool)
	done := false
	for !done {
		for symbol, chain := range multichain.chains {
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
					if !multichain.addMatchData(block.Data.(MatchData), &uncommitted) {
						uncommitted.undoTransactions(MATCH_CHAIN, multichain, true)
						iterator.Undo()
						break
					}
				} else {
					var uncommitted UncommittedTransactions
					if !multichain.addTokenData(symbol, block.Data.(TokenData), &uncommitted) {
						uncommitted.undoTransactions(symbol, multichain, true)
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
		for symbol := range multichain.chains {
			if !chainsDone[symbol] {
				done = false
				break
			}
		}
	}
}

func (multichain *Multichain) Cleanup() {
	multichain.chainsLock.Lock()
	multichain.mempoolsLock.Lock()

	close(multichain.miner.minerCh)
	multichain.db.Close()
}

func (multichain *Multichain) DumpChains(amt uint64) string {
	multichain.chainsLock.RLock()
	defer multichain.chainsLock.RUnlock()

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%v\n", len(multichain.chains)))

	for symbol, chain := range multichain.chains {
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
