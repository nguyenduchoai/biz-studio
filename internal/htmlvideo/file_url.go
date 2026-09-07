package htmlvideo

import (
	"net/url"
	"path/filepath"
	"strings"
)

func fileURL(absolutePath string) string {
	return fileURLForPlatform(absolutePath, filepath.Separator == '\\')
}

// A Windows drive is part of the URL path (/C:/...), never its host. Keep
// platform handling explicit so Windows paths are covered by tests on macOS
// too; replacing backslashes on Unix would corrupt valid local filenames.
func fileURLForPlatform(absolutePath string, windows bool) string {
	p, host := absolutePath, ""
	if windows {
		p = strings.ReplaceAll(p, `\`, "/")
		if len(p) >= 8 && strings.EqualFold(p[:8], "//?/UNC/") {
			p = "//" + p[8:]
		} else if strings.HasPrefix(p, "//?/") {
			p = p[4:]
		}
		if strings.HasPrefix(p, "//") {
			// UNC: \\server\share\scene.html → file://server/share/scene.html.
			parts := strings.SplitN(p[2:], "/", 2)
			host, p = parts[0], "/"
			if len(parts) == 2 {
				p += parts[1]
			}
		} else if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
	}
	return (&url.URL{Scheme: "file", Host: host, Path: p}).String()
}
