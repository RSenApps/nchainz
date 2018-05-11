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
	seeds, err := GetSeeds()
	if err != nil {
		return nil, errors.New("Couldn't find seeds file. Run \"echo NODE_IP:NODE_PORT > seeds.txt\" and try again.")
	}

	for _, seed := range seeds {
		rpc, err := rpc.Dial("tcp", seed)

		if err == nil {
			Log("Connected to node %s", seed)

			client := &Client{seed, rpc}
			return client, nil
		}
	}

	return nil, errors.New("No provided seed is online")
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

	order := Order{id, buySymbol, sellAmt, buyAmt, w.PublicKey, empty}
	tx := &GenericTransaction{order, ORDER}

	err := client.SendTx(tx, sellSymbol)
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

	transfer := Transfer{id, amount, wfrom.PublicKey, wto.PublicKey, empty}
	tx := &GenericTransaction{transfer, TRANSFER}

	err := client.SendTx(tx, symbol)
	if err == nil {
		Log("transaction id: %v", id)
	}
}

func (client *Client) Cancel(symbol string, orderId uint64) {
	Log("Client sending CANCEL_ORDER")
	defer Log("Client done sending CANCEL_ORDER")

	var empty []byte
	cancel := CancelOrder{symbol, orderId, empty}
	tx := &GenericTransaction{cancel, CANCEL_ORDER}

	client.SendTx(tx, MATCH_CHAIN)
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

	create := CreateToken{tokenInfo, w.PublicKey, empty}
	tx := &GenericTransaction{create, CREATE_TOKEN}

	client.SendTx(tx, MATCH_CHAIN)
}

func (client *Client) GetBalance(address string, symbol string) GetBalanceReply {
	Log("Client sending GETBALANCE")
	defer Log("Client done sending GETBALANCE")

	request := GetBalanceArgs{address, symbol}
	var reply GetBalanceReply
	err := client.rpc.Call("Node.GetBalance", &request, &reply)

	if err != nil {
		Log("Error communicating with node")
		Log(err.Error())
	}
	if !reply.Success {
		Log("Node rejected GetBalance")
	} else {
		Log("Adddress %v has amount: %v %v", address, reply.Amount, symbol)
	}

	return reply
}

func (client *Client) GetBook(symbol1 string, symbol2 string) (string, error) {
	Log("Client sending GETBOOK")
	defer Log("Client done sending GETBOOK")

	request := GetBookArgs{symbol1, symbol2}
	var reply GetBookReply
	err := client.rpc.Call("Node.GetBook", &request, &reply)

	if err != nil {
		return "", err
	}

	Log(reply.Serial)

	return reply.Serial, nil
}

func (client *Client) DumpChains(amt uint64) (string, error) {
	Log("Client sending DUMPCHAINS")
	defer Log("Client done sending DUMPCHAINS")

	request := DumpChainsArgs{amt}
	var reply DumpChainsReply
	err := client.rpc.Call("Node.DumpChains", &request, &reply)

	if err != nil {
		return "", err
	}

	Log(reply.Serial)

	return reply.Serial, nil
}
