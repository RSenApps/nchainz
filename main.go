package main

import (
	"encoding/gob"
	"github.com/fatih/color"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	gobRegister()
	color.NoColor = false

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
