package main

import (
	"flag"
	"fmt"
	"log"
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
	dataDir := flag.String("data", util.DefaultDataDir(), "thư mục dữ liệu")
	window := flag.Bool("window", true,
		"mở giao diện trong cửa sổ app riêng; -window=false cho máy chủ không màn hình")
	flag.Parse()
	configureStartupLogging(*dataDir)

	openWindow := *window && desktop.HasDisplay()

	// Đã có một bản đang chạy ở cổng này → mở thêm cửa sổ rồi thoát.
	//
	// Người dùng bấm icon lần thứ hai là chuyện thường. Không có nhánh này thì
	// bản thứ hai chết vì "address already in use" — với người dùng, đó là "bấm
	// vào app mà chẳng thấy gì".
	if runningURL := runningInstanceURL(*dataDir, *port); runningURL != "" {
		openRunningInstance(openWindow, runningURL, *dataDir)
		log.Printf("Biz Studio đã chạy sẵn ở %s — mở thêm cửa sổ.", runningURL)
		return
	}

	// Khóa ở cấp hệ điều hành, giữ suốt vòng đời process. Marker HTTP giúp tìm
	// URL, còn lock mới là thứ bảo đảm không bao giờ có hai writer mở cùng db.
	instance, acquired, err := acquireInstanceLock(*dataDir)
	if err != nil {
		fatalStartup("không khóa được thư mục dữ liệu: %v", err)
	}
	if !acquired {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if runningURL := runningInstanceURL(*dataDir, *port); runningURL != "" {
				openRunningInstance(openWindow, runningURL, *dataDir)
				log.Printf("Biz Studio đang khởi động ở %s — mở thêm cửa sổ.", runningURL)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		log.Printf("Một tiến trình Biz Studio khác đang khởi động với cùng thư mục dữ liệu; vui lòng thử lại sau vài giây.")
		return
	}
	defer instance.Close()

	// Control API chỉ nghe loopback. Nếu cổng ưa thích bị phần mềm khác chiếm,
	// chọn cổng trống thay vì nhận nhầm đó là Biz Studio rồi tự thoát.
	controlLn, actualPort, err := listenControl(*port)
	if err != nil {
		fatalStartup("không mở được cổng điều khiển: %v", err)
	}
	defer controlLn.Close()
	url := fmt.Sprintf("http://127.0.0.1:%d", actualPort)
	if actualPort != *port {
		log.Printf("Cổng %d đang bận — Biz Studio chuyển sang cổng %d.", *port, actualPort)
	}

	// Điện thoại chỉ được vào listener mobile tối giản (trang upload + upload),
	// không bao giờ chạm được API cài đặt/cấu hình trên control listener.
	lanIP := util.LanIP()
	// Listener mobile dùng wildcard để tiếp tục hoạt động khi Wi-Fi đổi IP/VPN
	// hoặc mạng LAN không có Internet. Mux này chỉ có trang/upload có token TTL;
	// toàn bộ control API vẫn chỉ nằm trên loopback.
	mobileLn, mobilePort, mobileErr := listenMobile("0.0.0.0", actualPort+1)
	if mobileErr != nil {
		log.Printf("không mở được cổng nhận file từ điện thoại (%v) — phần điều khiển vẫn hoạt động", mobileErr)
		mobilePort = 0
	} else {
		defer mobileLn.Close()
	}

	st, err := store.Open(*dataDir)
	if err != nil {
		fatalStartup("không mở được dữ liệu: %v", err)
	}

	srv := server.New(st, *dataDir, actualPort, mobilePort)
	if err := writeInstanceFile(*dataDir, url); err != nil {
		log.Printf("không ghi được marker tiến trình: %v", err)
	}
	log.Printf("🚀 Biz Studio — %s", url)
	if mobileLn != nil {
		log.Printf("📱 Nhận file QR — http://%s:%d", lanIP, mobilePort)
		mobileHTTP := &http.Server{
			Handler: srv.MobileHandler(), ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout: 0, WriteTimeout: 0,
			IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20,
		}
		go func() {
			if err := mobileHTTP.Serve(mobileLn); err != nil && err != http.ErrServerClosed {
				log.Printf("listener điện thoại dừng: %v", err)
			}
		}()
	}

	if openWindow {
		if cmd, err := desktop.OpenWindow(st, url, *dataDir); err != nil {
			log.Printf("không mở được cửa sổ app (%v) — mở bằng trình duyệt mặc định", err)
			_ = desktop.OpenDefault(url)
		} else if cmd != nil {
			go quitWhenWindowClosed(cmd, st, url)
		}
	}

	controlHTTP := &http.Server{
		Handler: srv, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 0, WriteTimeout: 0,
		IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20,
	}
	if err := controlHTTP.Serve(controlLn); err != nil && err != http.ErrServerClosed {
		fatalStartup("máy chủ điều khiển dừng: %v", err)
	}
}

func openRunningInstance(openWindow bool, url, dataDir string) {
	if !openWindow {
		return
	}
	if _, err := desktop.OpenWindowDefault(url, dataDir); err != nil {
		_ = desktop.OpenDefault(url)
	}
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
		installing := server.SetupInProgress()
		storageErr := st.PersistenceError()
		if n == 0 && !installing && storageErr == "" {
			log.Printf("Đã đóng cửa sổ — thoát Biz Studio.")
			os.Exit(0)
		}
		if storageErr != "" {
			log.Printf("Không thoát vì còn lỗi lưu dữ liệu chưa khắc phục: %s", storageErr)
		}
		log.Printf("Đã đóng cửa sổ nhưng còn %d việc và trạng thái cài đặt=%t — vẫn giữ máy chủ ở %s. "+
			"Xong hết sẽ tự thoát.", n, installing, url)
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
