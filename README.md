# Biz Studio

[Tiếng Việt](README.md) · [English](README.en.md)

Biz Studio là phần mềm sản xuất video chạy trên máy tính. Bạn đưa nội dung và tài nguyên vào, phần mềm hỗ trợ từ chuẩn bị kịch bản, giọng đọc, biên tập nhiều lớp đến kiểm tra và đóng gói video để đăng.

<img src="assets/demo.gif" width="100%" alt="Giao diện Biz Studio">

## Phục vụ công việc gì?

- Biến bài viết hoặc bản thảo thành video có kịch bản, giọng đọc, hình ảnh và phụ đề.
- Dùng Claude CLI để tự phân tích tài nguyên và thực hiện một phiên biên tập video theo yêu cầu tiếng Việt.
- Dựng video bằng HTML/CSS cho nội dung giới thiệu sản phẩm, số liệu, bài viết hoặc kho mã.
- Cắt khoảng lặng, bóc băng, dịch phụ đề, lồng tiếng và tạo giọng đọc tiếng Việt ngay trên máy.
- Biên tập một video nền với nhiều lớp lời đọc, nhạc, tiếng động và phụ đề; nghe thử trước khi dựng.
- Kiểm tra âm lượng, khung đen, hình đứng và khoảng lặng trước khi xuất bản.
- Chuẩn hóa video cho TikTok, Reels, Shorts, YouTube ngang hoặc khung vuông.
- Đóng gói video, thumbnail, phụ đề và nội dung đăng thành một bộ hoàn chỉnh.

## Cách sử dụng

1. Tạo dự án và đưa video, ảnh, âm thanh vào.
2. Chọn một luồng phù hợp: khuôn làm sẵn, Text → Video, HTML Video hoặc phiên AI.
3. Mở **Biên tập video** để xem trước, cắt khoảng lặng, sắp lớp âm thanh và chỉnh phụ đề.
4. Chạy QC, tạo thumbnail rồi xuất gói đăng.

Điện thoại chỉ dùng để gửi tài nguyên: mở dự án trên máy tính, quét QR bằng điện thoại cùng Wi-Fi rồi chọn file cần gửi. Điện thoại không truy cập được cấu hình, bộ cài hay tác vụ quản trị của ứng dụng.

## Cài đặt

Tải gói phù hợp tại trang [GitHub Releases](../../releases):

| Hệ điều hành | Gói cài |
|---|---|
| Windows 10/11 64-bit | `BizStudio-windows-amd64.zip` |
| macOS Apple Silicon | `BizStudio-macos-arm64.dmg` |
| macOS Intel | `BizStudio-macos-amd64.dmg` |
| Linux 64-bit | `BizStudio-linux-amd64.tar.gz` |
| Linux ARM64 | `BizStudio-linux-arm64.tar.gz` |

### Windows

Giải nén ZIP rồi bấm đúp **Biz Studio.exe**. Ở lần mở đầu, Biz Studio kiểm tra App Installer/WinGet, xin quyền Windows khi cần để điện thoại gửi file qua mạng Private và cài các thành phần còn thiếu. Rule Firewall chỉ áp dụng cho đúng file **Biz Studio.exe** trên mạng Private/Domain.

Nếu máy chưa có WinGet, bấm **Cài App Installer / WinGet**, cài từ Microsoft rồi mở lại Biz Studio. Không cần tắt Windows Firewall; nếu Wi-Fi đang để Public, đổi thuộc tính mạng sang Private trước khi quét QR.

Đăng nhập Claude là bước riêng: mở PowerShell và chạy `claude auth login`. Biz Studio không nhận hoặc lưu thông tin đăng nhập Claude.

### macOS

Mở DMG, kéo **Biz Studio.app** vào Applications. Nếu macOS chặn lần mở đầu, nhấp phải ứng dụng và chọn **Open**.

### Linux

Giải nén gói rồi chạy `./bizstudio`. File `bizstudio.desktop` đi kèm để thêm ứng dụng vào menu hệ thống.

## Tự cập nhật

Biz Studio tự kiểm tra GitHub Release khi mở. Khi có bản mới, ứng dụng hiện một thanh thông báo:

- Chọn đúng gói theo Windows, macOS hoặc Linux và đúng kiến trúc máy.
- Kiểm tra SHA-256 trước khi cài; sai checksum thì hủy cập nhật.
- Giữ nguyên dự án và dữ liệu người dùng.
- Bấm **Cập nhật ngay** để tải, cài và khởi động lại.

Bản ổn định chỉ nhận bản ổn định. Bản RC có thể nhận RC mới hơn để phục vụ kiểm thử trước phát hành.

## Claude CLI

Biz Studio gọi `claude` mà không gắn tên model. Model được chính Claude CLI và tài khoản của người dùng lựa chọn, tránh lỗi khi Anthropic đổi hoặc ngừng một model cụ thể.

Phiên AI chỉ làm việc trong thư mục dự án, dùng nhóm lệnh media/file đã cho phép và không nhận credential cloud từ môi trường của ứng dụng.

## Nhóm tính năng

| Nhóm | Nội dung |
|---|---|
| Bắt đầu | Tổng quan, Dự án, Xưởng làm sẵn |
| Tạo nội dung | Ý tưởng & Hàng đợi, Text → Video, Bài viết → Video, HTML Video, Vox-Director |
| Biên tập & âm thanh | Biên tập video, Tải Video, OCR/ASR, Dịch thuật, TTS/Giọng đọc, Diện mạo |
| Thư viện | Style Kit, Nhân vật |
| Hệ thống | Cấu hình & API, Nhật ký |

## Dữ liệu và riêng tư

Biz Studio là ứng dụng local-first. Dự án nằm trong thư mục dữ liệu trên máy; video không tự được tải lên máy chủ của Biz Studio. Chỉ nội dung bạn chủ động gửi tới Claude, Gemini, API trực tiếp hoặc dịch vụ media bên ngoài mới rời khỏi máy.

Cấu trúc chính:

```text
data/
├── db.json
├── projects/<id>/
│   ├── assets/
│   ├── outputs/
│   ├── publish/
│   └── timeline.json
├── downloads/
├── text2video/
├── vieneu/
└── whisper/
```

## Khắc phục nhanh

| Hiện tượng | Cách xử lý |
|---|---|
| Claude chưa chạy | Chạy `claude --version`, sau đó `claude auth login` |
| Tải video lỗi 403 | Vào Cấu hình & API → Công cụ trên máy → cập nhật yt-dlp |
| Cổng 6868 đang bận | Ứng dụng tự chọn cổng trống; nếu cần xem lỗi, chạy `bizstudio.exe -window=false` |
| QR không mở trên điện thoại | Hai thiết bị phải cùng Wi-Fi; trên Windows hãy để mạng là Private rồi mở lại Biz Studio để kiểm tra Firewall |
| VieNeu/Whisper chưa sẵn sàng | Bấm Cài tại Cấu hình & API → Công cụ trên máy |
| Timeline dựng khác lúc nghe thử | Bấm Lưu; nút Dựng video cũng tự lưu trước khi chạy |

## Chạy từ mã nguồn

Yêu cầu Go 1.22 trở lên:

```bash
git clone https://github.com/nguyenduchoai/biz-studio.git
cd biz-studio
go run ./cmd/bizstudio
```

Mặc định ứng dụng mở tại `http://127.0.0.1:6868`. Dùng `-window=false` để chỉ chạy máy chủ.

## Build và phát hành

```bash
go test ./...
./scripts/build-release.sh 2.14.0
```

Release được tạo tự động khi đẩy tag dạng `vX.Y.Z` hoặc `vX.Y.Z-rc.N`. Pipeline kiểm thử trên Windows, đóng gói Windows/macOS/Linux và phát hành kèm `SHA256SUMS.txt`.

Hợp đồng API và quy ước phát triển nằm tại [docs/contracts.md](docs/contracts.md).

---

Made with ❤️ by **Hoai Nguyen** · [MIT License](LICENSE)
