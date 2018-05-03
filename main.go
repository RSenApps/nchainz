package main

import (
	"encoding/gob"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	gobRegister()

	cli := CLI{}
	cli.Run()
}

func gobRegister() {
	gob.Register(MatchData{})
	gob.Register(TokenData{})
	gob.Register(CreateToken{})
	gob.Register(Match{})
	gob.Register(Order{})
	gob.Register(Transfer{})
	gob.Register(CancelOrder{})
	gob.Register(ClaimFunds{})
	gob.Register(GenericTransaction{})
}
