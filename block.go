package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"math/big"
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
	FREEZE
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
	Freezes    []Freeze
}

type GenericTransaction struct {
	Transaction     interface{}
	TransactionType TransactionType
}

func (gt *GenericTransaction) ID() string {
	switch gt.TransactionType {
	case CREATE_TOKEN:
		return fmt.Sprintf("%v,%v", gt.TransactionType, gt.Transaction.(CreateToken).TokenInfo.Symbol)
	case MATCH:
		return fmt.Sprintf("%v,%v", gt.TransactionType, gt.Transaction.(Match).MatchID)
	case ORDER:
		return fmt.Sprintf("%v,%v", gt.TransactionType, gt.Transaction.(Order).ID)
	case TRANSFER:
		return fmt.Sprintf("%v,%v", gt.TransactionType, gt.Transaction.(Transfer).ID)
	case CANCEL_ORDER:
		return fmt.Sprintf("%v,%v", gt.TransactionType, gt.Transaction.(CancelOrder).OrderID)
	case CLAIM_FUNDS:
		return fmt.Sprintf("%v,%v", gt.TransactionType, gt.Transaction.(ClaimFunds).ID)
	case FREEZE:
		return fmt.Sprintf("%v,%v", gt.TransactionType, gt.Transaction.(Freeze).ID)
	}
	return ""
}

type CreateToken struct {
	TokenInfo      TokenInfo
	CreatorAddress [addressLength]byte
	Signature      []byte
}

type UnsignedCreateToken struct {
	TokenInfo      TokenInfo
	CreatorAddress [addressLength]byte
}

//surplus goes to miner
type Match struct {
	MatchID     uint64
	SellSymbol  string
	SellOrderID uint64
	SellerGain  uint64
	BuySymbol   string
	BuyOrderID  uint64
	BuyerLoss   uint64
	TransferAmt uint64
}

type Order struct {
	ID            uint64
	BuySymbol     string
	AmountToSell  uint64
	AmountToBuy   uint64
	SellerAddress [addressLength]byte
	Signature     []byte
}

type UnsignedOrder struct {
	ID            uint64
	BuySymbol     string
	AmountToSell  uint64
	AmountToBuy   uint64
	SellerAddress [addressLength]byte
}

type Transfer struct {
	ID          uint64
	Amount      uint64
	FromAddress [addressLength]byte
	ToAddress   [addressLength]byte
	Signature   []byte
}

type Freeze struct {
	ID            uint64
	Amount        uint64
	FromAddress   [addressLength]byte
	UnfreezeBlock uint64
	Signature     []byte
}

type UnsignedFreeze struct {
	ID            uint64
	Amount        uint64
	FromAddress   [addressLength]byte
	UnfreezeBlock uint64
}

type UnsignedTransfer struct {
	ID          uint64
	Amount      uint64
	FromAddress [addressLength]byte
	ToAddress   [addressLength]byte
}

type CancelOrder struct { //goes on match chain
	OrderSymbol string
	OrderID     uint64
	Signature   []byte
}

type UnsignedCancelOrder struct { //goes on match chain
	OrderSymbol string
	OrderID     uint64
}

type ClaimFunds struct {
	ID      uint64
	Address [addressLength]byte
	Amount  uint64
}

func GetBytes(key interface{}) []byte {
	return []byte(fmt.Sprintf("%v", key))
}

func NewGenesisBlock() *Block {
	ws := NewWalletStore(true)
	addresses := ws.GetAddresses()
	if len(addresses) < 1 {
		LogPanic("No wallet found. Run `nchainz createwallet` before starting node.")
	}
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
		Freezes:    nil,
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
	case FREEZE:
		temp := newData.(TokenData)
		temp.Freezes = append(temp.Freezes, tx.Transaction.(Freeze))
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
		return fmt.Sprintf("%x %v %v %v %v", b.Hash, len(data.Orders), len(data.ClaimFunds), len(data.Transfers), len(data.Freezes))

	case MATCH_BLOCK:
		data := b.Data.(MatchData)
		return fmt.Sprintf("%x %v %v %v", b.Hash, len(data.Matches), len(data.CancelOrders), len(data.CreateTokens))

	default:
		return ""
	}
}

//
// Serialize a transaction (minus the signature) so we can hash and sign it
//
func (tx GenericTransaction) Serialize() []byte {
	switch tx.TransactionType {
	case MATCH:
		LogPanic("ERROR: should not sign match transaction")
	case ORDER:
		order := tx.Transaction.(Order)
		unsignedTx := UnsignedOrder{order.ID, order.BuySymbol, order.AmountToSell, order.AmountToBuy, order.SellerAddress}
		return GetBytes(unsignedTx)
	case TRANSFER:
		transfer := tx.Transaction.(Transfer)
		unsignedTx := UnsignedTransfer{transfer.ID, transfer.Amount, transfer.FromAddress, transfer.ToAddress}
		return GetBytes(unsignedTx)
	case FREEZE:
		freeze := tx.Transaction.(Freeze)
		unsignedTx := UnsignedFreeze{freeze.ID, freeze.Amount, freeze.FromAddress, freeze.UnfreezeBlock}
		return GetBytes(unsignedTx)
	case CANCEL_ORDER:
		cancel := tx.Transaction.(CancelOrder)
		unsignedTx := UnsignedCancelOrder{cancel.OrderSymbol, cancel.OrderID}
		return GetBytes(unsignedTx)
	case CLAIM_FUNDS:
		LogPanic("ERROR: should not sign claim funds transaction")
	case CREATE_TOKEN:
		create := tx.Transaction.(CreateToken)
		unsignedTx := UnsignedCreateToken{create.TokenInfo, create.CreatorAddress}
		return GetBytes(unsignedTx)
	default:
		LogPanic("ERROR: unknown transaction type")
	}

	return nil
}

//
// Sign a transaction
// Input: private key
// Output: signature
//
func Sign(privateKey ecdsa.PrivateKey, tx GenericTransaction) []byte {
	r, s, _ := ecdsa.Sign(rand.Reader, &privateKey, tx.Serialize())
	signature := append(r.Bytes(), s.Bytes()...)
	return signature
}

//
// Verify a transaction
//
func Verify(tx GenericTransaction, state ConsensusState) bool {
	// Always valid if transaction doesn't need to be signed
	if tx.TransactionType == MATCH || tx.TransactionType == CLAIM_FUNDS {
		return true
	}

	// Get public key
	ellipticCurve := elliptic.P256()
	address := tx.GetTxAddress(state)
	firstHalf := big.Int{}
	secondHalf := big.Int{}
	addrLen := len(address)
	firstHalf.SetBytes(address[:(addrLen / 2)])
	secondHalf.SetBytes(address[(addrLen / 2):])
	publicKey := ecdsa.PublicKey{ellipticCurve, &firstHalf, &secondHalf}

	// Get r, s (signature)
	r := big.Int{}
	s := big.Int{}
	signature := tx.GetTxSignature()
	sigLen := len(signature)
	r.SetBytes(signature[:(sigLen / 2)])
	s.SetBytes(signature[(sigLen / 2):])

	// Verify
	return ecdsa.Verify(&publicKey, tx.Serialize(), &r, &s)
}

//
// Get an address from a transaction
//
func (tx GenericTransaction) GetTxAddress(state ConsensusState) []byte {
	switch tx.TransactionType {
	case ORDER:
		address := tx.Transaction.(Order).SellerAddress
		return address[:]
	case TRANSFER:
		address := tx.Transaction.(Transfer).FromAddress
		return address[:]
	case FREEZE:
		address := tx.Transaction.(Freeze).FromAddress
		return address[:]
	case CANCEL_ORDER:
		cancelOrder := tx.Transaction.(CancelOrder)
		success, address := state.GetCancelAddress(cancelOrder)
		if !success {
			LogPanic("Failed to get an address from a cancel order")
			return []byte{}
		} else {
			return address[:]
		}
	case CREATE_TOKEN:
		address := tx.Transaction.(CreateToken).CreatorAddress
		return address[:]
	default:
		LogPanic("Getting an address from a transaction that doesn't need to be signed.")
		return []byte{}
	}
}

//
// Get a signature from a transaction
//
func (tx GenericTransaction) GetTxSignature() []byte {
	switch tx.TransactionType {
	case ORDER:
		return tx.Transaction.(Order).Signature
	case TRANSFER:
		return tx.Transaction.(Transfer).Signature
	case FREEZE:
		return tx.Transaction.(Freeze).Signature
	case CANCEL_ORDER:
		return tx.Transaction.(CancelOrder).Signature
	case CREATE_TOKEN:
		return tx.Transaction.(CreateToken).Signature
	default:
		LogPanic("Getting a signature from a transaction that doesn't need to be signed.")
		return []byte{}
	}
}

func (tx GenericTransaction) String() string {
	switch tx.TransactionType {
	case MATCH:
		match := tx.Transaction.(Match)
		return fmt.Sprintf("{MATCH#%v %s#%v / %s#%v : %v - %v/%v}", match.MatchID, match.BuySymbol, match.BuyOrderID, match.SellSymbol, match.SellOrderID, match.TransferAmt, match.BuyerLoss, match.SellerGain)
	case ORDER:
		order := tx.Transaction.(Order)
		return fmt.Sprintf("{ORDER#%v %v %s / %v %s}", order.ID, order.AmountToBuy, order.BuySymbol, order.AmountToSell, KeyToString(order.SellerAddress))
	case TRANSFER:
		transfer := tx.Transaction.(Transfer)
		return fmt.Sprintf("{TRANSFER#%v %v %s -> %s}", transfer.ID, transfer.Amount, KeyToString(transfer.FromAddress), KeyToString(transfer.ToAddress))
	case FREEZE:
		freeze := tx.Transaction.(Freeze)
		return fmt.Sprintf("{FREEZE#%v %v %s -> %s}", freeze.ID, freeze.Amount, KeyToString(freeze.FromAddress), freeze.UnfreezeBlock)
	case CANCEL_ORDER:
		cancel := tx.Transaction.(CancelOrder)
		return fmt.Sprintf("{CANCEL %s#%v}", cancel.OrderSymbol, cancel.OrderID)
	case CLAIM_FUNDS:
		claim := tx.Transaction.(ClaimFunds)
		return fmt.Sprintf("{CLAIM#%v %v %s}", claim.ID, claim.Amount, KeyToString(claim.Address))
	case CREATE_TOKEN:
		create := tx.Transaction.(CreateToken)
		return fmt.Sprintf("{CREATE %v %s}", create.TokenInfo, KeyToString(create.CreatorAddress))
	default:
		return "{invalid tx}"
	}
}

func (match Match) String() string {
	tx := GenericTransaction{match, MATCH}
	return tx.String()
}
func (order Order) String() string {
	tx := GenericTransaction{order, ORDER}
	return tx.String()
}
func (transfer Transfer) String() string {
	tx := GenericTransaction{transfer, TRANSFER}
	return tx.String()
}
func (freeze Freeze) String() string {
	tx := GenericTransaction{freeze, FREEZE}
	return tx.String()
}
func (cancel CancelOrder) String() string {
	tx := GenericTransaction{cancel, CANCEL_ORDER}
	return tx.String()
}
func (claim ClaimFunds) String() string {
	tx := GenericTransaction{claim, CLAIM_FUNDS}
	return tx.String()
}
func (create CreateToken) String() string {
	tx := GenericTransaction{create, CREATE_TOKEN}
	return tx.String()
}
