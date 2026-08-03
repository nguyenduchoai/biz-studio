package text2video

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	fetchTimeout  = 30 * time.Second
	fetchMaxBytes = 5 << 20 // 5MB
	browserUA     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// noiseTags — khối không phải nội dung bài viết: bỏ cả thẻ lẫn nội dung bên trong.
var noiseTags = []string{
	"script", "style", "noscript", "svg", "iframe", "form",
	"nav", "footer", "header", "aside", "template", "button", "select",
}

// reNoiseBlocks — mỗi thẻ nhiễu một regexp riêng (regexp của Go không hỗ trợ
// tham chiếu ngược \1 nên không gộp chung được).
var reNoiseBlocks = func() []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(noiseTags))
	for _, t := range noiseTags {
		out = append(out, regexp.MustCompile(`(?is)<`+t+`\b[^>]*>.*?</\s*`+t+`\s*>`))
	}
	return out
}()

var (
	reComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	reArticle  = regexp.MustCompile(`(?is)<article\b[^>]*>(.*?)</\s*article\s*>`)
	reMain     = regexp.MustCompile(`(?is)<main\b[^>]*>(.*?)</\s*main\s*>`)
	reBody     = regexp.MustCompile(`(?is)<body\b[^>]*>(.*?)</\s*body\s*>`)
	reTitleTag = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</\s*title\s*>`)
	reOgTitle  = regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)\s*=\s*["'](?:og:title|twitter:title)["'][^>]*content\s*=\s*["']([^"']*)["']`)
	reH1       = regexp.MustCompile(`(?is)<h1\b[^>]*>(.*?)</\s*h1\s*>`)
	reBr       = regexp.MustCompile(`(?is)<br\s*/?>`)
	reBlockEnd = regexp.MustCompile(`(?is)</\s*(p|div|section|article|main|li|ul|ol|tr|table|h[1-6]|blockquote|pre|figure|figcaption|dd|dt|td)\s*>`)
	reAnyTag   = regexp.MustCompile(`(?s)<[^>]*>`)
	// Khoảng trắng trong dòng: space, tab, NBSP (&nbsp;), zero-width space, BOM.
	reInlineWS   = regexp.MustCompile(`[ \t\x{00A0}\x{200B}\x{FEFF}]+`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

// FetchArticle tải một trang web và bóc lấy tiêu đề + nội dung văn bản để làm
// nguồn viết kịch bản. Ưu tiên vùng <article>/<main>, bỏ script/style/nav/
// footer/header/aside, giải mã HTML entity, gộp khoảng trắng và giữ xuống dòng
// giữa các khối.
func FetchArticle(ctx context.Context, rawURL string) (string, string, error) {
	target := strings.TrimSpace(rawURL)
	if target == "" {
		return "", "", errors.New("chưa nhập địa chỉ bài viết")
	}
	low := strings.ToLower(target)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		target = "https://" + target
	}
	if u, err := url.ParseRequestURI(target); err != nil || u.Host == "" {
		return "", "", fmt.Errorf("địa chỉ bài viết không hợp lệ: %q", rawURL)
	}

	page, err := download(ctx, target)
	if err != nil {
		return "", "", err
	}
	title := extractTitle(page)
	text := extractText(page)
	if strings.TrimSpace(text) == "" {
		return title, "", fmt.Errorf(
			"không bóc được nội dung văn bản từ %s — trang có thể tải nội dung bằng JavaScript hoặc chặn truy cập; "+
				"hãy mở trang rồi dán trực tiếp nội dung vào ô văn bản", target)
	}
	return title, text, nil
}

// download tải HTML thô của trang (giới hạn 5MB, timeout 30s, User-Agent trình duyệt).
func download(ctx context.Context, target string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return "", fmt.Errorf("không tạo được yêu cầu tới %s: %w", target, err)
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi,vi-VN;q=0.9,en;q=0.8")

	resp, err := (&http.Client{Timeout: fetchTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("không tải được trang %s: %w", target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("trang %s trả về HTTP %d (%s) — kiểm tra lại link, hoặc dán trực tiếp nội dung bài viết",
			target, resp.StatusCode, strings.TrimSpace(http.StatusText(resp.StatusCode)))
	}
	ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "text/") && !strings.Contains(ct, "xml") {
		return "", fmt.Errorf("địa chỉ này không phải trang HTML (Content-Type: %s) — hãy dán trực tiếp nội dung bài viết", ct)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, fetchMaxBytes))
	if err != nil {
		return "", fmt.Errorf("đọc nội dung trang %s thất bại: %w", target, err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("trang %s trả về nội dung rỗng", target)
	}
	return string(data), nil
}

// extractTitle lấy tiêu đề theo thứ tự og:title → <title> → <h1>.
func extractTitle(page string) string {
	for _, re := range []*regexp.Regexp{reOgTitle, reTitleTag, reH1} {
		if m := re.FindStringSubmatch(page); m != nil {
			t := cleanInline(m[1])
			// <title> hay kèm tên site sau dấu gạch — giữ phần đầu nếu đủ dài.
			for _, sep := range []string{" | ", " — ", " – ", " - "} {
				if i := strings.Index(t, sep); i > 20 {
					t = strings.TrimSpace(t[:i])
					break
				}
			}
			if t != "" {
				return shortText(t, 160)
			}
		}
	}
	return ""
}

// extractText bóc phần văn bản chính của trang.
func extractText(page string) string {
	clean := reComment.ReplaceAllString(page, " ")
	// Chạy 2 lượt: khối nhiễu lồng nhau (vd <nav> trong <header>).
	for range 2 {
		for _, re := range reNoiseBlocks {
			clean = re.ReplaceAllString(clean, " ")
		}
	}

	body := pickLongestMatch(reArticle, clean)
	if len([]rune(stripTags(body))) < 200 {
		if m := pickLongestMatch(reMain, clean); len([]rune(stripTags(m))) > len([]rune(stripTags(body))) {
			body = m
		}
	}
	if strings.TrimSpace(body) == "" {
		if m := reBody.FindStringSubmatch(clean); m != nil {
			body = m[1]
		} else {
			body = clean
		}
	}
	return normalizeText(stripTags(body))
}

// pickLongestMatch trả nội dung khối dài nhất khớp regexp (trang có thể có nhiều
// <article> — bài chính thường là khối dài nhất).
func pickLongestMatch(re *regexp.Regexp, page string) string {
	best := ""
	for _, m := range re.FindAllStringSubmatch(page, -1) {
		if len(m[1]) > len(best) {
			best = m[1]
		}
	}
	return best
}

// stripTags đổi thẻ xuống dòng/khối thành "\n", bỏ mọi thẻ còn lại, giải mã entity.
func stripTags(v string) string {
	v = reBr.ReplaceAllString(v, "\n")
	v = reBlockEnd.ReplaceAllString(v, "\n")
	v = reAnyTag.ReplaceAllString(v, " ")
	return html.UnescapeString(v)
}

// normalizeText gộp khoảng trắng trong dòng, bỏ dòng rác, giới hạn 2 dòng trống liên tiếp.
func normalizeText(v string) string {
	v = strings.ReplaceAll(v, "\r\n", "\n")
	v = strings.ReplaceAll(v, "\r", "\n")

	var b strings.Builder
	for _, line := range strings.Split(v, "\n") {
		l := strings.TrimSpace(reInlineWS.ReplaceAllString(line, " "))
		if l == "" {
			b.WriteString("\n")
			continue
		}
		// Bỏ dòng rác kiểu menu/nút bấm (quá ngắn và không có dấu câu).
		if len([]rune(l)) < 3 {
			continue
		}
		b.WriteString(l)
		b.WriteString("\n")
	}
	out := reBlankLines.ReplaceAllString(b.String(), "\n\n")
	return strings.TrimSpace(out)
}

// cleanInline gộp một đoạn HTML ngắn (tiêu đề) thành một dòng text sạch.
func cleanInline(v string) string {
	t := stripTags(v)
	t = strings.ReplaceAll(t, "\n", " ")
	return strings.TrimSpace(reInlineWS.ReplaceAllString(t, " "))
}
