package mempool

import (
	"bytes"
	"github.com/rsenapps/nchainz/multichain"
	"math/rand"
	"sync"
)

type Mempool struct {
	mempools           map[string]map[string]GenericTransaction // map of sets (of GenericTransaction ID)
	mempoolUncommitted map[string]*UncommittedTransactions
	mempoolsLock       *sync.Mutex // Lock on mempools
	finishedBlockCh    chan BlockMsg
	stopMiningCh       chan string
	miner              *Miner
	matcher            *Matcher
	minerChosenToken   string
	multichain         multichain.Multichain
}

func (mempool *Mempool) GetMultichain() *multichain.Multichain {
	return mempool.multichain
}

func (mempool *Mempool) CreateMempool(dbName string, startMining bool) *Mempool {

}

func (mempool *Mempool) AddTransaction(tx GenericTransaction, symbol string, takeLocks bool) bool {
	if blockchains.recovering {
		Log("Ignoring tx as still recovering")
		return false
	}

	unlock := func() {
		if takeLocks {
			blockchains.mempoolsLock.Unlock()
			blockchains.chainsLock.Unlock()
		}
	}

	if takeLocks {
		blockchains.chainsLock.Lock()
		blockchains.mempoolsLock.Lock()
	}

	if _, ok := blockchains.mempools[symbol][tx.ID()]; ok {
		Log("Tx already in mempool, %v", tx.ID())
		unlock()
		return false
	}

	// Validate transaction's signature
	if !Verify(tx, blockchains.consensusState) {
		Log("Failed due to invalid signature")
		unlock()
		return false
	}

	// Validate transaction against consensus state
	if !blockchains.addGenericTransaction(symbol, tx, blockchains.mempoolUncommitted[symbol], false) {
		Log("Failed to add tx to mempool consensus state")
		unlock()
		return false
	}

	Log("Tx added to mempool")
	// Add transaction to mempool
	blockchains.mempools[symbol][tx.ID()] = tx

	if blockchains.minerChosenToken == symbol {
		// Send transaction to miner
		minerRound := blockchains.miner.minerRound
		unlock()
		blockchains.miner.minerCh <- MinerMsg{tx, false, minerRound}
	} else {
		blockchains.mempoolUncommitted[symbol].rollbackLast(symbol, blockchains, false)
		unlock()
	}

	return true
}

func (mempool *Mempool) StartMining(chooseNewToken bool) {
	blockchains.chainsLock.Lock()
	blockchains.mempoolsLock.Lock()
	Log("Start mining")
	// Pick a random token to start mining
	if chooseNewToken {
		var tokens []string
		for token := range blockchains.mempools {
			tokens = append(tokens, token)
		}
		blockchains.minerChosenToken = tokens[rand.Intn(len(tokens))]
	}

	newToken := blockchains.minerChosenToken

	var txInPool []GenericTransaction
	for _, tx := range blockchains.mempools[blockchains.minerChosenToken] {
		txInPool = append(txInPool, tx)
	}

	blockchains.mempoolUncommitted[newToken].undoTransactions(newToken, blockchains, false)
	blockchains.mempoolUncommitted[newToken] = &UncommittedTransactions{}

	// Send new block message
	switch blockchains.minerChosenToken {
	case MATCH_CHAIN:
		Log("Starting match block")
		msg := MinerMsg{NewBlockMsg{MATCH_BLOCK, blockchains.chains[blockchains.minerChosenToken].tipHash, blockchains.minerChosenToken}, true, 0}
		blockchains.mempoolsLock.Unlock()
		blockchains.chainsLock.Unlock()
		blockchains.miner.minerCh <- msg
	default:
		Log("Starting %v block", blockchains.minerChosenToken)
		msg := MinerMsg{NewBlockMsg{TOKEN_BLOCK, blockchains.chains[blockchains.minerChosenToken].tipHash, blockchains.minerChosenToken}, true, 0}
		blockchains.mempoolsLock.Unlock()
		blockchains.chainsLock.Unlock()
		blockchains.miner.minerCh <- msg
	}

	Log("%v TX in mempool to revalidate and send", len(txInPool))
	//Send stuff currently in mem pool (re-validate too)
	go func(currentToken string, transactions []GenericTransaction) {
		for _, tx := range transactions {
			blockchains.chainsLock.Lock()
			blockchains.mempoolsLock.Lock()
			if currentToken != blockchains.minerChosenToken {
				blockchains.mempoolsLock.Unlock()
				blockchains.chainsLock.Unlock()
				return
			}

			// Validate transaction
			if !blockchains.addGenericTransaction(blockchains.minerChosenToken, tx, blockchains.mempoolUncommitted[blockchains.minerChosenToken], false) {
				delete(blockchains.mempools[blockchains.minerChosenToken], tx.ID())
				Log("TX in mempool failed revalidation")
				blockchains.mempoolsLock.Unlock()
				blockchains.chainsLock.Unlock()
				continue
			}
			minerRound := blockchains.miner.minerRound
			blockchains.mempoolsLock.Unlock()
			blockchains.chainsLock.Unlock()

			// Send transaction to miner
			Log("Sending from re-validated mempool")
			blockchains.miner.minerCh <- MinerMsg{tx, false, minerRound}
		}
	}(newToken, txInPool)
}

func (mempool *Mempool) ApplyLoop() {
	for {
		select {
		case blockMsg := <-blockchains.finishedBlockCh:
			Log("Received block from miner")
			// When miner finishes, try to add a block
			blockchains.chainsLock.Lock()
			//state was applied during validation so just add to chain
			chain, ok := blockchains.chains[blockMsg.Symbol]
			if !ok {
				Log("miner failed symbol no longer exists")
				blockchains.chainsLock.Unlock()
				continue
			}

			if !bytes.Equal(chain.tipHash, blockMsg.Block.PrevBlockHash) {
				//block failed so retry
				Log("miner prevBlockHash does not match tipHash %x != %x", chain.tipHash, blockMsg.Block.PrevBlockHash)

				blockchains.mempoolsLock.Lock()
				blockchains.mempoolUncommitted[blockMsg.Symbol].undoTransactions(blockMsg.Symbol, blockchains, false)
				blockchains.mempoolUncommitted[blockMsg.Symbol] = &UncommittedTransactions{}
				//WARNING taking both locks always take chain lock first
				blockchains.mempoolsLock.Unlock()
				blockchains.chainsLock.Unlock()
				blockchains.StartMining(false)
			} else {
				blockchains.chains[blockMsg.Symbol].AddBlock(blockMsg.Block)

				blockchains.mempoolsLock.Lock()
				txInBlock := blockMsg.TxInBlock
				Log("%v tx mined in block and added to chain %v", len(txInBlock), blockMsg.Symbol)

				var stillUncommitted UncommittedTransactions
				for _, tx := range blockchains.mempoolUncommitted[blockMsg.Symbol].transactions {
					if _, ok := txInBlock[tx.ID()]; !ok {
						stillUncommitted.transactions = append(stillUncommitted.transactions, tx)
					} else {
						Log("%tx mined in block and deleted from mempool %v", tx)
						switch tx.TransactionType {
						case ORDER:
							blockchains.matcher.AddOrder(tx.Transaction.(Order), blockMsg.Symbol)
						case MATCH:
							blockchains.matcher.AddMatch(tx.Transaction.(Match))
						case CANCEL_ORDER:
							blockchains.matcher.AddCancelOrder(tx.Transaction.(CancelOrder), blockMsg.Symbol)
						}
						delete(blockchains.mempools[blockMsg.Symbol], tx.ID())
					}
				}

				stillUncommitted.undoTransactions(blockMsg.Symbol, blockchains, false)
				blockchains.mempoolUncommitted[blockMsg.Symbol] = &UncommittedTransactions{}
				blockchains.mempoolsLock.Unlock()
				blockchains.chainsLock.Unlock()

				blockchains.matcher.FindAllMatches()
				blockchains.StartMining(true)

			}

		case symbol := <-blockchains.stopMiningCh:
			blockchains.chainsLock.Lock()
			if symbol != blockchains.minerChosenToken {
				blockchains.chainsLock.Unlock()
				continue
			}
			Log("Restarting mining due to new block being added or removed")
			blockchains.chainsLock.Unlock()
			blockchains.StartMining(false)
		}

	}
}
