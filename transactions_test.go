package main

import (
	"encoding/gob"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

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

	for {
		r1 := client.GetBalance("Satoshi", "NATIVE")
		r2 := client.GetBalance("x", "NATIVE")
		if !r1.Success || !r2.Success {
			t.Fatal("FAILED: basic transfers")
			return
		}
		if r1.Amount == 99999500 && r2.Amount == 500 {
			LogRed("Passed: basic transfers")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	LogRed("Passed: basic transfers")
	fmt.Println()
}
