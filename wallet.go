package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"golang.org/x/crypto/ripemd160" // go get -u golang.org/x/crypto/ripemd160
	"io/ioutil"
	"log"
	"math/big"
	"os"
)

const checksumLength = 4
const version = byte(0x00)
const walletFile = "wallet.dat"

type Wallet struct {
	PublicKey  []byte           // public key (concatenated X, Y coordinates)
	PrivateKey ecdsa.PrivateKey // private key
}

func NewWallet() *Wallet {
	publicKey, privateKey := generateKeys()
	return &Wallet{publicKey, privateKey}
}

//
// Helper method to construct a new Wallet
//
func generateKeys() ([]byte, ecdsa.PrivateKey) {
	ellipticCurve := elliptic.P256() // get elliptic curve
	privateKey, _ := ecdsa.GenerateKey(ellipticCurve, rand.Reader)
	publicKey := append(privateKey.PublicKey.X.Bytes(), privateKey.PublicKey.Y.Bytes()...)

	return publicKey, *privateKey
}

//
// Convert public key into Base 58 address
// encodeBase58(Version + Public key hash + Checksum)
//
func (w Wallet) GetAddress() []byte {
	// Get & append version
	rawAddress := []byte{version}

	// Get public key hash
	publicKeyHash := getPublicKeyHash(w.PublicKey)
	// Append public key hash
	rawAddress = append(rawAddress, publicKeyHash...)

	// Get checksum
	checksum := getChecksum(rawAddress)
	// Apppend checksum
	rawAddress = append(rawAddress, checksum...)

	return encodeBase58(rawAddress)
}

//
// Helper method to hash public key with RIPEMD160(SHA256(publicKey)) algorithm
//
func getPublicKeyHash(publicKey []byte) []byte {
	shaResult := sha256.Sum256(publicKey)

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

//
// Helper method to encode byte array to base 58
//
func encodeBase58(data []byte) []byte {
	var result []byte

	original := big.NewInt(0).SetBytes(data)

	remainder := &big.Int{}

	for original.Cmp(big.NewInt(0)) != 0 {
		original.DivMod(original, big.NewInt(int64(58)), remainder)
		result = append(result, b58Alphabet[remainder.Int64()])
	}

	if data[0] == 0x00 {
		result = append(result, b58Alphabet[0])
	}

	// Reverse result
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}

	return result
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
	address := string(wallet.GetAddress()[:])
	ws.Wallets[address] = wallet
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
