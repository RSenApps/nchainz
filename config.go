package main

import (
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
)

const SEED_FILE string = "seeds.txt"

func GetSeeds() ([]string, error) {
	content, err := ioutil.ReadFile(SEED_FILE)
	if err != nil {
		return []string{}, err
	}

	trimmed := strings.TrimSpace(string(content))
	seeds := strings.Split(trimmed, "\n")

	// Randomly shuffle seeds to load balance
	for i := range seeds {
		j := rand.Intn(i + 1)
		seeds[i], seeds[j] = seeds[j], seeds[i]
	}

	return seeds, nil
}

func SetSeeds(seeds []string, myIp string) {
	seeds = append(seeds, myIp)
	data := []byte(strings.Join(seeds, "\n"))
	ioutil.WriteFile(SEED_FILE, data, os.ModePerm)
}
