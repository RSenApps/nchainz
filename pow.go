package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/big"
	"math/rand"
	"time"
)

// Controls difficulty of mining
const targetBits = 24

// Maximum value of counter
var maxNonce = math.MaxInt64

type ProofOfWork struct {
	block  *Block   // pointer to block
	target *big.Int // pointer to target
}

func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits)) // left shift
	return &ProofOfWork{b, target}
}

//
// Helper method for prepareData
// Converts int64 to byte array
//
func IntToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

//
// Merge block fields with target and nonce (counter)
//
func (pow *ProofOfWork) prepareData(nonce int) []byte {
	blockBytes, _ := GetBytes(pow.block.Data)
	data := bytes.Join(
		[][]byte{
			pow.block.PrevBlockHash,
			blockBytes,
			IntToHex(pow.block.Timestamp),
			IntToHex(int64(targetBits)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)

	return data
}

func random(max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max)
}

//
// Generates random nonce for "iterations" number of times
// Returns nonce and hash
//
func (pow *ProofOfWork) Try(iterations int) (bool, int, []byte) {
	for i := 0; i < iterations; i++ {
		rand := random(maxNonce)
		success, nonce, hash := pow.Calculate(rand)
		if success {
			return success, nonce, hash
		}
	}

	return false, 0, nil
}

//
// Returns nonce and hash
//
func (pow *ProofOfWork) Calculate(nonce int) (bool, int, []byte) {
	var hashInt big.Int // int representation of hash
	var hash [32]byte

	data := pow.prepareData(nonce) // prepare data
	hash = sha256.Sum256(data)     // hash with SHA-256
	hashInt.SetBytes(hash[:])      // convert hash to a big integer

	if hashInt.Cmp(pow.target) == -1 { // compare integer with target
		return true, nonce, hash[:] // if hash < target, valid proof!
	}

	return false, nonce, hash[:]
}

func (pow *ProofOfWork) Run() (int, []byte) {
	var hashInt big.Int // int representation of hash
	var hash [32]byte
	nonce := 0 // counter

	fmt.Printf("Mining the block containing \"%s\"\n", pow.block.Data)
	for nonce < maxNonce {
		data := pow.prepareData(nonce) // prepare data
		hash = sha256.Sum256(data)     // hash with SHA-256
		fmt.Printf("\r%x", hash)
		hashInt.SetBytes(hash[:]) // convert hash to a big integer

		if hashInt.Cmp(pow.target) == -1 { // compare integer with target
			break // if hash < target, valid proof!
		} else {
			nonce++
		}
	}
	fmt.Print("\n\n")

	return nonce, hash[:]
}

func (pow *ProofOfWork) GetHash() []byte {
	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	return hash[:]
}

//
// Validate proof of work
//
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int

	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.target) == -1

	return isValid
}
