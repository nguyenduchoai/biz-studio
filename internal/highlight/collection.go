package highlight

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"bizstudio/internal/store"
)

const (
	// clusterPoolMax — số đoạn điểm cao nhất đưa cho AI gom nhóm.
	//
	// Gom nhóm cần AI ĐỌC HẾT rồi mới thấy các đoạn nào cùng một chủ đề, nên
	// không chia lô được như khâu chấm điểm. Phải chặn ở đầu vào. Lấy đoạn điểm
	// cao nhất chứ không lấy ngẫu nhiên: hợp tuyển dựng từ đoạn hạng hai thì
	// chẳng ai xem.
	clusterPoolMax = 60

	// minClipsPerCollection — dưới mức này thì không gọi là hợp tuyển, chỉ là
	// một clip lẻ bị gắn cho cái tên kêu.
	minClipsPerCollection = 3
)

// Collection — một hợp tuyển: nhóm đoạn cùng chủ đề, dựng thành một video.
type Collection struct {
	Title  string      `json:"title"`
	Why    string      `json:"why"`
	Clips  []Candidate `json:"clips"`
	Sec    float64     `json:"sec"`
	Output string      `json:"output,omitempty"`
}

// Cluster nhờ AI gom các đoạn điểm cao thành vài hợp tuyển theo chủ đề.
//
// Khác với rút clip: rút clip lấy các đoạn đắt nhất bất kể nói về cái gì, ghép
// thành MỘT video. Gom hợp tuyển đọc xem chúng nói về cái gì rồi tách thành
// NHIỀU video, mỗi video một chủ đề — từ một buổi phỏng vấn hai tiếng ra "chuyện
// khởi nghiệp", "sai lầm tuyển người", "chuyện gia đình".
//
// Trong mỗi hợp tuyển, các đoạn xếp lại THEO THỜI GIAN gốc, cùng lý do với Pick:
// ghép theo thứ tự điểm thì câu chuyện nhảy cóc, người xem không lần ra mạch.
func Cluster(ctx context.Context, st *store.Store, cs []Candidate, minScore float64,
	maxCollections, secondsEach int, genreID string) ([]Collection, error) {

	pool := topByScore(cs, minScore, clusterPoolMax)
	if len(pool) < minClipsPerCollection {
		return nil, fmt.Errorf("chỉ có %d đoạn đạt điểm %.0f trở lên — không đủ để gom hợp tuyển, hạ ngưỡng điểm xuống",
			len(pool), minScore)
	}
	if maxCollections <= 0 {
		maxCollections = 4
	}

	raw, err := runLLMFn(ctx, st, buildClusterPrompt(pool, maxCollections, secondsEach, FindGenre(genreID)))
	if err != nil {
		return nil, err
	}
	groups := parseClusters(raw)
	if len(groups) == 0 {
		return nil, fmt.Errorf("AI không gom được nhóm nào — thử lại hoặc đổi engine (nhận được: %s)",
			shortText(raw, 200))
	}

	byIndex := map[int]Candidate{}
	for _, c := range pool {
		byIndex[c.Index] = c
	}

	var out []Collection
	used := map[int]bool{}
	for _, g := range groups {
		col := Collection{Title: strings.TrimSpace(g.Title), Why: strings.TrimSpace(g.Why)}
		for _, i := range g.Indexes {
			c, ok := byIndex[i]
			if !ok || used[i] {
				continue // AI bịa số thứ tự, hoặc xếp một đoạn vào hai nhóm
			}
			if secondsEach > 0 && col.Sec+c.Dur() > float64(secondsEach) {
				continue
			}
			used[i] = true
			col.Clips = append(col.Clips, c)
			col.Sec += c.Dur()
		}
		if len(col.Clips) < minClipsPerCollection || col.Title == "" {
			// Trả các đoạn về để nhóm sau còn dùng được.
			for _, c := range col.Clips {
				delete(used, c.Index)
			}
			continue
		}
		sort.Slice(col.Clips, func(i, j int) bool { return col.Clips[i].Start < col.Clips[j].Start })
		out = append(out, col)
		if len(out) >= maxCollections {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("không nhóm nào đủ %d đoạn để thành hợp tuyển — video có thể chỉ xoay quanh một chủ đề, "+
			"dùng \"Rút clip ngắn\" hợp hơn", minClipsPerCollection)
	}
	return out, nil
}

// topByScore lấy tối đa n đoạn điểm cao nhất từ mức minScore trở lên.
func topByScore(cs []Candidate, minScore float64, n int) []Candidate {
	var ok []Candidate
	for _, c := range cs {
		if c.Score >= minScore {
			ok = append(ok, c)
		}
	}
	sort.SliceStable(ok, func(i, j int) bool {
		if ok[i].Score != ok[j].Score {
			return ok[i].Score > ok[j].Score
		}
		return ok[i].Start < ok[j].Start
	})
	if len(ok) > n {
		ok = ok[:n]
	}
	return ok
}

func buildClusterPrompt(cs []Candidate, maxCollections, secondsEach int, genre Genre) string {
	var b strings.Builder
	fmt.Fprintf(&b, `Dưới đây là những đoạn hay nhất rút ra từ một video dài thể loại %s, kèm số thứ tự và mốc thời gian.
Nhiệm vụ: gom chúng thành TỐI ĐA %d hợp tuyển theo CHỦ ĐỀ, mỗi hợp tuyển sẽ được dựng thành một video riêng dài khoảng %d giây.

Nguyên tắc:
- Gom theo NỘI DUNG nói về cái gì, không gom theo thời gian gần nhau.
- Mỗi hợp tuyển ít nhất %d đoạn. Không đủ thì bỏ hẳn nhóm đó, đừng gom cho có.
- Một đoạn chỉ thuộc MỘT hợp tuyển.
- Đoạn nào không hợp chủ đề nào thì bỏ ra, không cần dùng hết.
- Tiêu đề đặt như tiêu đề video ngắn: cụ thể, gợi tò mò, dưới 12 từ, KHÔNG bịa nội dung không có trong các đoạn.

Các đoạn:
`, genre.Name, maxCollections, secondsEach, minClipsPerCollection)
	for _, c := range cs {
		line := oneLine(c.Text)
		if c.Why != "" {
			line += "  (điểm " + fmt.Sprintf("%.0f", c.Score) + ": " + c.Why + ")"
		}
		fmt.Fprintf(&b, "[%d] %.0fs-%.0fs: %s\n", c.Index, c.Start, c.End, line)
	}
	b.WriteString(`
Trả về DUY NHẤT một JSON mảng:
[{"tieu_de":"Tên hợp tuyển","vi":"vì sao gom nhóm này, dưới 20 từ","doan":[3,17,42]}]
Không giải thích, không markdown, không văn bản nào khác ngoài JSON.`)
	return b.String()
}

type clusterGroup struct {
	Title   string
	Why     string
	Indexes []int
}

// parseClusters đọc kết quả gom nhóm. Chịu được khoá tiếng Anh và số thứ tự trả
// về dạng chuỗi — cùng lý do với parseScores.
func parseClusters(raw string) []clusterGroup {
	s := strings.TrimSpace(raw)
	if m := reFence.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	if i := strings.Index(s, "["); i >= 0 {
		if j := strings.LastIndex(s, "]"); j > i {
			s = s[i : j+1]
		}
	}
	var rows []map[string]any
	if json.Unmarshal([]byte(s), &rows) != nil {
		return nil
	}
	var out []clusterGroup
	for _, r := range rows {
		g := clusterGroup{
			Title: pickStr(r, "tieu_de", "title", "ten", "name"),
			Why:   pickStr(r, "vi", "why", "ly_do", "reason", "mo_ta"),
		}
		for _, key := range []string{"doan", "indexes", "clips", "ids", "items"} {
			arr, ok := r[key].([]any)
			if !ok {
				continue
			}
			for _, v := range arr {
				if f, ok := pickFloat(map[string]any{"x": v}, "x"); ok {
					g.Indexes = append(g.Indexes, int(f))
				}
			}
			break
		}
		if g.Title != "" && len(g.Indexes) > 0 {
			out = append(out, g)
		}
	}
	return out
}
