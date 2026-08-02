package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"bizstudio/internal/server"
	"bizstudio/internal/store"
)

func main() {
	port := flag.Int("port", 6868, "cổng HTTP")
	dataDir := flag.String("data", "data", "thư mục dữ liệu")
	flag.Parse()

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatalf("không mở được store: %v", err)
	}

	srv := server.New(st, *dataDir, *port)
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("🚀 Biz Studio — http://localhost:%d", *port)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
