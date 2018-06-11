package multichain

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/rsenapps/nchainz/blockchain"
	"github.com/rsenapps/nchainz/consensus"
	"github.com/rsenapps/nchainz/matcher"
	"github.com/rsenapps/nchainz/txs"
	"github.com/rsenapps/nchainz/utils"
	"os"
	"sync"
)

type Multichain struct {
	chains         map[string]*blockchain.Blockchain
	consensusState consensus.ConsensusState
	lock           *sync.RWMutex // Lock on consensus state and chains
	db             *bolt.DB
	recovering     bool
	matcher        *matcher.Matcher
	mempool        *Mempool
}

func (multichain *Multichain) Lock() {
	multichain.lock.Lock()
}

func (multichain *Multichain) RLock() {
	multichain.lock.RLock()
}

func (multichain *Multichain) Unlock() {
	multichain.lock.Unlock()
}

func (multichain *Multichain) RUnlock() {
	multichain.lock.RUnlock()
}

func createMultichain(dbName string, mempool *Mempool) *Multichain {
	//instantiates state and blockchains
	multichain := &Multichain{}
	newDatabase := true
	if _, err := os.Stat(dbName); err == nil {
		// path/to/whatever exists
		newDatabase = false
	}
	// Open BoltDB file
	db, err := bolt.Open(dbName, 0600, nil)
	if err != nil {
		utils.LogPanic(err.Error())
	}
	multichain.db = db

	multichain.matcher = matcher.StartMatcher(multichain, nil)
	multichain.consensusState = consensus.NewConsensusState()
	multichain.chains = make(map[string]*blockchain.Blockchain)
	multichain.chains[txs.MATCH_TOKEN] = blockchain.NewBlockchain(db, txs.MATCH_TOKEN)
	multichain.lock = &sync.RWMutex{}
	multichain.mempool = mempool

	if newDatabase {
		multichain.recovering = false
		multichain.AddBlock(txs.MATCH_TOKEN, *blockchain.NewGenesisBlock(), false)
	} else {
		multichain.recovering = true
		multichain.restoreFromDatabase()
		multichain.recovering = false
	}

	return multichain
}

////////////////////////////////
// Chain Manipulation

func (multichain *Multichain) RollbackAndAddBlocks(symbol string, height uint64) {
	//TODO:
}

func (multichain *Multichain) AddBlock(symbol string, block blockchain.Block, takeLock bool) bool {
	return multichain.AddBlocks(symbol, []blockchain.Block{block}, takeLock)
}

func (multichain *Multichain) AddBlocks(symbol string, blocks []blockchain.Block, takeLock bool) bool {
	defer multichain.matcher.FindAllMatches()

	if takeLock {
		multichain.Lock()
		defer multichain.Unlock()
	}

	if _, ok := multichain.chains[symbol]; !ok {
		utils.Log("AddBlocks failed as symbol not found: %v", symbol)
		return false
	}

	if !multichain.recovering && takeLock {
		multichain.mempool.rollbackTxForSymbol(symbol, true)
	}

	blocksAdded := 0
	var uncommitted UncommittedTransactions
	failed := false
	for _, block := range blocks {
		if !bytes.Equal(multichain.chains[symbol].TipHash, block.PrevBlockHash) {
			utils.Log("prevBlockHash does not match TipHash for symbol %v %x != %x \n", symbol, multichain.chains[symbol].TipHash, block.PrevBlockHash)
			failed = true
			break
		}

		if multichain.chains[symbol].Height > 1 {
			pow := blockchain.NewProofOfWork(&block)
			if !pow.Validate() {
				utils.Log("Proof of work of block is invalid")
			}
		}
		if symbol == txs.MATCH_TOKEN {
			if !multichain.addMatchData(block.Data.(blockchain.MatchData), &uncommitted) {
				failed = true
				break
			}
		} else {
			if !multichain.addTokenData(symbol, block.Data.(blockchain.TokenData), &uncommitted) {
				failed = true
				break
			}
		}
		multichain.chains[symbol].AddBlock(block)
		blocksAdded++
	}
	if failed {
		utils.Log("Adding blocks failed, rolling back.")
		uncommitted.undoTransactions(symbol, multichain, true)
		for i := 0; i < blocksAdded; i++ {
			multichain.chains[symbol].RemoveLastBlock()
		}
		return false
	}
	utils.Log("AddBlocks added %v blocks to %v chain", blocksAdded, symbol)

	return true
}

func (multichain *Multichain) RollbackToHeight(symbol string, height uint64, takeLock bool, takeMempoolLock bool) {
	if takeLock {
		defer multichain.matcher.FindAllMatches()
		multichain.Lock()
		defer multichain.Unlock()
	}
	utils.Log("Rolling back %v block to height: %v \n", symbol, height)

	if _, ok := multichain.chains[symbol]; !ok {
		utils.Log("RollbackToHeight failed as symbol not found: ", symbol)
		return
	}

	if !multichain.recovering {
		multichain.mempool.rollbackTxForSymbol(symbol, takeMempoolLock)
	}

	if symbol == txs.MATCH_TOKEN {
		multichain.rollbackMatchToHeight(height)
	} else {
		multichain.rollbackTokenToHeight(symbol, height)
	}

}

func (multichain *Multichain) AddTokenChain(createToken txs.CreateToken) {
	//no lock needed
	utils.Log("Adding token chain")
	chain := blockchain.NewBlockchain(multichain.db, createToken.TokenInfo.Symbol)
	multichain.chains[createToken.TokenInfo.Symbol] = chain
	multichain.mempool.transactions[createToken.TokenInfo.Symbol] = make(map[string]txs.Tx)
	multichain.mempool.uncommitted[createToken.TokenInfo.Symbol] = &UncommittedTransactions{}

	if !multichain.recovering { //recovery will replay this block normally
		multichain.AddBlock(createToken.TokenInfo.Symbol, *blockchain.NewTokenGenesisBlock(createToken), false)
	}
}

func (multichain *Multichain) RemoveTokenChain(createToken txs.CreateToken) {
	utils.Log("Removing token chain")
	multichain.chains[createToken.TokenInfo.Symbol].DeleteStorage()
	delete(multichain.chains, createToken.TokenInfo.Symbol)
	delete(multichain.mempool.transactions, createToken.TokenInfo.Symbol)
	delete(multichain.mempool.uncommitted, createToken.TokenInfo.Symbol)
}

////////////////////////////////
// State Getters

func (multichain *Multichain) GetBlock(symbol string, blockhash []byte) (*blockchain.Block, error) {
	multichain.RLock()
	defer multichain.RUnlock()

	bc, ok := multichain.chains[symbol]
	if !ok {
		return nil, errors.New("invalid chain")
	}

	block, blockErr := bc.GetBlock(blockhash)
	return block, blockErr
}

func (multichain *Multichain) GetHeights() map[string]uint64 {
	multichain.RLock()
	defer multichain.RUnlock()
	heights := make(map[string]uint64)
	for symbol, chain := range multichain.chains {
		heights[symbol] = chain.Height
	}
	return heights
}

func (multichain *Multichain) GetHeight(symbol string) uint64 {
	multichain.RLock()
	defer multichain.RUnlock()
	return multichain.chains[symbol].Height
}

func (multichain *Multichain) GetBlockhashes() map[string][][]byte {
	multichain.RLock()
	defer multichain.RUnlock()
	blockhashes := make(map[string][][]byte)
	for symbol, chain := range multichain.chains {
		blockhashes[symbol] = chain.Blockhashes
	}
	return blockhashes
}

func (multichain *Multichain) GetBalance(symbol string, address [utils.AddressLength]byte) uint64 {
	multichain.RLock()
	defer multichain.RUnlock()
	return multichain.consensusState.GetBalance(symbol, address)
}

func (multichain *Multichain) GetUnclaimedBalance(symbol string, address [utils.AddressLength]byte) uint64 {
	multichain.RLock()
	defer multichain.RUnlock()
	return multichain.consensusState.GetUnclaimedBalance(symbol, address)
}

func (multichain *Multichain) GetOpenOrders(symbol string) map[uint64]txs.Order {
	multichain.RLock()
	defer multichain.RUnlock()
	return multichain.consensusState.GetOpenOrders(symbol)
}

////////////////////////////////
// Private Implementation

func (multichain *Multichain) addTokenData(symbol string, tokenData blockchain.TokenData, uncommitted *UncommittedTransactions) bool {
	for _, claimFunds := range tokenData.ClaimFunds {
		tx := txs.Tx{
			Tx:     claimFunds,
			TxType: txs.CLAIM_FUNDS,
		}
		if !multichain.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}

	for _, transfer := range tokenData.Transfers {
		tx := txs.Tx{
			Tx:     transfer,
			TxType: txs.TRANSFER,
		}
		if !multichain.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}

	for _, order := range tokenData.Orders {
		tx := txs.Tx{
			Tx:     order,
			TxType: txs.ORDER,
		}
		if !multichain.addGenericTransaction(symbol, tx, uncommitted, true) {
			return false
		}
	}
	return true
}

func (multichain *Multichain) addMatchData(matchData blockchain.MatchData, uncommitted *UncommittedTransactions) bool {
	for _, createToken := range matchData.CreateTokens {
		tx := txs.Tx{
			Tx:     createToken,
			TxType: txs.CREATE_TOKEN,
		}
		if !multichain.addGenericTransaction(txs.MATCH_TOKEN, tx, uncommitted, true) {
			return false
		}
	}

	for _, match := range matchData.Matches {
		tx := txs.Tx{
			Tx:     match,
			TxType: txs.MATCH,
		}
		if !multichain.addGenericTransaction(txs.MATCH_TOKEN, tx, uncommitted, true) {
			return false
		}
	}

	for _, cancelOrder := range matchData.CancelOrders {
		tx := txs.Tx{
			Tx:     cancelOrder,
			TxType: txs.CANCEL_ORDER,
		}
		if !multichain.addGenericTransaction(txs.MATCH_TOKEN, tx, uncommitted, true) {
			return false
		}
	}

	return true
}

func (multichain *Multichain) rollbackTokenToHeight(symbol string, height uint64) {
	if height >= multichain.chains[symbol].Height {
		return
	}
	blocksToRemove := multichain.chains[symbol].Height - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := multichain.chains[symbol].RemoveLastBlock().(blockchain.TokenData)
		for j := len(removedData.Orders) - 1; j >= 0; j-- {
			multichain.rollbackUntilRollbackOrderSucceeds(symbol, removedData.Orders[j], true)
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
	if height >= multichain.chains[txs.MATCH_TOKEN].Height {
		return
	}

	blocksToRemove := multichain.chains[txs.MATCH_TOKEN].Height - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := multichain.chains[txs.MATCH_TOKEN].RemoveLastBlock().(blockchain.MatchData)

		for j := len(removedData.CancelOrders) - 1; j >= 0; j-- {
			multichain.rollbackUntilRollbackCancelOrderSucceeds(removedData.CancelOrders[j], true)
			multichain.matcher.RemoveCancelOrder(removedData.CancelOrders[j])
			multichain.consensusState.RollbackCancelOrder(removedData.CancelOrders[j])
		}

		for j := len(removedData.Matches) - 1; j >= 0; j-- {
			multichain.rollbackUntilRollbackMatchSucceeds(removedData.Matches[j], true)
			buyOrder, sellOrder := multichain.consensusState.GetBuySellOrdersForMatch(removedData.Matches[j])
			multichain.matcher.RemoveMatch(removedData.Matches[j], buyOrder, sellOrder)
			multichain.consensusState.RollbackMatch(removedData.Matches[j])
		}

		for j := len(removedData.CreateTokens) - 1; j >= 0; j-- {
			multichain.consensusState.RollbackCreateToken(removedData.CreateTokens[j])
			multichain.RemoveTokenChain(removedData.CreateTokens[j])
		}
	}
}

//only send mined transactions to matcher
func (multichain *Multichain) addGenericTransaction(symbol string, tx txs.Tx, uncommitted *UncommittedTransactions, mined bool) bool {
	success := false

	switch tx.TxType {
	case txs.ORDER:
		success = multichain.consensusState.AddOrder(symbol, tx.Tx.(txs.Order))
	case txs.CLAIM_FUNDS:
		success = multichain.consensusState.AddClaimFunds(symbol, tx.Tx.(txs.ClaimFunds))
	case txs.TRANSFER:
		success = multichain.consensusState.AddTransfer(symbol, tx.Tx.(txs.Transfer))
	case txs.MATCH:
		success = multichain.consensusState.AddMatch(tx.Tx.(txs.Match))
	case txs.CANCEL_ORDER:
		multichain.consensusState.AddCancelOrder(tx.Tx.(txs.CancelOrder))
	case txs.CREATE_TOKEN:
		success = multichain.consensusState.AddCreateToken(tx.Tx.(txs.CreateToken))
		multichain.AddTokenChain(tx.Tx.(txs.CreateToken))
	}

	if !success {
		return false
	}
	uncommitted.addTransaction(tx)

	if mined {
		switch tx.TxType {
		case txs.ORDER:
			multichain.matcher.AddOrder(tx.Tx.(txs.Order), symbol)
		case txs.MATCH:
			multichain.matcher.AddMatch(tx.Tx.(txs.Match))
		case txs.CANCEL_ORDER:
			multichain.matcher.AddCancelOrder(tx.Tx.(txs.CancelOrder), symbol)
		}
	}

	return true
}

func (multichain *Multichain) rollbackGenericTransaction(symbol string, tx txs.Tx, mined bool) {
	var buyOrder, sellOrder txs.Order
	if mined && tx.TxType == txs.MATCH {
		buyOrder, sellOrder = multichain.consensusState.GetBuySellOrdersForMatch(tx.Tx.(txs.Match))
	}

	switch tx.TxType {
	case txs.ORDER:
		multichain.rollbackUntilRollbackOrderSucceeds(symbol, tx.Tx.(txs.Order), mined)
		multichain.consensusState.RollbackOrder(symbol, tx.Tx.(txs.Order))
	case txs.CLAIM_FUNDS:
		multichain.consensusState.RollbackClaimFunds(symbol, tx.Tx.(txs.ClaimFunds))
	case txs.TRANSFER:
		multichain.consensusState.RollbackTransfer(symbol, tx.Tx.(txs.Transfer))
	case txs.MATCH:
		multichain.rollbackUntilRollbackMatchSucceeds(tx.Tx.(txs.Match), mined)
		multichain.consensusState.RollbackMatch(tx.Tx.(txs.Match))
	case txs.CANCEL_ORDER:
		multichain.rollbackUntilRollbackCancelOrderSucceeds(tx.Tx.(txs.CancelOrder), mined)
		multichain.consensusState.RollbackCancelOrder(tx.Tx.(txs.CancelOrder))
	case txs.CREATE_TOKEN:
		multichain.consensusState.RollbackCreateToken(tx.Tx.(txs.CreateToken))
		multichain.RemoveTokenChain(tx.Tx.(txs.CreateToken))
	}

	if mined {
		switch tx.TxType {
		case txs.ORDER:
			multichain.matcher.RemoveOrder(tx.Tx.(txs.Order), symbol)
		case txs.MATCH:
			multichain.matcher.RemoveMatch(tx.Tx.(txs.Match), buyOrder, sellOrder)
		case txs.CANCEL_ORDER:
			multichain.matcher.RemoveCancelOrder(tx.Tx.(txs.CancelOrder))
		}
	}
}

func (multichain *Multichain) restoreFromDatabase() {
	iterators := make(map[string]*blockchain.BlockchainForwardIterator)
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
				chain.Height = uint64(len(iterator.Hashes))
				iterators[symbol] = iterator
			}
			block, err := iterator.Next()
			for err == nil {
				if symbol == txs.MATCH_TOKEN {
					var uncommitted UncommittedTransactions
					if !multichain.addMatchData(block.Data.(blockchain.MatchData), &uncommitted) {
						uncommitted.undoTransactions(txs.MATCH_TOKEN, multichain, true)
						iterator.Undo()
						break
					}
				} else {
					var uncommitted UncommittedTransactions
					if !multichain.addTokenData(symbol, block.Data.(blockchain.TokenData), &uncommitted) {
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
	multichain.Lock()
	multichain.mempool.Lock()

	close(multichain.mempool.miner.MinerCh)
	multichain.db.Close()
}

func (multichain *Multichain) DumpChains(amt uint64) string {
	multichain.RLock()
	defer multichain.RUnlock()

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%v\n", len(multichain.chains)))

	for symbol, chain := range multichain.chains {
		chainDump, numBlocks := chain.DumpChain(amt)
		header := fmt.Sprintf("%s %v %v\n", symbol, chain.Height, numBlocks)

		buffer.WriteString(header)
		buffer.WriteString(chainDump)
	}

	return buffer.String()
}

type UncommittedTransactions struct {
	transactions []txs.Tx
}

func (uncommitted *UncommittedTransactions) addTransaction(transaction txs.Tx) {
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

func (multichain *Multichain) rollbackUntilRollbackOrderSucceeds(symbol string, order txs.Order, takeMempoolLock bool) {
	//if rolling back order and not in openOrders then it must have been matched (cancels would be rolled back already)
	//Rollback match chain until it works (start with current height to forget unmined)
	if count, ok := multichain.consensusState.GetOrderUpdatesCount(symbol, order.ID); count > 0 || !ok {
		if !ok {
			panic("error")
		}
		utils.Log("Rolling back unmined matches as Order %v has %v updates still", order.ID, count)
		multichain.RollbackToHeight(txs.MATCH_TOKEN, multichain.chains[txs.MATCH_TOKEN].Height, false, takeMempoolLock)
	}

	count, _ := multichain.consensusState.GetOrderUpdatesCount(symbol, order.ID)
	for count > 0 {
		utils.Log("Rolling back match chain to height %v as Order %v has %v updates still", multichain.chains[txs.MATCH_TOKEN].Height-1, order.ID, count)
		multichain.RollbackToHeight(txs.MATCH_TOKEN, multichain.chains[txs.MATCH_TOKEN].Height-1, false, takeMempoolLock)
		count, _ = multichain.consensusState.GetOrderUpdatesCount(symbol, order.ID)
	}
}

func (multichain *Multichain) rollbackUntilRollbackMatchSucceeds(match txs.Match, takeMempoolLock bool) {
	// were orders deleted?
	buyOrder, sellOrder := multichain.consensusState.GetBuySellOrdersForMatch(match)

	//if rolling back match and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	if unclaimed := multichain.consensusState.GetUnclaimedBalance(match.BuySymbol, sellOrder.SellerAddress); unclaimed < match.SellerGain {
		utils.Log("Rolling back unmined token %v as rolling back match %v would result in a negative unclaimed funds %v < %v", match.BuySymbol, match, unclaimed, match.SellerGain)
		panic("error") //TODO: DEBUG
		multichain.RollbackToHeight(match.BuySymbol, multichain.chains[match.BuySymbol].Height, false, takeMempoolLock)
	}

	for multichain.consensusState.GetUnclaimedBalance(match.BuySymbol, sellOrder.SellerAddress) < match.SellerGain {
		utils.Log("Rolling back token %v to height %v as rolling back match %v would result in a negative unclaimed funds", match.BuySymbol, multichain.chains[match.SellSymbol].Height, match)
		multichain.RollbackToHeight(match.BuySymbol, multichain.chains[match.BuySymbol].Height-1, false, takeMempoolLock)
	}

	//if rolling back match and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	if unclaimed := multichain.consensusState.GetUnclaimedBalance(match.SellSymbol, buyOrder.SellerAddress); unclaimed < match.TransferAmt {
		utils.Log("Rolling back unmined token %v as rolling back match %v would result in a negative unclaimed funds %v < %v", match.SellSymbol, match, unclaimed, match.TransferAmt)
		panic("error") //TODO: DEBUG
		multichain.RollbackToHeight(match.SellSymbol, multichain.chains[match.SellSymbol].Height, false, takeMempoolLock)
	}

	for multichain.consensusState.GetUnclaimedBalance(match.SellSymbol, buyOrder.SellerAddress) < match.TransferAmt {
		utils.Log("Rolling back token %v to height %v as rolling back match %v would result in a negative unclaimed funds", match.SellSymbol, multichain.chains[match.SellSymbol].Height, match)
		multichain.RollbackToHeight(match.SellSymbol, multichain.chains[match.SellSymbol].Height-1, false, takeMempoolLock)
	}
}

func (multichain *Multichain) rollbackUntilRollbackCancelOrderSucceeds(cancelOrder txs.CancelOrder, takeMempoolLock bool) {
	deletedOrder, _ := multichain.consensusState.GetDeletedOrder(cancelOrder.OrderSymbol, cancelOrder.OrderID)

	//if rolling back cancel order and unclaimed funds will become negative then it must rollback token chain until claim funds are removed
	if multichain.consensusState.GetUnclaimedBalance(cancelOrder.OrderSymbol, deletedOrder.SellerAddress) < deletedOrder.AmountToSell {
		utils.Log("Rolling back unmined token %v as rolling back cancel order %v would result in a negative unclaimed funds", cancelOrder.OrderSymbol, cancelOrder)
		multichain.RollbackToHeight(cancelOrder.OrderSymbol, multichain.chains[cancelOrder.OrderSymbol].Height, false, takeMempoolLock)
	}

	for multichain.consensusState.GetUnclaimedBalance(cancelOrder.OrderSymbol, deletedOrder.SellerAddress) < deletedOrder.AmountToSell {
		utils.Log("Rolling back token %v to height %v as rolling back cancel order %v would result in a negative unclaimed funds", cancelOrder.OrderSymbol, multichain.chains[cancelOrder.OrderSymbol].Height, cancelOrder)
		multichain.RollbackToHeight(cancelOrder.OrderSymbol, multichain.chains[cancelOrder.OrderSymbol].Height, false, takeMempoolLock)
	}
}

func (multichain *Multichain) verifyTxSig(tx txs.Tx) bool {
	if tx.TxType == txs.MATCH || tx.TxType == txs.CLAIM_FUNDS {
		return true
	}
	return utils.VerifySignature(multichain.GetTxAddress(tx), tx.Serialize(), tx.GetTxSignature())
}

func (multichain Multichain) GetTxAddress(tx txs.Tx) []byte {
	switch tx.TxType {
	case txs.ORDER:
		address := tx.Tx.(txs.Order).SellerAddress
		return address[:]
	case txs.TRANSFER:
		address := tx.Tx.(txs.Transfer).FromAddress
		return address[:]
	case txs.CANCEL_ORDER:
		cancelOrder := tx.Tx.(txs.CancelOrder)
		success, address := multichain.consensusState.GetCancelAddress(cancelOrder)
		if !success {
			utils.LogPanic("Failed to get an address from a cancel order")
			return []byte{}
		} else {
			return address[:]
		}
	case txs.CREATE_TOKEN:
		address := tx.Tx.(txs.CreateToken).CreatorAddress
		return address[:]
	default:
		utils.LogPanic("Getting an address from a transaction that doesn't need to be signed.")
		return []byte{}
	}
}
