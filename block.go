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
	Transaction     interface{}
	TransactionType TransactionType
}

func (gt *GenericTransaction) ID() string {
	switch gt.TransactionType {
	case CREATE_TOKEN: return string(gt.TransactionType) + gt.Transaction.(CreateToken).TokenInfo.Symbol
	case MATCH: return string(gt.TransactionType) + string(gt.Transaction.(Match).MatchID)
	case ORDER: return string(gt.TransactionType) + string(gt.Transaction.(Order).ID)
	case TRANSFER: return string(gt.TransactionType) + string(gt.Transaction.(Transfer).ID)
	case CANCEL_ORDER: return string(gt.TransactionType) + string(gt.Transaction.(CancelOrder).OrderID)
	case CLAIM_FUNDS: return string(gt.TransactionType) + string(gt.Transaction.(ClaimFunds).ID)
	}
	return ""
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
	ID          uint64
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
	ID uint64
	Address string
	Amount  uint64
}

func GetBytes(key interface{}) ([]byte) {
	return []byte(fmt.Sprintf("%v", key))
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
	block := &Block{2, MATCH_BLOCK, matchData, []byte{}, []byte{}, 0}
	block.Hash = NewProofOfWork(block).GetHash()
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
	newData := b.Data

	switch tx.TransactionType {
	case MATCH:
		temp := newData.(MatchData)
		temp.Matches = append(temp.Matches, tx.Transaction.(Match))
		newData = temp
	case ORDER:
		temp := newData.(TokenData)
		temp.Orders = append(temp.Orders, tx.Transaction.(Order))
		newData = temp
	case TRANSFER:
		temp := newData.(TokenData)
		temp.Transfers = append(temp.Transfers, tx.Transaction.(Transfer))
		newData = temp
	case CANCEL_ORDER:
		temp := newData.(MatchData)
		temp.CancelOrders = append(temp.CancelOrders, tx.Transaction.(CancelOrder))
		newData = temp
	case CLAIM_FUNDS:
		temp := newData.(TokenData)
		temp.ClaimFunds = append(temp.ClaimFunds, tx.Transaction.(ClaimFunds))
		newData = temp
	case CREATE_TOKEN:
		temp := newData.(MatchData)
		temp.CreateTokens = append(temp.CreateTokens, tx.Transaction.(CreateToken))
		newData = temp
	default:
		log.Panic("ERROR: unknown transaction type")
	}
	b.Data = newData
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
