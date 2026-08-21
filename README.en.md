<div align="center">

# 🚀 Biz Studio

**A local-first AI video studio in a single Go binary.**

Turn ideas, articles and raw footage into finished videos — AI editing, silence cutting, subtitles, dubbing, QC and a ready-to-post package. Nothing leaves your machine unless you send it.

[**English**](README.en.md) · [Tiếng Việt](README.md)

[![Release](https://img.shields.io/github/v/release/nguyenduchoai/biz-studio?style=flat-square)](../../releases)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![Platforms](https://img.shields.io/badge/macOS%20·%20Windows%20·%20Linux-lightgrey?style=flat-square)](../../releases)

<img src="assets/demo.gif" width="100%" alt="Biz Studio — dashboard, preset workshop, colour grading, voices, characters">

🎬 [**Watch the intro video**](Biz-Studio-intro-final.mp4) — *made with Biz Studio's own HTML Video module and VieNeu voice.*

</div>

---

## Why this exists

Most AI video tools are SaaS: you upload your footage, you pay per minute, and your raw material sits on someone else's disk. Biz Studio is the opposite.

- **One binary.** No Docker, no Node, no database, no build step. Download, run, open `http://localhost:6868`.
- **Your files stay put.** Video never leaves your machine except the text or media you explicitly send to an AI provider.
- **AI editing runs on your Claude subscription**, not a metered API. Biz Studio drives `claude -p` (Claude Code CLI) with your logged-in session — a complex edit can burn 15–60M tokens and it still comes out of your plan, not your credit card. *(A Max 5x plan or higher is recommended for long videos.)*
- **Built for one language properly, then 40 more.** Vietnamese TTS runs on-device at 48 kHz with regional accents; 41 languages are available from voices already installed on your machine.

---

## Quick start

```bash
# 1. Install ffmpeg (the only hard requirement)
brew install ffmpeg          # macOS
# apt install ffmpeg         # Linux
# https://ffmpeg.org/download.html   (Windows)

# 2. Grab a build from Releases, or run from source:
git clone https://github.com/nguyenduchoai/biz-studio.git
cd biz-studio && go run ./cmd/bizstudio

# → open http://localhost:6868
```

Prebuilt: [**Releases**](../../releases) → `.dmg` (macOS Intel/ARM), `.zip` (Windows), `.tar.gz` (Linux ARM/x86).

> On macOS the app is unsigned — right-click → **Open** → **Open** the first time.

---

## Command line & AI agents

Every stage also runs headless, so you can script it or let a coding agent drive it:

```bash
# assemble b-roll clips to match a voice-over track
bizstudio broll --clips ./footage --audio voice.wav --workdir ./job --aspect 16:9

# normalise for TikTok — no need to repeat paths, the manifest remembers
bizstudio normalize --workdir ./job --platform tiktok
```

Four conventions, each of them the fix for a specific way machine-reading-machine breaks:

| Convention | Why |
|---|---|
| **The last stdout line is always one JSON object.** Progress logs go to stderr. | Making an agent parse prose logs is making it guess — and logs change. |
| **Each command writes `bizstudio_manifest.json`** into the working directory. | The next command only needs `--workdir`; it reuses earlier artifacts instead of you copying paths around. |
| **`--dry-run`** validates arguments and writes a manifest without doing anything expensive. | Test the command before burning an hour of rendering or a paid API call. |
| **Errors are typed**, with matching exit codes: `usage` (2), `dependency` (3), `retryable` (4), `failed` (1). | A generic "it failed" leaves an agent retrying blindly. A type tells it whether to fix arguments, install a tool, or retry. |

Flags work **before or after** the positional argument — `normalize video.mp4 --platform tiktok` and `normalize --platform tiktok video.mp4` behave identically.

Commands: `probe` · `normalize` · `broll` · `autocut` · `platforms` · `templates` · `help`

Running with no command (or with a leading `-`) starts the web UI as before.

---

## What it does

| Area | Capability |
|---|---|
| 🤖 **AI video editing** | Describe what you want in plain language → Claude inspects your assets with `ffprobe`, writes and runs its own `ffmpeg` commands, checks the result. You watch every step stream in live. |
| ✂️ **Silence cutting** | Detects and removes dead air. Guarded by word-level timestamps so it never clips a syllable — 120 ms of head padding, 180 ms of tail (Vietnamese unvoiced finals like `c/t/p/ch` are quiet and need the extra room). |
| 🎬 **Long video → short clip** | Transcribe → AI scores every segment for how worth keeping it is → splices the best ones **in original time order** (score picks, time orders — shuffled clips lose the thread). Fits under each platform's duration cap. |
| 🎞 **B-roll assembly** | Point at a folder of footage and a voice track. Clips get chopped into short pieces, **rotated across every source file**, and cut to exactly the length of the narration. Odd aspect ratios get padded, never squashed. |
| 📝 **Subtitles** | Offline `faster-whisper` with **word-level** timing — no API key needed. Exports `.srt` and karaoke `.ass`. OCR pulls on-screen text too. |
| 🌐 **Translation** | SRT/TXT in 4 registers (film/vlog, terse subs, prose, technical), timings preserved. Runs on Claude CLI, Gemini, or any OpenAI-compatible endpoint. |
| 🎙 **TTS & voice cloning** | **VieNeu-TTS** on-device at 48 kHz — 14 Vietnamese voices across 3 regional accents. Plus macOS `say`, Gemini TTS, and voice cloning from a 3–8 second sample. 41 languages grouped and named. |
| 🧩 **HTML Video** | Video-as-code: a prompt, an article, or a **GitHub repo URL** → AI splits it into scenes → frames are built in **HTML/CSS** → rendered to MP4 through headless Chrome. Page-flip transitions, a comic-style "being drawn" reveal, 9:16 / 3:4 / 16:9 / 1:1. |
| 📜 **Text → Video** | Saved sessions: source → AI script (editable) → voice-over with **measured per-segment duration** → build. Picture follows the recorded voice, never the other way round. |
| 🧰 **Preset workshop** | 22 templates across 7 niches, 6 platform presets (TikTok/Reels/Shorts/YouTube/Facebook/square) with correct framing and −14 LUFS, 7 synthesised background-music moods. |
| 🎥 **Veo** | Text → 4/6/8-second clips **with audio** (Google Veo 3.1). Paid per second on **your own** key; cost is shown up front and every run needs confirmation. |
| 🗣 **Talking avatar** | One photo + one voice file → a lip-synced speaking video (LongCat-Video-Avatar, MIT). Requires an NVIDIA GPU, local or over the network. |
| 🎨 **Look** | 14 colour-grade presets with single-frame preview, 10 synthesised sound effects levelled to a common peak, bundled Vietnamese font so every machine renders identically. |
| 🛡 **Auto QC** | Loudness (LUFS), black frames, frozen segments, silences — reported with warnings. |
| 📦 **Publish package** | One click: final video + `.srt`/`.vtt` + `meta.json` (AI-written title, description, hashtags) + thumbnail, zipped and ready to upload. |
| 📱 **Phone handoff** | Scan a QR on the same Wi-Fi and send video straight into a project. No cable, no cloud drive. |
| ⬇️ **Downloader** | Batch fetch from YouTube/TikTok/Facebook via `yt-dlp`, with cookie support. |
| 🎞 **Film → narration** | Split a movie by visual scene change, let AI watch actual frames and write commentary, dub it in Vietnamese over the film with automatic ducking, export to CapCut. |

---

## Screenshots

<table>
<tr>
<td width="50%"><img src="assets/xuong-lam-san.png" alt="Preset workshop"><br><sub><b>Preset workshop</b> — 22 templates across 7 niches. Each one bundles script guidance, three-beat pacing, image style, framing, target platform, voice and music mood.</sub></td>
<td width="50%"><img src="assets/tts.png" alt="Voices"><br><sub><b>Voices</b> — VieNeu on-device Vietnamese at 48 kHz with regional accents, plus 41 languages grouped from voices already on your machine.</sub></td>
</tr>
<tr>
<td colspan="2"><img src="assets/dien-mao.png" alt="Colour grading"><br><sub><b>Look</b> — 14 colour-grade presets, each previewed on a single frame before you spend time on the whole video. Strength adjustable 10–100%.</sub></td>
</tr>
</table>

---

## How it works

Biz Studio is a single Go binary with the whole web UI embedded.

```
┌──────────────────────────────┐
│  Browser (vanilla-JS SPA)    │  ← live updates over SSE
└──────────────┬───────────────┘
               │ REST + Server-Sent Events
┌──────────────▼───────────────┐
│        Biz Studio (Go)       │
│  • HTTP server + embedded UI │
│  • Job queue (goroutines)    │
│  • JSON store (data/db.json) │
└──┬──────┬──────┬──────┬──────┘
   ▼      ▼      ▼      ▼
 ffmpeg  Claude  Gemini  yt-dlp
 ffprobe  CLI     API
```

Design rules:

- **Orchestrate, don't reinvent.** Encoding, cutting, analysis and inference go to purpose-built tools; Biz Studio schedules them and streams results back.
- **Every long task is a Job** — a goroutine, progress over SSE, state persisted in `data/db.json`.
- **Every project is a folder** — `assets/`, `outputs/`, `publish/`. Easy to back up, easy to inspect.
- **Local-first.** Your media stays on disk.

---

## Requirements

> **You do not have to install these by hand.** Open **Settings & API → 🧰 Tools on this machine** and
> hit **Install** or **Update** next to any row — it uses your OS package manager (brew / winget /
> apt) and streams the output into the page. Headless? `bizstudio setup yt-dlp --update`.
>
> A `HTTP Error 403: Forbidden` while downloading video is almost always a stale yt-dlp, not a block.
> The card tells you the age of your build and updates it in one click.

| Tool | Required? | Install |
|---|---|---|
| **ffmpeg + ffprobe** | ✅ Yes | `brew install ffmpeg` · `apt install ffmpeg` · [ffmpeg.org](https://ffmpeg.org/download.html) |
| **Claude CLI** (subscription login) | For AI editing, translation, publish metadata | `npm i -g @anthropic-ai/claude-code`, then run `claude` to log in |
| **Gemini API key** | For OCR/ASR, AI images, Gemini TTS, AI thumbnails | [aistudio.google.com](https://aistudio.google.com/apikey) |
| **yt-dlp** | For the downloader | one click in Settings — **keep it updated**, stale builds cause 403 errors |
| **Google Chrome** | For HTML Video rendering | auto-detected, or set the path in Settings |
| **VieNeu-TTS** (recommended) | Natural on-device Vietnamese voice | one click in Settings, or `./scripts/setup-vieneu.sh` |
| **faster-whisper** | Offline transcription with word timings | one click in Settings, or `./scripts/setup-whisper.sh` |
| **Pexels key** (optional) | Keyword stock imagery | free at [pexels.com/api](https://www.pexels.com/api/) |
| Go 1.22+ | Only to build from source | [go.dev/dl](https://go.dev/dl/) |

---

## Build your own packages

```bash
./scripts/build-release.sh 2.7.0
```

Cross-compiles and packages into `dist/`: `.dmg` for macOS (Intel + Apple Silicon, containing a `.app` that opens the browser for you), `.zip` for Windows, `.tar.gz` for Linux on both architectures. `.dmg` creation needs macOS; the script skips that step elsewhere.

---

## REST API

The backend is plain REST — automate anything without the UI. Full reference in [`docs/contracts.md`](docs/contracts.md).

| Endpoint | Purpose |
|---|---|
| `GET /api/state` | System status, tool availability, host stats |
| `GET /api/events/stream` | SSE: jobs, AI session events, logs, projects |
| `GET/POST /api/projects` · `GET/PUT/DELETE /api/projects/{id}` | Project CRUD |
| `POST /api/projects/{id}/sessions` · `/message` · `/stop` | AI editing session: start, follow-up, stop |
| `POST /api/studio/broll` | Assemble b-roll clips to a voice track |
| `POST /api/studio/highlight` | Long video → short clip via AI segment scoring |
| `POST /api/studio/normalize` | Reframe and re-level for a platform |
| `GET /api/studio/templates` · `/platforms` · `/moods` · `/voice-langs` | Preset lookups |
| `POST /api/tools/htmlvideo/plan` · `POST /api/tools/htmlvideo` | HTML Video: scene split → MP4 |

---

## Interface language

The UI has a **VI / EN** toggle in the top bar. Be aware of what that covers today:

| Layer | State |
|---|---|
| Every interface string — navigation, page copy, form hints, buttons, help text, error messages | ✅ English |
| Template names, recipes, platform presets, music moods (server-side) | ✅ English |
| Voice names, project names, your own content | — left as-is (they're proper nouns and your data) |

**All 1,541 interface strings are translated**, plus the server-side strings that surface in the UI — 1,683 entries in total. Coverage was verified by instrumenting the live app and walking every one of the 21 pages, not by counting source lines. Anything missing falls back to Vietnamese rather than breaking.

**This is the single best place to contribute.** Translations live in one file, [`web/static/js/i18n.en.js`](web/static/js/i18n.en.js), keyed by the Vietnamese source string:

```js
'Xem công thức': 'Show recipe',
```

Run the app, find an untranslated string, add a line. No build step, no key registry, no touching 24 page files.

**Adding a whole new language** is one copy of that file with the right-hand side translated, plus one `<script>` tag in `index.html`. If you do that for your language, please open a PR — it's the most useful thing anyone can contribute here.

---

## Contributing

Issues and pull requests are welcome — especially:

- **Interface translation** (see above) — the highest-impact, lowest-friction contribution right now.
- **Voices and languages.** The TTS layer is pluggable; adding a good on-device engine for your language would help a lot of people.
- **Platform presets.** If a platform's framing, duration cap or loudness target is wrong or missing, that's a one-line fix with real impact.
- **Bug reports with a repro.** A command line and the JSON output from the CLI is the fastest possible report.

Code is commented in Vietnamese, explaining *why* rather than *what*. Don't let that stop you — the tests are the executable spec, and they're thorough.

---

## Credits

Biz Studio learns from excellent open-source work in the AI video space — see the [credits section](README.md#cảm-hứng--ghi-nhận) for the full table with what was taken from where.

Standing on: [FFmpeg](https://ffmpeg.org) · [chromedp](https://github.com/chromedp/chromedp) · [yt-dlp](https://github.com/yt-dlp/yt-dlp) · [Claude Code](https://claude.com/claude-code) · [Gemini API](https://ai.google.dev) · [VieNeu-TTS](https://github.com/pnnbao97/VieNeu-TTS) · [faster-whisper](https://github.com/SYSTRAN/faster-whisper) · [Pexels](https://www.pexels.com/api/)

---

<div align="center">

Made with ❤️ by **Hoai Nguyen** · [MIT License](LICENSE)

⭐ If this saves you time, a star helps other people find it.

</div>
