package main

import (
	"errors"
	"log"
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

func (client *Client) Order(buyAmt uint64, buySymbol string, sellAmt uint64, sellSymbol string, seller string) {
	log.Printf("Client sending ORDER")
	defer log.Printf("Client done sending ORDER")

	var empty []byte
	order := Order{0, buySymbol, sellAmt, buyAmt, seller, empty}
	tx := &GenericTransaction{order, ORDER}

	client.SendTx(tx, sellSymbol)
}

func (client *Client) Transfer(amount uint64, symbol string, from string, to string) {
	log.Printf("Client sending TRANSFER")
	defer log.Printf("Client done sending TRANSFER")

	var empty []byte
	transfer := Transfer{amount, from, to, empty}
	tx := &GenericTransaction{transfer, TRANSFER}

	client.SendTx(tx, symbol)
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

	return nil
}
