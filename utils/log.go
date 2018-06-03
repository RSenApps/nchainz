package main

import (
	"github.com/fatih/color"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

//////////////////////////////////////
// LOGGING
// Colored by system component
// Blue: node.go
// Green: multichain.go, blockchain.go
// Magenta: consensus_state.go
// Yellow: miner.go, pow.go

func Log(format string, a ...interface{}) {
	_, path, _, _ := runtime.Caller(1)
	printf(path, format, a...)
}

func LogFatal(format string, a ...interface{}) {
	_, path, _, _ := runtime.Caller(1)
	printf(path, format, a...)
	os.Exit(1)
}

func LogPanic(format string, a ...interface{}) {
	_, path, _, _ := runtime.Caller(1)
	printf(path, format, a...)
	panic("log panic")
}

func LogRed(format string, a ...interface{}) {
	printf("red", format, a...)
}

func printf(path string, format string, a ...interface{}) {
	file := filepath.Base(path)
	var colored string

	if os.Getenv("NCHAINZ_COLORS") == "hi" {
		switch file {
		case "node.go":
			colored = color.HiBlueString(format)
		case "multichain.go", "blockchain.go":
			colored = color.HiGreenString(format)
		case "consensus_state.go":
			colored = color.HiMagentaString(format)
		case "miner.go", "pow.go":
			colored = color.HiYellowString(format)
		case "matcher.go", "orderbook.go":
			colored = color.HiRedString(format)
		case "red":
			colored = color.New(color.FgBlack, color.BgHiWhite).Sprint(format)
		default:
			colored = format
		}

	} else {
		switch file {
		case "node.go":
			colored = color.BlueString(format)
		case "multichain.go", "blockchain.go", "block.go":
			colored = color.GreenString(format)
		case "consensus_state.go":
			colored = color.MagentaString(format)
		case "miner.go", "pow.go":
			colored = color.YellowString(format)
		case "matcher.go", "orderbook.go":
			colored = color.RedString(format)
		case "red":
			colored = color.New(color.FgBlack, color.BgYellow).Sprint(format)
		default:
			colored = format
		}
	}

	log.Printf(colored, a...)
}
