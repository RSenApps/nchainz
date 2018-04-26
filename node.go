package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type Node struct {
	mu    sync.RWMutex
	myIp  string
	peers map[string]*Peer
	bcs   *Blockchains
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
	bcs := CreateNewBlockchains(dbName)
	node := Node{mu, myIp, peers, bcs}

	log.SetOutput(ioutil.Discard)
	rpc.Register(&node)
	log.SetOutput(os.Stdout)

	go node.connectPeerIfNew(seedIp)
	go node.invLoop()

	log.Printf("Listening on %s", myIp)
	rpc.Accept(inbound)
}

////////////////////////////////
// VERSION
// sends handshake to new peer

type VersionArgs struct {
	Version      int
	StartHeights []uint64
	From         string
}

func (node *Node) Version(args *VersionArgs, reply *bool) error {
	log.Printf("Received VERSION from %s", args.From)
	defer log.Printf("Done handling VERSION from %s", args.From)

	isNew, _, err := node.connectPeerIfNew(args.From)
	if !isNew {
		*reply = false
		return nil
	}
	if err != nil {
		*reply = false
		return err
	}

	// myStartHeights := node.bcs.GetHeights()
	// if short go sendGetBlocks

	*reply = true
	return nil
}

func (node *Node) SendVersion(peer *Peer) {
	version := 0
	startHeights := node.bcs.GetHeights()
	args := VersionArgs{version, startHeights, node.myIp}

	node.callVersion(peer, &args)
}

func (node *Node) callVersion(peer *Peer, args *VersionArgs) {
	log.Printf("Sending VERSION to %s", peer.ip)
	defer log.Printf("Done sending VERSION to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Version", &args, &reply)
	node.handleRpcReply(peer, err, &reply)
}

////////////////////////////////
// ADDR
// send list of known peers

type AddrArgs struct {
	Ips  []string
	From string
}

func (node *Node) Addr(args *AddrArgs, reply *bool) error {
	log.Printf("Received ADDR from %s", args.From)
	defer log.Printf("Done handling ADDR from %s", args.From)

	peerState := node.getPeerState(args.From)
	if peerState != ACTIVE && peerState != FOUND {
		log.Printf("Received INV from inactive or unknown peer %s", args.From)
		*reply = false
		return nil
	}

	for _, ip := range args.Ips {
		go node.connectPeerIfNew(ip)
	}

	*reply = true
	return nil
}

func (node *Node) SendAddr(peer *Peer) {
	args := node.getAddrArgs()
	node.callAddr(peer, args)
}

func (node *Node) BroadcastAddr() {
	args := node.getAddrArgs()

	for _, peer := range node.getActivePeers() {
		go node.callAddr(peer, args)
	}
}

func (node *Node) getAddrArgs() *AddrArgs {
	ips := node.getPeerIps()
	args := AddrArgs{ips, node.myIp}

	return &args
}

func (node *Node) callAddr(peer *Peer, args *AddrArgs) {
	log.Printf("Sending ADDR to %s", peer.ip)
	defer log.Printf("Done sending ADDR to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Addr", args, &reply)
	node.handleRpcReply(peer, err, &reply)
}

////////////////////////////////
// INV
// send all blockhashes to peer

type InvArgs struct {
	Blockhashes [][][]byte
	From        string
}

func (node *Node) Inv(args *InvArgs, reply *bool) error {
	log.Printf("Received INV from %s", args.From)
	defer log.Printf("Done handling INV from %s", args.From)

	peerState := node.getPeerState(args.From)
	if peerState != ACTIVE && peerState != FOUND {
		log.Printf("Received INV from inactive or unknown peer %s", args.From)
		*reply = false
		return nil
	}

	*reply = true
	return nil
}

func (node *Node) SendInv(peer *Peer) {
	args := node.getInvArgs()
	node.callInv(peer, args)
}

func (node *Node) BroadcastInv() {
	args := node.getInvArgs()

	for _, peer := range node.getActivePeers() {
		go node.callInv(peer, args)
	}
}

func (node *Node) getInvArgs() *InvArgs {
	blockhashes := node.bcs.GetBlockhashes()
	args := InvArgs{blockhashes, node.myIp}

	return &args
}

func (node *Node) callInv(peer *Peer, args *InvArgs) {
	log.Printf("Sending INV to %s", peer.ip)
	defer log.Printf("Done sending INV to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Inv", args, &reply)
	node.handleRpcReply(peer, err, &reply)
}

func (node *Node) invLoop() {
	interval := 5 * time.Second
	ticker := time.NewTicker(interval)

	for {
		<-ticker.C
		node.BroadcastInv()
	}
}

////////////////////////////////
// Utils: Connecting

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

	node.SendVersion(peer)
	node.setPeerState(peerIp, ACTIVE)

	node.BroadcastAddr()

	ips := node.getPeerIps()
	log.Printf("Connected peer %s, known peers: %v", peerIp, ips)

	return true, peer, nil
}

////////////////////////////////
// Utils: RPC Management

func (node *Node) handleRpcReply(peer *Peer, err error, reply *bool) {
	if err != nil {
		log.Print(err)
		node.setPeerState(peer.ip, INVALID)
	}
}

////////////////////////////////
// Utils: State Access

func (node *Node) getActivePeers() []*Peer {
	node.mu.Lock()
	defer node.mu.Unlock()

	peers := make([]*Peer, 0)

	for _, peer := range node.peers {
		if peer.state == ACTIVE {
			peers = append(peers, peer)
		}
	}

	return peers
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
