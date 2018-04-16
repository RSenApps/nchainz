package main

func main() {
	chain := NewBlockchain()
	defer chain.db.Close()

	cli := CLI{chain}
	cli.Run()
}
