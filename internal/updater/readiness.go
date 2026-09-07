package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/util"
)

func awaitRelaunch(cmd *exec.Cmd, stage Stage, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	client := &http.Client{
		Timeout:       500 * time.Millisecond,
		Transport:     &http.Transport{}, // Local readiness must not go through a proxy.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if updateReady(stage, client) {
			return nil
		}
		select {
		case err := <-done:
			return fmt.Errorf("ứng dụng dừng trước khi sẵn sàng: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
	}
	// Only stop our own new child; Windows needs it to release its EXE before
	// rollback can restore the old version.
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("ứng dụng mới chưa sẵn sàng và chưa dừng được: %w", err)
	}
	<-done
	return errors.New("ứng dụng mới không phản hồi sau khi khởi động")
}

func updateReady(stage Stage, client *http.Client) bool {
	dataDir, preferred := "", ""
	for i := 0; i+1 < len(stage.LaunchArgs); i++ {
		switch stage.LaunchArgs[i] {
		case "-data":
			dataDir = stage.LaunchArgs[i+1]
			i++
		case "-port":
			preferred = "http://127.0.0.1:" + stage.LaunchArgs[i+1]
			i++
		}
	}
	if dataDir == "" {
		return false
	}
	dataID := util.DataDirID(dataDir)
	var saved struct {
		URL    string `json:"url"`
		DataID string `json:"dataID"`
	}
	if body, err := os.ReadFile(filepath.Join(dataDir, "instance.json")); err == nil && json.Unmarshal(body, &saved) == nil && saved.DataID == dataID {
		if instanceReady(client, saved.URL, dataID, stage.Tag) {
			return true
		}
	}
	return instanceReady(client, preferred, dataID, stage.Tag)
}

func instanceReady(client *http.Client, baseURL, dataID, tag string) bool {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "http" || u.User != nil || !net.ParseIP(u.Hostname()).IsLoopback() {
		return false
	}
	u.Path, u.RawQuery, u.Fragment = "/api/instance", "", ""
	resp, err := client.Get(u.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var marker struct {
		App     string `json:"app"`
		Version string `json:"version"`
		DataID  string `json:"dataID"`
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2048)).Decode(&marker) == nil &&
		marker.App == "bizstudio" && marker.DataID == dataID &&
		(tag == "" || strings.TrimPrefix(marker.Version, "v") == strings.TrimPrefix(tag, "v"))
}
