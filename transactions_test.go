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

	time.Sleep(100 * time.Millisecond)

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

	time.Sleep(100 * time.Millisecond)

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

// Multiple client
// Lots of transfers
// Transfers that shouldn't work
// Cyclic transfers

//
// Helper method to check balances
//
func checkBalance(client *Client, users []string, amounts []uint64, symbol string) bool {
	duration := 100
	if len(users) != len(amounts) {
		LogRed("ERROR: Length of users != length of amounts in checkBalance()")
	}

	for {
		for i := 0; i < len(users); i++ {
			r := client.GetBalance(users[i], symbol)

			if !r.Success {
				return false
			}

			if r.Amount == amounts[i] {
				return true
			} else {
				break
			}
		}

		time.Sleep(100 * time.Millisecond)
		duration -= 1
		if duration == 0 {
			return false
		}
	}

	return false
}
