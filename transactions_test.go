package main

import (
	"encoding/gob"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// Example: `go test -run "TestBasicTransfer"`
// Run `sh config.sh RESET` between tests to reset dbs

const genesisAddress = "1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K"

// Transfer: Satoshi --500 NATIVE--> x
func TestBasicTransfer(t *testing.T) {
	LogRed("Testing: basic transfers")

	rand.Seed(time.Now().UTC().UnixNano())
	gob.RegisterName("main.Transfer", Transfer{})

	client, err := NewClient()
	if err != nil {
		t.Fatal("FAILED: basic transfers")
		return
	}

	ws := NewWalletStore(false)
	address := ws.AddWallet()
	client.Transfer(500, "NATIVE", genesisAddress, address)

	success := checkBalance(client, []string{genesisAddress, address}, []uint64{99999500, 500}, "NATIVE")

	client.Transfer(500, "NATIVE", address, genesisAddress)

	if success {
		LogRed("Passed: basic transfers")
	} else {
		t.Fatal("FAILED: basic transfers")
	}
	fmt.Println()
}

// Transfer: Satoshi --500 NATIVE--> x
// Transfer: x --500 NATIVE--> Satoshi
func TestSwapTransfer(t *testing.T) {
	LogRed("Testing: swap transfers")

	rand.Seed(time.Now().UTC().UnixNano())
	gob.RegisterName("main.Transfer", Transfer{})

	client, err := NewClient()
	if err != nil {
		t.Fatal("FAILED: swap transfers")
		return
	}

	ws := NewWalletStore(false)
	address := ws.AddWallet()

	client.Transfer(500, "NATIVE", genesisAddress, address)
	success := checkBalance(client, []string{genesisAddress, address}, []uint64{99999500, 500}, "NATIVE")

	if !success {
		t.Fatal("FAILED: swap transfers")
	}

	client.Transfer(500, "NATIVE", address, genesisAddress)
	success = checkBalance(client, []string{genesisAddress, address}, []uint64{100000000, 0}, "NATIVE")

	if success {
		LogRed("Passed: swap transfers")
	} else {
		t.Fatal("FAILED: swap transfers")
	}

	fmt.Println()
}

// Transfer: Satoshi --10 NATIVE--> x
func TestManyTransfers(t *testing.T) {
	LogRed("Testing: many transfers")

	rand.Seed(time.Now().UTC().UnixNano())
	gob.RegisterName("main.Transfer", Transfer{})

	client, err := NewClient()
	if err != nil {
		t.Fatal("FAILED: many transfers")
		return
	}

	ws := NewWalletStore(false)
	address := ws.AddWallet()

	totalTransfers := uint64(10000)
	amtPerTransfer := uint64(10)
	numClients := uint64(25)

	for i := uint64(0); i < numClients; i++ {
		go func() {
			client, _ = NewClient()

			for j := uint64(0); j < totalTransfers/numClients; j++ {
				client.Transfer(amtPerTransfer, "NATIVE", genesisAddress, address)
			}
		}()
	}
	success := checkBalance(client, []string{genesisAddress, address}, []uint64{100000000 - totalTransfers*amtPerTransfer, totalTransfers * amtPerTransfer}, "NATIVE")

	if success {
		LogRed("Passed: many transfers")
	} else {
		t.Fatal("FAILED: many transfers")
	}

	client.Transfer(totalTransfers*amtPerTransfer, "NATIVE", address, genesisAddress)

	fmt.Println()
}

func TestMultipleClients(t *testing.T) {
	LogRed("Testing: multiple clients")

	rand.Seed(time.Now().UTC().UnixNano())
	gob.RegisterName("main.Transfer", Transfer{})

	client1, err := NewClient()
	if err != nil {
		t.Fatal("FAILED: many transfers")
		return
	}

	client2, err := NewClient()
	if err != nil {
		t.Fatal("FAILED: many transfers")
		return
	}

	client3, err := NewClient()
	if err != nil {
		t.Fatal("FAILED: many transfers")
		return
	}

	ws := NewWalletStore(false)
	address := ws.AddWallet()

	amounts := []uint64{100, 300, 200, 400}

	for i := 0; i < len(amounts); i++ {
		client1.Transfer(amounts[i], "NATIVE", genesisAddress, address)
		client2.Transfer(amounts[i], "NATIVE", genesisAddress, address)
		client3.Transfer(amounts[i], "NATIVE", genesisAddress, address)
	}

	success := checkBalance(client1, []string{genesisAddress, address}, []uint64{99997000, 3000}, "NATIVE")
	if !success {
		t.Fatal("FAILED: multiple clients")
	}
	success = checkBalance(client1, []string{genesisAddress, address}, []uint64{99997000, 3000}, "NATIVE")
	if !success {
		t.Fatal("FAILED: multiple clients")
	}
	success = checkBalance(client1, []string{genesisAddress, address}, []uint64{99997000, 3000}, "NATIVE")
	if !success {
		t.Fatal("FAILED: multiple clients")
	}

	if success {
		LogRed("Passed: multiple clients")
	}
}

// Transfers that shouldn't work
// Cyclic transfers

func abs(x uint64, y uint64) uint64 {
	if x >= y {
		return x - y
	} else {
		return y - x
	}
}

//
// Helper method to check balances
//
func checkBalance(client *Client, users []string, amounts []uint64, symbol string) bool {
	if len(users) != len(amounts) {
		LogRed("ERROR: Length of users != length of amounts in checkBalance()")
	}

	for {
		success := true
		for i := 0; i < len(users); i++ {
			r := client.GetBalance(users[i], symbol)

			if abs(r.Amount, amounts[i]) >= 1000 {
				success = false
				break
			}
		}

		if success {
			return true
		}

		time.Sleep(1000 * time.Millisecond)
	}

	return false
}
