package main

type NewBlockMsg struct {
	BlockType BlockType // type of block we are adding transactions to
	LastHash  []byte    // hash of previous block
	Symbol    string
}

type MinerMsg struct {
	Msg        interface{}
	IsNewBlock bool
	MinerRound uint64
}

type Miner struct {
	minerCh         chan MinerMsg
	finishedBlockCh chan BlockMsg
	transactions    []GenericTransaction
	minerRound      uint64
}

type BlockMsg struct {
	Block     Block
	TxInBlock map[string]GenericTransaction
	Symbol    string
}

func (miner *Miner) mineLoop() {
	var block *Block
	var txInBlock map[string]GenericTransaction
	var symbol string
	var pow *ProofOfWork
	for {
		select {
		case msg := <-miner.minerCh:
			// Stop mining
			if msg.IsNewBlock {
				miner.transactions = []GenericTransaction{}
				newBlock := msg.Msg.(NewBlockMsg)
				symbol = newBlock.Symbol
				miner.minerRound++

				switch symbol {
				case MATCH_CHAIN:
					matchData := MatchData{
						Matches:      nil,
						CancelOrders: nil,
						CreateTokens: nil,
					}
					block = NewBlock(matchData, newBlock.BlockType, newBlock.LastHash)
					txInBlock = make(map[string]GenericTransaction)
				default:
					tokenData := TokenData{
						Orders:     nil,
						ClaimFunds: nil,
						Transfers:  nil,
					}
					block = NewBlock(tokenData, newBlock.BlockType, newBlock.LastHash)
					txInBlock = make(map[string]GenericTransaction)
					Log("Mining new block: %s %v", newBlock.Symbol, newBlock.BlockType)
				}
			} else if msg.MinerRound == miner.minerRound {
				transaction := msg.Msg.(GenericTransaction)

				if block == nil {
					miner.transactions = append(miner.transactions, transaction)
					Log("Transaction added to array: %v", transaction)
				} else {
					switch transaction.TransactionType {
					case MATCH, CANCEL_ORDER, CREATE_TOKEN:
						if block.Type != MATCH_BLOCK {
							continue
						}
					default:
						if block.Type != TOKEN_BLOCK {
							continue
						}
					}

					Log("Transaction added to block: %v", transaction)
					block.AddTransaction(transaction)
					txInBlock[transaction.ID()] = transaction
				}
			}
		default:
			// Try to mine
			if block != nil {
				if len(miner.transactions) > 0 {
					for t := range miner.transactions {
						block.AddTransaction(miner.transactions[t])
						txInBlock[miner.transactions[t].ID()] = miner.transactions[t]
					}
					miner.transactions = []GenericTransaction{}
				}
				pow = NewProofOfWork(block)

				success, nonce, hash := pow.Try(1000)
				if success {
					block.Hash = hash[:]
					block.Nonce = nonce
					Log("Sending mined block")
					go func(msg BlockMsg) { miner.finishedBlockCh <- msg }(BlockMsg{*block, txInBlock, symbol})
					block = nil
				}
			}
		}
	}
}

func NewMiner(finishedBlockCh chan BlockMsg) *Miner {
	minerCh := make(chan MinerMsg, 1000)
	var transactions []GenericTransaction
	miner := &Miner{minerCh, finishedBlockCh, transactions, 0}
	go miner.mineLoop()
	return miner
}
