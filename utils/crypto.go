package utils

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"golang.org/x/crypto/ripemd160"
	"io/ioutil"
	"log"
	"math/big"
	"os"
)

const checksumLength = 4
const version = byte(0x00)

const walletFile = "wallet.dat"
const genesisFile = "genesis.dat"

const addressLength = 64
const addressStringLength = 34
const addressChecksumLen = 4

//////////////////////////////////////////
// WALLET
// Store a pair of public and private keys

type Wallet struct {
	PublicKey  [addressLength]byte // public key (concatenated X, Y coordinates)
	PrivateKey ecdsa.PrivateKey    // private key
}

func NewWallet() *Wallet {
	publicKey, privateKey := generateKeys()
	return &Wallet{publicKey, privateKey}
}

func generateKeys() ([addressLength]byte, ecdsa.PrivateKey) {
	ellipticCurve := elliptic.P256() // get elliptic curve
	privateKey, _ := ecdsa.GenerateKey(ellipticCurve, rand.Reader)
	publicKeySlice := append(privateKey.PublicKey.X.Bytes(), privateKey.PublicKey.Y.Bytes()...)

	var publicKeyArray [addressLength]byte
	copy(publicKeyArray[:], publicKeySlice)
	return publicKeyArray, *privateKey
}

func (w Wallet) GetAddress() [addressLength]byte {
	return PublicKeyToAddress(w.PublicKey)
}

//
// Convert public key into Base 58 address
//
func PublicKeyToAddress(key [addressLength]byte) [addressLength]byte {
	// Get & append version
	rawAddress := []byte{version}

	// Get public key hash
	publicKeyHash := getPublicKeyHash(key)
	// Append public key hash
	rawAddress = append(rawAddress, publicKeyHash...)

	// Get checksum
	checksum := getChecksum(rawAddress)

	// Apppend checksum
	rawAddress = append(rawAddress, checksum...)

	// return encodeBase58(rawAddress)
	return Base58Encode(rawAddress)
}

//
// Get a Wallet from its address
//
func (ws *WalletStore) GetWallet(address string) Wallet {
	return *ws.Wallets[address[:addressStringLength]]
}

func KeyToString(key [addressLength]byte) string {
	address := PublicKeyToAddress(key)
	return string(address[:addressLength])
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

////////////////////////////////
// BASE58 ENCODING
// encode: byte array to base 58
// decode: base 58 to byte array
// Acknowledgement of Base58 code:
// https://github.com/Jeiwan/blockchain_go/blob/402b298d4f908d14df5d7e51e7ae917c0347da47/base58.go

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

/////////////////////////////
// WALLETSTORE
// Storage of list of wallets

type WalletStore struct {
	Wallets map[string]*Wallet
}

func NewWalletStore(isGenesis bool) *WalletStore {
	ws := WalletStore{}
	ws.Wallets = make(map[string]*Wallet)

	ws.Download(genesisFile)
	if !isGenesis {
		ws.Download(walletFile)
	}

	return &ws
}

func (ws *WalletStore) AddWallet() string {
	wallet := NewWallet()
	addressArray := wallet.GetAddress()
	address := string(addressArray[:addressLength])

	ws.Wallets[address] = wallet
	ws.Persist()
	return address
}

func (ws *WalletStore) Download(file string) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return
	}
	fileContent, err := ioutil.ReadFile(file)
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

	// Take union of two wallets so we can get genesis block
	for k, v := range newWS.Wallets {
		k = k[:addressStringLength]
		ws.Wallets[k] = v
	}
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
		addresses = append(addresses, address[:addressStringLength])
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

	return bytes.Compare(actualChecksum, goalChecksum) == 0
}
