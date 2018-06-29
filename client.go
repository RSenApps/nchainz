package main

import (
	"errors"
	"math/rand"
	"net/rpc"
)

type Client struct {
	serverIp string
	rpc      *rpc.Client
}

func NewClient() (*Client, error) {
	client := &Client{}
	err := client.findNode()

	if err != nil {
		panic(err)
	}

	return client, nil
}

func (client *Client) findNode() error {
	seeds, err := GetSeeds()
	if err != nil {
		return errors.New("Couldn't find seeds file. Run \"echo NODE_IP:NODE_PORT > seeds.txt\" and try again.")
	}

	for _, seed := range seeds {
		rpc, err := rpc.Dial("tcp", seed)

		if err == nil {
			Log("Connected to node %s", seed)
			client.rpc = rpc
			client.serverIp = seed

			return nil
		}
	}

	return errors.New("No provided seed is online")
}

func (client *Client) handleErr(err error) {
	if err != nil {
		findErr := client.findNode()

		if findErr != nil {
			panic(err)
		}
	}
}

func (client *Client) SendTx(tx *GenericTransaction, symbol string) error {
	args := TxArgs{*tx, symbol, ""}
	var reply bool

	err := client.rpc.Call("Node.Tx", &args, &reply)

	if err != nil {
		Log("Error communicating with node")
		Log(err.Error())
		return err
	}
	if !reply {
		Log("Node rejected transaction")
		return errors.New("node rejected transaction")
	}

	Log("Transaction successfully sent")
	return nil
}

func (client *Client) Order(buyAmt uint64, buySymbol string, sellAmt uint64, sellSymbol string, seller string) {
	Log("Client sending ORDER")
	defer Log("Client done sending ORDER")

	var empty []byte
	id := rand.Uint64()

	ws := NewWalletStore(false)
	w := ws.GetWallet(seller)

	unsignedOrder := Order{id, buySymbol, sellAmt, buyAmt, w.PublicKey, empty}
	unsignedTx := &GenericTransaction{unsignedOrder, ORDER}

	signature := Sign(w.PrivateKey, *unsignedTx)
	signedOrder := Order{id, buySymbol, sellAmt, buyAmt, w.PublicKey, signature}
	signedTx := &GenericTransaction{signedOrder, ORDER}

	err := client.SendTx(signedTx, sellSymbol)
	if err == nil {
		Log("transaction id: %v", id)
	}
}

func (client *Client) Transfer(amount uint64, symbol string, from string, to string) {
	Log("Client sending TRANSFER")
	defer Log("Client done sending TRANSFER")

	var empty []byte
	id := rand.Uint64()

	ws := NewWalletStore(false)
	wfrom := ws.GetWallet(from)
	wto := ws.GetWallet(to)

	unsignedTransfer := Transfer{id, amount, wfrom.PublicKey, wto.PublicKey, empty}
	unsignedTx := &GenericTransaction{unsignedTransfer, TRANSFER}

	signature := Sign(wfrom.PrivateKey, *unsignedTx)
	signedTransfer := Transfer{id, amount, wfrom.PublicKey, wto.PublicKey, signature}
	signedTx := &GenericTransaction{signedTransfer, TRANSFER}

	err := client.SendTx(signedTx, symbol)
	if err == nil {
		Log("transaction id: %v", id)
	}
}

func (client *Client) Freeze(amount uint64, symbol string, from string, unfreezeBlock uint64) {
	Log("Client sending FREEZE")
	defer Log("Client done sending FREEZE")

	var empty []byte
	id := rand.Uint64()

	ws := NewWalletStore(false)
	wfrom := ws.GetWallet(from)

	unsignedFreeze := Freeze{id, amount, wfrom.PublicKey, unfreezeBlock, empty}
	unsignedTx := &GenericTransaction{unsignedFreeze, FREEZE}

	signature := Sign(wfrom.PrivateKey, *unsignedTx)
	signedFreeze := Freeze{id, amount, wfrom.PublicKey, unfreezeBlock, signature}
	signedTx := &GenericTransaction{signedFreeze, FREEZE}

	err := client.SendTx(signedTx, symbol)
	if err == nil {
		Log("transaction id: %v", id)
	}
}

func (client *Client) Cancel(symbol string, orderId uint64) {
	Log("Client sending CANCEL_ORDER")
	defer Log("Client done sending CANCEL_ORDER")

	// Special case: sign cancel transaction in node.Tx()
	var empty []byte
	unsignedCancel := CancelOrder{symbol, orderId, empty}
	unsignedTx := &GenericTransaction{unsignedCancel, CANCEL_ORDER}

	client.SendTx(unsignedTx, MATCH_CHAIN)
}

func (client *Client) Claim(amount uint64, symbol string, address string) {
	Log("Client sending CLAIM_FUNDS")
	defer Log("Client done sending CLAIM_FUNDS")

	id := rand.Uint64()

	ws := NewWalletStore(false)
	w := ws.GetWallet(address)

	claim := ClaimFunds{id, w.PublicKey, amount}
	tx := &GenericTransaction{claim, CLAIM_FUNDS}

	err := client.SendTx(tx, symbol)
	if err == nil {
		Log("transaction id: %v", id)
	}
}

func (client *Client) Create(symbol string, supply uint64, decimals uint8, address string) {
	Log("Client sending CREATE_TOKEN")
	defer Log("Client done sending CREATE_TOKEN")

	var empty []byte
	tokenInfo := TokenInfo{symbol, supply, decimals}

	ws := NewWalletStore(false)
	w := ws.GetWallet(address)

	unsignedCreate := CreateToken{tokenInfo, w.PublicKey, empty}
	unsignedTx := &GenericTransaction{unsignedCreate, CREATE_TOKEN}

	signature := Sign(w.PrivateKey, *unsignedTx)
	signedCreate := CreateToken{tokenInfo, w.PublicKey, signature}
	signedTx := &GenericTransaction{signedCreate, CREATE_TOKEN}

	client.SendTx(signedTx, MATCH_CHAIN)
}

func (client *Client) GetBalance(address string, symbol string) GetBalanceReply {
	Log("Client sending GETBALANCE")
	defer Log("Client done sending GETBALANCE")

	ws := NewWalletStore(false)
	w := ws.GetWallet(address)

	request := GetBalanceArgs{w.PublicKey, symbol}
	var reply GetBalanceReply
	err := client.rpc.Call("Node.GetBalance", &request, &reply)

	if err != nil {
		Log("Error communicating with node")
		Log(err.Error())
	}
	Log("Address %v has Total Balance: %v (Available: %v, Unclaimed: %v)", address, reply.Amount+reply.Unclaimed, reply.Amount, reply.Unclaimed)
	return reply
}

func (client *Client) GetBook(symbol1 string, symbol2 string) (string, error) {
	Log("Client sending GETBOOK")
	defer Log("Client done sending GETBOOK")

	request := GetBookArgs{symbol1, symbol2}
	var reply GetBookReply
	err := client.rpc.Call("Node.GetBook", &request, &reply)

	client.handleErr(err)

	Log(reply.Serial)

	return reply.Serial, nil
}

func (client *Client) DumpChains(amt uint64) (string, error) {
	Log("Client sending DUMPCHAINS")
	defer Log("Client done sending DUMPCHAINS")

	request := DumpChainsArgs{amt}
	var reply DumpChainsReply
	err := client.rpc.Call("Node.DumpChains", &request, &reply)

	client.handleErr(err)

	Log(reply.Serial)

	return reply.Serial, nil
}
