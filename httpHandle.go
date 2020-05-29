package main

/*
http handle 服务端
*/

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
)

var (
	db       = database{"shoes": 50, "socks": 5}
	muUpdate sync.Mutex
)

func main() {
	http.HandleFunc("/list", db.list)     // 请求示例: curl "http://localhost:8000/list"
	http.HandleFunc("/price", db.price)   // 请求示例: curl "http://localhost:8000/price?item=shoes"
	http.HandleFunc("/update", db.update) // 请求示例: curl "http://localhost:8000/update?item=shoes&price=59"
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
}

type database map[string]float32

func (db database) list(w http.ResponseWriter, req *http.Request) {
	for item, price := range db {
		fmt.Fprintf(w, "%s: $%.2f\n", item, price)
	}
}

func (db database) price(w http.ResponseWriter, req *http.Request) {
	item := req.URL.Query().Get("item")
	price, ok := db[item]
	if !ok {
		w.WriteHeader(http.StatusNotFound) // 404
		fmt.Fprintf(w, "no such item: %q\n", item)
		return
	}
	fmt.Fprintf(w, "%.2f\n", price)
}

func (db database) update(w http.ResponseWriter, req *http.Request) {
	item := req.URL.Query().Get("item")
	price := req.URL.Query().Get("price")
	old, ok := db[item]
	new, err := strconv.ParseFloat(price, 32)
	if !ok {
		w.WriteHeader(http.StatusNotFound) // 404
		fmt.Fprintf(w, "no such item: %q\n", item)
	} else if err != nil {
		w.WriteHeader(http.StatusNotFound) // 404
		fmt.Fprintf(w, "invalid price: %q\n", price)
	} else {
		muUpdate.Lock()
		db[item] = float32(new)
		fmt.Fprintf(w, "update %s prices: %.2f to %.2f\n", item, old, new)
		muUpdate.Unlock()
	}
}
