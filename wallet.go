package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"golang.org/x/crypto/ripemd160" // go get -u golang.org/x/crypto/ripemd160
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strings"
)

// Based on Jeiwan's tutorial
// https://jeiwan.cc/posts/building-blockchain-in-go-part-5/
// Source of Base58 code: https://github.com/Jeiwan/blockchain_go/blob/402b298d4f908d14df5d7e51e7ae917c0347da47/base58.go

const checksumLength = 4
const version = byte(0x00)
const walletFile = "wallet.dat"
const addressLength = 64
const addressChecksumLen = 4

type Wallet struct {
	PublicKey  [addressLength]byte // public key (concatenated X, Y coordinates)
	PrivateKey ecdsa.PrivateKey    // private key
}

func NewWallet() *Wallet {
	publicKey, privateKey := generateKeys()
	return &Wallet{publicKey, privateKey}
}

//
// Helper method to construct a new Wallet
//
func generateKeys() ([addressLength]byte, ecdsa.PrivateKey) {
	ellipticCurve := elliptic.P256() // get elliptic curve
	privateKey, _ := ecdsa.GenerateKey(ellipticCurve, rand.Reader)
	publicKeySlice := append(privateKey.PublicKey.X.Bytes(), privateKey.PublicKey.Y.Bytes()...)

	var publicKeyArray [addressLength]byte
	copy(publicKeyArray[:], publicKeySlice)
	return publicKeyArray, *privateKey
}

//
// Convert public key into Base 58 address
// encodeBase58(Version + Public key hash + Checksum)
//
func (w Wallet) GetAddress() [addressLength]byte {
	// Get & append version
	rawAddress := []byte{version}

	// Get public key hash
	publicKeyHash := getPublicKeyHash(w.PublicKey)
	// Append public key hash
	rawAddress = append(rawAddress, publicKeyHash...)

	// Get checksum
	checksum := getChecksum(rawAddress)

	fmt.Printf("original checksum %x, made up of version and %x\n", checksum, publicKeyHash)
	// Apppend checksum
	rawAddress = append(rawAddress, checksum...)

	// return encodeBase58(rawAddress)
	return Base58Encode(rawAddress)
}

//
// Helper method to hash public key with RIPEMD160(SHA256(publicKey)) algorithm
//
func getPublicKeyHash(publicKey [addressLength]byte) []byte {
	shaResult := sha256.Sum256(publicKey[:])

	ripHasher := ripemd160.New()
	ripHasher.Write(shaResult[:])
	return ripHasher.Sum(nil)
}

//
// Helper method to get checksum
//
func getChecksum(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:checksumLength]
}

var b58Alphabet = []byte("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")

// Base58Encode encodes a byte array to Base58
func Base58Encode(input []byte) [addressLength]byte {
	var result []byte

	x := big.NewInt(0).SetBytes(input)

	base := big.NewInt(int64(len(b58Alphabet)))
	zero := big.NewInt(0)
	mod := &big.Int{}

	for x.Cmp(zero) != 0 {
		x.DivMod(x, base, mod)
		result = append(result, b58Alphabet[mod.Int64()])
	}

	// https://en.bitcoin.it/wiki/Base58Check_encoding#Version_bytes
	if input[0] == 0x00 {
		result = append(result, b58Alphabet[0])
	}

	ReverseBytes(result)

	var resultArray [addressLength]byte
	copy(resultArray[:], result)
	return resultArray
}

// Base58Decode decodes Base58-encoded data
func Base58Decode(input []byte) []byte {
	result := big.NewInt(0)

	for _, b := range input {
		charIndex := bytes.IndexByte(b58Alphabet, b)
		result.Mul(result, big.NewInt(58))
		result.Add(result, big.NewInt(int64(charIndex)))
	}

	decoded := result.Bytes()

	if input[0] == b58Alphabet[0] {
		decoded = append([]byte{0x00}, decoded...)
	}

	return decoded
}

// ReverseBytes reverses a byte array
func ReverseBytes(data []byte) {
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}

// Storage for group of wallets
type WalletStore struct {
	Wallets map[string]*Wallet
}

func NewWalletStore() *WalletStore {
	ws := WalletStore{}
	ws.Wallets = make(map[string]*Wallet)
	ws.Download()
	return &ws
}

//
// Add a Wallet to WalletStore
//
func (ws *WalletStore) AddWallet() string {
	wallet := NewWallet()
	addressArray := wallet.GetAddress()
	address := fmt.Sprintf("%s", addressArray)

	fmt.Printf("Adding wallet with address %v\n", address)
	ws.Wallets[address] = wallet
	fmt.Println("Walletstore is now", ws.Wallets)
	return address
}

func (ws *WalletStore) Download() {
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		return
	}
	fileContent, err := ioutil.ReadFile(walletFile)
	if err != nil {
		log.Panic(err)
	}

	var newWS WalletStore
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&newWS)
	if err != nil {
		log.Panic(err)
	}

	ws.Wallets = newWS.Wallets
}

//
// Get a Wallet from its address
//
func (ws *WalletStore) GetWallet(address string) Wallet {
	fmt.Println("Walletstore is now", ws.Wallets)

	for w := range ws.Wallets {
		fmt.Println("w in wallet is the same", strings.TrimSpace(w) == strings.TrimSpace(address))
		fmt.Println(".........", w, "........", address, ".........")
	}

	fmt.Println("The wallet we want to get is", *ws.Wallets[address])
	fmt.Println("The wallet's public key is", (*ws.Wallets[address]).PublicKey)
	return *ws.Wallets[address]
}

//
// Persist to file
//
func (ws WalletStore) Persist() {
	var content bytes.Buffer

	gob.Register(elliptic.P256())

	e := gob.NewEncoder(&content)
	err := e.Encode(ws)
	if err != nil {
		log.Panic(err)
	}
	err = ioutil.WriteFile(walletFile, content.Bytes(), 0644)
	if err != nil {
		log.Panic(err)
	}
}

func (ws *WalletStore) GetAddresses() []string {
	var addresses []string

	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}

	return addresses
}

//
// Check if an address is valid
//
func ValidateAddress(address string) bool {
	// publicKeyHash := decodeBase58([]byte(address))
	publicKeyHash := Base58Decode([]byte(address))
	actualChecksum := publicKeyHash[len(publicKeyHash)-checksumLength:]
	version := publicKeyHash[0]
	publicKeyHash = publicKeyHash[1 : len(publicKeyHash)-checksumLength]

	goalChecksum := getChecksum(append([]byte{version}, publicKeyHash...))

	fmt.Printf("%x %x\n", actualChecksum, goalChecksum)
	fmt.Printf("Goal checksum made up of version and %x\n", publicKeyHash)
	return bytes.Compare(actualChecksum, goalChecksum) == 0
}
