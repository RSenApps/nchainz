package main

type NewBlockMsg struct {
	BlockType   BlockType          // type of block we are adding transactions to
	LastHash    []byte             // hash of previous block
}

type MinerMsg struct {
	Msg interface{}
	IsNewBlock bool
}

type Miner struct {
	minerCh chan MinerMsg
	finishedBlockCh chan Block
}

func (miner *Miner) mineLoop() {
	/*var block *Block
	for {
		select {
		case msg := <-miner.minerCh:
			// Stop mining
			if msg.IsNewBlock {
				newBlock := msg.Msg.(NewBlockMsg)
				block = NewBlock([]byte{}, newBlock.BlockType, newBlock.LastHash)
			} else {
				transaction := msg.Msg.(GenericTransaction)
				block.AddTransaction(transaction)
			}
		default:
			// Try to mine
			pow := NewProofOfWork(block)
			success, nonce, hash := pow.Try(1000)
			if success {
				block.Hash = hash[:]
				block.Nonce = nonce
				miner.finishedBlockCh <- *block
			}
		}
	}*/
}

func NewMiner(finishedBlockCh chan Block) *Miner {
	minerCh := make(chan MinerMsg)
	miner := &Miner{minerCh, finishedBlockCh}
	go miner.mineLoop()
	return miner
}
