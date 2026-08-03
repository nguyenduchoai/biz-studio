package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"bizstudio/internal/server"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

func main() {
	// App mở từ Finder (dmg) chỉ có PATH tối thiểu — bổ sung để thấy claude/ffmpeg/yt-dlp.
	util.AugmentPATH()
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
