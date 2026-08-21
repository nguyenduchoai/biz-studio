package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"bizstudio/internal/cli"
	"bizstudio/internal/desktop"
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
	window := flag.Bool("window", true,
		"mở giao diện trong cửa sổ app riêng; -window=false cho máy chủ không màn hình")
	flag.Parse()

	url := fmt.Sprintf("http://localhost:%d", *port)
	openWindow := *window && desktop.HasDisplay()

	// Đã có một bản đang chạy ở cổng này → mở thêm cửa sổ rồi thoát.
	//
	// Người dùng bấm icon lần thứ hai là chuyện thường. Không có nhánh này thì
	// bản thứ hai chết vì "address already in use" — với người dùng, đó là "bấm
	// vào app mà chẳng thấy gì".
	if alreadyRunning(*port) {
		if openWindow {
			st, err := store.Open(*dataDir)
			if err != nil {
				_ = desktop.OpenDefault(url)
				return
			}
			if _, err := desktop.OpenWindow(st, url, *dataDir); err != nil {
				_ = desktop.OpenDefault(url)
			}
		}
		log.Printf("Biz Studio đã chạy sẵn ở %s — mở thêm cửa sổ.", url)
		return
	}

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatalf("không mở được store: %v", err)
	}

	srv := server.New(st, *dataDir, *port)
	// Lắng nghe TRƯỚC khi mở cửa sổ: mở sớm quá thì cửa sổ hiện trang lỗi kết
	// nối và người dùng phải tự bấm tải lại.
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("không mở được cổng %d: %v", *port, err)
	}
	log.Printf("🚀 Biz Studio — %s", url)

	if openWindow {
		if cmd, err := desktop.OpenWindow(st, url, *dataDir); err != nil {
			log.Printf("không mở được cửa sổ app (%v) — mở bằng trình duyệt mặc định", err)
			_ = desktop.OpenDefault(url)
		} else if cmd != nil {
			go quitWhenWindowClosed(cmd, st, url)
		}
	}

	if err := http.Serve(ln, srv); err != nil {
		log.Fatal(err)
	}
}

// alreadyRunning kiểm tra có bản Biz Studio nào đang giữ cổng này không.
func alreadyRunning(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// quitWhenWindowClosed thoát khi người dùng đóng cửa sổ app — trừ khi còn việc
// đang chạy.
//
// Đóng cửa sổ mà tắt luôn server thì một lượt render dài đang chạy dở bị giết,
// mất trắng công. Ngược lại, cứ để server sống mãi thì thành tiến trình ma
// người dùng không biết đường tắt. Nên: hết việc mới thoát, còn việc thì sống
// tiếp và nói rõ vì sao.
func quitWhenWindowClosed(cmd *exec.Cmd, st *store.Store, url string) {
	_ = cmd.Wait()
	for {
		n := runningJobs(st)
		if n == 0 {
			log.Printf("Đã đóng cửa sổ — thoát Biz Studio.")
			os.Exit(0)
		}
		log.Printf("Đã đóng cửa sổ nhưng còn %d việc đang chạy — vẫn giữ máy chủ ở %s. "+
			"Xong hết sẽ tự thoát.", n, url)
		time.Sleep(15 * time.Second)
	}
}

func runningJobs(st *store.Store) int {
	n := 0
	for _, j := range st.Jobs() {
		if j.Status == "running" || j.Status == "queued" {
			n++
		}
	}
	return n
}
