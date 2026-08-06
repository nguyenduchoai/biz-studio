#!/usr/bin/env python3
"""
longcat-worker — xưởng render đặt trên máy có GPU NVIDIA.

Vì sao cần: LongCat-Video-Avatar là model 13,6 tỉ tham số, bắt buộc CUDA —
không có bản cho Apple Silicon hay CPU. Trong khi Biz Studio phần lớn chạy trên
máy cá nhân. Worker này để máy cá nhân làm bàn điều khiển, máy GPU làm xưởng.

Cố ý CHỈ dùng thư viện chuẩn của Python: máy GPU đã phải cài cả đống thứ cho
model rồi, không nên bắt cài thêm web framework nữa.

Chạy:
    python3 longcat-worker.py \\
        --repo /duong/dan/LongCat-Video \\
        --checkpoint /duong/dan/weights/LongCat-Video-Avatar-1.5 \\
        --port 7070 --gpus 2 --int8

Giao thức (Biz Studio gọi vào):
    GET  /health          → tình trạng máy
    POST /generate        → {prompt, image_b64, image_ext, audio_b64, audio_ext}
                            trả {job_id}
    GET  /status/<id>     → {state, progress, detail, error}
    GET  /result/<id>     → file mp4
"""

import argparse
import base64
import json
import os
import re
import shutil
import subprocess
import tempfile
import threading
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

DEMO_SCRIPT = "run_demo_avatar_single_audio_to_video.py"
MAX_BODY = 512 * 1024 * 1024  # 512 MB — đủ cho ảnh + vài phút giọng dạng base64

CFG = {}
JOBS = {}
JOBS_LOCK = threading.Lock()
RUN_LOCK = threading.Lock()  # model chiếm trọn GPU: mỗi lúc chỉ chạy một việc


# ---------- tiện ích ----------

def gpu_name():
    try:
        out = subprocess.run(
            ["nvidia-smi", "--query-gpu=name", "--format=csv,noheader"],
            capture_output=True, text=True, timeout=10)
        names = [l.strip() for l in out.stdout.splitlines() if l.strip()]
        return " + ".join(names) if names else "không thấy GPU"
    except Exception:
        return "không chạy được nvidia-smi"


def safe_ext(ext, default):
    """Chỉ nhận đuôi file lành — phần mở rộng đi thẳng vào tên file trên đĩa."""
    e = (ext or "").lower().strip()
    return e if re.fullmatch(r"\.[a-z0-9]{1,5}", e or "") else default


def set_job(job_id, **kw):
    with JOBS_LOCK:
        JOBS.setdefault(job_id, {}).update(kw)


def get_job(job_id):
    with JOBS_LOCK:
        j = JOBS.get(job_id)
        return dict(j) if j else None


# ---------- chạy model ----------

def build_cmd(input_json, out_dir):
    args = ["torchrun", f"--nproc_per_node={CFG['gpus']}"]
    if CFG["gpus"] > 1:
        args.append(f"--context_parallel_size={CFG['gpus']}")
    args += [
        DEMO_SCRIPT,
        f"--checkpoint_dir={CFG['checkpoint']}",
        "--stage_1=at2v",
        f"--input_json={input_json}",
        "--use_distill",
        "--model_type", "avatar-v1.5",
        f"--output_dir={out_dir}",
    ]
    if CFG["int8"]:
        args.append("--use_int8")
    return args


def find_mp4(root):
    for dirpath, _, names in os.walk(root):
        for n in names:
            if n.lower().endswith(".mp4"):
                return os.path.join(dirpath, n)
    return None


def run_job(job_id, work, image_path, audio_path, prompt):
    """Chạy một việc. Nối đuôi nhau vì model chiếm trọn GPU."""
    try:
        set_job(job_id, state="queued", progress=2, detail="Đang chờ GPU rảnh…")
        with RUN_LOCK:
            set_job(job_id, state="running", progress=10, detail="Nạp model và dựng video…")
            out_dir = os.path.join(work, "out")
            os.makedirs(out_dir, exist_ok=True)
            input_json = os.path.join(work, "input.json")
            with open(input_json, "w", encoding="utf-8") as f:
                json.dump({
                    "prompt": prompt,
                    "cond_image": image_path,
                    "cond_audio": {"person1": audio_path},
                }, f, ensure_ascii=False, indent=2)

            proc = subprocess.run(build_cmd(input_json, out_dir), cwd=CFG["repo"],
                                  capture_output=True, text=True)
            if proc.returncode != 0:
                tail = (proc.stderr or proc.stdout or "")[-800:]
                set_job(job_id, state="error", error=f"LongCat lỗi: {tail}")
                return

            mp4 = find_mp4(out_dir)
            if not mp4:
                tail = (proc.stdout or "")[-500:]
                set_job(job_id, state="error",
                        error=f"chạy xong nhưng không thấy file mp4 — nhật ký: {tail}")
                return

            final = os.path.join(work, "result.mp4")
            shutil.move(mp4, final)
            set_job(job_id, state="done", progress=100, detail="Xong", result=final)
    except Exception as e:  # noqa: BLE001 — worker không được chết vì một việc hỏng
        set_job(job_id, state="error", error=str(e))


# ---------- HTTP ----------

class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):
        print("[worker] " + fmt % args, flush=True)

    def _json(self, code, obj):
        raw = json.dumps(obj, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):  # noqa: N802
        if self.path == "/health":
            ready = os.path.isdir(CFG["checkpoint"]) and os.path.isfile(
                os.path.join(CFG["repo"], DEMO_SCRIPT))
            detail = "sẵn sàng"
            if not os.path.isfile(os.path.join(CFG["repo"], DEMO_SCRIPT)):
                detail = f"không thấy {DEMO_SCRIPT} trong {CFG['repo']}"
            elif not os.path.isdir(CFG["checkpoint"]):
                detail = f"không thấy trọng số tại {CFG['checkpoint']}"
            self._json(200, {
                "ok": ready, "gpu": gpu_name(),
                "checkpoint": os.path.isdir(CFG["checkpoint"]),
                "busy": RUN_LOCK.locked(), "detail": detail,
            })
            return

        if self.path.startswith("/status/"):
            j = get_job(self.path[len("/status/"):])
            if not j:
                self._json(404, {"error": "không có việc này"})
                return
            self._json(200, {
                "state": j.get("state", "queued"),
                "progress": j.get("progress", 0),
                "detail": j.get("detail", ""),
                "error": j.get("error", ""),
            })
            return

        if self.path.startswith("/result/"):
            j = get_job(self.path[len("/result/"):])
            if not j or j.get("state") != "done" or not j.get("result"):
                self._json(404, {"error": "chưa có kết quả"})
                return
            path = j["result"]
            size = os.path.getsize(path)
            self.send_response(200)
            self.send_header("Content-Type", "video/mp4")
            self.send_header("Content-Length", str(size))
            self.end_headers()
            with open(path, "rb") as f:
                shutil.copyfileobj(f, self.wfile)
            return

        self._json(404, {"error": "không có đường dẫn này"})

    def do_POST(self):  # noqa: N802
        if self.path != "/generate":
            self._json(404, {"error": "không có đường dẫn này"})
            return
        try:
            n = int(self.headers.get("Content-Length") or 0)
        except ValueError:
            n = 0
        if n <= 0 or n > MAX_BODY:
            self._json(400, {"error": "thân yêu cầu rỗng hoặc quá lớn"})
            return
        try:
            body = json.loads(self.rfile.read(n))
        except Exception:  # noqa: BLE001
            self._json(400, {"error": "thân yêu cầu không phải JSON hợp lệ"})
            return

        img_b64, aud_b64 = body.get("image_b64"), body.get("audio_b64")
        if not img_b64 or not aud_b64:
            self._json(400, {"error": "thiếu ảnh hoặc giọng"})
            return

        job_id = uuid.uuid4().hex[:12]
        work = tempfile.mkdtemp(prefix=f"longcat-{job_id}-")
        try:
            img_p = os.path.join(work, "face" + safe_ext(body.get("image_ext"), ".png"))
            aud_p = os.path.join(work, "voice" + safe_ext(body.get("audio_ext"), ".wav"))
            with open(img_p, "wb") as f:
                f.write(base64.b64decode(img_b64))
            with open(aud_p, "wb") as f:
                f.write(base64.b64decode(aud_b64))
        except Exception as e:  # noqa: BLE001
            shutil.rmtree(work, ignore_errors=True)
            self._json(400, {"error": f"không giải mã được dữ liệu: {e}"})
            return

        prompt = (body.get("prompt") or "").strip() or "A person speaking to the camera."
        set_job(job_id, state="queued", progress=0, work=work)
        threading.Thread(target=run_job, args=(job_id, work, img_p, aud_p, prompt),
                         daemon=True).start()
        self._json(200, {"job_id": job_id})


def main():
    ap = argparse.ArgumentParser(description="Xưởng render LongCat-Video-Avatar")
    ap.add_argument("--repo", required=True, help="thư mục mã nguồn LongCat-Video")
    ap.add_argument("--checkpoint", required=True, help="thư mục trọng số Avatar-1.5")
    ap.add_argument("--port", type=int, default=7070)
    ap.add_argument("--host", default="0.0.0.0")
    ap.add_argument("--gpus", type=int, default=1)
    ap.add_argument("--int8", action="store_true", help="nén INT8 cho đỡ VRAM")
    a = ap.parse_args()

    CFG.update(repo=os.path.abspath(a.repo), checkpoint=os.path.abspath(a.checkpoint),
               gpus=max(1, a.gpus), int8=a.int8)

    print(f"[worker] mã nguồn : {CFG['repo']}")
    print(f"[worker] trọng số : {CFG['checkpoint']}")
    print(f"[worker] GPU      : {gpu_name()} ({CFG['gpus']} tiến trình)")
    print(f"[worker] lắng nghe: http://{a.host}:{a.port}")
    print("[worker] Điền địa chỉ này vào Cấu hình & API của Biz Studio.")
    ThreadingHTTPServer((a.host, a.port), Handler).serve_forever()


if __name__ == "__main__":
    main()
