package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"math/big"
)

////////////////////////////////////////////
// BLOCKDATA
// MatchData or TokenData stored on a block

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

////////////////////
// BLOCK

type BlockType uint8

const (
	TOKEN_BLOCK BlockType = iota + 1
	MATCH_BLOCK
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

func NewGenesisBlock() *Block {
	ws := NewWalletStore(true)
	addresses := ws.GetAddresses()
	w := ws.GetWallet(addresses[0])

	createToken := CreateToken{
		TokenInfo: TokenInfo{
			Symbol:      NATIVE_CHAIN,
			TotalSupply: 100 * 1000 * 1000,
			Decimals:    18,
		},
		CreatorAddress: w.PublicKey,
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
		LogPanic("ERROR: unknown transaction type")
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
		LogPanic(err.Error())
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
		LogPanic(err.Error())
	}

	return &block
}

func (b *Block) Dump() string {
	switch b.Type {
	case TOKEN_BLOCK:
		data := b.Data.(TokenData)
		return fmt.Sprintf("%x %v %v %v", b.Hash, len(data.Orders), len(data.ClaimFunds), len(data.Transfers))

	case MATCH_BLOCK:
		data := b.Data.(MatchData)
		return fmt.Sprintf("%x %v %v %v", b.Hash, len(data.Matches), len(data.CancelOrders), len(data.CreateTokens))

	default:
		return ""
	}
}
