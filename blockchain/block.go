package blockchain

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/rsenapps/nchainz/txs"
	"github.com/rsenapps/nchainz/utils"
)

////////////////////////////////////////////
// BLOCKDATA
// MatchData or TokenData stored on a block

type BlockData interface{}

type MatchData struct {
	Matches      []txs.Match
	CancelOrders []txs.CancelOrder
	CreateTokens []txs.CreateToken
}

type TokenData struct {
	Orders     []txs.Order
	ClaimFunds []txs.ClaimFunds
	Transfers  []txs.Transfer
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
	ws := utils.NewWalletStore(true)
	addresses := ws.GetAddresses()
	w := ws.GetWallet(addresses[0])

	createToken := txs.CreateToken{
		TokenInfo: txs.TokenInfo{
			Symbol:      txs.NATIVE_TOKEN,
			TotalSupply: 100 * 1000 * 1000,
			Decimals:    18,
		},
		CreatorAddress: w.PublicKey,
		Signature:      nil,
	}
	matchData := MatchData{
		Matches:      nil,
		CancelOrders: nil,
		CreateTokens: []txs.CreateToken{createToken},
	}
	block := &Block{2, MATCH_BLOCK, matchData, []byte{}, []byte{}, 0}
	block.Hash = NewProofOfWork(block).GetHash()
	return NewBlock(matchData, MATCH_BLOCK, []byte{})
}

func NewTokenGenesisBlock(createToken txs.CreateToken) *Block {
	claimFunds := txs.ClaimFunds{
		Address: createToken.CreatorAddress,
		Amount:  createToken.TokenInfo.TotalSupply,
	}
	tokenData := TokenData{
		Orders:     nil,
		ClaimFunds: []txs.ClaimFunds{claimFunds},
		Transfers:  nil,
	}
	return NewBlock(tokenData, TOKEN_BLOCK, []byte{})
}

func NewBlock(data BlockData, blockType BlockType, prevBlockHash []byte) *Block {
	block := &Block{2, blockType, data, prevBlockHash, []byte{}, 0}
	block.Hash = NewProofOfWork(block).GetHash()
	return block
}

func (b *Block) AddTransaction(tx txs.Tx) {
	newData := b.Data

	switch tx.TxType {
	case txs.MATCH:
		temp := newData.(MatchData)
		temp.Matches = append(temp.Matches, tx.Tx.(txs.Match))
		newData = temp
	case txs.ORDER:
		temp := newData.(TokenData)
		temp.Orders = append(temp.Orders, tx.Tx.(txs.Order))
		newData = temp
	case txs.TRANSFER:
		temp := newData.(TokenData)
		temp.Transfers = append(temp.Transfers, tx.Tx.(txs.Transfer))
		newData = temp
	case txs.CANCEL_ORDER:
		temp := newData.(MatchData)
		temp.CancelOrders = append(temp.CancelOrders, tx.Tx.(txs.CancelOrder))
		newData = temp
	case txs.CLAIM_FUNDS:
		temp := newData.(TokenData)
		temp.ClaimFunds = append(temp.ClaimFunds, tx.Tx.(txs.ClaimFunds))
		newData = temp
	case txs.CREATE_TOKEN:
		temp := newData.(MatchData)
		temp.CreateTokens = append(temp.CreateTokens, tx.Tx.(txs.CreateToken))
		newData = temp
	default:
		utils.LogPanic("ERROR: unknown transaction type")
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
		utils.LogPanic(err.Error())
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
		utils.LogPanic(err.Error())
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
