# 🚀 Biz Studio

**Studio video AI chạy hoàn toàn trên máy của bạn** — biến ý tưởng, bài viết và footage thô thành video hoàn chỉnh: AI tự edit, tự cắt, tự tạo phụ đề, tự lồng tiếng, tự QC và tự đóng gói xuất bản.

Điểm khác biệt lớn nhất: phần AI edit video chạy qua **Claude CLI** (`claude -p`) — tức là dùng **subscription Claude (khuyên dùng gói Max 5x trở lên)**, *không tốn phí API theo token*. Một phiên edit phức tạp có thể tiêu 15M–60M token, nhưng tất cả nằm trong hạn mức subscription của bạn.

---

## Mục lục

- [Biz Studio làm được gì](#biz-studio-làm-được-gì)
- [Hệ thống vận hành thế nào](#hệ-thống-vận-hành-thế-nào)
- [Phiên AI edit video hoạt động ra sao](#phiên-ai-edit-video-hoạt-động-ra-sao)
- [Cài đặt & chạy](#cài-đặt--chạy)
- [Đóng gói exe / dmg / linux](#đóng-gói-exe--dmg--linux)
- [Cấu hình](#cấu-hình)
- [Các module chi tiết](#các-module-chi-tiết)
- [REST API](#rest-api)
- [Cấu trúc dữ liệu & mã nguồn](#cấu-trúc-dữ-liệu--mã-nguồn)
- [Khắc phục sự cố](#khắc-phục-sự-cố)

---

## Biz Studio làm được gì

| Nhóm | Tính năng |
|---|---|
| 🤖 **AI Edit video** | Mô tả yêu cầu bằng tiếng Việt → AI (Claude) tự phân tích asset, tự chạy ffmpeg cắt/ghép/chèn ảnh đúng thứ tự, xuất video final. Theo dõi từng bước AI làm việc realtime. |
| ✂️ **Tự động cắt** | Phát hiện & cắt bỏ khoảng lặng, đoạn thừa trong video dài (silence detection). |
| 📝 **Phụ đề** | Bóc băng âm thanh (ASR) và bóc chữ trên hình (OCR) thành file SRT; tùy chọn burn phụ đề vào video. |
| 🌐 **Dịch thuật** | Dịch SRT/TXT theo 4 phong cách (Phim/Vlog, Sub ngắn gọn, Truyện, Khoa học) — engine Claude CLI (subscription) hoặc Gemini API. |
| 🎙 **TTS / Giọng đọc** | Chuyển văn bản thành giọng đọc: hàng chục giọng macOS (có giọng Việt) + giọng Gemini TTS. |
| 🎬 **Bài viết → Video** | Dán bài viết → AI tách thành danh sách cảnh (tiêu đề, lời đọc, từ khóa media) → tự TTS + ghép ảnh + phụ đề + nhạc nền → render mp4 dọc 9:16 hoặc ngang 16:9. |
| 🎭 **Vox-Director** | Như trên nhưng gắn vào dự án, gán media cụ thể cho từng cảnh — làm video dạng TVC khi có đủ source. |
| 🛡 **QC tự động** | Đo loudness (LUFS), phát hiện frame đen, đoạn đứng hình, khoảng lặng — báo cáo kèm cảnh báo. |
| 🖼 **Thumbnail** | Tạo thumbnail từ frame video hoặc sinh bằng AI (Gemini). |
| 📦 **Gói xuất bản** | Một nút: video final + phụ đề .srt/.vtt + meta.json (tiêu đề, mô tả, hashtag do AI viết) + thumbnail → nén zip sẵn sàng đăng. |
| 📱 **Kết nối điện thoại** | Quét QR bằng camera điện thoại (cùng Wi-Fi) → gửi video/ảnh thẳng vào dự án, không cần cáp, không cần Drive. |
| ⬇️ **Tải video** | Tải hàng loạt từ YouTube/TikTok/Facebook… qua yt-dlp (dán link hoặc file TXT, chọn chất lượng, hỗ trợ cookies). |
| 🎞 **Studio Editor** | Thư viện media, preview, timeline trực quan, cắt khoảng lặng, render final. |

## Hệ thống vận hành thế nào

Biz Studio là **một binary Go duy nhất** nhúng sẵn toàn bộ giao diện web. Chạy lên là có studio tại `http://localhost:6868` — không cần database, không cần Node, không cần Docker.

```
┌──────────────────────────────┐
│  Trình duyệt (SPA vanilla JS) │  ← giao diện studio, cập nhật realtime qua SSE
└──────────────┬───────────────┘
               │ REST + Server-Sent Events
┌──────────────▼───────────────┐
│        Biz Studio (Go)        │
│  • HTTP server + embed UI     │
│  • Job queue (goroutine)      │
│  • Store JSON (data/db.json)  │
└──┬──────┬──────┬──────┬──────┘
   │      │      │      │
   ▼      ▼      ▼      ▼
 ffmpeg  Claude  Gemini  yt-dlp
 ffprobe  CLI     API
```

Nguyên tắc thiết kế:

- **Điều phối, không tái phát minh**: mọi việc nặng (encode, cắt, phân tích, AI) giao cho công cụ chuyên dụng — Biz Studio điều phối chúng qua job queue và stream kết quả về UI.
- **Mọi tác vụ dài là một Job**: chạy nền bằng goroutine, tiến độ đẩy realtime qua SSE (`/api/events/stream`), trạng thái lưu bền trong `data/db.json`.
- **Mỗi dự án là một thư mục**: `data/projects/<id>/` chứa `assets/` (nguồn), `outputs/` (kết quả), `publish/` (gói xuất bản) — dễ backup, dễ soi.
- **Local-first**: dữ liệu và video của bạn không rời khỏi máy, trừ phần văn bản/media bạn chủ động gửi tới AI (Claude/Gemini).

## Phiên AI edit video hoạt động ra sao

Đây là trái tim của Biz Studio (trang **Dự án** → nút *"Bắt đầu Edit bằng AI với phiên mới"*):

1. **Chuẩn bị dự án**: tải asset lên (hoặc quét QR gửi từ điện thoại), viết *Mô tả video gốc* + *Yêu cầu edit*, mô tả từng asset và đánh thứ tự, bật/tắt: tự cắt ngắn, tạo phụ đề, làm nổi bật key chính, thêm keyword.
2. **Build prompt**: server tổng hợp toàn bộ thành một prompt tiếng Việt chi tiết (thông số khung hình, danh sách asset kèm mô tả, các yêu cầu, quy ước output).
3. **Chạy Claude CLI**: `claude -p --output-format stream-json --dangerously-skip-permissions` với thư mục làm việc là thư mục dự án. Claude tự khám phá asset bằng `ffprobe`, tự viết và chạy lệnh `ffmpeg`, tự kiểm tra kết quả.
4. **Stream realtime**: từng dòng stream-json (khởi tạo, suy nghĩ, tool call, kết quả) được parse → lưu event → đẩy qua SSE → panel "AI của project" hiển thị y như bạn đang nhìn AI làm việc.
5. **Nhận kết quả**: AI ghi video vào `outputs/` + file `meta.json {"status":"done","output":"..."}`. Server đọc meta, cập nhật video output, pipeline 6 bước (Phân tích → Dựng scene → Render draft → Lắp draft → Render final → Hoàn thành) chuyển xanh.
6. **Dặn dò thêm**: chưa ưng? Gõ vào ô *"Dặn dò thêm cho AI…"* — phiên được resume (`--resume <session>`) với đầy đủ ngữ cảnh cũ, AI sửa tiếp.

> 💡 Vì chạy qua Claude CLI đăng nhập subscription, chi phí token không tính theo API. Gói Max 5x trở lên dùng thoải mái với video dài/phức tạp.

## Cài đặt & chạy

### Yêu cầu

| Công cụ | Bắt buộc? | Cài đặt |
|---|---|---|
| **ffmpeg + ffprobe** | ✅ Bắt buộc | macOS: `brew install ffmpeg` · Windows: [ffmpeg.org](https://ffmpeg.org/download.html) · Linux: `apt install ffmpeg` |
| **Claude CLI** (đăng nhập subscription) | Cho Phiên AI, dịch thuật, meta xuất bản | `npm i -g @anthropic-ai/claude-code` rồi chạy `claude` đăng nhập |
| **Gemini API key** | Cho OCR/ASR, ảnh AI, TTS Gemini, thumbnail AI | Lấy tại [aistudio.google.com](https://aistudio.google.com/apikey), dán vào **Cấu hình & API** |
| **yt-dlp** | Cho module Tải Video | `brew install yt-dlp` / `pip install yt-dlp` |
| Go 1.22+ | Chỉ khi build từ source | [go.dev/dl](https://go.dev/dl/) |

### Chạy từ bản đóng gói

Tải bản phù hợp từ trang [Releases](../../releases):

- **macOS**: mở `BizStudio-macos-*.dmg`, kéo **Biz Studio.app** vào Applications, mở app (lần đầu: chuột phải → Open để qua Gatekeeper). Trình duyệt tự mở `http://localhost:6868`.
- **Windows**: giải nén `BizStudio-windows-amd64.zip`, chạy `bizstudio.exe`, mở `http://localhost:6868`.
- **Linux**: giải nén `BizStudio-linux-*.tar.gz`, chạy `./bizstudio`.

Dữ liệu lưu trong thư mục `data/` cạnh chỗ chạy (tùy chỉnh bằng `-data`, cổng bằng `-port`).

### Chạy từ source

```bash
git clone https://github.com/nguyenduchoai/biz-studio.git
cd biz-studio
go run ./cmd/bizstudio
# → http://localhost:6868
```

## Đóng gói exe / dmg / linux

```bash
./scripts/build-release.sh
```

Script cross-compile và đóng gói vào `dist/`:

| File | Nền tảng |
|---|---|
| `BizStudio-macos-arm64.dmg` / `BizStudio-macos-amd64.dmg` | macOS (Apple Silicon / Intel) — chứa **Biz Studio.app** tự mở trình duyệt |
| `BizStudio-windows-amd64.zip` | Windows 64-bit (`bizstudio.exe`) |
| `BizStudio-linux-amd64.tar.gz` / `BizStudio-linux-arm64.tar.gz` | Linux |

(Tạo .dmg cần chạy trên macOS — script tự bỏ qua bước dmg trên hệ khác.)

## Cấu hình

Tất cả trong trang **Cấu hình & API** (lưu tại `data/db.json`):

| Mục | Ý nghĩa |
|---|---|
| Gemini Base / API Key / Model | Kết nối Gemini (mặc định `gemini-2.5-flash`) |
| Claude bin / model | Đường dẫn `claude` CLI + model tùy chọn |
| yt-dlp bin / Thư mục tải / Cookies / Chất lượng / Luồng | Cấu hình tải video |
| Giao diện / Kích thước / Gradient / Hiệu năng | Tuỳ biến UI (sáng/tối, scale…) |
| Nhớ bản dịch / Cache TTS | Tăng tốc thao tác lặp lại |
| **Kiểm tra kết nối** | Test 1 chạm: ffmpeg, Claude CLI, Gemini, yt-dlp |
| **Dọn file tạm** | Giải phóng `data/tmp` + tmp của các dự án |

## Các module chi tiết

- **Tổng quan** — số dự án, tác vụ đang chạy, trạng thái 4 công cụ, dự án & job gần đây.
- **Tải Video** — dán links (mỗi dòng 1 link) hoặc thả file TXT; mỗi link một job có progress; chọn chất lượng 1080/720/audio; hỗ trợ cookies để tải nội dung cần đăng nhập.
- **OCR / ASR** — kéo-thả file video/audio lên thẳng giao diện (hoặc nhập đường dẫn); ASR bóc âm thanh thành SRT, OCR bóc chữ trên khung hình (chọn FPS lấy mẫu); kết quả preview + tải về.
- **Dịch thuật** — kéo-thả SRT/TXT hoặc dán văn bản; 4 phong cách dịch; giữ nguyên timing SRT; engine Claude CLI hoặc Gemini; văn bản ngắn trả kết quả ngay, file dài chạy job nền theo batch.
- **TTS / Giọng đọc** — lưới giọng đọc (giọng Việt ưu tiên đầu), nghe thử bằng `<audio>`, chỉnh tốc độ đọc, xuất WAV.
- **Bài viết → Video** — quy trình 4 bước; bảng cảnh chỉnh sửa inline từng ô; cấu hình theme, khung hình, giọng, nhạc nền, burn phụ đề.
- **Vox-Director** — quy trình 5 bước, chọn dự án đích, gán media từng cảnh.
- **Studio Editor** — chọn dự án → thư viện media, preview, thuộc tính file, timeline theo thời lượng thật; cắt khoảng lặng, render final.
- **Dự án** — trang điều phối trung tâm (chi tiết ở phần Phiên AI phía trên) + QC, thumbnail, gói xuất bản, QR điện thoại, quản lý prompt mẫu.
- **Nhật ký** — log hệ thống realtime, lọc theo mức độ.

## REST API

Backend là REST thuần — bạn có thể tự động hóa mọi thứ không cần UI:

| Endpoint | Chức năng |
|---|---|
| `GET /api/state` | Trạng thái hệ thống, công cụ, thống kê máy |
| `GET /api/events/stream` | SSE: job, session_event, session, log, project |
| `GET/POST /api/projects` · `GET/PUT/DELETE /api/projects/{id}` | CRUD dự án |
| `POST /api/projects/{id}/assets` · `POST /m/{id}/upload` | Upload asset (desktop / điện thoại) |
| `POST /api/projects/{id}/sessions` · `/api/sessions/{id}/message` · `/stop` | Phiên AI: tạo, dặn dò thêm, dừng |
| `POST /api/projects/{id}/qc` · `/thumbnail` · `/publish` · `/render-final` | QC, thumbnail, gói xuất bản, render |
| `POST /api/tools/upload` | Upload file cho OCR/ASR/Dịch (vào `data/uploads/`) |
| `POST /api/tools/download` · `/asr` · `/ocr` · `/translate` · `/tts` · `/scenes` · `/vox` · `/autocut` | Các công cụ (đều trả Job) |
| `GET /api/tools/voices` · `GET /api/jobs` · `GET /api/logs` · CRUD `/api/prompts` | Tra cứu |
| `GET /api/qr.png?project=ID` · `GET /m/{id}` | QR + trang upload điện thoại |

Chi tiết request/response: [`docs/contracts.md`](docs/contracts.md).

## Cấu trúc dữ liệu & mã nguồn

```
data/                       # sinh khi chạy (không commit)
├── db.json                 # store JSON: projects, assets, sessions, jobs, settings, logs
├── projects/<id>/          # mỗi dự án một thư mục
│   ├── assets/  outputs/  publish/  tmp/
│   ├── qc.json  meta.json  thumb.jpg
├── uploads/                # file người dùng upload cho OCR/ASR/Dịch
├── downloads/              # video tải về
└── tmp/

cmd/bizstudio/              # entry point
internal/
├── server/                 # HTTP + SSE + routes
├── store/                  # JSON store an toàn goroutine
├── jobs/                   # job queue nền
├── agent/                  # phiên AI: chạy & parse Claude CLI stream-json
├── media/                  # wrapper ffmpeg: probe, autocut, concat, subs, LUT…
├── qc/                     # phân tích loudness / black / freeze / silence
├── gemini/                 # client REST Gemini (text, vision, audio, image, TTS)
├── tts/                    # giọng macOS say + Gemini
├── translate/              # dịch SRT/TXT giữ timing
├── downloader/             # yt-dlp
├── publishpkg/             # gói xuất bản + meta AI
├── vox/                    # engine render bài viết → video
└── util/                   # exec, thống kê máy
web/static/                 # SPA: index.html, css/, js/pages/*.js (nhúng vào binary)
```

## Khắc phục sự cố

| Hiện tượng | Cách xử lý |
|---|---|
| Phiên AI báo lỗi ngay khi tạo | Kiểm tra `claude` CLI: chạy `claude --version`, đăng nhập subscription. Xem **Cấu hình & API → Kiểm tra kết nối**. |
| OCR/ASR báo "chưa cấu hình Gemini API key" | Dán key vào **Cấu hình & API**, bấm Lưu rồi Kiểm tra kết nối. |
| Tải video lỗi "chưa cài yt-dlp" | `brew install yt-dlp` (macOS) hoặc `pip install yt-dlp`. |
| Video không preview được | Kiểm tra file có trong `data/` và URL bắt đầu bằng `/data/`. |
| Điện thoại không mở được trang QR | Điện thoại phải cùng mạng Wi-Fi; kiểm tra firewall cho phép cổng 6868. |
| Muốn đổi cổng / thư mục dữ liệu | `./bizstudio -port 8080 -data /duong/dan/khac` |

---

Made with ❤️ by **Hoai Nguyen** · [MIT License](LICENSE)
