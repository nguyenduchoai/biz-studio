package htmlvideo

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// FindChrome dò binary Chrome/Chromium: Settings().ChromeBin → app macOS → PATH.
func FindChrome(st *store.Store) (string, error) {
	configured := strings.TrimSpace(st.Settings().ChromeBin)
	if bin := util.FindChromium(configured, true); bin != "" {
		return bin, nil
	}
	if configured != "" {
		return "", fmt.Errorf("đường dẫn trình duyệt trong Cấu hình & API không tồn tại: %s", configured)
	}
	return "", fmt.Errorf("không tìm thấy Chrome/Edge/Chromium — hãy cài trình duyệt hoặc điền đường dẫn trong Cấu hình & API")
}

// browser — MỘT phiên Chrome headless dùng chung cho cả video
// (chụp frame qua CDP, không spawn Chrome mỗi frame).
type browser struct {
	ctx     context.Context
	cancels []context.CancelFunc
}

// newBrowser khởi động Chrome headless với viewport đúng kích thước video.
func newBrowser(parent context.Context, chromeBin string, w, h int) (*browser, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromeBin),
		chromedp.Flag("headless", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("force-device-scale-factor", "1"),
		// Lần khởi động Chrome đầu tiên trên macOS có thể chậm (Gatekeeper
		// quét binary) — nới thời gian chờ URL DevTools so với mặc định 20s.
		chromedp.WSURLReadTimeout(60*time.Second),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, opts...)
	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	b := &browser{ctx: tabCtx, cancels: []context.CancelFunc{cancelTab, cancelAlloc}}
	if err := chromedp.Run(tabCtx,
		network.Enable(),
		network.SetBlockedURLs().WithURLPatterns([]*network.BlockPattern{
			{URLPattern: "http://*:*/*", Block: true},
			{URLPattern: "https://*:*/*", Block: true},
		}),
		chromedp.EmulateViewport(int64(w), int64(h)),
	); err != nil {
		b.close()
		return nil, fmt.Errorf("khởi động Chrome headless thất bại (%s): %w", chromeBin, err)
	}
	return b, nil
}

func (b *browser) close() {
	for _, c := range b.cancels {
		c()
	}
}

// captureScene mở trang HTML của cảnh rồi chụp từng frame:
// Evaluate window.seek(t) → CaptureScreenshot → frame-%05d.png.
func (b *browser) captureScene(htmlPath, frameDir string, durS float64, fps int, onFrame func()) error {
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục frame %s: %w", frameDir, err)
	}
	var hasSeek bool
	if err := chromedp.Run(b.ctx,
		chromedp.Navigate(fileURL(htmlPath)),
		chromedp.WaitReady("#stage", chromedp.ByQuery),
		waitFontsReady(),
		chromedp.Evaluate("typeof window.seek === 'function'", &hasSeek),
	); err != nil {
		return fmt.Errorf("mở trang cảnh thất bại: %w", err)
	}
	if !hasSeek {
		return fmt.Errorf("trang cảnh không định nghĩa window.seek — template HTML lỗi")
	}
	total := frameCount(durS, fps)
	for f := 0; f < total; f++ {
		t := float64(f) / float64(fps)
		var png []byte
		if err := chromedp.Run(b.ctx,
			chromedp.Evaluate(fmt.Sprintf("window.seek(%.5f); true", t), nil),
			chromedp.CaptureScreenshot(&png),
		); err != nil {
			return fmt.Errorf("chụp frame %d/%d thất bại: %w", f+1, total, err)
		}
		name := filepath.Join(frameDir, fmt.Sprintf("frame-%05d.png", f+1))
		if err := os.WriteFile(name, png, 0o644); err != nil {
			return fmt.Errorf("ghi frame %s: %w", name, err)
		}
		if onFrame != nil {
			onFrame()
		}
	}
	return nil
}

// waitFontsReady chờ trình duyệt nạp xong toàn bộ font @font-face TRƯỚC khi
// chụp frame đầu tiên.
//
// Không chờ thì frame đầu vẽ bằng font dự phòng của hệ điều hành, vài frame sau
// font riêng mới nạp xong — người xem thấy chữ đột ngột đổi kiểu giữa cảnh.
func waitFontsReady() chromedp.Action {
	return chromedp.Evaluate(`document.fonts.ready.then(function () { return true; })`, nil,
		func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		})
}

// frameCount — số frame của cảnh (tối thiểu 1).
func frameCount(durS float64, fps int) int {
	n := int(durS*float64(fps) + 0.5)
	if n < 1 {
		return 1
	}
	return n
}

// fileURL chuyển path tuyệt đối → URL file:// (an toàn với dấu cách, tiếng Việt).
func fileURL(p string) string {
	u := url.URL{Scheme: "file", Path: p}
	return u.String()
}
