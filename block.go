package main


type TokenType uint8
const (
	BNB    TokenType = iota + 1
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

type MatchData struct {
	Matches []Match
	Cancels []Cancel
}

type TokenData struct {
	Orders []Order
	Cancels []Cancel
	Transfers []Transfer
}

type Match struct {
	SellTokenType TokenType
	BuyTokenType TokenType
	AmountSold uint64
	SellerBlockIndex uint64
	BuyerBlockIndex uint64
	SellerOrderOffset uint8
	BuyerOrderOffset uint8
	SellerBlockHash []byte
	BuyerBlockHash []byte
}

type Order struct {
	BuyTokenType TokenType
	AmountToSell uint64
	AmountToBuy uint64
	SellerAddress []byte
	Signature []byte
}

type Transfer struct {
	Amount uint64
	FromAddress []byte
	ToAddress[]byte
	Signature []byte
}

type Cancel struct {
	BlockIndex uint64
	BlockOffset uint8
	Signature []byte
}