package store

import "time"

// Project — dự án video (trang điều phối AI edit) hoặc dự án vox/article.
type Project struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"` // video | vox | article
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	FPS        int       `json:"fps"`
	Status     string    `json:"status"`   // draft | running | done | error
	Progress   int       `json:"progress"` // 0..6: Phân tích, Dựng scene, Render draft, Lắp draft, Render final, Hoàn thành
	Tags       []string  `json:"tags"`
	BriefDesc  string    `json:"briefDesc"`  // Mô tả video gốc
	EditPrompt string    `json:"editPrompt"` // Yêu cầu edit (prompt)
	AutoCut    bool      `json:"autoCut"`    // Tự động cắt ngắn video
	AutoSub    bool      `json:"autoSub"`    // Tạo phụ đề
	AutoKey    bool      `json:"autoKey"`    // Làm nổi bật key chính
	Keywords   []string  `json:"keywords"`
	OutputFile string    `json:"outputFile"` // tương đối data dir
	ThumbFile  string    `json:"thumbFile"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Asset — nguồn media của dự án.
type Asset struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Kind      string    `json:"kind"` // video | image | audio | other
	Name      string    `json:"name"`
	Path      string    `json:"path"` // tương đối data dir
	Size      int64     `json:"size"`
	Duration  float64   `json:"duration"`
	Desc      string    `json:"desc"`
	Order     int       `json:"order"`
	CreatedAt time.Time `json:"createdAt"`
}

// Session — một phiên AI (Claude CLI) của dự án.
type Session struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"projectId"`
	ClaudeSessionID string    `json:"claudeSessionId"`
	Title           string    `json:"title"`
	Status          string    `json:"status"` // running | done | error | stopped
	AutoContinue    bool      `json:"autoContinue"`
	NumTurns        int       `json:"numTurns"`
	CostUSD         float64   `json:"costUsd"`
	StartedAt       time.Time `json:"startedAt"`
	EndedAt         time.Time `json:"endedAt"`
}

// SessionEvent — một dòng log của phiên AI.
type SessionEvent struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Type      string    `json:"type"` // tool | text | result | error | user | init
	Name      string    `json:"name"` // tên tool nếu type=tool
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"createdAt"`
}

// Job — tác vụ nền (download, render, qc, tts, translate, ocr, asr, vox, autocut, publish, thumbnail).
type Job struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	ProjectID string    `json:"projectId"`
	Status    string    `json:"status"` // queued | running | done | error
	Progress  float64   `json:"progress"`
	Detail    string    `json:"detail"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Idea — ý tưởng video do AI đề xuất, chờ người duyệt rồi mới sản xuất.
type Idea struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Angle    string   `json:"angle"` // góc tiếp cận / tóm tắt nội dung
	Hook     string   `json:"hook"`  // câu mở đầu giữ chân người xem
	Keywords []string `json:"keywords"`
	Status   string   `json:"status"` // proposed | approved | rejected | queued | producing | done | error

	// Cấu hình sản xuất (kế thừa khi tạo phiên Text → Video)
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`

	// Attempts đếm số lần đã thử sản xuất — để biết ý tưởng nào hỏng dai dẳng
	// chứ không phải trục trặc một lần.
	Attempts int `json:"attempts"`

	T2VSessionID string    `json:"t2vSessionId"` // phiên được tạo khi sản xuất
	OutputPath   string    `json:"outputPath"`
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Character — nhân vật xuất hiện lặp lại trong nhiều cảnh; mô tả ngoại hình
// được chèn vào prompt sinh ảnh để nhân vật trông giống nhau xuyên suốt video.
//
// Look là mô tả NGẮN dùng cho mọi prompt cảnh — giữ nguyên vai trò cũ. Persona,
// Voice, Sheet* là hồ sơ đầy đủ, sinh sau và không bắt buộc: nhân vật cũ chỉ có
// Look vẫn chạy y như trước.
type Character struct {
	ID       string `json:"id"`
	Name     string `json:"name"`     // tên gọi ngắn, dùng để gán cho cảnh
	Look     string `json:"look"`     // mô tả ngoại hình (tiếng Anh cho model hiểu)
	Role     string `json:"role"`     // vai trò/tính cách (ghi chú cho người dùng)
	RefImage string `json:"refImage"` // ảnh tham chiếu, tương đối data dir

	// ---- Hồ sơ nhân vật đầy đủ (tuỳ chọn) ----

	Persona   *Persona   `json:"persona,omitempty"`
	VoiceSpec *VoiceSpec `json:"voiceSpec,omitempty"`

	// Prompt cho MÁY — luôn tiếng Anh, không theo ngôn ngữ giao diện: model ảnh
	// và engine giọng ăn tiếng Anh ổn định nhất.
	ImagePrompt string `json:"imagePrompt,omitempty"`
	Negative    string `json:"negative,omitempty"`
	SheetImage  string `json:"sheetImage,omitempty"` // bản vẽ ba góc nhìn, tương đối data dir

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Persona — phần hồ sơ dành cho NGƯỜI đọc, viết bằng ngôn ngữ của người dùng.
type Persona struct {
	Gender      string   `json:"gender"`
	AgeRange    string   `json:"ageRange"`
	Identity    string   `json:"identity"`    // thân phận / nghề nghiệp
	Appearance  string   `json:"appearance"`  // ngoại hình đầy đủ (dài hơn Look)
	Personality []string `json:"personality"` // 3-5 từ
	Temperament string   `json:"temperament"` // khí chất, cách nói năng
	Motivation  string   `json:"motivation"`
	Arc         string   `json:"arc"`      // tuyến phát triển
	Region      string   `json:"region"`   // vùng miền giọng: Bắc | Trung | Nam
	Evidence    []string `json:"evidence"` // trích NGUYÊN VĂN từ nguồn, không dịch
}

// VoiceSpec — thiết kế giọng của nhân vật. Prompt luôn tiếng Anh; VoiceID là
// giọng trong máy đã ghép được, để đọc luôn không phải chọn tay.
type VoiceSpec struct {
	Timbre  string `json:"timbre"`
	Pitch   string `json:"pitch"`
	Pace    string `json:"pace"`
	Accent  string `json:"accent"`
	Emotion string `json:"emotion"`
	Prompt  string `json:"prompt"`         // tiếng Anh, cho engine thiết kế giọng
	VoiceID string `json:"voiceId"`        // giọng đã ghép trong máy
	Note    string `json:"note,omitempty"` // vì sao giọng ghép chưa khớp hẳn
}

// StyleKit — bộ style hình ảnh dùng chung cho mọi cảnh của một video,
// để hình sinh ra đồng nhất như cùng một bộ phim.
type StyleKit struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// --- Sinh ảnh ---
	StylePrompt string   `json:"stylePrompt"` // ghép vào MỌI prompt sinh ảnh
	Negative    string   `json:"negative"`    // thứ cần tránh
	Palette     []string `json:"palette"`     // mã màu hex (gợi ý cho model)
	Theme       string   `json:"theme"`       // vivid | dark | light (nền khung chữ)

	// --- Giao diện video (đổ thẳng vào biến CSS khi render) ---
	BgDeep    string `json:"bgDeep"`    // màu nền chính
	TextMain  string `json:"textMain"`  // màu chữ chính
	Accent    string `json:"accent"`    // màu nhấn
	FontHead  string `json:"fontHead"`  // font tiêu đề & số lớn (CSS font stack)
	FontBody  string `json:"fontBody"`  // font nội dung & phụ đề
	SizeTitle int    `json:"sizeTitle"` // px — cỡ chữ tiêu đề
	SizeBig   int    `json:"sizeBig"`   // px — cỡ số lớn
	SizeBody  int    `json:"sizeBody"`  // px — cỡ chữ nội dung

	// --- Nhận diện kênh ---
	ChannelName string `json:"channelName"`
	LogoPath    string `json:"logoPath"` // tương đối data dir
	LogoPos     string `json:"logoPos"`  // left | center | right

	// --- Tư liệu nền chạy lặp dưới lớp chữ ---
	StockPaths []string `json:"stockPaths"` // tương đối data dir

	// --- Giới hạn độ dài kịch bản AI ---
	MaxVoiceChars int `json:"maxVoiceChars"` // 0 = không giới hạn
	MaxImageChars int `json:"maxImageChars"`

	// --- Nền tảng giao diện ---
	BaseTemplate string `json:"baseTemplate"` // builtin | custom
	CustomHTML   string `json:"customHtml"`   // dùng khi baseTemplate=custom

	IsDefault bool      `json:"isDefault"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// T2VSegment — một đoạn kịch bản đọc; Seconds là thời lượng ĐO THẬT sau khi tổng hợp giọng.
type T2VSegment struct {
	Text      string  `json:"text"`
	Chars     int     `json:"chars"`
	Seconds   float64 `json:"seconds"`
	AudioPath string  `json:"audioPath"` // tương đối data dir

	// Storyboard — hình của cảnh này
	ImagePath   string `json:"imagePath"`   // tương đối data dir
	ImagePrompt string `json:"imagePrompt"` // mô tả cảnh để sinh ảnh (sửa được)
	ImageSource string `json:"imageSource"` // ai | stock | upload

	// Nhân vật xuất hiện trong cảnh này (ID của store.Character)
	CharacterIDs []string `json:"characterIds"`

	// Vai của cảnh trong mạch video: hook (câu mở giữ chân) | body (nội dung
	// chính) | cta (kêu gọi hành động). Rỗng = suy theo vị trí như trước, nhờ
	// vậy các phiên cũ render lại vẫn ra y hệt.
	Role string `json:"role"`
}

// T2VSession — một phiên Text → Video (nguồn → kịch bản → giọng đọc → cấu hình → dựng video).
type T2VSession struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceKind string `json:"sourceKind"` // text | link
	SourceURL  string `json:"sourceUrl"`
	SourceText string `json:"sourceText"`

	ScriptEngine  string       `json:"scriptEngine"` // claude | gemini | openai
	ScriptModel   string       `json:"scriptModel"`
	TargetSeconds int          `json:"targetSeconds"` // 0 = tự động
	Segments      []T2VSegment `json:"segments"`

	VoiceID     string `json:"voiceId"`
	VoiceEngine string `json:"voiceEngine"`
	VoiceStyle  string `json:"voiceStyle"`

	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`

	Status         string  `json:"status"` // draft | script | voice | building | done | error
	Step           int     `json:"step"`   // 0..5 cho stepper
	VoicePath      string  `json:"voicePath"`
	TranscriptPath string  `json:"transcriptPath"`
	OutputPath     string  `json:"outputPath"`
	VoiceSeconds   float64 `json:"voiceSeconds"`
	ProjectID      string  `json:"projectId"`
	BuildMode      string  `json:"buildMode"` // ai | html

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CloneVoice — giọng nhân bản từ clip mẫu (VieNeu instant voice cloning).
type CloneVoice struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"` // tương đối data dir: vieneu/clones/<id>.wav
	Gender    string    `json:"gender"`
	Note      string    `json:"note"`
	Duration  float64   `json:"duration"`
	CreatedAt time.Time `json:"createdAt"`
}

// PromptTemplate — prompt mẫu.
type PromptTemplate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Body string `json:"body"`
}

// LogEntry — nhật ký hệ thống.
type LogEntry struct {
	ID        string    `json:"id"`
	Level     string    `json:"level"` // info | warn | error
	Module    string    `json:"module"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// Settings — cấu hình toàn cục (Cấu hình & API).
type Settings struct {
	GeminiAPIKey string `json:"geminiApiKey"`

	// Veo — sinh video bằng AI. Đây là tính năng TRẢ PHÍ TÍNH THEO GIÂY trên
	// khoá Google của người dùng, nên tách riêng khoá: người dùng có thể để
	// khoá Veo ở một dự án Google đã bật thanh toán, khác khoá Gemini thường.
	// Rỗng = dùng lại GeminiAPIKey.
	VeoAPIKey     string `json:"veoApiKey"`
	VeoModel      string `json:"veoModel"`      // rỗng = veo-3.1-fast-generate-preview
	VeoResolution string `json:"veoResolution"` // 720p | 1080p | 4k
	VeoSeconds    int    `json:"veoSeconds"`    // 4 | 6 | 8

	// LongCat — dựng video người nói từ ảnh + giọng (LongCat-Video-Avatar).
	// Model 13,6 tỉ tham số, BẮT BUỘC GPU NVIDIA — không có bản cho Apple
	// Silicon hay CPU. Nên có hai chế độ: chạy ngay trên máy có GPU (local),
	// hoặc đẩy việc sang máy GPU khác đang chạy scripts/longcat-worker.py
	// (remote) — máy cá nhân làm bàn điều khiển, máy GPU làm xưởng render.
	LongCatMode       string `json:"longcatMode"`       // "" tắt | local | remote
	LongCatPython     string `json:"longcatPython"`     // rỗng = dò venv cạnh mã nguồn
	LongCatRepo       string `json:"longcatRepo"`       // thư mục mã nguồn LongCat-Video
	LongCatCheckpoint string `json:"longcatCheckpoint"` // thư mục trọng số model
	LongCatWorkerURL  string `json:"longcatWorkerUrl"`  // vd http://192.168.1.50:7070
	LongCatGPUs       int    `json:"longcatGpus"`       // số GPU, 0 = 1
	LongCatInt8       bool   `json:"longcatInt8"`       // nén INT8 cho đỡ VRAM

	GeminiBase  string `json:"geminiBase"`
	GeminiModel string `json:"geminiModel"`

	// Model sinh ảnh và đọc giọng KHÔNG dùng chung với model văn bản: model
	// văn bản không sinh được ảnh, model ảnh không đọc được giọng. Trước đây
	// hai thứ này đóng cứng trong mã nguồn nên người dùng không đổi được, mãi
	// kẹt ở đời model cũ dù API đã có bản mới.
	GeminiImageModel string `json:"geminiImageModel"` // rỗng = gemini-2.5-flash-image
	GeminiTTSModel   string `json:"geminiTtsModel"`   // rỗng = gemini-2.5-flash-preview-tts

	ClaudeBin            string `json:"claudeBin"`
	ClaudeModel          string `json:"claudeModel"`
	YtdlpBin             string `json:"ytdlpBin"`
	DownloadDir          string `json:"downloadDir"`
	CookiesFile          string `json:"cookiesFile"`
	Quality              string `json:"quality"`
	Threads              int    `json:"threads"`
	Theme                string `json:"theme"` // light | dark
	UIScale              int    `json:"uiScale"`
	PerfMode             string `json:"perfMode"`
	GradientBg           bool   `json:"gradientBg"`
	RememberTranslations bool   `json:"rememberTranslations"`
	CacheTTS             bool   `json:"cacheTts"`

	// API Trực Tiếp — endpoint OpenAI-compatible (OpenAI, LM Studio, Ollama, OpenRouter…)
	OpenAIBase  string `json:"openaiBase"`
	OpenAIKey   string `json:"openaiKey"`
	OpenAIModel string `json:"openaiModel"`
	// Media Xu hướng — kho media stock theo từ khóa
	PexelsKey string `json:"pexelsKey"`
	// Trình duyệt render HTML Video (rỗng = tự dò Chrome/Chromium)
	ChromeBin string `json:"chromeBin"`
	// VieNeu-TTS — python của venv (rỗng = tự dò data/vieneu/venv)
	VieNeuPython string `json:"vieneuPython"`
	// faster-whisper — bóc băng offline có mốc TỪNG TỪ
	WhisperPython  string `json:"whisperPython"`  // rỗng = tự dò data/whisper/venv
	WhisperModel   string `json:"whisperModel"`   // large-v3 | medium | small (mặc định small)
	WhisperCompute string `json:"whisperCompute"` // int8 | float16 | auto
}
