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
- settings/test thêm key "vieneu"; state.tools thêm "vieneu". Cài đặt: nút Cài ở Cấu hình (POST /api/setup/vieneu) hoặc scripts/setup-vieneu.sh.

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

## Mở rộng v1.5 — Style Kit (bộ style hình ảnh dùng chung)

### Store (ĐÃ CÓ — chỉ dùng)
`store.StyleKit{ID, Name, StylePrompt, Negative, Palette []string, Theme (vivid|dark|light), IsDefault, CreatedAt, UpdatedAt}`.
CRUD: `StyleKits()`, `StyleKit(id)`, `ActiveStyleKit() (StyleKit, bool)`, `SaveStyleKit(*StyleKit)` (đặt IsDefault tự bỏ mặc định bộ khác), `DeleteStyleKit(id)`. Đã seed sẵn 6 bộ mẫu, bộ đầu là mặc định.

### internal/stylekit (ĐÃ CÓ — chỉ dùng)
`stylekit.Apply(st, base string) string` — ghép style prompt + bảng màu + negative vào mô tả cảnh; `ApplyKit(k, base)`; `Theme(st) string`.
ĐÃ nối vào: `internal/vox/scene.go` (Gemini sinh ảnh cảnh) và `internal/htmlvideo/render.go` (ảnh template product).

### routes_style.go (server.go ĐÃ gọi s.routesStyle)
- `GET /api/styles` → []StyleKit
- `POST /api/styles` body `{name, stylePrompt, negative, palette, theme, isDefault}` → tạo (thiếu name → 400)
- `PUT /api/styles/{id}` → cập nhật (404 nếu không có)
- `DELETE /api/styles/{id}` → xoá; nếu xoá bộ đang mặc định thì đặt bộ còn lại mới nhất làm mặc định
- `POST /api/styles/{id}/default` → đặt làm mặc định
- `POST /api/styles/{id}/preview` body `{subject?}` → Job kind=style_preview: dùng `stylekit.ApplyKit` + `gemini.GenerateImage` sinh ảnh mẫu (subject mặc định "a person working at a desk near a window") vào `data/styles/<id>-preview.png`; output = đường dẫn tương đối DataDir. Chưa có Gemini key → lỗi 400 tiếng Việt hướng dẫn vào Cấu hình & API.

## Mở rộng v1.6 — Storyboard (ảnh từng cảnh, sửa được)

Mục tiêu: video Text → Video không còn là chữ trên nền gradient mà **mỗi đoạn có ảnh riêng**; người dùng sửa prompt từng cảnh, tạo lại đúng một cảnh, hoặc tự tải ảnh thay thế.

### Store (ĐÃ CÓ — chỉ dùng)
`store.T2VSegment` thêm 3 field: `ImagePath` (tương đối DataDir), `ImagePrompt` (mô tả cảnh để sinh ảnh — sửa được), `ImageSource` (`ai|stock|upload`).

### internal/text2video/storyboard.go (MỚI)
- `SuggestPrompt(seg store.T2VSegment) string` — sinh mô tả ảnh mặc định từ nội dung đoạn: dịch ý sang câu mô tả cảnh bằng tiếng Anh ngắn gọn (danh từ + bối cảnh + cảm xúc), KHÔNG chữ trong ảnh. Dùng LLM nếu có (engine như WriteScript), fallback heuristic từ chính text.
- `BuildStoryboard(ctx, st, sess *store.T2VSession, workDir string, upd func(float64,string)) error` — với MỌI đoạn chưa có ImagePath: sinh prompt nếu trống → sinh ảnh → lưu `shot-<i>.png` trong workDir → gán ImagePath/ImageSource. upd theo từng đoạn. Đoạn đã có ảnh (nhất là `upload`) thì BỎ QUA, không ghi đè.
- `BuildSegmentImage(ctx, st, sess, idx int, prompt string, workDir string) error` — sinh lại ảnh CHO ĐÚNG một đoạn (prompt rỗng → dùng ImagePrompt hiện có, vẫn rỗng → SuggestPrompt). Ghi đè `shot-<idx>.png`.
- Chuỗi nguồn ảnh: Gemini (`gemini.GenerateImage`, prompt qua `stylekit.Apply`) → Pexels (`stockmedia.SearchImage`, dùng ImagePrompt làm keyword) → lỗi tiếng Việt rõ nếu không có nguồn nào (KHÔNG tạo card màu ở bước này — để UI báo cho người dùng biết cần key).

### internal/htmlvideo — template mới `photo`
Thêm vào `templates/scene.html` + xử lý ảnh trong `render.go`: template `photo` = **ảnh tràn khung** (object-fit cover, zoom nhẹ 1.0→1.06 theo t) + lớp phủ gradient tối ở đáy + `Title` hiện ở 1/3 dưới (chữ lớn, đổ bóng, tối đa 3 dòng). `prepareImage` hiện chỉ lấy ảnh khi Template=="product" → mở rộng cho cả `photo`, và khi `Scene.Image` là đường dẫn file có thật thì copy dùng luôn (đã hỗ trợ).

### internal/text2video/video.go
`buildScenes`: đoạn nào có `ImagePath` → Template `photo`, `Image` = đường dẫn TUYỆT ĐỐI (DataDir + ImagePath), `Title` = câu nói rút gọn. Đoạn không có ảnh → giữ nguyên logic cũ (hero/bullets/quote/outro).

### routes_t2v.go — 3 endpoint mới
- `POST /api/t2v/sessions/{id}/storyboard` → Job kind=`t2v_storyboard` → BuildStoryboard → lưu session
- `POST /api/t2v/sessions/{id}/segments/{idx}/image` body `{prompt?}` → Job kind=`t2v_shot` → BuildSegmentImage (idx là số thứ tự 0-based, sai → 400)
- `POST /api/t2v/sessions/{id}/segments/{idx}/image/upload` — multipart `files` (1 ảnh) → lưu `shot-<idx>.png` (convert bằng ffmpeg nếu không phải png), ImageSource="upload" → trả session

## Mở rộng v1.7 — Nhân vật nhất quán

Mục tiêu: nhân vật xuất hiện ở nhiều cảnh trông GIỐNG NHAU xuyên suốt video, bằng cách chèn mô tả ngoại hình cố định vào prompt sinh ảnh của mọi cảnh có nhân vật đó.

### Store (ĐÃ CÓ — chỉ dùng)
`store.Character{ID, Name, Look (mô tả ngoại hình, tiếng Anh), Role (ghi chú), RefImage (tương đối DataDir), CreatedAt, UpdatedAt}`.
CRUD: `Characters()`, `Character(id)`, `SaveCharacter(*Character)`, `DeleteCharacter(id)`.
`store.T2VSegment` thêm `CharacterIDs []string` (json `characterIds`).

### internal/text2video/storyboard.go (SỬA)
- Prompt sinh ảnh của cảnh phải chèn mô tả nhân vật: `<mô tả cảnh>. Featuring <Tên>: <Look>[. <Tên2>: <Look2>]`. Thứ tự bắt buộc: mô tả cảnh → nhân vật → (Style Kit thêm ở lớp ngoài qua `stylekit.Apply`). Nhân vật không tồn tại trong store thì bỏ qua, KHÔNG lỗi.
- `SuggestPrompt` khi cảnh có nhân vật: yêu cầu LLM mô tả cảnh có người, KHÔNG mô tả ngoại hình (ngoại hình do Character lo).
- Tách hàm export `CharacterClause(st *store.Store, ids []string) string` để nơi khác dùng lại được.

### routes_chars.go (server.go ĐÃ gọi s.routesChars)
- `GET /api/characters` → []Character
- `POST /api/characters` body `{name, look, role}` → tạo (thiếu name → 400)
- `PUT /api/characters/{id}` · `DELETE /api/characters/{id}` (404 tiếng Việt; DELETE gỡ luôn id khỏi CharacterIDs của mọi T2VSession)
- `POST /api/characters/{id}/ref` — multipart `files` (1 ảnh) → lưu `data/characters/<id>.png` (convert ffmpeg nếu cần) → RefImage
- `POST /api/characters/{id}/preview` body `{scene?}` → Job kind=`char_preview`: sinh ảnh thử nhân vật (`stylekit.Apply` + Look + scene mặc định "standing in a simple room, medium shot") vào `data/characters/<id>-preview.png`; thiếu Gemini key → 400 tiếng Việt.

### FE
- Trang mới `characters` (nav đã đăng ký, file web/static/js/pages/characters.js): lưới nhân vật (avatar từ RefImage/preview, tên, vai trò, mô tả ngoại hình rút gọn), nút Tạo/Sửa/Xoá/Tải ảnh tham chiếu/Xem thử.
- Trong Storyboard (text2video-storyboard.js): mỗi cảnh thêm hàng chọn nhân vật — chip bật/tắt theo danh sách `GET /api/characters`, lưu vào `segment.characterIds` qua PUT session.

## Mở rộng v1.8 — Ý tưởng & Hàng đợi sản xuất

Mục tiêu: AI đề xuất hàng loạt ý tưởng video cho một chủ đề/kênh → người duyệt → hệ thống tự sản xuất tuần tự (viết kịch bản → giọng đọc → storyboard → dựng video) mà không cần bấm từng bước.

### Store (ĐÃ CÓ — chỉ dùng)
`store.Idea{ID, Title, Angle, Hook, Keywords []string, Status (proposed|approved|rejected|queued|producing|done|error), Width, Height, FPS, T2VSessionID, OutputPath, Error, CreatedAt, UpdatedAt}`.
CRUD: `Ideas()`, `Idea(id)`, `SaveIdea(*Idea)`, `DeleteIdea(id)`, `NextQueuedIdea() (Idea, bool)`.

### internal/ideas (MỚI)
- `Generate(ctx, st *store.Store, topic string, count int, audience, tone string) ([]store.Idea, error)` — hỏi LLM (engine như `text2video.WriteScript`: openai → gemini → claude CLI) trả JSON mảng `{title, angle, hook, keywords}`; tiêu đề tiếng Việt, hấp dẫn, không clickbait rẻ tiền; status="proposed". Strip codefence, lỗi parse → fallback tách dòng.
- `Runner` — bộ chạy hàng đợi TUẦN TỰ (mỗi lúc chỉ 1 ý tưởng, tránh tranh Chrome/ffmpeg/TTS):
  - `NewRunner(st *store.Store, dataDir string, broadcast func(string, any)) *Runner`
  - `(*Runner).Start()` / `Stop()` / `Running() bool` / `Status() (running bool, currentIdeaID string)`
  - Vòng lặp: `NextQueuedIdea()` → status="producing" → tạo `store.T2VSession` (tên = Title, kích thước từ Idea, SourceText = Title + Angle + Hook + Keywords) → `text2video.WriteScript` → `BuildVoice` → `BuildStoryboard` (lỗi thiếu key ảnh thì BỎ QUA, vẫn dựng được bằng chữ) → `BuildVideoHTML` → lưu OutputPath, status="done". Lỗi ở bước nào → status="error" + Error, chạy tiếp ý tưởng sau.
  - Mỗi bước gọi `broadcast("idea", idea)` và ghi `st.AddLog("info","ideas",...)`.
  - Stop() = ngừng NHẬN ý tưởng mới, ý tưởng đang chạy vẫn chạy xong (dùng context riêng, hủy khi Stop → đánh dấu error "đã dừng hàng đợi").

### routes_ideas.go (server.go ĐÃ gọi s.routesIdeas)
- `GET /api/ideas` → []Idea
- `POST /api/ideas/generate` body `{topic, count (mặc định 8, tối đa 20), audience, tone, width, height, fps}` → Job kind=`idea_gen` → lưu các Idea mới (kèm width/height/fps)
- `POST /api/ideas` body `{title, angle, hook, keywords, width, height, fps}` → tự thêm 1 ý tưởng thủ công (status="approved")
- `PUT /api/ideas/{id}` → sửa (title/angle/hook/keywords/status/width/height/fps)
- `DELETE /api/ideas/{id}`
- `POST /api/ideas/{id}/approve` → status="approved" · `POST /api/ideas/{id}/reject` → status="rejected"
- `POST /api/ideas/{id}/queue` → status="queued" (chỉ từ proposed/approved/error/rejected)
- `GET /api/ideas/queue` → `{running: bool, currentIdeaId: string, queued: int, producing: int}`
- `POST /api/ideas/queue/start` · `POST /api/ideas/queue/stop`
Runner khởi tạo 1 lần trong server (singleton như aiRunner ở routes_sessions.go).

### FE — trang `ideas` (nav đã đăng ký)
2 khu: **Sinh ý tưởng** (ô Chủ đề/kênh, số lượng, Đối tượng xem, Tông giọng, chọn khung hình → nút "✨ Sinh ý tưởng") và **Hàng đợi** (trạng thái runner + nút Bắt đầu/Dừng + đếm chờ/đang chạy). Danh sách ý tưởng dạng card: tiêu đề, badge trạng thái, góc tiếp cận, hook, chips keyword; nút theo trạng thái: Duyệt / Bỏ / Đưa vào hàng đợi / Mở phiên (khi có T2VSessionID) / Xem video (khi done) / Sửa / Xoá. Realtime qua `Bus.on('idea')` và `Bus.on('job')`.

## Mở rộng v1.9 — Template thiết kế đầy đủ (nâng Style Kit)

Mục tiêu: Style Kit không chỉ điều khiển prompt sinh ảnh mà điều khiển **toàn bộ giao diện video** — font, cỡ chữ, màu, logo, tên kênh, tư liệu nền — và **xem trước được ngay** trước khi render.

### Store (ĐÃ CÓ — chỉ dùng)
`store.StyleKit` mở rộng: `BgDeep, TextMain, Accent` (3 biến màu CSS), `FontHead, FontBody` (CSS font stack), `SizeTitle, SizeBig, SizeBody` (px), `ChannelName, LogoPath, LogoPos (left|center|right)`, `StockPaths []string`, `MaxVoiceChars, MaxImageChars`, `BaseTemplate (builtin|custom)`, `CustomHTML`.
`applyStyleDefaults` trong seed.go tự bù mặc định cho bộ cũ. 5 font stack sẵn có (dùng font hệ thống, render offline không lỗi): `fontStackModern/Impact/Serif/Round/Mono` — FE cho chọn theo nhãn tiếng Việt và cho nhập tự do.

### internal/htmlvideo — nhận Style Kit
- `Config` thêm `Kit *store.StyleKit` (nil → dùng bộ đang mặc định của store; vẫn nil → giá trị mặc định như hiện tại).
- `templates/scene.html`: thay màu/font/cỡ chữ hard-code bằng biến CSS `--bg-deep --text-main --accent --font-head --font-body --size-title --size-big --size-body` đổ từ Kit. Thêm 2 lớp mới:
  - **Logo + tên kênh**: hiện ở đáy khung theo `LogoPos`; ảnh logo nhúng base64 (render offline), tên kênh dùng font body cỡ nhỏ, mờ 0.85. Không có logo/tên thì không render gì.
  - **Tư liệu nền**: nếu `StockPaths` có ảnh/video, dùng làm nền chạy dưới lớp chữ cho các template KHÔNG có ảnh riêng (hero/bullets/quote/outro) — ảnh thì đặt cover + zoom nhẹ, video thì trích 1 khung hình đại diện (ffmpeg) rồi dùng như ảnh (giữ deterministic theo `seek(t)`).
- `BaseTemplate=custom`: dùng `CustomHTML` thay cho template dựng sẵn; các biến `{{TITLE}} {{SUBTITLE}} {{CHANNEL_NAME}} {{ACCENT}} {{BG_DEEP}} {{TEXT_MAIN}} {{IMAGE}}` được thay trước khi render; phải tự định nghĩa `window.seek(t)` — nếu thiếu thì render tĩnh (ghi log warn, KHÔNG lỗi).

### internal/text2video — áp giới hạn ký tự
`WriteScript` và `SuggestPrompt` đọc `MaxVoiceChars`/`MaxImageChars` của bộ style đang dùng để đưa vào prompt LLM và cắt hậu kiểm.

### routes_style.go — 4 endpoint mới
- `GET /api/styles/{id}/preview.html?template=hero&title=...&subtitle=...` → trả **HTML sống** dựng bằng ĐÚNG template render video (Content-Type text/html). FE nhúng iframe để xem trước — chỉnh gì thấy nấy, không cần render video.
- `POST /api/styles/{id}/logo` — multipart `files` (1 ảnh) → `data/styles/<id>-logo.png` → LogoPath
- `POST /api/styles/{id}/stock` — multipart `files` (nhiều) → `data/styles/<id>-stock-<n>.<ext>` → thêm vào StockPaths
- `DELETE /api/styles/{id}/stock?path=<path>` — gỡ 1 tư liệu nền (xoá file)

### FE — trang stylekit.js dựng lại theo 4 nhóm (như tham chiếu)
1. **Phong cách chuyển động**: chọn nền tảng (Dựng sẵn / Custom HTML + textarea mã), style prompt, negative
2. **Khung hình & giới hạn**: cỡ chữ tiêu đề/số lớn/nội dung (slider), giới hạn ký tự thoại + mô tả hình
3. **Màu sắc & phông chữ**: 4 preset màu bấm phát ăn ngay + 3 ô màu (nền/chữ/nhấn) + palette gợi ý cho AI + 2 select font
4. **Nhận diện & stock**: tên kênh, logo + vị trí, thư viện tư liệu nền (thêm/xoá)
Bên phải: **XEM TRƯỚC** iframe `preview.html` cập nhật khi đổi cài đặt (debounce ~400ms), có nút đổi template xem thử (hero/bullets/photo/outro) và nút phóng to.

## Mở rộng v2.0 — Âm thanh chuẩn xác (mốc từng từ)

Nền tảng: có mốc thời gian TỪNG TỪ thì mới cắt khoảng lặng an toàn, làm phụ đề karaoke, và hạ nhạc đúng lúc có tiếng nói.

### Vì sao (số đo thực tế, đã kiểm chứng ở dự án khác)
- Ngưỡng cắt im lặng là **thuộc tính của FILE, không phải hằng số**: cùng một file, −40dB tìm 0 khoảng lặng, −30dB tìm 13, −25dB tìm 21. Ngưỡng cứng hoặc vô dụng hoặc nuốt tiếng.
- Chỉ đo độ to **không phân biệt được "đang ngừng" với "nói nhỏ"**: ở −30dB máy báo 48.6s im lặng nhưng chỉ 18.2s là khoảng nghỉ thật; 30.4s còn lại nằm TRONG từ (âm cuối tiếng Việt c/t/p/ch không hữu thanh). Cắt theo độ to là cắt vào chữ.
→ Kết luận: khoảng lặng chỉ được cắt khi **nằm ngoài mọi từ** trong transcript.

### internal/whisper (MỚI) — faster-whisper offline
Cùng khuôn với internal/tts/vieneu.go (venv riêng + runner python nhúng trong binary + `/usr/bin/arch -arm64` trên Apple Silicon):
- `PythonPath(st) string` — Settings().WhisperPython → `<DataDir>/whisper/venv/bin/python3`; rỗng = chưa cài.
- `Available(st) bool`
- `Transcribe(ctx, st, audioOrVideo string, lang string, upd func(float64,string)) (*Transcript, error)`
- Types: `Word{Text string; Start, End float64}`, `Segment{Index int; Start, End float64; Text string; Words []Word}`, `Transcript{Language string; Duration float64; Segments []Segment}`
- Runner python: `from faster_whisper import WhisperModel; model=WhisperModel(<model>, device="auto", compute_type=<compute>)`; `segments, info = model.transcribe(path, language=lang or None, word_timestamps=True, vad_filter=True)`; in NDJSON từng segment ra stdout để Go cập nhật tiến độ, dòng cuối in JSON tổng.
- Ghi kèm `SaveJSON(tr, path)` và `LoadJSON(path)`.
- Xuất phụ đề: `SRT(tr) string` (theo segment) và `KaraokeASS(tr, style KaraokeStyle) string` — ASS có hiệu ứng \k từng từ, dùng cho burn phụ đề karaoke.
  `KaraokeStyle{FontName string; FontSize int; Primary, Highlight, Outline string (hex); MarginV int}` — lấy từ Style Kit đang dùng.

## Rút clip & hợp tuyển (internal/highlight)

### Chấm điểm theo LÔ — bắt buộc

`Score(ctx, st, cands, targetSec, goal, genreID, onProgress) ([]Candidate, ScoreReport, error)`

- Chia lô `scoreChunk = 60` đoạn/lượt. Đo thật: video 2 tiếng ra 1.107 đoạn;
  gửi hết một lượt thì model bị cắt giữa chừng, hoặc chấm vài trăm dòng rồi tự
  đóng mảng — kiểu sau hỏng LẶNG LẼ (gửi 300 nhận 100, clip chỉ lấy 2% đầu video).
- Sau mỗi lô, ĐẾM LẠI: đoạn nào thiếu thì hỏi lại **đúng những đoạn đó**, một lần.
- Thiếu > `maxUnscoredRatio = 0.20` → **lỗi**, không dựng clip.
  Thiếu ít hơn → chạy tiếp nhưng `ScoreReport.Warn` phải được hiện lên UI.
- `runLLMFn` là biến để test thay chỗ gọi AI; mã chạy thật luôn dùng `runLLM`.

### Thể loại (`Genres()`, `FindGenre(id)`)

7 thể loại; `auto` đứng đầu và là mặc định. Mỗi thể loại đóng góp hai câu vào
prompt: `high` (thế nào là 9-10 điểm) và `low` (thế nào bị hạ điểm). ID lạ hoặc
rỗng → `auto` (dữ liệu người dùng cũ không có trường này).
`GET /api/studio/highlight/genres` trả danh mục cho UI.

### Hợp tuyển (`Cluster`)

`POST /api/studio/collections` — bóc băng → chấm điểm → gom nhóm → dựng mỗi
nhóm một video. Body: `{path, secondsEach, max, platform, minScore, lang, genre}`.

- Chỉ đưa AI `clusterPoolMax = 60` đoạn điểm cao nhất: gom nhóm cần đọc hết mới
  thấy đoạn nào cùng chủ đề nên KHÔNG chia lô được như chấm điểm.
- Bất biến phải giữ: một đoạn chỉ thuộc MỘT hợp tuyển; nhóm dưới
  `minClipsPerCollection = 3` đoạn bị bỏ và các đoạn của nó **trả lại** cho nhóm
  sau; trong mỗi hợp tuyển các đoạn xếp theo THỜI GIAN gốc (cùng lý do với `Pick`).
- Số thứ tự AI bịa ra bị bỏ qua, không làm sập.
- Job chỉ mang một `Output` → trả video ĐẦU TIÊN để xem trước; danh sách đầy đủ
  ghi ở nhật ký.

## Cửa sổ app (internal/desktop)

Binary mở giao diện trong cửa sổ app riêng thay vì tab trình duyệt. Cờ:

| Cờ | Mặc định | Ý nghĩa |
|---|---|---|
| `-window` | `true` | Mở cửa sổ app. `-window=false` cho máy chủ không màn hình. |
| `-port` | `6868` | Cổng HTTP. |
| `-data` | `data` | Thư mục dữ liệu. |

Cách chạy: dò trình duyệt họ Chromium (htmlvideo.FindChrome trước, để tôn trọng
ChromeBin người dùng đã điền; rồi tới Edge/Brave/Vivaldi/Cốc Cốc) → chạy với
`--app=<url> --user-data-dir=<data>/appwindow`. Không tìm được → lui về trình
duyệt mặc định (`open` / `rundll32` / `xdg-open`).

Hai hành vi phải giữ:
- **Cổng đã có bản đang chạy** → mở thêm cửa sổ rồi thoát mã 0. KHÔNG được chết
  vì "address already in use": người dùng bấm icon lần hai là chuyện thường.
- **Đóng cửa sổ** → thoát, TRỪ KHI `store.Jobs()` còn job `running`/`queued`.
  Còn việc thì giữ máy chủ, kiểm lại mỗi 15 giây, xong hết mới thoát. Giết một
  lượt render dài vì người dùng đóng nhầm cửa sổ là mất trắng công.

Linux: `--class=BizStudio` phải khớp `StartupWMClass` trong `bizstudio.desktop`,
nếu không thanh tác vụ hiện icon Chrome thay vì mục Biz Studio.

## /api/setup — cài & cập nhật công cụ ngoài

`GET /api/setup/tools` → `[{id, label, desc, manual, installed, detail}]`.
Chỉ kiểm tra CỤC BỘ (`--version`, import trong venv), không gọi mạng — trang Cấu
hình gọi ngay khi mở.

`POST /api/setup/{id}?action=install|update` → trả NGAY `{tool, action, cmds, manual}`
rồi chạy nền. `id` nhận cả ID nội bộ lẫn tên quen: `ytdlp` = `yt-dlp` = `YT-DLP`.
Lỗi:
- `404` — không có công cụ đó
- `400` — hành động lạ, hoặc máy thiếu brew/winget/sudo (message là hướng dẫn cho người dùng)
- `409` — công cụ đó đang được cài

`POST /api/setup/{id}/cancel` → hủy lượt đang chạy (`404` nếu không có).

Tiến trình phát qua SSE, sự kiện `setup`:
```
{"tool":"ytdlp","line":"…","state":"running"}
{"tool":"ytdlp","state":"done"}
{"tool":"ytdlp","state":"error","error":"…","manual":"https://…"}
```
Chạy bằng context nền, KHÔNG phải context của request: đóng tab giữa chừng thì
việc cài vẫn chạy tiếp thay vì bỏ dở một venv nửa vời.

CLI tương đương: `bizstudio setup [<công-cụ>] [--update] [--dry-run] [--data DIR]`.
Không có tham số = liệt kê. Thiếu brew/winget trả `kind: "dependency"` (exit 3).

### scripts/setup-whisper.sh
Tạo venv `data/whisper/venv`, `pip install faster-whisper`, tải sẵn model theo `WhisperModel`, in hướng dẫn. Bỏ qua model bằng `SKIP_MODEL=1`.

### internal/media — AutoCut v2
`AutoCutGuarded(ctx, src, dst string, tr *whisper.Transcript, opt AutoCutOpt, upd) (Report, error)`
- `AutoCutOpt{SilenceDb float64 (0 = tự đo), MinSilence float64 (0 = 0.6s), PadStart, PadEnd float64 (mặc định 0.12/0.18), MinKeep float64 (0.25s)}`
- **Ngưỡng tự đo**: chạy `ffmpeg -af volumedetect` lấy `mean_volume`; ngưỡng = `mean_volume - 18dB`, kẹp trong [−45, −20]. Ghi log giá trị đã chọn.
- **Transcript bảo vệ**: khoảng lặng nào giao với `[word.Start-Pad, word.End+Pad]` của BẤT KỲ từ nào thì KHÔNG cắt. Không có transcript → chỉ cắt khoảng lặng ≥ 2× MinSilence và ghi log cảnh báo "cắt không có transcript bảo vệ".
- `Report{ThresholdDb float64; Guarded bool; TotalSilence, CutSilence float64; Cuts int; BeforeS, AfterS float64}` — để UI nói rõ đã cắt bao nhiêu.
- Giữ nguyên `AutoCut` cũ (gọi AutoCutGuarded với tr=nil) để không phá API hiện có.

### internal/media — nhạc nền né giọng
`MixBgmDucked(ctx, voiceOrVideo, bgm, dst string, vol float64, speech []Span) error` — nhạc chạy `volume=vol`, quanh mỗi đoạn có tiếng nói hạ thêm còn `vol*0.25` với fade 0.4s (dùng `volume` + biểu thức `enable`/`between` hoặc chuỗi `afade`); không có speech ranges → dùng `sidechaincompress` lấy giọng làm tín hiệu điều khiển. Vox và HTML Video chuyển sang dùng hàm này.

### routes_tools.go — ASR dùng whisper khi có
`POST /api/tools/asr` thêm `engine: "auto"|"whisper"|"gemini"` (mặc định auto = whisper nếu đã cài, ngược lại Gemini) và `karaoke: bool` → khi bật xuất thêm file `.ass` karaoke cạnh `.srt`. Output job vẫn là `.srt`; đường dẫn `.ass` ghi trong `detail`.
`POST /api/tools/autocut` thêm `transcriptPath?` và `guard: bool` (mặc định true) → dùng AutoCutGuarded; detail job in Report cho người dùng thấy ngưỡng đã chọn và số đoạn cắt.
`GET /api/state` thêm `tools.whisper`.
