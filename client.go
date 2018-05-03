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

func NewClient(serverIp string) (*Client, error) {
	rpc, err := rpc.Dial("tcp", serverIp)
	if err != nil {
		return nil, err
	}

	client := &Client{serverIp, rpc}
	return client, nil
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
	order := Order{id, buySymbol, sellAmt, buyAmt, seller, empty}
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
	transfer := Transfer{rand.Uint64(), amount, from, to, empty}
	tx := &GenericTransaction{transfer, TRANSFER}

	client.SendTx(tx, symbol)
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

	claim := ClaimFunds{rand.Uint64(),address, amount}
	tx := &GenericTransaction{claim, CLAIM_FUNDS}

	client.SendTx(tx, symbol)
}

func (client *Client) Create(symbol string, supply uint64, decimals uint8, address string) {
	Log("Client sending CREATE_TOKEN")
	defer Log("Client done sending CREATE_TOKEN")

	var empty []byte
	tokenInfo := TokenInfo{symbol, supply, decimals}
	create := CreateToken{tokenInfo, address, empty}
	tx := &GenericTransaction{create, CREATE_TOKEN}

	client.SendTx(tx, MATCH_CHAIN)
}
