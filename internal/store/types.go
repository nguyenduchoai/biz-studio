package store

import "time"

// Project — dự án video (trang điều phối AI edit) hoặc dự án vox/article.
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"` // video | vox | article
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	FPS       int       `json:"fps"`
	Status    string    `json:"status"`   // draft | running | done | error
	Progress  int       `json:"progress"` // 0..6: Phân tích, Dựng scene, Render draft, Lắp draft, Render final, Hoàn thành
	Tags      []string  `json:"tags"`
	BriefDesc string    `json:"briefDesc"`  // Mô tả video gốc
	EditPrompt string   `json:"editPrompt"` // Yêu cầu edit (prompt)
	AutoCut   bool      `json:"autoCut"`    // Tự động cắt ngắn video
	AutoSub   bool      `json:"autoSub"`    // Tạo phụ đề
	AutoKey   bool      `json:"autoKey"`    // Làm nổi bật key chính
	Keywords  []string  `json:"keywords"`
	OutputFile string   `json:"outputFile"` // tương đối data dir
	ThumbFile  string   `json:"thumbFile"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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

// Character — nhân vật xuất hiện lặp lại trong nhiều cảnh; mô tả ngoại hình
// được chèn vào prompt sinh ảnh để nhân vật trông giống nhau xuyên suốt video.
type Character struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`      // tên gọi ngắn, dùng để gán cho cảnh
	Look      string    `json:"look"`      // mô tả ngoại hình (tiếng Anh cho model hiểu)
	Role      string    `json:"role"`      // vai trò/tính cách (ghi chú cho người dùng)
	RefImage  string    `json:"refImage"`  // ảnh tham chiếu, tương đối data dir
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// StyleKit — bộ style hình ảnh dùng chung cho mọi cảnh của một video,
// để hình sinh ra đồng nhất như cùng một bộ phim.
type StyleKit struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	StylePrompt string    `json:"stylePrompt"` // ghép vào MỌI prompt sinh ảnh
	Negative    string    `json:"negative"`    // thứ cần tránh
	Palette     []string  `json:"palette"`     // mã màu hex
	Theme       string    `json:"theme"`       // vivid | dark | light (nền khung chữ)
	IsDefault   bool      `json:"isDefault"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
	GeminiBase   string `json:"geminiBase"`
	GeminiModel  string `json:"geminiModel"`
	ClaudeBin    string `json:"claudeBin"`
	ClaudeModel  string `json:"claudeModel"`
	YtdlpBin     string `json:"ytdlpBin"`
	DownloadDir  string `json:"downloadDir"`
	CookiesFile  string `json:"cookiesFile"`
	Quality      string `json:"quality"`
	Threads      int    `json:"threads"`
	Theme        string `json:"theme"` // light | dark
	UIScale      int    `json:"uiScale"`
	PerfMode     string `json:"perfMode"`
	GradientBg   bool   `json:"gradientBg"`
	RememberTranslations bool `json:"rememberTranslations"`
	CacheTTS     bool   `json:"cacheTts"`

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
}
