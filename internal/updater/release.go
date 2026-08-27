package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const releasesAPI = "https://api.github.com/repos/nguyenduchoai/biz-studio/releases?per_page=20"

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

type release struct {
	TagName    string  `json:"tag_name"`
	Body       string  `json:"body"`
	HTMLURL    string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

type Info struct {
	Supported      bool   `json:"supported"`
	CurrentVersion string `json:"currentVersion"`
	Available      bool   `json:"available"`
	Version        string `json:"version,omitempty"`
	Notes          string `json:"notes,omitempty"`
	ReleaseURL     string `json:"releaseUrl,omitempty"`
	AssetName      string `json:"assetName,omitempty"`
	Ready          bool   `json:"ready"`
	Message        string `json:"message,omitempty"`
}

type Manager struct {
	mu        sync.Mutex
	client    *http.Client
	apiURL    string
	current   string
	dataDir   string
	goos      string
	goarch    string
	candidate *release
	selected  *asset
	archive   string
}

func New(current, dataDir string) *Manager {
	return &Manager{
		client:  &http.Client{Timeout: 30 * time.Second},
		apiURL:  releasesAPI,
		current: strings.TrimPrefix(strings.TrimSpace(current), "v"),
		dataDir: dataDir,
		goos:    runtime.GOOS,
		goarch:  runtime.GOARCH,
	}
}

func (m *Manager) Info(ctx context.Context) (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.checkLocked(ctx)
}

func (m *Manager) checkLocked(ctx context.Context) (Info, error) {
	cur, err := parseVersion(m.current)
	if err != nil || m.current == "dev" {
		return Info{CurrentVersion: m.current, Message: "Bản phát triển không tự cập nhật."}, nil
	}
	wantName := assetName(m.goos, m.goarch)
	if wantName == "" {
		return Info{CurrentVersion: m.current, Message: "Nền tảng này chưa có gói cập nhật tự động."}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.apiURL, nil)
	if err != nil {
		return Info{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := m.client.Do(req)
	if err != nil {
		return Info{}, fmt.Errorf("kiểm tra GitHub Release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("GitHub Release trả HTTP %d", resp.StatusCode)
	}
	var releases []release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases); err != nil {
		return Info{}, fmt.Errorf("đọc danh sách Release: %w", err)
	}

	allowPrerelease := len(cur.pre) > 0
	var best *release
	var bestAsset *asset
	var bestVersion version
	for i := range releases {
		r := &releases[i]
		if r.Draft || (r.Prerelease && !allowPrerelease) {
			continue
		}
		v, err := parseVersion(r.TagName)
		if err != nil || compareVersion(v, cur) <= 0 {
			continue
		}
		a := findAsset(r.Assets, wantName)
		if a == nil || findAsset(r.Assets, "SHA256SUMS.txt") == nil {
			continue
		}
		if best == nil || compareVersion(v, bestVersion) > 0 {
			best, bestAsset, bestVersion = r, a, v
		}
	}
	if best == nil {
		m.candidate, m.selected, m.archive = nil, nil, ""
		return Info{Supported: true, CurrentVersion: m.current, Message: "Đang dùng bản mới nhất."}, nil
	}
	if m.candidate == nil || m.candidate.TagName != best.TagName {
		m.archive = ""
	}
	m.candidate, m.selected = best, bestAsset
	return m.infoLocked(true), nil
}

func (m *Manager) Prepare(ctx context.Context) (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.candidate == nil || m.selected == nil {
		if _, err := m.checkLocked(ctx); err != nil {
			return Info{}, err
		}
	}
	if m.candidate == nil || m.selected == nil {
		return Info{}, errors.New("không có bản cập nhật mới")
	}
	dir := filepath.Join(m.dataDir, "updates", safeTag(m.candidate.TagName))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Info{}, err
	}
	manifestAsset := findAsset(m.candidate.Assets, "SHA256SUMS.txt")
	manifest, err := m.download(ctx, *manifestAsset, 2<<20)
	if err != nil {
		return Info{}, err
	}
	want, err := checksumFor(manifest, m.selected.Name)
	if err != nil {
		return Info{}, err
	}
	data, err := m.download(ctx, *m.selected, 1<<30)
	if err != nil {
		return Info{}, err
	}
	got := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(got[:]), want) {
		return Info{}, errors.New("checksum gói cập nhật không khớp — đã hủy cài đặt")
	}
	path := filepath.Join(dir, m.selected.Name)
	tmp := path + ".part"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return Info{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return Info{}, err
	}
	m.archive = path
	return m.infoLocked(true), nil
}

func (m *Manager) Stage() (Stage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.archive == "" || m.candidate == nil || m.selected == nil {
		return Stage{}, errors.New("gói cập nhật chưa được tải và kiểm tra")
	}
	return NewStage(m.archive, m.candidate.TagName, m.goos), nil
}

func (m *Manager) infoLocked(available bool) Info {
	info := Info{Supported: true, CurrentVersion: m.current, Available: available}
	if m.candidate != nil && m.selected != nil {
		info.Version = strings.TrimPrefix(m.candidate.TagName, "v")
		info.Notes = strings.TrimSpace(m.candidate.Body)
		info.ReleaseURL = m.candidate.HTMLURL
		info.AssetName = m.selected.Name
		info.Ready = m.archive != ""
	}
	return info
}

func (m *Manager) download(ctx context.Context, a asset, limit int64) ([]byte, error) {
	if a.BrowserDownloadURL == "" {
		return nil, fmt.Errorf("Release thiếu đường dẫn %s", a.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.BrowserDownloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tải %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tải %s trả HTTP %d", a.Name, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s vượt giới hạn tải", a.Name)
	}
	return data, nil
}

func assetName(goos, goarch string) string {
	switch goos {
	case "windows":
		if goarch == "amd64" {
			return "BizStudio-windows-amd64.zip"
		}
	case "darwin":
		if goarch == "amd64" || goarch == "arm64" {
			return "BizStudio-macos-" + goarch + ".tar.gz"
		}
	case "linux":
		if goarch == "amd64" || goarch == "arm64" {
			return "BizStudio-linux-" + goarch + ".tar.gz"
		}
	}
	return ""
}

func findAsset(assets []asset, name string) *asset {
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i]
		}
	}
	return nil
}

func checksumFor(manifest []byte, name string) (string, error) {
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			manifestName := strings.TrimPrefix(strings.TrimPrefix(fields[1], "*"), "./")
			if manifestName != name {
				continue
			}
			if len(fields[0]) != 64 {
				break
			}
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS.txt không có checksum cho %s", name)
}

func safeTag(tag string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' {
			return r
		}
		return '-'
	}, tag)
}
