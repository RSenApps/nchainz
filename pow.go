package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"math/rand"
)

// Controls difficulty of mining
const targetBits = 24

// Maximum value of counter
var maxNonce = math.MaxInt32

type ProofOfWork struct {
	block  *Block
	target *big.Int
}

func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))
	return &ProofOfWork{b, target}
}

func (pow *ProofOfWork) Try(iterations int) (bool, int, []byte) {
	blockBytes := GetBytes(pow.block.Data)
	blockData := pow.prepareData(blockBytes)

	for i := 0; i < iterations; i++ {
		nonce := rand.Intn(maxNonce)
		success, hash := pow.Calculate(blockData, nonce)
		if success {
			Log("MINING SUCCESS")
			return success, nonce, hash
		}
	}

	return false, 0, nil
}

// Returns nonce and hash
func (pow *ProofOfWork) Calculate(dataHash []byte, nonce int) (bool, []byte) {
	var hashInt big.Int

	hash := nonceHash(dataHash, nonce)
	hashInt.SetBytes(hash[:])

	if hashInt.Cmp(pow.target) == -1 {
		return true, hash[:]
	}

	return false, hash[:]
}

// Validate proof of work
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int
	hash := pow.GetHash()
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.target) == -1

	return isValid
}

func (pow *ProofOfWork) GetHash() []byte {
	blockBytes := GetBytes(pow.block.Data)
	blockData := pow.prepareData(blockBytes)
	return nonceHash(blockData, pow.block.Nonce)
}

// Merge block fields with target
func (pow *ProofOfWork) prepareData(dataBytes []byte) []byte {
	data := bytes.Join(
		[][]byte{
			pow.block.PrevBlockHash,
			dataBytes,
			IntToBytes(pow.block.Timestamp),
			IntToBytes(int64(targetBits)),
		},
		[]byte{},
	)
	dataHash := sha256.Sum256(data)
	return dataHash[:]
}

func nonceHash(blockData []byte, nonce int) []byte {
	withNonce := bytes.Join(
		[][]byte{
			blockData,
			IntToBytes(int64(nonce)),
		},
		[]byte{},
	)
	hash := sha256.Sum256(withNonce)
	return hash[:]
}

func IntToBytes(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		LogPanic(err.Error())
	}

	return buff.Bytes()
}
