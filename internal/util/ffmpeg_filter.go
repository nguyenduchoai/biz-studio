package util

import (
	"path/filepath"
	"strings"
)

// FFmpegFilterPath returns one unquoted filter option value, ready to append
// after filename= or fontfile=. argv bypasses the shell but FFmpeg still parses
// option values and the filtergraph separately; each layer consumes escapes.
func FFmpegFilterPath(path string) string {
	// Only Windows separators become slashes; Unix filenames may contain '\\'.
	return FFmpegFilterValue(filepath.ToSlash(path))
}

func FFmpegFilterValue(value string) string {
	option := escapeFFmpegToken(value, "\\':")
	return escapeFFmpegToken(option, "\\'[],;")
}

func escapeFFmpegToken(value, special string) string {
	var b strings.Builder
	for _, char := range value {
		if strings.ContainsRune(special, char) || strings.ContainsRune(" \t\r\n", char) {
			b.WriteByte('\\')
		}
		b.WriteRune(char)
	}
	return b.String()
}
