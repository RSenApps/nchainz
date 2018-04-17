package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"
)

type TokenType uint8

const (
	BNB TokenType = iota + 1
	BTC
)

type BlockType uint8

const (
	MATCH BlockType = iota + 1
	ORDER
	TRANSFER
	CANCEL
	STRING
)

type Block struct {
	Timestamp     int64
	Type          BlockType
	Data          BlockData
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
}

type BlockData interface{}

type MatchData struct {
	Matches []Match
	Cancels []Cancel
}

type TokenData struct {
	Orders    []Order
	Cancels   []Cancel
	Transfers []Transfer
}

type Match struct {
	SellTokenType     TokenType
	BuyTokenType      TokenType
	AmountSold        uint64
	SellerBlockIndex  uint64
	BuyerBlockIndex   uint64
	SellerOrderOffset uint8
	BuyerOrderOffset  uint8
	SellerBlockHash   []byte
	BuyerBlockHash    []byte
}

type Order struct {
	BuyTokenType  TokenType
	AmountToSell  uint64
	AmountToBuy   uint64
	SellerAddress []byte
	Signature     []byte
}

type Transfer struct {
	Amount      uint64
	FromAddress []byte
	ToAddress   []byte
	Signature   []byte
}

type Cancel struct {
	BlockIndex  uint64
	BlockOffset uint8
	BlockHash []byte
	Address []byte
	Signature   []byte
}

func GetBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func NewTokenBlock(tokenData TokenData, prevBlockHash []byte) *Block {
	tokenBytes, _ := GetBytes(tokenData)
	block := &Block{time.Now().Unix(), STRING, tokenBytes, prevBlockHash, []byte{}, 0}

	// Add block
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()
	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

func NewMatchBlock(matchData MatchData, prevBlockHash []byte) *Block {
	matchBytes, _ := GetBytes(matchData)
	block := &Block{time.Now().Unix(), STRING, matchBytes, prevBlockHash, []byte{}, 0}

	// Add block
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()
	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

func NewGenesisTokenBlock(tokenData TokenData) *Block {
	return NewTokenBlock(tokenData, []byte{})
}

func NewGenesisMatchBlock(matchData MatchData) *Block {
	return NewMatchBlock(matchData, []byte{})
}

func NewBlock(data string, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), STRING, []byte(data), prevBlockHash, []byte{}, 0}

	// Add block
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()
	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

//
// Serializes block
//
func (b *Block) Serialize() []byte {
	var result bytes.Buffer // buffer to store serialized data
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(b) // encode block
	if err != nil {
		log.Panic(err)
	}

	return result.Bytes()
}

//
// Deserializes block
//
func DeserializeBlock(d []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}

	return &block
}
