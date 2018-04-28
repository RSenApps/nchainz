package main

import (
	"bytes"
	"errors"
	"github.com/boltdb/bolt" // Run "go get github.com/boltdb/bolt/..."
	"log"
)

const blocksBucket = "blocks"

type Blockchain struct {
	tipHash []byte   // Tip of chain
	db      *bolt.DB // DB connection
	height  uint64
}

// To iterate over blocks
type BlockchainIterator struct {
	currentHash []byte   // Hash of current block
	db          *bolt.DB // DB connection
}

func NewBlockchain(dbFile string) *Blockchain {
	var tipHash []byte // Tip of chain

	// Open BoltDB file
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	// Read-write transaction to store genesis block in DB
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket)) // Get bucket storing blocks

		if b == nil { // Bucket exists
			// Create genesis block
			genesisBlock := NewGenesisBlock("Genesis Block")

			// Create bucket
			b, err := tx.CreateBucket([]byte(blocksBucket))
			if err != nil {
				log.Panic(err)
			}

			// Store block in bucket
			err = b.Put(genesisBlock.Hash, genesisBlock.Serialize())
			if err != nil {
				log.Panic(err)
			}

			// Update "l" key --> hash of last block in chain
			err = b.Put([]byte("l"), genesisBlock.Hash)
			if err != nil {
				log.Panic(err)
			}

			// Update tip
			tipHash = genesisBlock.Hash
		} else { // Bucket doesn't exist
			// Update tip to hash of last block in chain
			tipHash = b.Get([]byte("l"))
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	return &Blockchain{tipHash, db, 0}
}

func (bc *Blockchain) AddBlockData(data BlockData, blockType BlockType) {
	var lastHash []byte // Hash of last block

	// Read-only transaction to get hash of last block
	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
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
		b := tx.Bucket([]byte(blocksBucket))
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
	//TODO: Verify block

	// Read-write transaction to store new block in DB
	bc.db.Update(func(tx *bolt.Tx) error {
		// Store block in bucket
		b := tx.Bucket([]byte(blocksBucket))
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
	bci := &BlockchainIterator{bc.tipHash, bc.db}
	return bci
}

//
// Next returns next block starting from the tip
//
func (bci *BlockchainIterator) Next() (*Block, error) {
	var block *Block

	// Read only transaction to get next block
	err := bci.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
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
	for err != nil {
		blockhashes = append(blockhashes, block.Hash)
		_, err = bci.Next()
	}

	return blockhashes
}

func (bc *Blockchain) GetBlock(blockhash []byte) (*Block, error) {
	bci := bc.Iterator()

	block, err := bci.Next()
	for err != nil {
		if bytes.Equal(block.Hash, blockhash) {
			return block, nil
		}
		_, err = bci.Next()
	}

	return nil, errors.New("block not found")
}
