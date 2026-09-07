package qc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Fingerprint lets the publisher verify that its staged copy is the exact file
// that passed QC. Check cancellation between chunks, even for large media.
func Fingerprint(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("không đọc được video để đối chiếu QC: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if err == io.EOF {
			return hex.EncodeToString(h.Sum(nil)), nil
		}
		if err != nil {
			return "", fmt.Errorf("không đọc được toàn bộ video để đối chiếu QC: %w", err)
		}
	}
}
