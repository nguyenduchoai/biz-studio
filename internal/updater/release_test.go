package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckAndPreparePrerelease(t *testing.T) {
	pkg := []byte("verified update package")
	sum := sha256.Sum256(pkg)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[
 {"tag_name":"v2.14.0-rc.3","prerelease":true,"html_url":"%s/release","body":"Bản mới","assets":[
  {"name":"BizStudio-windows-amd64.zip","browser_download_url":"%s/pkg"},
  {"name":"SHA256SUMS.txt","browser_download_url":"%s/sums"}]},
 {"tag_name":"v2.13.0","html_url":"%s/old","assets":[]}
]`, srv.URL, srv.URL, srv.URL, srv.URL)
		case "/pkg":
			_, _ = w.Write(pkg)
		case "/sums":
			fmt.Fprintf(w, "%s  BizStudio-windows-amd64.zip\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := New("2.14.0-rc.2", t.TempDir())
	m.apiURL = srv.URL + "/releases"
	m.client = srv.Client()
	m.goos, m.goarch = "windows", "amd64"
	info, err := m.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Version != "2.14.0-rc.3" || info.Ready {
		t.Fatalf("Info = %+v", info)
	}
	info, err = m.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Ready {
		t.Fatalf("gói hợp lệ phải sẵn sàng: %+v", info)
	}
	stage, err := m.Stage()
	if err != nil || stage.Archive == "" || stage.Kind != "zip-dir" {
		t.Fatalf("Stage = %+v, %v", stage, err)
	}
}

func TestStableChannelIgnoresPrerelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"v2.15.0-rc.1","prerelease":true,"assets":[
 {"name":"BizStudio-linux-amd64.tar.gz"},{"name":"SHA256SUMS.txt"}]}]`))
	}))
	defer srv.Close()
	m := New("2.14.0", t.TempDir())
	m.apiURL, m.client, m.goos, m.goarch = srv.URL, srv.Client(), "linux", "amd64"
	info, err := m.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Available {
		t.Fatalf("bản ổn định không được tự nhảy sang prerelease: %+v", info)
	}
}

func TestChecksumMismatchIsRejected(t *testing.T) {
	pkg := []byte("tampered")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			fmt.Fprintf(w, `[{"tag_name":"v3.0.0","assets":[
 {"name":"BizStudio-linux-amd64.tar.gz","browser_download_url":"%s/pkg"},
 {"name":"SHA256SUMS.txt","browser_download_url":"%s/sums"}]}]`, srv.URL, srv.URL)
		case "/pkg":
			_, _ = w.Write(pkg)
		case "/sums":
			fmt.Fprintln(w, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  BizStudio-linux-amd64.tar.gz")
		}
	}))
	defer srv.Close()
	m := New("2.14.0", t.TempDir())
	m.apiURL, m.client, m.goos, m.goarch = srv.URL+"/releases", srv.Client(), "linux", "amd64"
	if _, err := m.Prepare(context.Background()); err == nil {
		t.Fatal("checksum sai phải bị từ chối")
	}
}
