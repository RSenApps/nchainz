package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"
)

type TokenInfo struct {
	Symbol      string
	TotalSupply uint64
	Decimals    uint8
}

type BlockType uint8

const (
	TOKEN_BLOCK BlockType = iota + 1
	MATCH_BLOCK
	STRING
)

type TransactionType uint8

const (
	MATCH TransactionType = iota + 1
	ORDER
	TRANSFER
	CANCEL_MATCH
	CANCEL_ORDER
	TRANSACTION_CONFIRMED
	CREATE_TOKEN
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
	Matches       []Match
	CancelMatches []CancelMatch
	CreateTokens  []CreateToken
}

type TokenData struct {
	Orders               []Order
	CancelOrders         []CancelOrder
	TransactionConfirmed []TransactionConfirmed
	Transfers            []Transfer
}

type GenericTransaction struct {
	transaction     interface{}
	transactionType TransactionType
}

type CreateToken struct {
	TokenInfo      TokenInfo
	CreatorAddress string //TODO: []byte
	Signature      []byte
}

//surplus goes to miner
type Match struct {
	MatchID     uint64
	SellOrderID uint64
	BuyOrderID  uint64
	AmountSold  uint64
}

type Order struct {
	BuyTokenType  string
	AmountToSell  uint64
	AmountToBuy   uint64
	SellerAddress string //TODO: []byte
	Signature     []byte
}

type Transfer struct {
	Amount      uint64
	FromAddress string //TODO: []byte
	ToAddress   string //TODO: []byte
	Signature   []byte
}

type CancelMatch struct {
	CancelMatchID uint64
	OrderID       uint64
	Signature     []byte
}

type CancelOrder struct {
	CancelMatchID uint64
}

type TransactionConfirmed struct {
	MatchID uint64
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

func NewGenesisBlock(data BlockData) *Block {
	return NewBlock(data, STRING, []byte{})
}

func NewBlock(data BlockData, blockType BlockType, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), blockType, data, prevBlockHash, []byte{}, 0}

	// Add block
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()
	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

func (b *Block) AddTransaction(tx *GenericTransaction) {
	temp := b.Data
	switch tx.transactionType {
	case MATCH:
		temp := b.Data.(MatchData)
		temp.Matches = append(temp.Matches, tx.transaction.(Match))
	case ORDER:
		temp := b.Data.(TokenData)
		temp.Orders = append(temp.Orders, tx.transaction.(Order))
	case TRANSFER:
		temp := b.Data.(TokenData)
		temp.Transfers = append(temp.Transfers, tx.transaction.(Transfer))
	case CANCEL_MATCH:
		temp := b.Data.(MatchData)
		temp.CancelMatches = append(temp.CancelMatches, tx.transaction.(CancelMatch))
	case CANCEL_ORDER:
		temp := b.Data.(TokenData)
		temp.CancelOrders = append(temp.CancelOrders, tx.transaction.(CancelOrder))
	case TRANSACTION_CONFIRMED:
		temp := b.Data.(TokenData)
		temp.TransactionConfirmed = append(temp.TransactionConfirmed, tx.transaction.(TransactionConfirmed))
	case CREATE_TOKEN:
		temp := b.Data.(MatchData)
		temp.CreateTokens = append(temp.CreateTokens, tx.transaction.(CreateToken))
	default:
		log.Panic("ERROR: unknown transaction type")
	}
	b.Data = temp
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
