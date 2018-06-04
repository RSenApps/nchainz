package multichain

import (
	"bytes"
	"github.com/rsenapps/nchainz/blockchain"
	"github.com/rsenapps/nchainz/miner"
	"github.com/rsenapps/nchainz/txs"
	"github.com/rsenapps/nchainz/utils"
	"math/rand"
	"sync"
)

type Mempool struct {
	transactions     map[string]map[string]txs.Tx // map of sets (of GenericTransaction ID)
	uncommitted      map[string]*UncommittedTransactions
	lock             *sync.Mutex // Lock on mempools
	finishedBlockCh  chan miner.BlockMsg
	stopMiningCh     chan string
	miner            *miner.Miner
	minerChosenToken string
	multichain       *Multichain
}

func (mempool *Mempool) Lock() {
	mempool.lock.Lock()
}

func (mempool *Mempool) Unlock() {
	mempool.lock.Unlock()
}

func (mempool *Mempool) GetMultichain() *Multichain {
	return mempool.multichain
}

func CreateMempool(dbName string, startMining bool) *Mempool {
	mempool := &Mempool{}
	mempool.multichain = CreateMultichain(dbName, mempool)

	mempool.finishedBlockCh = make(chan miner.BlockMsg, 1000)
	mempool.miner = miner.NewMiner(mempool.finishedBlockCh)
	mempool.stopMiningCh = make(chan string, 1000)
	mempool.transactions = make(map[string]map[string]txs.Tx)
	mempool.uncommitted = make(map[string]*UncommittedTransactions)

	mempool.transactions[txs.MATCH_TOKEN] = make(map[string]txs.Tx)
	mempool.uncommitted[txs.MATCH_TOKEN] = &UncommittedTransactions{}

	mempool.lock = &sync.Mutex{}

	if startMining {
		go mempool.StartMining(true)
		go mempool.ApplyLoop()
	}
}

func (mempool *Mempool) AddTransaction(tx txs.Tx, symbol string, takeLocks bool) bool {
	unlock := func() {
		if takeLocks {
			mempool.lock.Unlock()
			mempool.multichain.Unlock()
		}
	}

	if takeLocks {
		mempool.multichain.Lock()
		mempool.lock.Lock()
	}

	if _, ok := mempool.transactions[symbol][tx.ID()]; ok {
		utils.Log("Tx already in mempool, %v", tx.ID())
		unlock()
		return false
	}

	// Validate transaction's signature
	if !blockchain.Verify(tx, mempool.GetMultichain().consensusState) {
		utils.Log("Failed due to invalid signature")
		unlock()
		return false
	}

	// Validate transaction against consensus state
	if !mempool.GetMultichain().addGenericTransaction(symbol, tx, mempool.uncommitted[symbol], false) {
		utils.Log("Failed to add tx to mempool consensus state")
		unlock()
		return false
	}

	utils.Log("Tx added to mempool")
	// Add transaction to mempool
	mempool.transactions[symbol][tx.ID()] = tx

	if mempool.minerChosenToken == symbol {
		// Send transaction to miner
		minerRound := mempool.miner.minerRound
		unlock()
		mempool.miner.minerCh <- miner.MinerMsg{tx, false, minerRound}
	} else {
		mempool.uncommitted[symbol].rollbackLast(symbol, mempool, false)
		unlock()
	}

	return true
}

func (mempool *Mempool) StartMining(chooseNewToken bool) {
	mempool.multichain.Lock()
	mempool.lock.Lock()
	utils.Log("Start mining")
	// Pick a random token to start mining
	if chooseNewToken {
		var tokens []string
		for token := range mempool.transactions {
			tokens = append(tokens, token)
		}
		mempool.minerChosenToken = tokens[rand.Intn(len(tokens))]
	}

	newToken := mempool.minerChosenToken

	var txInPool []txs.Tx
	for _, tx := range mempool.transactions[mempool.minerChosenToken] {
		txInPool = append(txInPool, tx)
	}

	mempool.uncommitted[newToken].undoTransactions(newToken, mempool.multichain, false)
	mempool.uncommitted[newToken] = &UncommittedTransactions{}

	// Send new block message
	switch mempool.minerChosenToken {
	case txs.MATCH_TOKEN:
		utils.Log("Starting match block")
		msg := miner.MinerMsg{miner.NewBlockMsg{blockchain.MATCH_BLOCK, mempool.multichain.chains[mempool.minerChosenToken].tipHash, mempool.minerChosenToken}, true, 0}
		mempool.lock.Unlock()
		mempool.multichain.Unlock()
		mempool.miner.minerCh <- msg
	default:
		utils.Log("Starting %v block", mempool.minerChosenToken)
		msg := miner.MinerMsg{miner.NewBlockMsg{blockchain.TOKEN_BLOCK, mempool.multichain.chains[blockchains.minerChosenToken].tipHash, blockchains.minerChosenToken}, true, 0}
		mempool.lock.Unlock()
		mempool.multichain.Unlock()
		mempool.miner.minerCh <- msg
	}

	utils.Log("%v TX in mempool to revalidate and send", len(txInPool))
	//Send stuff currently in mem pool (re-validate too)
	go func(currentToken string, transactions []txs.Tx) {
		for _, tx := range transactions {
			mempool.multichain.Lock()
			mempool.lock.Lock()
			if currentToken != mempool.minerChosenToken {
				mempool.lock.Unlock()
				mempool.multichain.Unlock()
				return
			}

			// Validate transaction
			if !mempool.multichain.addGenericTransaction(mempool.minerChosenToken, tx, mempool.uncommitted[mempool.minerChosenToken], false) {
				delete(mempool.transactions[mempool.minerChosenToken], tx.ID())
				utils.Log("TX in mempool failed revalidation")
				mempool.lock.Unlock()
				mempool.multichain.Unlock()
				continue
			}
			minerRound := mempool.miner.minerRound
			mempool.lock.Unlock()
			mempool.multichain.Unlock()

			// Send transaction to miner
			utils.Log("Sending from re-validated mempool")
			mempool.miner.minerCh <- miner.MinerMsg{tx, false, minerRound}
		}
	}(newToken, txInPool)
}

func (mempool *Mempool) ApplyLoop() {
	for {
		select {
		case blockMsg := <-mempool.finishedBlockCh:
			utils.Log("Received block from miner")
			// When miner finishes, try to add a block
			mempool.multichain.Lock()
			//state was applied during validation so just add to chain
			chain, ok := mempool.multichain.chains[blockMsg.Symbol]
			if !ok {
				utils.Log("miner failed symbol no longer exists")
				mempool.multichain.Unlock()
				continue
			}

			if !bytes.Equal(chain.tipHash, blockMsg.Block.PrevBlockHash) {
				//block failed so retry
				utils.Log("miner prevBlockHash does not match tipHash %x != %x", chain.tipHash, blockMsg.Block.PrevBlockHash)

				mempool.lock.Lock()
				mempool.uncommitted[blockMsg.Symbol].undoTransactions(blockMsg.Symbol, blockchains, false)
				mempool.uncommitted[blockMsg.Symbol] = &UncommittedTransactions{}
				//WARNING taking both locks always take chain lock first
				mempool.lock.Unlock()
				mempool.multichain.Unlock()
				mempool.StartMining(false)
			} else {
				mempool.multichain.chains[blockMsg.Symbol].AddBlock(blockMsg.Block)

				mempool.lock.Lock()
				txInBlock := blockMsg.TxInBlock
				utils.Log("%v tx mined in block and added to chain %v", len(txInBlock), blockMsg.Symbol)

				var stillUncommitted UncommittedTransactions
				for _, tx := range mempool.uncommitted[blockMsg.Symbol].transactions {
					if _, ok := txInBlock[tx.ID()]; !ok {
						stillUncommitted.transactions = append(stillUncommitted.transactions, tx)
					} else {
						utils.Log("%tx mined in block and deleted from mempool %v", tx)
						switch tx.TxType {
						case txs.ORDER:
							mempool.multichain.matcher.AddOrder(tx.Tx.(txs.Order), blockMsg.Symbol)
						case txs.MATCH:
							mempool.multichain.matcher.AddMatch(tx.Tx.(txs.Match))
						case txs.CANCEL_ORDER:
							mempool.multichain.matcher.AddCancelOrder(tx.Tx.(txs.CancelOrder), blockMsg.Symbol)
						}
						delete(mempool.transactions[blockMsg.Symbol], tx.ID())
					}
				}

				stillUncommitted.undoTransactions(blockMsg.Symbol, mempool.multichain, false)
				mempool.uncommitted[blockMsg.Symbol] = &UncommittedTransactions{}
				mempool.lock.Unlock()
				mempool.multichain.Unlock()

				mempool.multichain.matcher.FindAllMatches()
				mempool.StartMining(true)

			}

		case symbol := <-mempool.stopMiningCh:
			mempool.multichain.Lock()
			if symbol != mempool.minerChosenToken {
				mempool.multichain.Unlock()
				continue
			}
			utils.Log("Restarting mining due to new block being added or removed")
			mempool.multichain.Unlock()
			mempool.StartMining(false)
		}

	}
}
