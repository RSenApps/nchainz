package main

import (
	"errors"
	"log"
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
		log.Printf("Error communicating with node")
		log.Printf(err.Error())
		return err
	}
	if !reply {
		log.Printf("Node rejected transaction")
		return errors.New("node rejected transaction")
	}

	log.Printf("Transaction successfully sent")
	return nil
}

func (client *Client) Order(buyAmt uint64, buySymbol string, sellAmt uint64, sellSymbol string, seller string) {
	log.Printf("Client sending ORDER")
	defer log.Printf("Client done sending ORDER")

	var empty []byte
	id := rand.Uint64()
	order := Order{id, buySymbol, sellAmt, buyAmt, seller, empty}
	tx := &GenericTransaction{order, ORDER}

	err := client.SendTx(tx, sellSymbol)
	if err == nil {
		log.Printf("transaction id: %v", id)
	}
}

func (client *Client) Transfer(amount uint64, symbol string, from string, to string) {
	log.Printf("Client sending TRANSFER")
	defer log.Printf("Client done sending TRANSFER")

	var empty []byte
	transfer := Transfer{rand.Uint64(), amount, from, to, empty}
	tx := &GenericTransaction{transfer, TRANSFER}

	client.SendTx(tx, symbol)
}

func (client *Client) Cancel(symbol string, orderId uint64) {
	log.Printf("Client sending CANCEL_ORDER")
	defer log.Printf("Client done sending CANCEL_ORDER")

	var empty []byte
	cancel := CancelOrder{symbol, orderId, empty}
	tx := &GenericTransaction{cancel, CANCEL_ORDER}

	client.SendTx(tx, MATCH_CHAIN)
}

func (client *Client) Claim(amount uint64, symbol string, address string) {
	log.Printf("Client sending CLAIM_FUNDS")
	defer log.Printf("Client done sending CLAIM_FUNDS")

	claim := ClaimFunds{address, amount}
	tx := &GenericTransaction{claim, CLAIM_FUNDS}

	client.SendTx(tx, symbol)
}

func (client *Client) Create(symbol string, supply uint64, decimals uint8, address string) {
	log.Printf("Client sending CREATE_TOKEN")
	defer log.Printf("Client done sending CREATE_TOKEN")

	var empty []byte
	tokenInfo := TokenInfo{symbol, supply, decimals}
	create := CreateToken{tokenInfo, address, empty}
	tx := &GenericTransaction{create, CREATE_TOKEN}

	client.SendTx(tx, MATCH_CHAIN)
}
