package main

import (
	"encoding/gob"
)

func main() {
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
