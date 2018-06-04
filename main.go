package main

import (
	"encoding/gob"
	"github.com/fatih/color"
	"math/rand"
	"time"

	"github.com/rsenapps/nchainz/cli"
	"github.com/rsenapps/nchainz/tx"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	gobRegister()
	color.NoColor = false

	nchainzCLI := cli.CLI{}
	nchainzCLI.Run()
}

func gobRegister() {
	gob.Register(tx.MatchData{})
	gob.Register(tx.TokenData{})
	gob.Register(tx.CreateToken{})
	gob.Register(tx.Match{})
	gob.Register(tx.Order{})
	gob.Register(tx.Transfer{})
	gob.Register(tx.CancelOrder{})
	gob.Register(tx.ClaimFunds{})
	gob.Register(tx.GenericTransaction{})
}
