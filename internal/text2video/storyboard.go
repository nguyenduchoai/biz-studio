package text2video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/stockmedia"
	"bizstudio/internal/store"
	"bizstudio/internal/stylekit"
)

const (
	// noTextRule — mọi ảnh cảnh đều KHÔNG được có chữ (chữ do template lo).
	noTextRule = "no text, no letters, no words, no captions, no watermark, no logo"
	// maxPromptRunes — mô tả cảnh chỉ cần ngắn gọn, dài quá làm model đi lạc.
	maxPromptRunes = 240
	// noSourceHint — chưa có nguồn ảnh nào để sinh storyboard.
	noSourceHint = "chưa có nguồn ảnh nào: cần Gemini API key (sinh ảnh AI) hoặc Pexels API key (ảnh stock) " +
		"trong trang \"Cấu hình & API\" — hoặc tự tải ảnh lên cho từng cảnh"
)

// ShotFile — tên file ảnh của đoạn thứ idx (0-based) trong thư mục phiên.
func ShotFile(idx int) string { return fmt.Sprintf("shot-%d.png", idx) }

// BuildStoryboard sinh ảnh cho MỌI đoạn chưa có ảnh: thiếu mô tả cảnh thì tạo
// mô tả trước, rồi sinh ảnh lưu vào shot-<i>.png trong thư mục phiên.
//
// Đoạn đã có ảnh (nhất là ảnh người dùng tự tải lên) được BỎ QUA — chạy lại
// storyboard không bao giờ ghi đè ảnh cũ. Kết quả cập nhật thẳng vào sess
// (ImagePath/ImagePrompt/ImageSource); hàm KHÔNG tự lưu store.
func BuildStoryboard(ctx context.Context, st *store.Store, sess *store.T2VSession, workDir string, upd func(float64, string)) error {
	if upd == nil {
		upd = func(float64, string) {}
	}
	if sess == nil {
		return errors.New("thiếu phiên Text → Video")
	}
	if len(sess.Segments) == 0 {
		return errors.New("chưa có kịch bản — hãy viết kịch bản trước khi tạo storyboard")
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục phiên %s: %w", workDir, err)
	}

	todo := pendingShots(sess, workDir)
	if len(todo) == 0 {
		upd(99, "Mọi cảnh đã có ảnh — không cần tạo lại")
		return nil
	}
	if err := CheckImageSource(st); err != nil {
		return err
	}

	upd(4, fmt.Sprintf("Chuẩn bị mô tả cảnh cho %d cảnh…", len(todo)))
	fillPrompts(ctx, st, sess, todo)

	w, h := sessionSize(sess)
	done := 0
	var fails []string
	for k, i := range todo {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("tạo storyboard bị hủy ở cảnh %d/%d: %w", k+1, len(todo), err)
		}
		upd(6+float64(k)/float64(len(todo))*90, fmt.Sprintf("Tạo ảnh cảnh %d/%d", k+1, len(todo)))

		name := ShotFile(i)
		src, err := generateShot(ctx, st, sess.Segments[i].ImagePrompt, sess.Segments[i].CharacterIDs,
			w, h, filepath.Join(workDir, name))
		if err != nil {
			fails = append(fails, fmt.Sprintf("cảnh %d: %v", i+1, err))
			st.AddLog("warn", "text2video", fmt.Sprintf("Tạo ảnh cảnh %d thất bại: %v", i+1, err))
			continue
		}
		sess.Segments[i].ImagePath = dataRelPath(workDir, sess.ID, name)
		sess.Segments[i].ImageSource = src
		done++
	}

	if done == 0 {
		return fmt.Errorf("không tạo được ảnh nào cho %d cảnh — %s", len(todo), strings.Join(fails, " | "))
	}
	if len(fails) > 0 {
		upd(99, fmt.Sprintf("Xong %d/%d cảnh — %d cảnh lỗi, thử tạo lại hoặc tải ảnh lên", done, len(todo), len(fails)))
		return nil
	}
	upd(99, fmt.Sprintf("Đã tạo ảnh cho %d cảnh", done))
	return nil
}

// BuildSegmentImage sinh lại ảnh cho ĐÚNG một đoạn (ghi đè shot-<idx>.png).
// prompt rỗng → dùng ImagePrompt hiện có; vẫn rỗng → tự sinh mô tả cảnh.
func BuildSegmentImage(ctx context.Context, st *store.Store, sess *store.T2VSession, idx int, prompt, workDir string) error {
	if sess == nil {
		return errors.New("thiếu phiên Text → Video")
	}
	if idx < 0 || idx >= len(sess.Segments) {
		return fmt.Errorf("không có đoạn số %d — phiên đang có %d đoạn", idx+1, len(sess.Segments))
	}
	if err := CheckImageSource(st); err != nil {
		return err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục phiên %s: %w", workDir, err)
	}

	p := cleanPrompt(prompt)
	if p == "" {
		p = cleanPrompt(sess.Segments[idx].ImagePrompt)
	}
	if p == "" {
		p = suggestOne(ctx, st, sess, idx)
	}

	w, h := sessionSize(sess)
	name := ShotFile(idx)
	src, err := generateShot(ctx, st, p, sess.Segments[idx].CharacterIDs, w, h, filepath.Join(workDir, name))
	if err != nil {
		return fmt.Errorf("tạo lại ảnh cảnh %d thất bại: %w", idx+1, err)
	}
	sess.Segments[idx].ImagePrompt = p
	sess.Segments[idx].ImagePath = dataRelPath(workDir, sess.ID, name)
	sess.Segments[idx].ImageSource = src
	return nil
}

// ImportSegmentImage nhận ảnh người dùng tự tải lên cho một đoạn: chuyển sang
// PNG (nếu cần) và lưu thành shot-<idx>.png, đánh dấu ImageSource="upload".
func ImportSegmentImage(ctx context.Context, sess *store.T2VSession, idx int, srcPath, workDir string) error {
	if sess == nil {
		return errors.New("thiếu phiên Text → Video")
	}
	if idx < 0 || idx >= len(sess.Segments) {
		return fmt.Errorf("không có đoạn số %d — phiên đang có %d đoạn", idx+1, len(sess.Segments))
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục phiên %s: %w", workDir, err)
	}
	name := ShotFile(idx)
	dst := filepath.Join(workDir, name)
	tmp := dst + ".upload.png"

	if strings.ToLower(filepath.Ext(srcPath)) == ".png" {
		if err := copyFile(srcPath, tmp); err != nil {
			return fmt.Errorf("không lưu được ảnh tải lên: %w", err)
		}
	} else if err := runFFmpeg(ctx, "-y", "-i", srcPath, "-frames:v", "1", tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("không đọc được ảnh tải lên (chỉ nhận file ảnh: png, jpg, webp, heic…): %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("không ghi được ảnh cảnh %d: %w", idx+1, err)
	}
	sess.Segments[idx].ImagePath = dataRelPath(workDir, sess.ID, name)
	sess.Segments[idx].ImageSource = "upload"
	return nil
}

// CheckImageSource báo lỗi tiếng Việt khi chưa có nguồn ảnh nào (Gemini/Pexels).
func CheckImageSource(st *store.Store) error {
	set := st.Settings()
	if strings.TrimSpace(set.GeminiAPIKey) == "" && strings.TrimSpace(set.PexelsKey) == "" {
		return errors.New(noSourceHint)
	}
	return nil
}

// ---------- sinh ảnh ----------

// generateShot sinh ảnh một cảnh theo chuỗi nguồn: Gemini (prompt qua Style Kit)
// → Pexels (từ khóa rút từ prompt). Ghi ra file tạm rồi mới thay file đích để
// ảnh cũ không bị mất khi sinh lỗi. Trả nguồn ảnh: "ai" | "stock".
func generateShot(ctx context.Context, st *store.Store, prompt string, charIDs []string, w, h int, dst string) (string, error) {
	scene := cleanPrompt(prompt)
	if scene == "" {
		return "", errors.New("chưa có mô tả cảnh để sinh ảnh")
	}
	tmp := dst + ".part.png"
	defer os.Remove(tmp)

	set := st.Settings()
	var fails []string
	if strings.TrimSpace(set.GeminiAPIKey) != "" {
		err := gemini.NewFromSettings(st).GenerateImage(ctx, shotPrompt(st, scene, charIDs, w, h), tmp)
		if err == nil {
			return "ai", commitShot(tmp, dst)
		}
		fails = append(fails, "Gemini: "+err.Error())
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(set.PexelsKey) != "" {
		// Ảnh kho tìm theo từ khóa CẢNH, không kèm mô tả nhân vật: kho ảnh có
		// sẵn không thể khớp ngoại hình đã đặt, thêm vào chỉ làm lệch kết quả.
		err := stockmedia.SearchImage(ctx, st, stockKeyword(scene), w, h, tmp)
		if err == nil {
			return "stock", commitShot(tmp, dst)
		}
		fails = append(fails, "Pexels: "+err.Error())
	}
	if len(fails) == 0 {
		return "", errors.New(noSourceHint)
	}
	return "", errors.New(strings.Join(fails, " | "))
}

// commitShot thay file ảnh đích bằng file vừa sinh xong.
func commitShot(tmp, dst string) error {
	if fi, err := os.Stat(tmp); err != nil || fi.Size() == 0 {
		return fmt.Errorf("file ảnh sinh ra rỗng: %s", filepath.Base(dst))
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("không ghi được file ảnh %s: %w", filepath.Base(dst), err)
	}
	return nil
}

// pendingShots liệt kê các đoạn cần sinh ảnh (chưa có ImagePath, hoặc file ảnh
// đã bị xoá khỏi đĩa).
func pendingShots(sess *store.T2VSession, workDir string) []int {
	dataDir := dataDirOf(workDir, sess.ID)
	out := make([]int, 0, len(sess.Segments))
	for i := range sess.Segments {
		p := strings.TrimSpace(sess.Segments[i].ImagePath)
		if p != "" && fileExists(absFromData(dataDir, p)) {
			continue
		}
		sess.Segments[i].ImagePath = ""
		out = append(out, i)
	}
	return out
}

// sessionSize trả kích thước khung hình của phiên (mặc định dọc 1080×1920).
func sessionSize(sess *store.T2VSession) (int, int) {
	if sess.Width > 0 && sess.Height > 0 {
		return sess.Width, sess.Height
	}
	return 1080, 1920
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// ---------- nhân vật nhất quán ----------

// characterLead — mở đầu mệnh đề nhân vật, đặt NGAY SAU mô tả cảnh.
const characterLead = "Featuring "

// CharacterClause dựng mệnh đề mô tả ngoại hình các nhân vật của một cảnh:
// "Featuring <Tên>: <Look>. <Tên2>: <Look2>". Nhờ mệnh đề này mà nhân vật ở
// mọi cảnh trông giống nhau xuyên suốt video.
//
// Nhân vật không còn trong store (đã xoá) được BỎ QUA im lặng — thiếu một nhân
// vật không đáng làm hỏng cả việc sinh ảnh. Không có nhân vật dùng được → "".
func CharacterClause(st *store.Store, ids []string) string {
	if st == nil || len(ids) == 0 {
		return ""
	}
	seen := make(map[string]bool, len(ids))
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		c, ok := st.Character(id)
		if !ok {
			continue
		}
		name, look := cleanPrompt(c.Name), cleanPrompt(c.Look)
		switch {
		case name != "" && look != "":
			parts = append(parts, name+": "+look)
		case look != "":
			parts = append(parts, look)
		case name != "":
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return characterLead + strings.Join(parts, ". ")
}

// shotPrompt dựng prompt cuối cùng gửi model sinh ảnh, THỨ TỰ BẮT BUỘC:
// mô tả cảnh → mô tả nhân vật → khung hình + luật không chữ → Style Kit (lớp
// ngoài cùng). Đặt nhân vật ngay sau cảnh để ngoại hình không bị style ghi đè.
func shotPrompt(st *store.Store, scene string, charIDs []string, w, h int) string {
	return stylekit.Apply(st, imagePrompt(withCharacters(st, scene, charIDs), w, h))
}

// withCharacters ghép mô tả cảnh với mệnh đề nhân vật: "<cảnh>. Featuring …".
func withCharacters(st *store.Store, scene string, charIDs []string) string {
	clause := CharacterClause(st, charIDs)
	switch {
	case clause == "":
		return scene
	case strings.TrimSpace(scene) == "":
		return clause
	default:
		return strings.TrimRight(strings.TrimSpace(scene), ".") + ". " + clause
	}
}

// ---------- mô tả cảnh ----------

const shotSystem = "Bạn là đạo diễn hình ảnh. Luôn trả về đúng JSON mảng chuỗi được yêu cầu, không thêm nội dung nào khác."

// SuggestPrompt sinh mô tả cảnh mặc định cho một đoạn kịch bản bằng suy luận từ
// chính nội dung đoạn (không gọi mạng) — dùng làm phương án dự phòng khi không
// hỏi được LLM. Kết quả là câu tiếng Anh ngắn: chủ thể + bối cảnh + cảm xúc +
// ánh sáng, và luôn cấm chữ trong ảnh.
func SuggestPrompt(seg store.T2VSegment) string {
	scene := guessScene(seg.Text)
	if len(seg.CharacterIDs) > 0 {
		// Cảnh có nhân vật thì phải thấy người; ngoại hình KHÔNG tả ở đây —
		// phần đó do hồ sơ nhân vật lo (CharacterClause).
		scene = withPersonInFrame(scene)
	}
	return cleanPrompt(scene + ". " + noTextRule)
}

// personWords — dấu hiệu cảnh đã có người, khỏi thêm gợi ý.
var personWords = []string{"person", "people", "someone", "man", "woman", "team", "family", "student", "traveler"}

// withPersonInFrame bảo đảm cảnh có người xuất hiện (không tả ngoại hình).
func withPersonInFrame(scene string) string {
	low := strings.ToLower(scene)
	for _, w := range personWords {
		if strings.Contains(low, w) {
			return scene
		}
	}
	return scene + ", with the character clearly visible in frame"
}

// fillPrompts điền ImagePrompt cho các đoạn còn trống: hỏi LLM MỘT lần cho cả
// danh sách, đoạn nào LLM không trả thì dùng SuggestPrompt.
func fillPrompts(ctx context.Context, st *store.Store, sess *store.T2VSession, idxs []int) {
	need := make([]int, 0, len(idxs))
	asks := make([]shotAsk, 0, len(idxs))
	for _, i := range idxs {
		if strings.TrimSpace(sess.Segments[i].ImagePrompt) == "" {
			need = append(need, i)
			asks = append(asks, askOf(sess.Segments[i]))
		}
	}
	if len(need) == 0 {
		return
	}
	got := suggestPromptsLLM(ctx, st, sess.ScriptEngine, sess.ScriptModel, asks)
	for k, i := range need {
		p := ""
		if k < len(got) {
			p = got[k]
		}
		if p == "" {
			p = SuggestPrompt(sess.Segments[i])
		}
		sess.Segments[i].ImagePrompt = p
	}
}

// suggestOne sinh mô tả cảnh cho đúng một đoạn (LLM → heuristic).
func suggestOne(ctx context.Context, st *store.Store, sess *store.T2VSession, idx int) string {
	got := suggestPromptsLLM(ctx, st, sess.ScriptEngine, sess.ScriptModel, []shotAsk{askOf(sess.Segments[idx])})
	if len(got) > 0 && got[0] != "" {
		return got[0]
	}
	return SuggestPrompt(sess.Segments[idx])
}

// shotAsk — một đoạn cần mô tả cảnh; withChars = đoạn đã gán nhân vật.
type shotAsk struct {
	text      string
	withChars bool
}

// askOf dựng yêu cầu mô tả cảnh từ một đoạn kịch bản.
func askOf(seg store.T2VSegment) shotAsk {
	return shotAsk{text: seg.Text, withChars: len(seg.CharacterIDs) > 0}
}

// suggestPromptsLLM nhờ LLM mô tả cảnh bằng tiếng Anh cho từng đoạn — MỘT lần
// gọi cho cả danh sách. Lỗi hoặc thiếu phần tử → chuỗi rỗng (caller tự dự phòng).
func suggestPromptsLLM(ctx context.Context, st *store.Store, engine, model string, asks []shotAsk) []string {
	out := make([]string, len(asks))
	if len(asks) == 0 {
		return out
	}
	raw, err := runScriptLLM(ctx, st, engine, model, buildShotPrompt(asks))
	if err != nil {
		st.AddLog("warn", "text2video",
			fmt.Sprintf("Không nhờ được AI mô tả cảnh: %v — dùng mô tả tự suy từ nội dung", err))
		return out
	}
	var arr []string
	if body := extractJSONArray(stripFence(raw)); body != "" {
		_ = json.Unmarshal([]byte(body), &arr)
	}
	for i := range out {
		if i < len(arr) {
			out[i] = cleanPrompt(arr[i])
		}
	}
	return out
}

// charMark — nhãn đánh dấu đoạn có nhân vật trong prompt gửi LLM.
const charMark = "[CÓ NHÂN VẬT] "

// buildShotPrompt dựng prompt yêu cầu LLM mô tả cảnh cho từng đoạn lời đọc.
func buildShotPrompt(asks []shotAsk) string {
	var b strings.Builder
	b.WriteString(`Với mỗi đoạn lời đọc tiếng Việt bên dưới, hãy viết MỘT câu mô tả CẢNH QUAY minh họa cho đoạn đó, bằng TIẾNG ANH.

Quy tắc bắt buộc:
- Mô tả những gì NHÌN THẤY: chủ thể, bối cảnh, hành động, cảm xúc, ánh sáng. Không dịch lại lời đọc.
- Ngắn gọn 12 đến 25 từ, viết liền một câu, không markdown, không đánh số.
- TUYỆT ĐỐI không có chữ, số, biển hiệu, logo hay watermark trong ảnh.
- Không mô tả phong cách vẽ hay chất liệu ảnh (phong cách do bộ style riêng lo).
- Các cảnh phải liên quan tới nhau như trong cùng một video.
`)
	if anyWithChars(asks) {
		b.WriteString(`- Đoạn có nhãn ` + strings.TrimSpace(charMark) + `: cảnh PHẢI có nhân vật đó xuất hiện (đang làm gì, ở đâu, cảm xúc ra sao), nhưng TUYỆT ĐỐI không tả ngoại hình (tuổi, giới tính, tóc, khuôn mặt, trang phục, màu da) — ngoại hình đã có hồ sơ nhân vật riêng lo.
`)
	}
	b.WriteString(`
Trả về DUY NHẤT một JSON mảng chuỗi, đúng thứ tự và ĐÚNG SỐ LƯỢNG đoạn đầu vào.

Các đoạn lời đọc:
`)
	for i, a := range asks {
		mark := ""
		if a.withChars {
			mark = charMark
		}
		fmt.Fprintf(&b, "%d. %s%s\n", i+1, mark, shortText(strings.TrimSpace(a.text), 400))
	}
	return b.String()
}

// anyWithChars — trong danh sách có đoạn nào gán nhân vật không.
func anyWithChars(asks []shotAsk) bool {
	for _, a := range asks {
		if a.withChars {
			return true
		}
	}
	return false
}

// imagePrompt hoàn thiện prompt trước khi gửi model sinh ảnh: thêm khung hình
// theo tỉ lệ video và luôn bảo đảm luật "không chữ trong ảnh".
func imagePrompt(prompt string, w, h int) string {
	parts := []string{prompt, framingHint(w, h)}
	if !strings.Contains(strings.ToLower(prompt), "no text") {
		parts = append(parts, noTextRule)
	}
	return strings.Join(parts, ", ")
}

// framingHint mô tả khung hình theo tỉ lệ video (dọc / ngang / vuông).
func framingHint(w, h int) string {
	switch {
	case h > w:
		return "vertical 9:16 portrait framing, subject centered with headroom"
	case w > h:
		return "horizontal 16:9 cinematic framing"
	default:
		return "square 1:1 framing"
	}
}

// stockKeyword rút từ khóa tìm ảnh stock từ mô tả cảnh: lấy mệnh đề đầu, bỏ các
// từ nối, giữ tối đa 6 từ (Pexels tìm theo từ khóa, câu dài ra kết quả rỗng).
func stockKeyword(prompt string) string {
	head := prompt
	if i := strings.IndexAny(head, ",."); i > 0 {
		head = head[:i]
	}
	skip := map[string]bool{"a": true, "an": true, "the": true, "of": true, "with": true, "in": true, "on": true, "at": true, "and": true}
	words := make([]string, 0, 6)
	for _, w := range strings.Fields(strings.ToLower(head)) {
		w = strings.Trim(w, "\"'()[]{}")
		if w == "" || skip[w] {
			continue
		}
		words = append(words, w)
		if len(words) >= 6 {
			break
		}
	}
	if len(words) == 0 {
		return strings.TrimSpace(shortText(prompt, 60))
	}
	return strings.Join(words, " ")
}

// cleanPrompt làm sạch mô tả cảnh: bỏ ký hiệu markdown, gộp khoảng trắng, cắt độ dài.
func cleanPrompt(v string) string {
	t := strings.TrimSpace(v)
	t = reBulletPrefix.ReplaceAllString(t, "")
	t = reMarkdownChar.ReplaceAllString(t, "")
	t = strings.Trim(t, "\"' ")
	t = reSpaceRun.ReplaceAllString(t, " ")
	t = strings.TrimSpace(t)
	if r := []rune(t); len(r) > maxPromptRunes {
		t = strings.TrimSpace(string(r[:maxPromptRunes]))
	}
	return t
}

// sceneHints — đoán bối cảnh từ từ khóa tiếng Việt trong lời đọc.
var sceneHints = []struct {
	keys  []string
	scene string
}{
	{[]string{"công nghệ", "trí tuệ nhân tạo", "phần mềm", "ứng dụng", "máy tính", "điện thoại", "dữ liệu", "internet"},
		"a focused person working on a laptop in a modern minimal workspace, calm concentration, soft daylight from a window"},
	{[]string{"kinh doanh", "doanh nghiệp", "công ty", "bán hàng", "khách hàng", "thị trường", "doanh thu", "thương hiệu"},
		"a small team discussing around a table in a bright modern office, confident and friendly mood, natural window light"},
	{[]string{"tiền", "tài chính", "đầu tư", "ngân hàng", "chi phí", "lợi nhuận", "tiết kiệm"},
		"close-up of hands writing in a notebook beside a calculator on a wooden desk, thoughtful mood, warm morning light"},
	{[]string{"sức khỏe", "bệnh", "bác sĩ", "tập luyện", "thể dục", "dinh dưỡng", "giấc ngủ"},
		"a person exercising outdoors in a green park, healthy and energetic mood, golden morning light"},
	{[]string{"học", "giáo dục", "sinh viên", "học sinh", "trường", "khóa học", "kiến thức", "sách"},
		"a young student reading with books and a notebook beside a window, curious mood, soft daylight"},
	{[]string{"du lịch", "biển", "núi", "chuyến đi", "khám phá", "hành trình"},
		"a traveler standing before a wide open landscape at sunrise, hopeful mood, warm golden light"},
	{[]string{"gia đình", "trẻ em", "con cái", "cha mẹ", "ngôi nhà", "tổ ấm"},
		"a warm family moment together in a cozy living room, gentle happy mood, soft afternoon light"},
	{[]string{"ăn", "món", "nấu", "quán", "cà phê", "ẩm thực", "thực phẩm"},
		"close-up of a freshly prepared dish on a rustic table, appetizing mood, warm side light"},
	{[]string{"môi trường", "thiên nhiên", "cây", "khí hậu", "xanh"},
		"a lush green forest with sunlight streaming through the leaves, peaceful mood"},
	{[]string{"xe", "giao thông", "đường phố", "thành phố"},
		"a busy city street with light traffic at dusk, dynamic mood, glowing city lights"},
	{[]string{"làm việc", "công việc", "nghề", "nhân viên", "văn phòng"},
		"a person at a tidy desk in a quiet office, determined mood, soft natural light"},
}

// defaultScene — cảnh mặc định khi không đoán được chủ đề.
const defaultScene = "a candid photo of a person in a real everyday setting, thoughtful mood, soft natural light, shallow depth of field"

// guessScene đoán mô tả cảnh tiếng Anh từ nội dung tiếng Việt của đoạn.
func guessScene(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return defaultScene
	}
	words := make(map[string]bool)
	for _, w := range strings.FieldsFunc(low, func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9') && r < 0x80
	}) {
		words[w] = true
	}
	for _, h := range sceneHints {
		for _, k := range h.keys {
			if strings.Contains(k, " ") {
				if strings.Contains(low, k) {
					return h.scene
				}
				continue
			}
			if words[k] || strings.Contains(low, " "+k+" ") {
				return h.scene
			}
		}
	}
	return defaultScene
}
