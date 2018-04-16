package main

import (
	"github.com/boltdb/bolt" // Run "go get github.com/boltdb/bolt/..."
	"log"
)

const dbFile = "blockchain.db"
const blocksBucket = "blocks"

type Blockchain struct {
	tipHash []byte   // Tip of chain
	db      *bolt.DB // DB connection
}

// To iterate over blocks
type BlockchainIterator struct {
	currentHash []byte   // Hash of current block
	db          *bolt.DB // DB connection
}

func NewBlockchain() *Blockchain {
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
			genesisBlock := NewBlock("Genesis Block", []byte{})

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

	return &Blockchain{tipHash, db}
}

func (bc *Blockchain) AddBlock(data string) {
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
	newBlock := NewBlock(data, lastHash)

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
func (bci *BlockchainIterator) Next() *Block {
	var block *Block

	// Read only transaction to get next block
	err := bci.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		encodedBlock := b.Get(bci.currentHash)
		block = DeserializeBlock(encodedBlock)

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	// Blocks are obtained newest to oldest
	bci.currentHash = block.PrevBlockHash

	return block
}
