package main

import (
	"strconv"
	"bytes"
	"crypto/sha256"
	"time"
	"encoding/gob"
)

type TokenType uint8

const (
	BNB TokenType = iota + 1
	BTC
)

type BlockType uint8

const (
	MATCH    BlockType = iota + 1
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

func (b *Block) SetHash() {
	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))
	t := make([]byte, 1)
	t[0] = byte(b.Type)
	databytes, _ := GetBytes(b.Data)
	headers := bytes.Join([][]byte{b.PrevBlockHash, databytes, timestamp, t}, []byte{})
	hash := sha256.Sum256(headers)
	b.Hash = hash[:]
}

func NewBlock(data string, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), STRING, []byte(data), prevBlockHash, []byte{}}
	block.SetHash()
	return block
}
