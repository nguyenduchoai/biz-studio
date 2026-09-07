---
date: 2026-09-07
session: agent-runtime-stability
---

# Journal: Agent Runtime Stability

## Context

Result events marked sessions/projects done before process exit. `findOutput` accepted missing, stale, or invalid outputs; per-session process tracking neither serialized project writers nor protected immediate Stop.

## What Happened

Completion now requires successful process exit, a successful result without `is_error`, and `meta.status: done` referencing a regular, nonempty `outputs/*.mp4`. Symlinks/traversal are rejected; SHA-256 must differ from every existing project output fingerprint; `media.ValidateVideo` must fully decode the video.

Shared AI/job project leases cover Stop, process cleanup, and verification. Cleanup manages the owned Unix process group or Windows Job Object. Owned pipes with concurrent Wait, descendant termination, then result draining fix inherited stdout/stderr hangs.

Windows starts suspended, assigns the job, then resumes through Thread32/OpenThread/ResumeThread; ambiguous threads or failures terminate the process. Suspension prevents execution before containment ([Microsoft process creation flags](https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags)). Windows tests cover a forced 150 ms assignment window and immediate Stop against delayed child writes.

Two false-done regressions failed before fixes. Latest local macOS agent/agentsdk/jobs/projectwork race checks passed (20.366/3.779/6.080/2.688s). Agent vet and Windows/Darwin amd64 builds passed. Windows test-package cross-compilation passed; native execution remains unverified. Local FFmpeg tests accepted valid video and rejected audio-only/corrupt files. Independent review closed inherited-pipe and assignment-gap findings; focused race checks passed (3.969s).

Final integrated verification: `go test -race -count=1 ./...`, `go vet ./...`, eight browser regressions and three offline SDK bridge tests passed. Built app installed the real SDK in temporary data and passed import/bundled-CLI checks without API calls. Native ARM64 and Intel-under-Rosetta executables both passed port collision, second launch, QR audio/video upload, timeline render/playback and restart persistence. Physical Intel Mac, native Windows and paid-provider runs remain separate gates.

## Reflection

Completion requires both a terminated writer and validated new media. Passing local checks does not establish native Windows behavior, paid provider E2E, or release readiness.

## Decisions Made

- Preserve prior valid `OutputFile` on failed reruns and session ID/history for manual resume; no automatic retries.
- Capture immutable invocation settings. Keep default CLI model-neutral; optional official SDK uses a separate API key/budget, independent of subscription quota.
- Protect isolated SDK runtime with a read lease during sessions to prevent reinstall.
- Bound execution to two hours, snapshotting to ten minutes, and validation to ten minutes.

## Next Steps

Run native Windows/macOS CI and authorized paid SDK E2E. Native Windows tests, provider E2E, and full release remain open; no tag or shipped claim.
