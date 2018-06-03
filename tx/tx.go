package tx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/rsenapps/nchainz/utils"
	"math/big"
)

//////////////////////
// TRANSACTION

type TxType uint8

const (
	MATCH TxType = iota + 1
	ORDER
	TRANSFER
	CANCEL_ORDER
	CLAIM_FUNDS
	CREATE_TOKEN
)

type Tx struct {
	Tx     interface{}
	TxType TxType
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
	SellerAddress [utils.AddressLength]byte
	Signature     []byte
}

type Transfer struct {
	ID          uint64
	Amount      uint64
	FromAddress [utils.AddressLength]byte
	ToAddress   [utils.AddressLength]byte
	Signature   []byte
}

type CancelOrder struct {
	OrderSymbol string
	OrderID     uint64
	Signature   []byte
}

type ClaimFunds struct {
	ID      uint64
	Address [utils.AddressLength]byte
	Amount  uint64
}

////////////////////
// TOKEN

type TokenInfo struct {
	Symbol      string
	TotalSupply uint64
	Decimals    uint8
}

type CreateToken struct {
	TokenInfo      TokenInfo
	CreatorAddress [utils.AddressLength]byte
	Signature      []byte
}

//////////////////////
// TRANSACTION SIGNING

type UnsignedOrder struct {
	ID            uint64
	BuySymbol     string
	AmountToSell  uint64
	AmountToBuy   uint64
	SellerAddress [utils.AddressLength]byte
}

type UnsignedTransfer struct {
	ID          uint64
	Amount      uint64
	FromAddress [utils.AddressLength]byte
	ToAddress   [utils.AddressLength]byte
}

type UnsignedCancelOrder struct { //goes on match chain
	OrderSymbol string
	OrderID     uint64
}

type UnsignedCreateToken struct {
	TokenInfo      TokenInfo
	CreatorAddress [utils.AddressLength]byte
}

//
// Helper function for serializing a transaction
//
func GetBytes(key interface{}) []byte {
	return []byte(fmt.Sprintf("%v", key))
}

//
// Serialize a transaction (minus the signature) so we can hash and sign it
//
func (tx Tx) Serialize() []byte {
	switch tx.TxType {
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
	case CANCEL_ORDER:
		return tx.Transaction.(CancelOrder).Signature
	case CREATE_TOKEN:
		return tx.Transaction.(CreateToken).Signature
	default:
		LogPanic("Getting a signature from a transaction that doesn't need to be signed.")
		return []byte{}
	}
}

//////////////////////
// TRANSACTION LOGGING

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
	}
	return ""
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
