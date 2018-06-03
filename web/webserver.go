package main

import (
	"fmt"
	"net/http"
	"strings"
)

type Webserver struct {
	port   uint64
	client *Client
}

func StartWebserver(port uint64) {
	client, err := NewClient()
	if err != nil {
		panic(err)
	}

	webserver := &Webserver{port, client}

	http.Handle("/", http.FileServer(http.Dir("./web")))
	http.HandleFunc("/book/", webserver.book)
	http.HandleFunc("/chains", webserver.chains)

	localPath := fmt.Sprintf(":%v", port)
	if err := http.ListenAndServe(localPath, nil); err != nil {
		panic(err)
	}
}

func (ws *Webserver) book(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	chunks := strings.Split(path, "/")

	if len(chunks) != 4 || chunks[0] != "" || chunks[1] != "book" || chunks[2] == "" || chunks[3] == "" {
		err := "invalid path"
		w.Write([]byte(err))
		return
	}

	symbol1 := chunks[2]
	symbol2 := chunks[3]

	bookData, err := ws.client.GetBook(symbol1, symbol2)

	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	w.Write([]byte(bookData))
}

func (ws *Webserver) chains(w http.ResponseWriter, r *http.Request) {
	amt := uint64(10)
	chainData, err := ws.client.DumpChains(amt)

	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	w.Write([]byte(chainData))
}
