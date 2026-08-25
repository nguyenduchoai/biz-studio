package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/util"
)

const instanceFileName = "instance.json"

func runningInstanceURL(dataDir string, preferredPort int) string {
	dataID := util.DataDirID(dataDir)
	path := filepath.Join(dataDir, instanceFileName)
	if body, err := os.ReadFile(path); err == nil {
		var saved struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(body, &saved) == nil && isBizStudioAt(saved.URL, dataID) {
			return saved.URL
		}
	}
	preferred := fmt.Sprintf("http://127.0.0.1:%d", preferredPort)
	if isBizStudioAt(preferred, dataID) {
		return preferred
	}
	return ""
}

func writeInstanceFile(dataDir, url string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"url": url, "dataID": util.DataDirID(dataDir)})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dataDir, ".instance-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(dataDir, instanceFileName))
}

// isBizStudioAt xác minh marker HTTP của chính ứng dụng. Chỉ mở được TCP chưa
// đủ: cổng đó có thể là IIS, Docker hoặc phần mềm khác.
func isBizStudioAt(baseURL, expectedDataID string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/api/instance")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var marker struct {
		App    string `json:"app"`
		DataID string `json:"dataID"`
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2048)).Decode(&marker) == nil &&
		marker.App == "bizstudio" && marker.DataID == expectedDataID
}

func listenControl(preferred int) (net.Listener, int, error) {
	ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", preferred))
	if err != nil {
		ln, err = net.Listen("tcp4", "127.0.0.1:0")
	}
	if err != nil {
		return nil, 0, err
	}
	return ln, ln.Addr().(*net.TCPAddr).Port, nil
}

func listenMobile(host string, preferred int) (net.Listener, int, error) {
	ln, err := net.Listen("tcp4", fmt.Sprintf("%s:%d", host, preferred))
	if err != nil {
		ln, err = net.Listen("tcp4", fmt.Sprintf("%s:0", host))
	}
	if err != nil {
		return nil, 0, err
	}
	return ln, ln.Addr().(*net.TCPAddr).Port, nil
}
