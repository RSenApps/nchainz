package main

import (
	"strconv"
	"bytes"
	"crypto/sha256"
	"time"
)

type TokenType uint8

const (
	BNB TokenType = iota + 1
	BTC
)

type Block struct {
	Timestamp     int64
	Data          BlockData
	PrevBlockHash []byte
	Hash          []byte
}

type BlockData interface {
	bytes() []byte
}

type RawData struct {
	Data []byte
}

func (data RawData) bytes() []byte {
	return data.Data
}

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
	Signature   []byte
}

func (b *Block) SetHash() {
	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))
	headers := bytes.Join([][]byte{b.PrevBlockHash, b.Data.bytes(), timestamp}, []byte{})
	hash := sha256.Sum256(headers)
	b.Hash = hash[:]
}

func NewBlock(data string, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), RawData{[]byte(data)}, prevBlockHash, []byte{}}
	block.SetHash()
	return block
}
