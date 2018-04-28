package main

type ActionType uint8

const (
	START ActionType = iota + 1
	STOP
)

type TransactionMsg struct {
	transaction GenericTransaction // transaction to add
	action      ActionType         // START or STOP mining
	blockType   BlockType          // type of block we are adding transactions to
	lastHash    []byte             // hash of previous block
}

type Miner struct {
	transactionCh chan TransactionMsg
}

func (miner *Miner) mineLoop() Block {
	block := Block{}
	for {
		select {
		case msg := <-miner.transactionCh:
			// Stop mining
			if msg.action == STOP {
				return block
			} else { // Continue mining
				// These should always be the same
				block.Type = msg.blockType
				block.PrevBlockHash = msg.lastHash

				// Initialize or modify data
				block.AddTransaction(&msg.transaction)

				// Try to mine
				pow := NewProofOfWork(&block)
				success, nonce, hash := pow.Try(1000)
				if success {
					block.Hash = hash[:]
					block.Nonce = nonce
					return block
				}
			}
		default: // Stop mining once there are no more transactions
			return block
		}
	}
}

func NewMiner() *Miner {
	transactionCh := make(chan TransactionMsg)
	miner := &Miner{transactionCh}
	miner.mineLoop()
	return miner
}
