# Kiểm chứng Windows và macOS

Chạy trên hệ điều hành đích với Go theo `go.mod`, Node.js, FFmpeg/FFprobe:

```sh
go test -race -count=1 ./...
go vet ./...
npm ci --prefix tools/qa
cd tools/qa && npx playwright install chromium && cd ../..
npm test --prefix tools/qa
go build -trimpath -o artifacts/bizstudio ./cmd/bizstudio
node tools/qa/native-smoke.mjs artifacts/bizstudio
```

Windows: build `artifacts/Biz Studio.exe` với `-ldflags "-H windowsgui"`, truyền
đường dẫn đó vào smoke. Bài smoke gọi `-window=false`: kiểm chứng executable,
giao diện web, media và dữ liệu; vòng đời cửa sổ Chromium có kiểm thử riêng.

Gate phát hành chạy trên Windows Server runner, macOS Intel và macOS ARM64:

- Cổng bận, lần mở thứ hai, thư mục có dấu/khoảng trắng, kho dữ liệu chỉ có một writer.
- QR thật: giải mã, chọn và gửi video + audio từ giao diện mobile; chặn API quản trị.
- Dựng timeline thật, gắn output vào dự án, giải mã video bằng FFmpeg và phát trong Chromium.
- Khởi động lại giữ dự án, assets và timeline.
- Bộ cài Whisper thực tế; test lỗi model/pip, chọn Python và giữ thông báo lỗi Windows.
- Cập nhật: checksum, tải chậm/hủy, chờ app cũ thoát, rollback, xác minh version và dataID.

Smoke tạo dữ liệu riêng dưới thư mục tạm, không đọc dữ liệu hoặc đăng nhập của
người dùng. Ảnh giao diện nằm tại đường dẫn evidence được in cuối bài thử.

Kiểm thử còn cần trên máy sử dụng thực: UAC, chính sách Firewall/antivirus,
Gatekeeper, điện thoại qua Wi-Fi vật lý và đăng nhập Claude/API. Không đồng nhất
runner Windows Server với mọi cấu hình Windows 10/11; không coi việc mock lỗi
download là kiểm chứng model TTS/ASR trên mọi phần cứng.
