package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"time"
)

type Node struct {
	myIp  string
	peers []*Peer
	bc    *Blockchain
}

type Peer struct {
	ip      string
	client  *rpc.Client
	addedAt time.Time
}

type MsgType uint

const (
	VERSION MsgType = iota + 1
	GETBLOCKS
	INV
	GETDATA
	BLOCK
	TX
	ADDR
)

func StartNode(port uint, seedIp string) {
	myIp := fmt.Sprintf("127.0.0.1:%v", port)
	dbName := fmt.Sprintf("db/%v.db", port)

	addr, err := net.ResolveTCPAddr("tcp", myIp)
	if err != nil {
		log.Fatal(err)
	}

	inbound, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	knownPeers := make([]*Peer, 0)
	bc := NewBlockchain(dbName)
	node := Node{myIp, knownPeers, bc}

	rpc.Register(&node)
	go node.connectPeer(seedIp)

	log.Printf("Listening on %s", myIp)
	rpc.Accept(inbound)
}

func (node *Node) connectPeer(peerIp string) (*Peer, error) {
	log.Printf("Attempting to connect to peer %s", peerIp)

	client, err := rpc.Dial("tcp", peerIp)
	if err != nil {
		log.Printf("Dialing error connecting to peer %s", peerIp)
		return nil, err
	}

	if !node.isNewPeer(peerIp) {
		log.Printf("Peer %s is not new", peerIp)
		return nil, nil
	}
	peer := Peer{peerIp, client, time.Now()}
	node.peers = append(node.peers, &peer)

	node.sendVersion(&peer)
	go node.broadcastAddr()

	log.Printf("Connected peer %s, known peers: %v", peerIp, node.getPeerIps())
	return &peer, nil
}

func (node *Node) isNewPeer(peerIp string) bool {
	if peerIp == node.myIp {
		return false
	}

	for _, peer := range node.peers {
		if peer.ip == peerIp {
			return false
		}
	}

	return true
}

func (node *Node) getPeerIps() []string {
	numPeers := len(node.peers)
	ips := make([]string, numPeers, numPeers)

	for i := range ips {
		ips[i] = node.peers[i].ip
	}

	return ips
}

////////////////////////////////
// VERSION

type VersionArgs struct {
	Version     int
	StartHeight int
	From        string
}

func (node *Node) Version(args *VersionArgs, reply *bool) error {
	log.Printf("Received VERSION from %s", args.From)
	defer log.Printf("Done handling VERSION from %s", args.From)

	if !node.isNewPeer(args.From) {
		*reply = false
		return nil
	}

	peer, err := node.connectPeer(args.From)
	if err != nil {
		*reply = false
		return err
	}

	myStartHeight := node.bc.GetStartHeight()
	if myStartHeight < args.StartHeight {
		// go sendGetBlocks
	}

	go node.sendAddr(peer)

	*reply = true
	return nil
}

func (node *Node) sendVersion(peer *Peer) {
	log.Printf("Sending VERSION to %s", peer.ip)
	defer log.Printf("Done sending VERSION to %s", peer.ip)

	version := 0
	startHeight := node.bc.GetStartHeight()

	args := VersionArgs{version, startHeight, node.myIp}
	var reply bool

	err := peer.client.Call("Node.Version", &args, &reply)
	if err != nil {
		log.Print(err)
	}
}

////////////////////////////////
// ADDR

type AddrArgs struct {
	Ips  []string
	From string
}

func (node *Node) Addr(args *AddrArgs, reply *bool) error {
	log.Printf("Received ADDR from %s", args.From)
	defer log.Printf("Done handling ADDR from %s", args.From)

	if node.isNewPeer(args.From) {
		log.Printf("Received addr from unknown peer")
		*reply = false
		return nil
	}

	for _, ip := range args.Ips {
		if node.isNewPeer(ip) {
			go node.connectPeer(ip)
		}
	}

	*reply = true
	return nil
}

func (node *Node) sendAddr(peer *Peer) {
	log.Printf("Sending ADDR to %s", peer.ip)
	defer log.Printf("Done sending ADDR to %s", peer.ip)

	ips := node.getPeerIps()
	args := AddrArgs{ips, node.myIp}
	var reply bool

	err := peer.client.Call("Node.Addr", &args, &reply)
	if err != nil {
		log.Print(err)
	}
}

func (node *Node) broadcastAddr() {
	for _, peer := range node.peers {
		go node.sendAddr(peer)
	}
}
