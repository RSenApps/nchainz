package main

import (
	"bytes"
	"errors"
	"github.com/boltdb/bolt" // Run "go get github.com/boltdb/bolt/..."
	"log"
)

type Blockchain struct {
	tipHash []byte   // Tip of chain
	db      *bolt.DB // DB connection
	height  uint64
	bucketName string
}

// To iterate over blocks
type BlockchainIterator struct {
	currentHash []byte   // Hash of current block
	db          *bolt.DB // DB connection
	bucketName string
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
	return &Blockchain{[]byte{}, db, 0, symbol}
}


func (bc *Blockchain) AddBlockData(data BlockData, blockType BlockType) {
	var lastHash []byte // Hash of last block

	// Read-only transaction to get hash of last block
	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bc.bucketName))
		lastHash = b.Get([]byte("l"))
		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	// New block to add
	newBlock := NewBlock(data, blockType, lastHash)

	// Read-write transaction to store new block in DB
	err = bc.db.Update(func(tx *bolt.Tx) error {
		// Store block in bucket
		b := tx.Bucket([]byte(bc.bucketName))
		err := b.Put(newBlock.Hash, newBlock.Serialize())
		if err != nil {
			log.Panic(err)
		}

		// Update "l" key
		err = b.Put([]byte("l"), newBlock.Hash)
		if err != nil {
			log.Panic(err)
		}

		// Update tip
		bc.tipHash = newBlock.Hash

		return nil
	})

	bc.height += 1
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
		bc.tipHash = block.Hash

		return nil
	})

	bc.height += 1
}

func (bc *Blockchain) RemoveLastBlock() BlockData {
	//TODO:
	return nil
}

//
// Create iterator for a blockchain
//
func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.tipHash, bc.db, bc.bucketName}
	return bci
}

//
// Next returns next block starting from the tip
//
func (bci *BlockchainIterator) Next() (*Block, error) {
	var block *Block

	// Read only transaction to get next block
	err := bci.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bci.bucketName))
		encodedBlock := b.Get(bci.currentHash)
		block = DeserializeBlock(encodedBlock)

		return nil
	})

	// Blocks are obtained newest to oldest
	bci.currentHash = block.PrevBlockHash

	return block, err
}

// TODO: Write this in an efficient way
func (bc *Blockchain) GetStartHeight() uint64 {
	/*bci := bc.Iterator()
	var height uint64

	_, err := bci.Next()
	for err != nil {
		height++
		_, err = bci.Next()
	}

	return height*/
	return bc.height
}

func (bc *Blockchain) GetBlockhashes() [][]byte {
	blockhashes := make([][]byte, 0)
	bci := bc.Iterator()

	block, err := bci.Next()
	for err == nil {
		blockhashes = append(blockhashes, block.Hash)
		block, err = bci.Next()
	}

	return blockhashes
}

func (bc *Blockchain) GetBlock(blockhash []byte) (*Block, error) {
	bci := bc.Iterator()

	block, err := bci.Next()
	for err == nil {
		if bytes.Equal(block.Hash, blockhash) {
			return block, nil
		}
		block, err = bci.Next()
	}

	return nil, errors.New("block not found")
}

func (bc *Blockchain) GetTipHash() []byte {
	if bc.height == 0 {
		return []byte{}
	}
	bci := bc.Iterator()
	block, err := bci.Next()
	for err == nil {
		block, err = bci.Next()
	}
	return block.Hash
}
