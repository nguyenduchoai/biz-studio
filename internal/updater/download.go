package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (m *Manager) downloadTo(ctx context.Context, a asset, dst io.Writer, limit int64) error {
	if a.BrowserDownloadURL == "" {
		return fmt.Errorf("Release thiếu đường dẫn %s", a.Name)
	}
	if a.Size > limit {
		return fmt.Errorf("%s vượt giới hạn tải", a.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.BrowserDownloadURL, nil)
	if err != nil {
		return err
	}
	// Metadata requests stay short; a release download needs time on slow Wi-Fi.
	client := *m.client
	client.Timeout = 10 * time.Minute
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("tải %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tải %s trả HTTP %d", a.Name, resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return fmt.Errorf("%s vượt giới hạn tải", a.Name)
	}
	n, err := io.Copy(dst, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return fmt.Errorf("tải %s: %w", a.Name, err)
	}
	if n > limit {
		return fmt.Errorf("%s vượt giới hạn tải", a.Name)
	}
	if a.Size > 0 && n != a.Size {
		return fmt.Errorf("%s chưa tải đủ dữ liệu", a.Name)
	}
	return nil
}

func (m *Manager) downloadArchive(ctx context.Context, a asset, path, checksum string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".bizstudio-download-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	hash := sha256.New()
	err = m.downloadTo(ctx, a, io.MultiWriter(f, hash), 1<<30)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), checksum) {
		return errors.New("checksum gói cập nhật không khớp — đã hủy cài đặt")
	}
	// Windows cannot rename onto an existing file. Only remove a previous
	// downloaded archive after the replacement has passed verification.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(f.Name(), path)
}
