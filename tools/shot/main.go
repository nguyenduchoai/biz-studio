package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

type page struct{ hash, name string }

func main() {
	out := os.Args[1]
	os.MkdirAll(out, 0o755)
	pages := []page{
		{"dashboard", "01-tong-quan"},
		{"studio", "02-xuong-lam-san"},
		{"htmlvideo", "03-html-video"},
		{"text2video", "04-text-to-video"},
		{"projects", "05-du-an"},
		{"characters", "06-nhan-vat"},
		{"look", "07-dien-mao"},
		{"tts", "08-tts"},
	}
	bin := os.Getenv("CHROME")
	if bin == "" {
		for _, c := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		} {
			if _, err := os.Stat(c); err == nil {
				bin = c
				break
			}
		}
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(bin),
		chromedp.WindowSize(1600, 1000),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("force-device-scale-factor", "2"), // ảnh nét gấp đôi
	)
	alloc, cancelA := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelA()
	ctx, cancel := chromedp.NewContext(alloc)
	defer cancel()
	ctx, cancelT := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelT()

	if err := chromedp.Run(ctx, chromedp.EmulateViewport(1600, 1000)); err != nil {
		fmt.Fprintln(os.Stderr, "khởi động Chrome:", err)
		os.Exit(1)
	}
	for _, p := range pages {
		var png []byte
		err := chromedp.Run(ctx,
			chromedp.Navigate("http://localhost:6868/#/"+p.hash),
			chromedp.Sleep(2500*time.Millisecond), // chờ gọi API và vẽ xong
			chromedp.CaptureScreenshot(&png),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", p.name, err)
			continue
		}
		f := filepath.Join(out, p.name+".png")
		os.WriteFile(f, png, 0o644)
		fmt.Printf("  %-22s %d KB\n", p.name, len(png)/1024)
	}
}
