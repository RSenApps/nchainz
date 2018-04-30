package main

import (
	"errors"
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

func (client *Client) SendTx(tx *GenericTransaction) error {
	args := TxArgs{*tx, ""}
	var reply bool

	err := client.rpc.Call("Node.Tx", &args, &reply)

	if err != nil {
		return err
	}
	if !reply {
		return errors.New("node rejected transaction")
	}

	return nil
}
