package txs

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"github.com/rsenapps/nchainz/utils"
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
		utils.LogPanic("ERROR: should not sign match transaction")
	case ORDER:
		order := tx.Tx.(Order)
		unsignedTx := UnsignedOrder{order.ID, order.BuySymbol, order.AmountToSell, order.AmountToBuy, order.SellerAddress}
		return GetBytes(unsignedTx)
	case TRANSFER:
		transfer := tx.Tx.(Transfer)
		unsignedTx := UnsignedTransfer{transfer.ID, transfer.Amount, transfer.FromAddress, transfer.ToAddress}
		return GetBytes(unsignedTx)
	case CANCEL_ORDER:
		cancel := tx.Tx.(CancelOrder)
		unsignedTx := UnsignedCancelOrder{cancel.OrderSymbol, cancel.OrderID}
		return GetBytes(unsignedTx)
	case CLAIM_FUNDS:
		utils.LogPanic("ERROR: should not sign claim funds transaction")
	case CREATE_TOKEN:
		create := tx.Tx.(CreateToken)
		unsignedTx := UnsignedCreateToken{create.TokenInfo, create.CreatorAddress}
		return GetBytes(unsignedTx)
	default:
		utils.LogPanic("ERROR: unknown transaction type")
	}

	return nil
}

//
// Sign a transaction
// Input: private key
// Output: signature
//
func Sign(privateKey ecdsa.PrivateKey, tx Tx) []byte {
	r, s, _ := ecdsa.Sign(rand.Reader, &privateKey, tx.Serialize())
	signature := append(r.Bytes(), s.Bytes()...)
	return signature
}

/*
//
// Verify a transaction
//
func Verify(tx Tx, state consensus.ConsensusState) bool {
	// Always valid if transaction doesn't need to be signed
	if tx.TxType == MATCH || tx.TxType == CLAIM_FUNDS {
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
func (tx Tx) GetTxAddress(state consensus.ConsensusState) []byte {
	switch tx.TxType {
	case ORDER:
		address := tx.Tx.(Order).SellerAddress
		return address[:]
	case TRANSFER:
		address := tx.Tx.(Transfer).FromAddress
		return address[:]
	case CANCEL_ORDER:
		cancelOrder := tx.Tx.(CancelOrder)
		success, address := state.GetCancelAddress(cancelOrder)
		if !success {
			utils.LogPanic("Failed to get an address from a cancel order")
			return []byte{}
		} else {
			return address[:]
		}
	case CREATE_TOKEN:
		address := tx.Tx.(CreateToken).CreatorAddress
		return address[:]
	default:
		utils.LogPanic("Getting an address from a transaction that doesn't need to be signed.")
		return []byte{}
	}
}
*/

//
// Get a signature from a transaction
//
func (tx Tx) GetTxSignature() []byte {
	switch tx.TxType {
	case ORDER:
		return tx.Tx.(Order).Signature
	case TRANSFER:
		return tx.Tx.(Transfer).Signature
	case CANCEL_ORDER:
		return tx.Tx.(CancelOrder).Signature
	case CREATE_TOKEN:
		return tx.Tx.(CreateToken).Signature
	default:
		utils.LogPanic("Getting a signature from a transaction that doesn't need to be signed.")
		return []byte{}
	}
}

//////////////////////
// TRANSACTION LOGGING

func (gt *Tx) ID() string {
	switch gt.TxType {
	case CREATE_TOKEN:
		return fmt.Sprintf("%v,%v", gt.TxType, gt.Tx.(CreateToken).TokenInfo.Symbol)
	case MATCH:
		return fmt.Sprintf("%v,%v", gt.TxType, gt.Tx.(Match).MatchID)
	case ORDER:
		return fmt.Sprintf("%v,%v", gt.TxType, gt.Tx.(Order).ID)
	case TRANSFER:
		return fmt.Sprintf("%v,%v", gt.TxType, gt.Tx.(Transfer).ID)
	case CANCEL_ORDER:
		return fmt.Sprintf("%v,%v", gt.TxType, gt.Tx.(CancelOrder).OrderID)
	case CLAIM_FUNDS:
		return fmt.Sprintf("%v,%v", gt.TxType, gt.Tx.(ClaimFunds).ID)
	}
	return ""
}

func (tx Tx) String() string {
	switch tx.TxType {
	case MATCH:
		match := tx.Tx.(Match)
		return fmt.Sprintf("{MATCH#%v %s#%v / %s#%v : %v - %v/%v}", match.MatchID, match.BuySymbol, match.BuyOrderID, match.SellSymbol, match.SellOrderID, match.TransferAmt, match.BuyerLoss, match.SellerGain)
	case ORDER:
		order := tx.Tx.(Order)
		return fmt.Sprintf("{ORDER#%v %v %s / %v %s}", order.ID, order.AmountToBuy, order.BuySymbol, order.AmountToSell, utils.KeyToString(order.SellerAddress))
	case TRANSFER:
		transfer := tx.Tx.(Transfer)
		return fmt.Sprintf("{TRANSFER#%v %v %s -> %s}", transfer.ID, transfer.Amount, utils.KeyToString(transfer.FromAddress), utils.KeyToString(transfer.ToAddress))
	case CANCEL_ORDER:
		cancel := tx.Tx.(CancelOrder)
		return fmt.Sprintf("{CANCEL %s#%v}", cancel.OrderSymbol, cancel.OrderID)
	case CLAIM_FUNDS:
		claim := tx.Tx.(ClaimFunds)
		return fmt.Sprintf("{CLAIM#%v %v %s}", claim.ID, claim.Amount, utils.KeyToString(claim.Address))
	case CREATE_TOKEN:
		create := tx.Tx.(CreateToken)
		return fmt.Sprintf("{CREATE %v %s}", create.TokenInfo, utils.KeyToString(create.CreatorAddress))
	default:
		return "{invalid tx}"
	}
}

func (match Match) String() string {
	tx := Tx{match, MATCH}
	return tx.String()
}
func (order Order) String() string {
	tx := Tx{order, ORDER}
	return tx.String()
}
func (transfer Transfer) String() string {
	tx := Tx{transfer, TRANSFER}
	return tx.String()
}
func (cancel CancelOrder) String() string {
	tx := Tx{cancel, CANCEL_ORDER}
	return tx.String()
}
func (claim ClaimFunds) String() string {
	tx := Tx{claim, CLAIM_FUNDS}
	return tx.String()
}
func (create CreateToken) String() string {
	tx := Tx{create, CREATE_TOKEN}
	return tx.String()
}
