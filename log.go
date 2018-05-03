package main

import (
	"github.com/fatih/color"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

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

func printf(path string, format string, a ...interface{}) {
	file := filepath.Base(path)
	var colored string

	switch file {
	case "node.go":
		colored = color.BlueString(format)
	case "blockchains.go", "blockchain.go", "block.go":
		colored = color.GreenString(format)
	case "consensus_state.go":
		colored = color.MagentaString(format)
	case "miner.go", "pow.go":
		colored = color.YellowString(format)
	case "matcher.go", "orderbook.go":
		colored = color.RedString(format)
	default:
		colored = format
	}

	log.Printf(colored, a...)
}
