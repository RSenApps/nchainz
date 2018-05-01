package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
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
	CANCEL_ORDER
	CLAIM_FUNDS
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
	Matches      []Match
	CancelOrders []CancelOrder
	CreateTokens []CreateToken
}

type TokenData struct {
	Orders     []Order
	ClaimFunds []ClaimFunds
	Transfers  []Transfer
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
	SellSymbol  string
	SellOrderID uint64
	BuySymbol   string
	BuyOrderID  uint64
	AmountSold  uint64
}

type Order struct {
	ID            uint64
	BuySymbol     string
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

type CancelOrder struct { //goes on match chain
	OrderSymbol string
	OrderID     uint64
	Signature   []byte
}

type ClaimFunds struct {
	Address string
	Amount  uint64
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

func NewGenesisBlock() *Block {
	createToken := CreateToken{
		TokenInfo: TokenInfo{
			Symbol:      NATIVE_CHAIN,
			TotalSupply: 100 * 1000 * 1000,
			Decimals:    18,
		},
		CreatorAddress: "Satoshi",
		Signature:      nil,
	}
	matchData := MatchData{
		Matches:      nil,
		CancelOrders: nil,
		CreateTokens: []CreateToken{createToken},
	}
	return NewBlock(matchData, MATCH_BLOCK, []byte{})
}

func NewTokenGenesisBlock(createToken CreateToken) *Block {
	claimFunds := ClaimFunds{
		Address: createToken.CreatorAddress,
		Amount:  createToken.TokenInfo.TotalSupply,
	}
	tokenData := TokenData{
		Orders:     nil,
		ClaimFunds: []ClaimFunds{claimFunds},
		Transfers:  nil,
	}
	return NewBlock(tokenData, TOKEN_BLOCK, []byte{})
}

func NewBlock(data BlockData, blockType BlockType, prevBlockHash []byte) *Block {
	block := &Block{2, blockType, data, prevBlockHash, []byte{}, 0}
	block.Hash = NewProofOfWork(block).GetHash()
	return block
}

func (b *Block) AddTransaction(tx GenericTransaction) {
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
		fmt.Println("Block data is now", temp.Transfers)
	case CANCEL_ORDER:
		temp := b.Data.(MatchData)
		temp.CancelOrders = append(temp.CancelOrders, tx.transaction.(CancelOrder))
	case CLAIM_FUNDS:
		temp := b.Data.(TokenData)
		temp.ClaimFunds = append(temp.ClaimFunds, tx.transaction.(ClaimFunds))
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
