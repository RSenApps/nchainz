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

	client.Transfer(500, "NATIVE", "Satoshi", "x")

	success := checkBalance(client, []string{"Satoshi", "x"}, []uint64{99999500, 500}, "NATIVE")

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

	client.Transfer(500, "NATIVE", "Satoshi", "x")
	success := checkBalance(client, []string{"Satoshi", "x"}, []uint64{99999500, 500}, "NATIVE")
	if !success {
		t.Fatal("FAILED: swap transfers")
	}

	client.Transfer(500, "NATIVE", "x", "Satoshi")
	success = checkBalance(client, []string{"Satoshi", "x"}, []uint64{100000000, 0}, "NATIVE")

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

	for i := 0; i < 10000; i++ {
		client.Transfer(10, "NATIVE", "Satoshi", "x")
	}
	success := checkBalance(client, []string{"Satoshi", "x"}, []uint64{99900000, 100000}, "NATIVE")

	if success {
		LogRed("Passed: many transfers")
	} else {
		t.Fatal("FAILED: many transfers")
	}

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

	client1.Transfer(500, "NATIVE", "Satoshi", "x")
	client2.Transfer(600, "NATIVE", "Satoshi", "x")
	client3.Transfer(1, "NATIVE", "x", "Satoshi")

	success := checkBalance(client1, []string{"Satoshi", "x"}, []uint64{99998901, 1099}, "NATIVE")
	if !success {
		t.Fatal("FAILED: multiple clients")
	}
	success = checkBalance(client1, []string{"Satoshi", "x"}, []uint64{99998901, 1099}, "NATIVE")
	if !success {
		t.Fatal("FAILED: multiple clients")
	}
	success = checkBalance(client1, []string{"Satoshi", "x"}, []uint64{99998901, 1099}, "NATIVE")
	if !success {
		t.Fatal("FAILED: multiple clients")
	}

	if success {
		LogRed("Passed: multiple clients")
	}
}

// Transfers that shouldn't work
// Cyclic transfers

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

			if !r.Success {
				return false
			}

			if r.Amount != amounts[i] {
				success = false
				break
			}
		}

		if success {
			return true
		}

		time.Sleep(100 * time.Millisecond)
	}

	return false
}
