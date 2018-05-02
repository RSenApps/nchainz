package main

import "fmt"

type NewBlockMsg struct {
	BlockType BlockType // type of block we are adding transactions to
	LastHash  []byte    // hash of previous block
	Symbol    string
}

type MinerMsg struct {
	Msg        interface{}
	IsNewBlock bool
}

type Miner struct {
	minerCh         chan MinerMsg
	finishedBlockCh chan BlockMsg
	transactions    []GenericTransaction
}

type BlockMsg struct {
	Block  Block
	Symbol string
}

func (miner *Miner) mineLoop() {
	var block *Block
	var symbol string
	for {
		select {
		case msg := <-miner.minerCh:
			// Stop mining
			if msg.IsNewBlock {
				newBlock := msg.Msg.(NewBlockMsg)
				symbol = newBlock.Symbol
				switch symbol {
				case MATCH_CHAIN:
					matchData := MatchData{
						Matches:      nil,
						CancelOrders: nil,
						CreateTokens: nil,
					}
					block = NewBlock(matchData, newBlock.BlockType, newBlock.LastHash)
				default:
					tokenData := TokenData{
						Orders:     nil,
						ClaimFunds: nil,
						Transfers:  nil,
					}
					block = NewBlock(tokenData, newBlock.BlockType, newBlock.LastHash)
					fmt.Println("Mining new block:", newBlock.Symbol, newBlock.BlockType)
				}
			} else {
				transaction := msg.Msg.(GenericTransaction)
				if block == nil {
					miner.transactions = append(miner.transactions, transaction)
					fmt.Println("Transaction added to array:", transaction)
				} else {
					fmt.Println("Transaction added to block:", transaction)
					block.AddTransaction(transaction)
				}
			}
		default:
			// Try to mine
			if block != nil {
				if len(miner.transactions) > 0 {
					for t := range miner.transactions {
						block.AddTransaction(miner.transactions[t])
					}
					miner.transactions = []GenericTransaction{}
				}

				pow := NewProofOfWork(block)
				fmt.Println("Before try")
				success, nonce, hash := pow.Try(1000)
				fmt.Println("After try")
				if success {
					block.Hash = hash[:]
					block.Nonce = nonce
					miner.finishedBlockCh <- BlockMsg{*block, symbol}
				}
			}
		}
	}
}

func NewMiner(finishedBlockCh chan BlockMsg) *Miner {
	minerCh := make(chan MinerMsg)
	transactions := []GenericTransaction{}
	miner := &Miner{minerCh, finishedBlockCh, transactions}
	go miner.mineLoop()
	return miner
}
