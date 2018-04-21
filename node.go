package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

type Node struct {
	mu    sync.RWMutex
	myIp  string
	peers map[string]*Peer
	bc    *Blockchain
}

type Peer struct {
	ip      string
	state   PeerState
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

type PeerState uint

const (
	FOUND PeerState = iota + 1
	ACTIVE
	EXPIRED
	INVALID
	UNKNOWN
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

	mu := sync.RWMutex{}
	peers := make(map[string]*Peer)
	bc := NewBlockchain(dbName)
	node := Node{mu, myIp, peers, bc}

	rpc.Register(&node)
	go node.connectPeerIfNew(seedIp)

	log.Printf("Listening on %s", myIp)
	rpc.Accept(inbound)
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

	isNew, peer, err := node.connectPeerIfNew(args.From)
	if !isNew {
		*reply = false
		return nil
	}
	if err != nil {
		*reply = false
		return err
	}

	myStartHeight := node.getStartHeight()
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
	startHeight := node.getStartHeight()

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

	peerState := node.getPeerState(args.From)
	if peerState != ACTIVE && peerState != FOUND {
		log.Printf("Received addr from inactive or unknown peer %s", args.From)
		*reply = false
		return nil
	}

	for _, ip := range args.Ips {
		go node.connectPeerIfNew(ip)
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
	node.mu.Lock()
	defer node.mu.Unlock()

	for _, peer := range node.peers {
		if peer.state == ACTIVE {
			go node.sendAddr(peer)
		}
	}
}

////////////////////////////////
// Utils

func (node *Node) connectPeerIfNew(peerIp string) (isNew bool, peer *Peer, err error) {
	node.mu.Lock()

	testPeer, ok := node.peers[peerIp]
	if peerIp == node.myIp || ok && (testPeer.state == ACTIVE || testPeer.state == FOUND) {
		node.mu.Unlock()
		return false, testPeer, nil
	}

	peer = &Peer{peerIp, FOUND, nil, time.Now()}
	node.peers[peerIp] = peer

	node.mu.Unlock()

	log.Printf("Attempting to connect to peer %s", peerIp)

	client, err := rpc.Dial("tcp", peerIp)
	if err != nil {
		log.Printf("Dialing error connecting to peer %s", peerIp)
		node.setPeerState(peerIp, INVALID)

		return true, nil, err
	}

	node.mu.Lock()
	peer.client = client
	node.mu.Unlock()

	node.sendVersion(peer)
	node.setPeerState(peerIp, ACTIVE)

	node.broadcastAddr()

	ips := node.getPeerIps()
	log.Printf("Connected peer %s, known peers: %v", peerIp, ips)

	return true, peer, nil
}

func (node *Node) getPeerIps() []string {
	node.mu.Lock()
	defer node.mu.Unlock()

	ips := make([]string, 0)

	for ip, peer := range node.peers {
		if peer.state == ACTIVE {
			ips = append(ips, ip)
		}
	}

	return ips
}

func (node *Node) getPeerState(peerIp string) PeerState {
	node.mu.Lock()
	defer node.mu.Unlock()

	peer, ok := node.peers[peerIp]
	if !ok {
		return UNKNOWN
	}

	return peer.state
}

func (node *Node) setPeerState(peerIp string, state PeerState) {
	node.mu.Lock()
	defer node.mu.Unlock()

	node.peers[peerIp].state = state
}

func (node *Node) getStartHeight() int {
	node.mu.Lock()
	defer node.mu.Unlock()

	return node.bc.GetStartHeight()
}
