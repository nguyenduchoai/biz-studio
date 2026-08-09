package tts

import "strings"

// ---------- Gắn nhãn ngôn ngữ cho giọng ----------
//
// Máy đã sẵn giọng của hàng chục ngôn ngữ, nhưng danh sách chỉ hiện mã kiểu
// "ja_JP" nên người dùng không tìm ra. Ở đây đổi mã thành tên tiếng Việt và gom
// nhóm, để làm video tiếng Anh / Nhật / Trung… không phải đi tìm ở đâu khác.

// LangName đổi mã ngôn ngữ thành tên tiếng Việt kèm vùng. Mã lạ thì trả lại
// nguyên mã — thà hiện mã còn hơn đoán sai tên nước.
func LangName(code string) string {
	c := strings.TrimSpace(code)
	if c == "" {
		return ""
	}
	// giọng VieNeu ghi kiểu "vi-VN · Bắc", giữ nguyên phần vùng miền
	if strings.HasPrefix(c, "vi-VN") {
		return strings.Replace(c, "vi-VN", "Tiếng Việt", 1)
	}
	if strings.EqualFold(c, "đa ngôn ngữ") {
		return c
	}
	norm := strings.ReplaceAll(c, "-", "_")
	if n, ok := langNames[norm]; ok {
		return n
	}
	// chỉ có phần ngôn ngữ, không có vùng
	if i := strings.Index(norm, "_"); i > 0 {
		if n, ok := baseNames[norm[:i]]; ok {
			return n
		}
	}
	if n, ok := baseNames[norm]; ok {
		return n
	}
	return c
}

// LangBase trả mã ngôn ngữ gốc để gom nhóm ("en_US" → "en").
func LangBase(code string) string {
	c := strings.ReplaceAll(strings.TrimSpace(code), "-", "_")
	if i := strings.Index(c, "_"); i > 0 {
		return strings.ToLower(c[:i])
	}
	if i := strings.Index(c, " "); i > 0 {
		return strings.ToLower(c[:i])
	}
	return strings.ToLower(c)
}

var baseNames = map[string]string{
	"vi": "Tiếng Việt", "en": "Tiếng Anh", "zh": "Tiếng Trung", "ja": "Tiếng Nhật",
	"ko": "Tiếng Hàn", "fr": "Tiếng Pháp", "de": "Tiếng Đức", "es": "Tiếng Tây Ban Nha",
	"it": "Tiếng Ý", "pt": "Tiếng Bồ Đào Nha", "ru": "Tiếng Nga", "th": "Tiếng Thái",
	"id": "Tiếng Indonesia", "ms": "Tiếng Mã Lai", "hi": "Tiếng Hindi", "ar": "Tiếng Ả Rập",
	"nl": "Tiếng Hà Lan", "sv": "Tiếng Thuỵ Điển", "da": "Tiếng Đan Mạch", "fi": "Tiếng Phần Lan",
	"no": "Tiếng Na Uy", "nb": "Tiếng Na Uy", "pl": "Tiếng Ba Lan", "tr": "Tiếng Thổ Nhĩ Kỳ",
	"cs": "Tiếng Séc", "el": "Tiếng Hy Lạp", "he": "Tiếng Do Thái", "hu": "Tiếng Hungary",
	"ro": "Tiếng Romania", "sk": "Tiếng Slovak", "uk": "Tiếng Ukraina", "hr": "Tiếng Croatia",
	"ca": "Tiếng Catalan", "bg": "Tiếng Bulgaria", "bn": "Tiếng Bengal", "ta": "Tiếng Tamil",
	"te": "Tiếng Telugu", "mr": "Tiếng Marathi", "sq": "Tiếng Albania", "bs": "Tiếng Bosnia",
	"is": "Tiếng Iceland", "lt": "Tiếng Litva", "lv": "Tiếng Latvia", "et": "Tiếng Estonia",
	"sl": "Tiếng Slovenia", "sr": "Tiếng Serbia", "mk": "Tiếng Macedonia", "fa": "Tiếng Ba Tư",
	"ur": "Tiếng Urdu", "af": "Tiếng Afrikaans", "sw": "Tiếng Swahili",
	"kn": "Tiếng Kannada", "kk": "Tiếng Kazakh", "gu": "Tiếng Gujarat",
	"ml": "Tiếng Malayalam", "pa": "Tiếng Punjab", "or": "Tiếng Odia",
	"si": "Tiếng Sinhala", "km": "Tiếng Khmer", "lo": "Tiếng Lào",
	"my": "Tiếng Miến Điện", "ne": "Tiếng Nepal", "am": "Tiếng Amhara",
	"az": "Tiếng Azerbaijan", "hy": "Tiếng Armenia", "ka": "Tiếng Gruzia",
	"be": "Tiếng Belarus", "cy": "Tiếng Wales", "ga": "Tiếng Ireland",
	"gl": "Tiếng Galicia", "eu": "Tiếng Basque", "mt": "Tiếng Malta",
	"tl": "Tiếng Tagalog", "fil": "Tiếng Philippines", "mn": "Tiếng Mông Cổ",
	"uz": "Tiếng Uzbek", "bh": "Tiếng Bhojpuri", "as": "Tiếng Assam",
}

// langNames — nơi cần phân biệt vùng vì giọng khác hẳn nhau.
var langNames = map[string]string{
	"en_US": "Tiếng Anh (Mỹ)", "en_GB": "Tiếng Anh (Anh)", "en_AU": "Tiếng Anh (Úc)",
	"en_IN": "Tiếng Anh (Ấn Độ)", "en_IE": "Tiếng Anh (Ireland)", "en_ZA": "Tiếng Anh (Nam Phi)",
	"en_SC": "Tiếng Anh (Scotland)",
	"zh_CN": "Tiếng Trung (giản thể)", "zh_TW": "Tiếng Trung (Đài Loan)", "zh_HK": "Tiếng Quảng Đông",
	"pt_BR": "Tiếng Bồ Đào Nha (Brazil)", "pt_PT": "Tiếng Bồ Đào Nha",
	"es_ES": "Tiếng Tây Ban Nha", "es_MX": "Tiếng Tây Ban Nha (Mexico)", "es_AR": "Tiếng Tây Ban Nha (Argentina)",
	"fr_FR": "Tiếng Pháp", "fr_CA": "Tiếng Pháp (Canada)",
	"nl_NL": "Tiếng Hà Lan", "nl_BE": "Tiếng Hà Lan (Bỉ)",
}

// LangGroup — một nhóm ngôn ngữ kèm số giọng.
type LangGroup struct {
	Base   string  `json:"base"`  // mã gốc: vi, en, ja…
	Name   string  `json:"name"`  // tên tiếng Việt
	Count  int     `json:"count"` // số giọng
	Voices []Voice `json:"voices"`
}

// GroupByLang gom giọng theo ngôn ngữ, tiếng Việt luôn đứng đầu.
func GroupByLang(voices []Voice) []LangGroup {
	idx := map[string]*LangGroup{}
	var order []string
	for _, v := range voices {
		base := LangBase(v.Lang)
		if base == "" {
			base = "khac"
		}
		g, ok := idx[base]
		if !ok {
			name := baseNames[base]
			if name == "" {
				name = LangName(v.Lang)
			}
			if name == "" {
				name = "Khác"
			}
			g = &LangGroup{Base: base, Name: name}
			idx[base] = g
			order = append(order, base)
		}
		g.Voices = append(g.Voices, v)
		g.Count++
	}
	out := make([]LangGroup, 0, len(order))
	// tiếng Việt trước, phần còn lại giữ nguyên thứ tự gặp
	if g, ok := idx["vi"]; ok {
		out = append(out, *g)
	}
	for _, b := range order {
		if b == "vi" {
			continue
		}
		out = append(out, *idx[b])
	}
	return out
}
