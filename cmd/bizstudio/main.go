package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"bizstudio/internal/cli"
	"bizstudio/internal/server"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

func main() {
	// App mở từ Finder (dmg) chỉ có PATH tối thiểu — bổ sung để thấy claude/ffmpeg/yt-dlp.
	util.AugmentPATH()

	// Có tên lệnh ở đối số đầu → chạy dòng lệnh. Không có, hoặc đối số đầu bắt
	// đầu bằng "-", → chạy giao diện web như trước. Ca thứ hai BẮT BUỘC phải
	// giữ: bản .app trên macOS gọi thẳng `bizstudio -port 6868 -data …`.
	if args := os.Args[1:]; !cli.IsServeMode(args) {
		os.Exit(cli.Dispatch(args))
	}

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
