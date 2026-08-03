# Biz Studio — Contracts (API + Conventions)

MỌI agent PHẢI đọc file này + `internal/store/types.go` trước khi code.
Ngôn ngữ UI: **Tiếng Việt**. Go package path gốc: `bizstudio`.

## Kiến trúc

```
cmd/bizstudio/main.go        — entry, flag -port (6868) -data (data/)
internal/store/            — JSON store (types.go, store.go, crud.go) [SCAFFOLD - KHÔNG SỬA]
internal/server/           — HTTP server, SSE hub, helpers, state [server.go/sse.go/helpers.go/state.go/static.go SCAFFOLD - KHÔNG SỬA]
internal/server/routes_*.go — route handlers (stub → agent thay thế)
internal/jobs/             — job manager [SCAFFOLD - KHÔNG SỬA]
internal/util/             — exec helpers, sys stats [SCAFFOLD - KHÔNG SỬA]
internal/agent/            — Claude CLI session runner
internal/media/            — ffmpeg/ffprobe wrappers
internal/qc/               — QC tự động
internal/gemini/           — Gemini REST client (text/vision/audio/image/tts)
internal/tts/              — TTS engines (macOS say + gemini)
internal/translate/        — dịch SRT/TXT (claude CLI | gemini)
internal/downloader/       — yt-dlp
internal/publishpkg/       — gói xuất bản
internal/vox/              — render Bài viết→Video / Vox-Director
web/embed.go               — embed.FS [SCAFFOLD]
web/static/                — index.html, mobile.html, css/, js/
```

Dữ liệu runtime: `data/` — `db.json`, `projects/<projectID>/` (assets/, outputs/, publish/, tmp/), `downloads/`, `tmp/`.

## Store (đã có sẵn — dùng, không sửa)

Xem `internal/store/types.go`. Store methods (crud.go): `Projects() []Project`, `Project(id)`, `SaveProject(*Project)`, `DeleteProject(id)`, `AssetsByProject(pid)`, `Asset(id)`, `SaveAsset(*Asset)`, `DeleteAsset(id)`, `SessionsByProject(pid)`, `Session(id)`, `SaveSession(*Session)`, `AddEvent(*SessionEvent)`, `EventsBySession(sid)`, `Jobs()`, `Job(id)`, `SaveJob(*Job)`, `Prompts()`, `SavePrompt(*PromptTemplate)`, `DeletePrompt(id)`, `AddLog(level, module, msg)`, `Logs(limit)`, `Settings()`, `SaveSettings(Settings)`, `NewID(prefix)`.
Mọi Save tự persist xuống disk. `store.Store` an toàn goroutine.

## Server helpers (đã có sẵn)

- `writeJSON(w, status, v)`, `httpErr(w, status, msg)`, `readJSON(r, &v) error`
- `s.Hub.Broadcast(event string, data any)` — SSE. Events chuẩn: `job` (data=store.Job), `session_event` (data={sessionId, event: store.SessionEvent}), `session` (data=store.Session), `log` (data=store.LogEntry).
- `s.Jobs.Submit(kind, projectID, detail string, fn func(upd func(progress float64, detail string)) (output string, err error)) *store.Job` — chạy nền, tự cập nhật store + SSE.
- `s.DataDir` string; `s.ProjectDir(id)` = data/projects/<id> (tự mkdir).
- `util.Run(ctx, name, args...) (stdout string, err)`; `util.RunErr(...)(stdout, stderr string, err)`; `util.Exists(bin) bool`; `util.LanIP() string`.
- Route đăng ký: mỗi file `routes_X.go` có method `func (s *Server) routesX(mux *http.ServeMux)` — server.go đã gọi sẵn. **Chỉ được sửa đúng file routes được giao.**
- Path param: dùng Go 1.22 pattern `mux.HandleFunc("GET /api/projects/{id}", ...)`, `r.PathValue("id")`.

## REST API (FE và BE phải khớp chính xác)

Mọi response lỗi: `{"error": "..."}` với status 4xx/5xx.

### Hệ thống
- `GET /api/state` → `{app:{name,version}, host:{cpuPct,ramPct,diskPct,ramUsedMB}, tools:{ffmpeg:bool,claude:bool,ytdlp:bool,geminiKey:bool}, counts:{projects,jobsRunning}, lanIP, port}` [SCAFFOLD có sẵn]
- `GET /api/events/stream` — SSE [SCAFFOLD]
- `GET /data/{path...}` — serve file trong data dir (video preview, thumb...) [SCAFFOLD]

### Settings (routes_settings.go)
- `GET /api/settings` → store.Settings
- `PUT /api/settings` body=store.Settings → lưu, trả settings
- `POST /api/settings/test` → `{gemini:{ok,detail}, claude:{ok,detail}, ffmpeg:{ok,detail}, ytdlp:{ok,detail}}`
- `POST /api/settings/cleanup` → xoá data/tmp + projects/*/tmp, trả `{freedMB}`

### Projects (routes_projects.go)
- `GET /api/projects` → `[]Project`
- `POST /api/projects` body `{name,kind,width,height,fps}` → Project (default 1080×1920@30 nếu 0)
- `GET /api/projects/{id}` → `{project, assets:[]Asset, sessions:[]Session, jobs:[]Job}`
- `PUT /api/projects/{id}` body=Project (chỉ update các field editable: name, briefDesc, editPrompt, autoCut/autoSub/autoKey, keywords, tags, width/height/fps, status, progress)
- `DELETE /api/projects/{id}` — xoá cả thư mục
- `POST /api/projects/{id}/duplicate` → Project mới (copy assets)
- `POST /api/projects/{id}/qc` → Job (kind=qc; output=đường dẫn qc.json; FE đọc qua /data/)
- `POST /api/projects/{id}/thumbnail` body `{mode:"frame"|"ai", t:float, prompt}` → Job; xong set project.thumbFile
- `POST /api/projects/{id}/publish` → Job kind=publish; tạo publish/ (video, .srt, .vtt, meta.json title/desc/hashtags qua LLM, thumbnail) + zip
- `POST /api/projects/{id}/render-final` → Job kind=render (copy/re-encode output draft → final, đặt outputFile)

### Assets (routes_assets.go)
- `POST /api/projects/{id}/assets` — multipart, field `files` (nhiều), lưu vào projects/<id>/assets/, tự phân loại kind theo ext (video/image/audio/other), ffprobe lấy duration/size → []Asset
- `PUT /api/assets/{id}` body `{desc,order}` → Asset
- `DELETE /api/assets/{id}`

### Phiên AI (routes_sessions.go)
- `POST /api/projects/{id}/sessions` body `{extra:""}` → Session (start claude chạy nền)
- `POST /api/sessions/{id}/message` body `{text}` → 202 (resume phiên với dặn dò thêm)
- `POST /api/sessions/{id}/stop` → 200
- `GET /api/sessions/{id}/events` → []SessionEvent

### Tools (routes_tools.go)
- `POST /api/tools/download` body `{links:[], quality, threads}` → []Job (mỗi link 1 job, kind=download)
- `POST /api/tools/asr` body `{path, lang}` (path tương đối data/ hoặc tuyệt đối) → Job kind=asr, output=file .srt
- `POST /api/tools/ocr` body `{path, fps}` → Job kind=ocr, output=.srt
- `POST /api/tools/translate` body `{path?, text?, mode:"phim|sub|truyen|khoahoc", engine:"claude|gemini", targetLang}` → Job kind=translate (nếu text ngắn: trả `{text}` đồng bộ khi <2000 chars)
- `GET /api/tools/voices` → `[{id,name,gender,lang,engine}]` (parse `say -v ?` + gemini presets)
- `POST /api/tools/tts` body `{text, voice, rate, engine}` → Job kind=tts, output=file audio
- `POST /api/tools/scenes` body `{content, count, style}` → `{scenes:[{title,voiceText,mediaKeyword,duration}]}` (LLM đồng bộ, timeout 120s)
- `POST /api/tools/vox` body `{projectId?, scenes:[...], config:{aspect:"16:9|9:16", voice, engine, theme, bgmPath, bgmVolume, quality, burnSub:bool}}` → Job kind=vox → render mp4
- `POST /api/tools/autocut` body `{path, silenceDb, minSilence}` → Job kind=autocut → mp4 đã cắt khoảng lặng
- `POST /api/tools/upload` — multipart `files` → lưu vào data/uploads/, trả `[{name, path, size}]` (path tương đối DataDir; dùng cho OCR/ASR/Dịch thuật)

### Khác (routes_misc.go)
- `GET/POST /api/prompts`, `PUT/DELETE /api/prompts/{id}` — CRUD PromptTemplate
- `GET /api/logs?limit=200` → []LogEntry
- `GET /api/qr.png?project=ID` → PNG QR trỏ `http://<lanIP>:<port>/m/<projectID>`
- `GET /m/{projectID}` → mobile.html (trang upload từ điện thoại)
- `POST /m/{projectID}/upload` — multipart `files` → thêm asset (dùng chung logic assets)

## Module Go signatures (route agents ĐỌC code module thật trước khi gọi)

- `agent.New(st *store.Store, broadcast func(string, any), dataDir string) *Runner`; `(*Runner).Start(projectID, extra string) (*store.Session, error)`; `Resume(sessionID, text string) error`; `Stop(sessionID) error`. Chạy `claude -p --output-format stream-json --verbose --dangerously-skip-permissions` (bin từ Settings.ClaudeBin, cwd=ProjectDir). Prompt build từ project (brief, editPrompt, toggles, keywords, danh sách asset + mô tả, yêu cầu output `outputs/<id>-vN.mp4` + `meta.json {status:"done", output:"..."}`). Parse NDJSON: system.init→claudeSessionId; assistant content blocks (text|tool_use)→AddEvent+SSE; result→cập nhật session (status, numTurns, costUSD). Sự kiện SSE: `session_event`, `session`.
- `media.Probe(path) (Info{Duration float64, Width, Height int, FPS float64, Size int64}, error)`; `media.Thumbnail(src, dst string, t float64, w int) error`; `media.AutoCut(ctx, src, dst string, silenceDb float64, minSil float64, upd func(float64,string)) error`; `media.BurnSubs(ctx, src, srt, dst) error`; `media.ExtractAudioWav16k(ctx, src, dst) error`; `media.ExtractFrames(ctx, src, outDir string, fps float64) ([]string, error)`; `media.Concat(ctx, parts []string, dst) error`; `media.ApplyLUT(ctx, src, lut, dst) error`
- `qc.Run(ctx, videoPath string) (Report, error)` — Report{DurationS, Width, Height, LoudnessLUFS, BlackSpans, FreezeSpans, SilenceSpans []Span{Start,End}, Warnings []string}; route lưu JSON vào projects/<id>/qc.json
- `gemini.NewFromSettings(st) *Client` (đọc key/base/model từ Settings; nếu key rỗng → các call trả error "chưa cấu hình Gemini API key"); `GenerateText(ctx, system, user string) (string, error)`; `GenerateWithFiles(ctx, prompt string, paths []string) (string, error)` (inline_data, đoán mime); `GenerateImage(ctx, prompt, dstPNG string) error`; `TTS(ctx, text, voice, dstWav string) error`
- `tts.Voices() []Voice{ID,Name,Gender,Lang,Engine}`; `tts.Speak(ctx, st *store.Store, text, voiceID string, rate float64, engine, dst string) error` (engine "say": say -o aiff → ffmpeg wav; "gemini": gemini.TTS)
- `translate.File(ctx, st, path, mode, engine, targetLang string, upd func(float64,string)) (outPath string, error)`; `translate.Text(ctx, st, text, mode, engine, targetLang string) (string, error)` — engine "claude": `claude -p` plain; "gemini": gemini.GenerateText. Giữ nguyên timing SRT.
- `downloader.Download(ctx, st, link, quality string, upd func(float64,string)) (outPath string, error)` — yt-dlp -o data/downloads/...; parse % progress; lỗi rõ nếu thiếu yt-dlp
- `publishpkg.Build(ctx, st, p *store.Project, dir string, upd func(float64,string)) (zipPath string, error)`
- `vox.Render(ctx, st, scenes []Scene, cfg Config, workDir string, upd func(float64,string)) (mp4 string, error)` — Scene{Title, VoiceText, MediaPath, MediaKeyword, Duration}; mỗi cảnh: TTS → ảnh (MediaPath | tìm asset theo keyword | gemini image | card màu drawtext title) → clip ffmpeg (loop ảnh + audio + drawtext) → concat + bgm + subs.

## Frontend conventions

File: `web/static/index.html`, `css/nova.css`, `js/api.js`, `js/ui.js`, `js/app.js`, `js/pages/<page>.js`, `mobile.html`.
KHÔNG dùng framework/CDN. ES modules KHÔNG dùng — script thường theo thứ tự: api.js, ui.js, app.js, pages/*.js (index.html liệt kê đủ).

### Global objects
- `API.get(p)`, `API.post(p, body)`, `API.put(p, body)`, `API.del(p)`, `API.upload(p, files, extra)` → Promise<json>, throw Error(message từ {error}).
- `Bus.on(event, fn)` / `Bus.off(event, fn)` — SSE events: `job`, `session_event`, `session`, `log`.
- `UI.*` helpers: `h(tag, attrs, ...children)` (attrs: class, onclick, value...; children: node|string), `UI.card({title, desc, icon, body, foot})`, `UI.btn(label, {variant:'primary|ghost|danger', icon, onclick, small})`, `UI.toggle(label, desc, checked, onchange)`, `UI.slider(label, {min,max,step,value,oninput})`, `UI.select(label, options[{value,label}], value, onchange)`, `UI.field(label, inputEl)`, `UI.input({value,placeholder,type,oninput})`, `UI.textarea({...})`, `UI.dropzone({hint, accept, multiple, onfiles})`, `UI.table(cols[{key,label,w}], rows[], renderCell?)`, `UI.chip(text, {onremove})`, `UI.progress(pct)`, `UI.toast(msg, type)`, `UI.modal({title, body, actions})`, `UI.empty(msg)`, `UI.fmtBytes(n)`, `UI.fmtDur(s)`, `UI.timeAgo(iso)`.
- Đăng ký trang: `App.pages['<id>'] = { title:'...', subtitle:'...', render(el, param){...} }`. Router hash: `#/<id>` hoặc `#/<id>/<param>` (vd `#/projects/prj_x`).
- Nav (app.js quản lý, id trang): `dashboard` Tổng quan; nhóm MODULE SÁNG TẠO: `download` Tải Video, `ocr` OCR / ASR, `translate` Dịch thuật, `tts` TTS / Giọng đọc, `article` Bài viết → Video, `vox` Vox-Director, `editor` Studio Editor; nhóm HỆ THỐNG: `projects` Dự án, `settings` Cấu hình & API, `logs` Nhật ký. Trang `prompts` (Quản lý prompt) không nằm nav, mở qua `#/prompts`.

### Design tokens (nova.css `:root`)
`--blue:#2563EB; --blue-h:#1D4ED8; --green:#10B981; --red:#EF4444; --amber:#F59E0B; --bg:#F5F7FB; --card:#FFF; --border:#E5EAF2; --text:#0F172A; --muted:#64748B; --r-card:14px; --r-input:10px; --sidebar-w:230px; --statusbar-h:38px;` Dark theme: `[data-theme=dark]` đổi bg #0B1220, card #111A2C, border #1E293B, text #E2E8F0, muted #94A3B8. Font: Inter, -apple-system, sans-serif. Layout: `.shell{display:grid;grid-template-columns:var(--sidebar-w) 1fr}`, header module (title+subtitle+actions), `.workspace` scroll, status bar cố định đáy hiển thị trạng thái + CPU/RAM/Disk (poll /api/state 5s) + chấm xanh "Backend hoạt động".

### Job pattern (FE)
Gọi POST tools/* → nhận Job → hiện progress card, cập nhật qua `Bus.on('job', ...)` khớp job.id; job.status done → hiện output (link `/data/<output>` nếu là file trong data, hoặc đường dẫn tuyệt đối hiển thị text + nút copy).

## Quy tắc chung agents
- Chỉ tạo/sửa file được giao. Đọc contracts + scaffold trước.
- `go build ./...` phải xanh cho package của mình trước khi kết thúc.
- File <400 dòng, hàm <50 dòng, kebab-case cho file FE, xử lý lỗi rõ ràng (không nuốt lỗi), text UI tiếng Việt.
- Kết thúc báo: **Status:** DONE|BLOCKED + Summary ngắn.

## Mở rộng v1.1 — HTML Video, API Trực Tiếp, Media Xu hướng

### internal/htmlvideo (engine render video từ HTML/CSS bằng chromedp)
- Scene (json camelCase): `{template, title, subtitle, bullets []string, code, image, chart [{label,value float64}], voiceText, duration float64, accent}`
- template: `hero | bullets | code | chart | product | quote | outro`
- Config: `{Aspect "9:16|16:9|1:1", Theme "vivid|dark|light", FPS int (default 24), Narration bool, Voice, Engine, BgmPath string, BgmVolume float64, BurnSub bool}`
- `htmlvideo.Render(ctx, st *store.Store, scenes []Scene, cfg Config, workDir string, upd func(float64,string)) (mp4 string, error)`
- Chrome dò theo thứ tự: Settings().ChromeBin → "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" → "/Applications/Chromium.app/Contents/MacOS/Chromium" → PATH "google-chrome","chromium". Export `FindChrome(st) (string, error)`.

### routes_htmlvideo.go
- `POST /api/tools/htmlvideo/plan` body `{prompt?, content?, repoUrl?, count, style}` → `{scenes:[Scene]}` (LLM đồng bộ ≤180s; ưu tiên engine: openai nếu OpenAIKey → gemini nếu GeminiAPIKey → claude CLI; repoUrl github → fetch README raw main/master)
- `POST /api/tools/htmlvideo` body `{projectId?, scenes, config}` → Job kind=htmlvideo; projectId → render vào ProjectDir/outputs + cập nhật project như vox; else data/tmp.

### internal/openaiapi (API Trực Tiếp — OpenAI-compatible)
- `Client{Base, Key, Model string; HTTP *http.Client}`; `NewFromSettings(st) *Client` (Key rỗng → mọi call lỗi "chưa cấu hình API Trực Tiếp (Cấu hình & API)")
- `ChatText(ctx, system, user string) (string, error)` — POST {Base}/chat/completions {model, messages}; parse choices[0].message.content; lỗi HTTP đọc body.
- translate: engine "openai" (thêm case trong engines.go).

### internal/stockmedia (Media Xu hướng — Pexels)
- `SearchImage(ctx, st *store.Store, keyword string, w, h int, dst string) error` — GET https://api.pexels.com/v1/search?query=&orientation=(portrait|landscape|square theo w/h)&per_page=3, header Authorization: PexelsKey; tải photos[0].src.large2x (fallback .original) về dst; PexelsKey rỗng → lỗi "chưa cấu hình Pexels API key (Media Xu hướng)".
- vox: chuỗi fallback ảnh cảnh: MediaPath → stockmedia.SearchImage (nếu có key + MediaKeyword) → gemini.GenerateImage → card màu.

### Settings fields mới (đã có trong types.go): openaiBase, openaiKey, openaiModel, pexelsKey, chromeBin.
### settings/test (v1.1) trả thêm keys: openai, pexels, chrome — FE render ĐỘNG mọi key trong response.

## Mở rộng v1.2 — VieNeu-TTS (engine giọng mặc định)
- internal/tts: engine mới "vieneu" — VieNeu-TTS on-device 48 kHz (https://github.com/pnnbao97/VieNeu-TTS). Venv tại data/vieneu/venv (Settings.vieneuPython override). Runner python NHÚNG trong binary (tts/vieneu.go) → tự ghi data/vieneu/runner.py.
- Speak engine rỗng: tự chọn vieneu → say → gemini; nếu voiceID thuộc engine khác thì theo engine của giọng. voiceID VieNeu hỗ trợ "Tên@style" (tu_nhien|tin_tuc|doc_truyen).
- tts.VoicesFor(st) = giọng VieNeu (đọc data/vieneu/voices.json, fallback 5 tên) + Voices() cũ. Route /api/tools/voices dùng VoicesFor.
- settings/test thêm key "vieneu"; state.tools thêm "vieneu". Cài đặt: scripts/setup-vieneu.sh.

## Mở rộng v1.3 — Dubbing chất lượng & Clone voice

### Store (ĐÃ CÓ SẴN — dùng, không sửa)
`store.CloneVoice{ID, Name, Path (tương đối DataDir: vieneu/clones/<id>.wav), Gender, Note, Duration, CreatedAt}`
CRUD: `CloneVoices() []CloneVoice`, `CloneVoice(id) (CloneVoice, bool)`, `SaveCloneVoice(*CloneVoice)`, `DeleteCloneVoice(id)`.

### internal/tts — clone voice
- Voice ID của giọng clone: **`clone:<cloneID>`** (kèm style: `clone:<id>@tin_tuc`). Engine field = `"clone"`.
- `VoicesFor(st)` = giọng clone (engine "clone", Lang "vi-VN · nhân bản") + VieNeu + say + gemini.
- `Speak(ctx, st, text, voiceID, rate, engine, dst)`: voiceID bắt đầu `clone:` → tra store, lấy file ref (abs = DataDir + Path) → gọi runner với `--ref-audio <path>` (KHÔNG truyền --voice). engine rỗng + voiceID clone → tự dùng vieneu.
- Runner python (const `vieneuRunner` trong tts/vieneu.go) thêm arg `--ref-audio`; khi có thì gọi `v.infer(text, ref_audio=..., style=...)` (SDK tự denoise).
- SDK thật (đã kiểm chứng): `Vieneu().infer(text, ref_audio=None, voice=None, style="tu_nhien", denoise=True, ...)`, `list_preset_voices() -> [(label, id)]`.

### routes_clone.go
- `GET /api/tools/clone-voices` → []store.CloneVoice
- `POST /api/tools/clone-voices` multipart: field `files` (1 file audio/video), `name` (bắt buộc), `gender`, `note` → ffmpeg convert sang wav mono 44100 lưu `data/vieneu/clones/<id>.wav`; ffprobe kiểm tra 2s ≤ duration ≤ 20s (ngoài khoảng → 400 kèm hướng dẫn "clip mẫu nên dài 3–8 giây, giọng rõ, không nhạc nền"); trả CloneVoice.
- `DELETE /api/tools/clone-voices/{id}` → xoá record + file.
- `POST /api/tools/clone-voices/{id}/preview` body `{text?}` → Job kind=tts (mặc định text "Xin chào, đây là giọng nhân bản của bạn từ Biz Studio.") → dùng `clone:<id>`.

### internal/dubbing — lồng tiếng video theo SRT
```go
type Config struct {
  Voice, Engine, Style string
  TargetLang, TranslateEngine string // TargetLang != "" → dịch SRT trước khi đọc
  KeepOriginal bool; OriginalVolume float64 // default 0.12
  FitTiming bool; MaxSpeed float64          // default true / 1.6
}
type Result struct { VideoPath, AudioPath, SrtPath string }
func Run(ctx, st *store.Store, videoPath, srtPath string, cfg Config, workDir string, upd func(float64,string)) (Result, error)
```
Luồng: (1) srtPath rỗng + videoPath có → ASR bằng gemini.GenerateWithFiles trên wav 16k (media.ExtractAudioWav16k) → .srt; không có Gemini key → lỗi rõ "cần file phụ đề .srt hoặc cấu hình Gemini API key để tự bóc băng". (2) TargetLang != "" → translate.File. (3) Parse SRT (translate.ParseSRT / tự parse) → mỗi cue: tts.Speak → ffprobe duration → nếu FitTiming và dài hơn slot: atempo (chuỗi atempo ≤2.0 mỗi tầng, clamp ≤ MaxSpeed) cho vừa; ngắn hơn → giữ nguyên. (4) Ghép track: **concat tuần tự** silence(khoảng trống) + segment (KHÔNG dùng amix nhiều input) → dub.wav. (5) videoPath có → mux: nếu KeepOriginal thì amix audio gốc volume=OriginalVolume với dub; ffmpeg -c:v copy → dubbed.mp4. videoPath rỗng → chỉ trả AudioPath. upd() theo từng cue.

### routes_dubbing.go
- `POST /api/tools/dub` body `{videoPath?, srtPath?, voice, engine, style, targetLang, translateEngine, keepOriginal, originalVolume, fitTiming, maxSpeed}` → Job kind=dub; workDir = data/tmp/dub_<jobid>; output = đường dẫn tương đối DataDir của video (hoặc audio nếu không có video). Resolve path bằng s.toolAbsPath/s.toolSrcPath (đã có trong routes_tools.go, cùng package).

## Mở rộng v1.4 — Text → Video (phiên làm việc lưu được)

Module mới `text2video`: nguồn (link bài viết / dán văn bản) → kịch bản đọc (LLM, chia đoạn, sửa tay) → giọng đọc (đo thời lượng THẬT từng đoạn) → cấu hình → dựng video. Mỗi lần chạy là 1 **phiên lưu trong store**, quay lại sửa tiếp được.

### Store (ĐÃ CÓ SẴN — dùng, không sửa)
`store.T2VSession` + `store.T2VSegment` (xem types.go). CRUD: `T2VSessions()`, `T2VSession(id)`, `SaveT2VSession(*T2VSession)`, `DeleteT2VSession(id)`.
Thư mục dữ liệu phiên: `data/text2video/<sessionID>/` — `seg-<i>.wav`, `voice.wav`, `transcript.json`.

### internal/text2video
- `FetchArticle(ctx, url string) (title, text string, err error)` — GET (User-Agent trình duyệt, timeout 30s, giới hạn 5MB), bóc text: ưu tiên `<article>`/`<main>`, bỏ `<script>/<style>/<nav>/<footer>/<header>/<aside>`, giải mã HTML entity, gộp khoảng trắng, giữ xuống dòng giữa các block; lỗi HTTP/khác HTML → lỗi tiếng Việt rõ.
- `WriteScript(ctx, st, src string, engine, model string, targetSeconds int) ([]store.T2VSegment, error)` — prompt tiếng Việt: viết kịch bản ĐỌC (văn nói tự nhiên, không markdown/emoji/ký hiệu), chia thành các đoạn 1–3 câu, mỗi đoạn 1 ý; nếu targetSeconds>0 ước lượng ~15 ký tự/giây để canh độ dài; trả JSON mảng string. engine: `claude` (claude CLI `-p --output-format text`, thêm `--model <model>` nếu model≠""), `gemini`, `openai`. Chars = utf8.RuneCountInString.
- `BuildVoice(ctx, st, sess *store.T2VSession, workDir string, upd func(float64,string)) error` — TTS từng đoạn (tts.Speak) → `seg-<i>.wav` → ffprobe đo **Seconds thật** ghi vào segment → concat toàn bộ thành `voice.wav` (chuẩn hoá 44100 mono trước khi concat) → ghi `transcript.json` `{"language":"vi","duration":<tổng>,"segments":[{"index","start","end","text"}]}` → cập nhật sess.VoicePath/TranscriptPath/VoiceSeconds.
- `BuildVideoHTML(ctx, st, sess, workDir, upd) (string, error)` — mỗi segment → 1 cảnh htmlvideo (template `hero` cho đoạn đầu, `bullets`/`quote` xen kẽ, `outro` đoạn cuối), Duration = Seconds thật của đoạn, Narration=false (đã có voice.wav) → render → **ghép voice.wav vào** (thay audio) → mp4.
- `BuildProject(ctx, st, sess, projectDir string) (projectID string, err error)` — tạo `store.Project` (kích thước từ sess), copy `voice.wav` + `transcript.json` vào assets, đặt BriefDesc/EditPrompt mô tả yêu cầu dựng video bám theo giọng đọc; trả projectID để route khởi động phiên AI.

### routes_t2v.go
- `GET /api/t2v/sessions` → []T2VSession · `POST /api/t2v/sessions` body `{name?, width?, height?, fps?}` → tạo (default 1080×1920@30, tên "Phiên <thời gian>")
- `GET/PUT/DELETE /api/t2v/sessions/{id}` — PUT chỉ ghi field editable (name, sourceKind, sourceUrl, sourceText, scriptEngine, scriptModel, targetSeconds, segments, voiceId, voiceEngine, voiceStyle, width, height, fps, step, buildMode); DELETE xoá cả thư mục phiên
- `POST /api/t2v/sessions/{id}/fetch` body `{url}` → đồng bộ (timeout 60s): FetchArticle → lưu sourceText/sourceUrl/sourceKind="link" → trả session
- `POST /api/t2v/sessions/{id}/script` → Job kind=t2v_script → WriteScript → lưu segments (Seconds=0), status="script", step≥2
- `POST /api/t2v/sessions/{id}/voice` → Job kind=t2v_voice → BuildVoice → status="voice", step≥3; output = voice.wav (đường dẫn tương đối DataDir)
- `POST /api/t2v/sessions/{id}/build` body `{mode:"ai"|"html"}` → Job kind=t2v_build. mode "html": BuildVideoHTML → OutputPath. mode "ai": BuildProject → tạo project, gán sess.ProjectID, rồi khởi động phiên AI (dùng chung runner như routes_sessions.go — ĐỌC file đó) và trả projectID; FE mở project để theo dõi.
Mọi handler: session không tồn tại → 404 tiếng Việt. Job xong luôn SaveT2VSession.
