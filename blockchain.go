package main

import (
	"bytes"
	"errors"
	"github.com/boltdb/bolt" // Run "go get github.com/boltdb/bolt/..."
	"log"
)

type Blockchain struct {
	tipHash     []byte   // Tip of chain
	db          *bolt.DB // DB connection
	height      uint64
	bucketName  string
	blockhashes [][]byte
}

// To iterate over blocks
type BlockchainIterator struct {
	currentHash []byte   // Hash of current block
	db          *bolt.DB // DB connection
	bucketName  string
}

type BlockchainForwardIterator struct {
	hashes       [][]byte // Hash of current block
	currentIndex int
	db           *bolt.DB // DB connection
	bucketName   string
}

func NewBlockchain(db *bolt.DB, symbol string) *Blockchain {
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(symbol)) // Get bucket storing blocks
		if b == nil {
			// Create bucket
			_, err := tx.CreateBucket([]byte(symbol))
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})

	if err != nil {
		log.Panic(err)
	}
	blockchain := Blockchain{[]byte{}, db, 0, symbol, [][]byte{}}
	blockchain.tipHash = blockchain.getTipHash()
	blockchain.blockhashes = blockchain.getBlockhashes()

	return &blockchain
}

func (bc *Blockchain) AddBlock(block Block) {
	// Read-write transaction to store new block in DB
	bc.db.Update(func(tx *bolt.Tx) error {
		// Store block in bucket
		b := tx.Bucket([]byte(bc.bucketName))
		err := b.Put(block.Hash, block.Serialize())
		if err != nil {
			log.Panic(err)
		}

		// Update "l" key
		err = b.Put([]byte("l"), block.Hash)
		if err != nil {
			log.Panic(err)
		}

		// Update tip
		temp := make([]byte, len(block.Hash))
		copy(temp, block.Hash)
		bc.tipHash = temp
		return nil
	})

	bc.height += 1
	bc.blockhashes = append([][]byte{block.Hash}, bc.blockhashes...)
}

func (bc *Blockchain) RemoveLastBlock() BlockData {
	var lastBlock *Block
	bc.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		b := tx.Bucket([]byte(bc.bucketName))

		// Get second to last block
		bci := bc.Iterator()
		lastBlock, _ = bci.Prev()

		// Delete last bock
		b.Delete(bc.getTipHash())

		// Update "l" key to second to last block
		err := b.Put([]byte("l"), lastBlock.PrevBlockHash)
		if err != nil {
			log.Panic(err)
		}

		// Update tip to second to last block
		temp := make([]byte, len(lastBlock.PrevBlockHash))
		copy(temp, lastBlock.PrevBlockHash)
		bc.tipHash = temp
		return nil
	})

	bc.height -= 1
	bc.blockhashes = bc.blockhashes[1:]

	return lastBlock.Data
}

//
// Create iterator for a blockchain
//
func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.tipHash, bc.db, bc.bucketName}
	return bci
}

func (bc *Blockchain) ForwardIterator() *BlockchainForwardIterator {
	hashes := bc.blockhashes
	return &BlockchainForwardIterator{hashes, len(hashes) - 1, bc.db, bc.bucketName}
}

//
// Next returns next block starting from the tip
//
func (bci *BlockchainIterator) Prev() (*Block, error) {
	var block *Block

	// Read only transaction to get next block
	err := bci.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bci.bucketName))
		encodedBlock := b.Get(bci.currentHash)
		if encodedBlock == nil {
			return errors.New("out of blocks")
		}

		block = DeserializeBlock(encodedBlock)

		return nil
	})
	if err == nil {
		// Blocks are obtained newest to oldest
		bci.currentHash = block.PrevBlockHash
	}

	return block, err
}

func (bci *BlockchainForwardIterator) Next() (*Block, error) {
	var block *Block
	if bci.currentIndex < 0 {
		return nil, errors.New("out of blocks")
	}

	// Read only transaction to get next block
	err := bci.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bci.bucketName))
		encodedBlock := b.Get(bci.hashes[bci.currentIndex])
		if encodedBlock == nil {
			return errors.New("out of blocks")
		}
		block = DeserializeBlock(encodedBlock)

		return nil
	})
	if err == nil {
		// Blocks are obtained newest to oldest
		bci.currentIndex--
	}

	return block, err
}

func (bci *BlockchainForwardIterator) Undo() {
	bci.currentIndex++
}

func (bc *Blockchain) GetStartHeight() uint64 {
	return bc.height
}

func (bc *Blockchain) getBlockhashes() [][]byte {
	blockhashes := make([][]byte, 0)
	bci := bc.Iterator()

	block, err := bci.Prev()
	for err == nil {
		blockhashes = append(blockhashes, block.Hash)
		block, err = bci.Prev()
	}

	return blockhashes
}

func (bc *Blockchain) GetBlock(blockhash []byte) (*Block, error) {
	bci := bc.Iterator()

	block, err := bci.Prev()
	for err == nil {
		if bytes.Equal(block.Hash, blockhash) {
			return block, nil
		}
		block, err = bci.Prev()
	}

	return nil, errors.New("block not found")
}

func (bc *Blockchain) getTipHash() []byte {
	var lastHash []byte // Hash of last block

	// Read-only transaction to get hash of last block
	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bc.bucketName))
		temp := b.Get([]byte("l"))
		buf := make([]byte, len(temp))
		copy(buf, temp)
		lastHash = buf
		return nil
	})

	if err != nil || lastHash == nil {
		return []byte{}
	} else {
		return lastHash
	}
}
