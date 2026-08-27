# Biz Studio

[Tiếng Việt](README.md) · [English](README.en.md)

Biz Studio is a local desktop video-production app. Bring in your content and media, then move from script and voice-over to layered editing, quality checks, and a publish-ready package.

<img src="assets/demo.gif" width="100%" alt="Biz Studio interface">

## What it helps you do

- Turn articles or drafts into videos with a script, voice-over, visuals, and subtitles.
- Use Claude CLI to analyze project media and execute a video-editing brief.
- Render HTML/CSS videos for products, data, articles, and code repositories.
- Remove silence, transcribe, translate subtitles, dub video, and create Vietnamese speech locally.
- Edit one video track with layered narration, music, sound effects, and subtitles.
- Check loudness, black frames, frozen frames, and silence before publishing.
- Normalize videos for TikTok, Reels, Shorts, landscape YouTube, and square feeds.
- Package the final video, thumbnail, subtitles, and post copy together.

## Typical workflow

1. Create a project and add video, images, and audio.
2. Start from a preset, Text → Video, HTML Video, or an AI editing session.
3. Open **Video Editor** to preview, remove silence, arrange audio layers, and edit subtitles.
4. Run QC, create a thumbnail, and export the publish package.

The phone experience is upload-only: scan a project QR code while both devices are on the same Wi-Fi, then send media to the desktop. Mobile devices cannot access settings, installers, or administrative jobs.

## Install

Download the appropriate package from [GitHub Releases](../../releases):

| Platform | Package |
|---|---|
| Windows 10/11 64-bit | `BizStudio-windows-amd64.zip` |
| macOS Apple Silicon | `BizStudio-macos-arm64.dmg` |
| macOS Intel | `BizStudio-macos-amd64.dmg` |
| Linux 64-bit | `BizStudio-linux-amd64.tar.gz` |
| Linux ARM64 | `BizStudio-linux-arm64.tar.gz` |

On Windows, unzip the package and open **Biz Studio.exe**. The first-run wizard checks App Installer/WinGet, requests UAC approval when needed so phones can upload on Private networks, and installs missing components. The firewall rule is limited to the exact **Biz Studio.exe** path and Private/Domain profiles.

If WinGet is unavailable, use **Install App Installer / WinGet**, install it from Microsoft, then reopen Biz Studio. Keep Windows Firewall enabled and set the current Wi-Fi network to Private before scanning a QR code.

Claude sign-in remains a separate user-controlled step: run `claude auth login`. Biz Studio never receives or stores Claude credentials.

On macOS, drag **Biz Studio.app** from the DMG into Applications. On Linux, extract the archive and run `./bizstudio`.

## Automatic updates

Biz Studio checks GitHub Releases at startup. When a newer version is available, it downloads the correct package for the current OS and architecture, verifies its SHA-256 checksum, preserves user data, installs the update, and restarts the app.

Stable builds stay on the stable channel. Release candidates can receive newer release candidates.

## Claude CLI model selection

Biz Studio invokes `claude` without pinning a model name. Claude CLI and the user's account choose the current default model, preventing breakage when model identifiers change.

## Feature groups

| Group | Features |
|---|---|
| Get started | Overview, Projects, Preset Workshop |
| Create | Ideas & Queue, Text → Video, Article → Video, HTML Video, Vox-Director |
| Edit & audio | Video Editor, Downloader, OCR/ASR, Translation, TTS/Voices, Look |
| Library | Style Kit, Characters |
| System | Settings & API, Logs |

## Privacy and data

Biz Studio is local-first. Projects stay in the app's data directory. Media leaves the computer only when the user explicitly invokes Claude, Gemini, a direct API, or an external media provider.

## Run from source

Go 1.22 or newer is required:

```bash
git clone https://github.com/nguyenduchoai/biz-studio.git
cd biz-studio
go run ./cmd/bizstudio
```

## Build and release

```bash
go test ./...
./scripts/build-release.sh 2.14.0
```

Pushing a `vX.Y.Z` or `vX.Y.Z-rc.N` tag triggers Windows tests, packages Windows/macOS/Linux, and publishes a GitHub Release with `SHA256SUMS.txt`.

API and contributor contracts are documented in [docs/contracts.md](docs/contracts.md).

---

Made with ❤️ by **Hoai Nguyen** · [MIT License](LICENSE)
