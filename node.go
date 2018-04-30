package main

import (
	//"bytes"
	"errors"
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

//////////////////////////////////
// VERSION
// Handshake for a new peer
// request: data about yourself
// response: true if successful

type VersionArgs struct {
	Version      int
	StartHeights map[string]uint64
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
	node.handleRpcReply(peer, err)
}

////////////////////////////////
// ADDR
// Share list of known peers
// request: peer list
// response: true if successful

type AddrArgs struct {
	Ips  []string
	From string
}

func (node *Node) Addr(args *AddrArgs, reply *bool) error {
	log.Printf("Received ADDR from %s", args.From)
	defer log.Printf("Done handling ADDR from %s", args.From)

	peerState := node.getPeerState(args.From)
	if peerState != ACTIVE && peerState != FOUND {
		log.Printf("Received ADDR from inactive or unknown peer %s", args.From)
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
	node.handleRpcReply(peer, err)
}

//////////////////////////////////////////
// INV
// Share blockhashes with peer
// request: all blockhashes for all chains
// response: true if successful

type InvArgs struct {
	Blockhashes  map[string][][]byte
	StartHeights map[string]uint64
	From         string
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

	myHeights := node.bcs.GetHeights()

	for symbol, startHeight := range args.StartHeights {
		if myHeights[symbol] < startHeight {
			node.reconcileChain(args.From, symbol, args.Blockhashes[symbol], startHeight)
		}
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
	startHeights := node.bcs.GetHeights()
	args := InvArgs{blockhashes, startHeights, node.myIp}

	return &args
}

func (node *Node) callInv(peer *Peer, args *InvArgs) {
	log.Printf("Sending INV to %s", peer.ip)
	defer log.Printf("Done sending INV to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Inv", args, &reply)
	node.handleRpcReply(peer, err)
}

func (node *Node) invLoop() {
	interval := 10 * time.Second
	ticker := time.NewTicker(interval)

	for {
		<-ticker.C
		node.BroadcastInv()
	}
}

/////////////////////////////////////////
// GETBLOCK
// Get a block on a chain
// request: blockhash of requested block
// response: the requested block

type GetBlockArgs struct {
	Blockhash []byte
	Symbol    string
	From      string
}

type GetBlockReply struct {
	Success bool
	Block   Block
}

func (node *Node) GetBlock(args *GetBlockArgs, reply *GetBlockReply) error {
	log.Printf("Received GETBLOCK from %s", args.From)
	defer log.Printf("Done handling GETBLOCK from %s", args.From)

	peerState := node.getPeerState(args.From)
	if peerState != ACTIVE && peerState != FOUND {
		log.Printf("Received GETBLOCK from inactive or unknown peer %s", args.From)
		*reply = GetBlockReply{false, Block{}}
	}

	block, err := node.bcs.GetBlock(args.Symbol, args.Blockhash)
	if err != nil {
		*reply = GetBlockReply{false, Block{}}
	} else {
		*reply = GetBlockReply{true, *block}
	}

	return nil
}

func (node *Node) SendGetBlock(peer *Peer, blockhash []byte, symbol string) (*Block, error) {
	args := &GetBlockArgs{blockhash, symbol, node.myIp}
	reply, err := node.callGetBlock(peer, args)
	node.handleRpcReply(peer, err)

	if err != nil {
		return nil, err
	} else if !reply.Success {
		return nil, errors.New("GETBLOCK unsuccessful")
	}

	return &reply.Block, nil
}

func (node *Node) callGetBlock(peer *Peer, args *GetBlockArgs) (*GetBlockReply, error) {
	log.Printf("Sending GETBLOCK to %s", peer.ip)
	defer log.Printf("Done sending GETBLOCK to %s", peer.ip)

	reply := &GetBlockReply{}
	err := peer.client.Call("Node.GetBlock", args, reply)
	return reply, err
}

//////////////////////////////////////////
// TX
// Share transaction with peers
// request: transaction data
// response: true if successful

type TxArgs struct {
	Tx   GenericTransaction
	From string
}

func (node *Node) Tx(args *TxArgs, reply *bool) error {
	log.Printf("Received TX from %s", args.From)
	defer log.Printf("Done handling TX from %s", args.From)

	// TODO: add tx to mempool if it isn't there
	// TODO: gossip tx to peers if new

	*reply = true
	return nil
}

func (node *Node) BroadcastTx(tx *GenericTransaction) {
	args := &TxArgs{*tx, node.myIp}

	for _, peer := range node.getActivePeers() {
		go node.callTx(peer, args)
	}
}

func (node *Node) callTx(peer *Peer, args *TxArgs) {
	log.Printf("Sending TX to %s", peer.ip)
	defer log.Printf("Done sending TX to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Tx", args, &reply)
	node.handleRpcReply(peer, err)
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
// Utils: Chain Management

func (node *Node) reconcileChain(peerIp string, symbol string, blockhashes [][]byte, theirHeight uint64) {
	/*peer := node.peers[peerIp]
	bc, _ := node.bcs.GetChain(symbol)

	firstMissing := theirHeight
	found := false
	for _, blockhash := range blockhashes {
		if bytes.Equal(blockhash, bc.tipHash) {
			firstMissing++
			found = true
			break
		}

		firstMissing--
	}

	if !found {
		// there was a fork
	}

	for i := firstMissing; i <= theirHeight; i++ {
		block, err := node.SendGetBlock(peer, blockhashes[theirHeight-i], symbol)
		if err != nil {
			log.Printf(err.Error())
			return
		}

		bc.AddBlock(block.Data, block.Type)
	}*/
}

////////////////////////////////
// Utils: RPC Management

func (node *Node) handleRpcReply(peer *Peer, err error) {
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
