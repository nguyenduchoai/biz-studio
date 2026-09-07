<!-- release: v2.14.2 -->

## Làm việc ổn định hơn trên Windows và macOS

Bản cập nhật giúp cài đặt, biên tập và mở lại Biz Studio đáng tin cậy hơn.

### Cải thiện

- Nhận đúng Python tương thích và thư viện đã cài, kể cả thư mục có dấu hoặc khoảng trắng.
- Mac tự nhận bộ FFmpeg đầy đủ để dựng phụ đề và chữ; hướng dẫn thiết lập ngay lần mở đầu.
- Cài thư viện vẫn tiếp tục khi chưa thiết lập được Firewall; có thể dùng trên máy tính và bật QR sau.
- Mở lại app ổn định hơn khi cổng đang bận hoặc cửa sổ trình duyệt đã có sẵn; giữ tác vụ và phiên AI đang chạy.
- Video dựng từ timeline xuất hiện ngay trong kết quả dự án để xem, kiểm tra và xuất bản.
- Chỉ cập nhật khi không có công việc đang chạy; giữ dữ liệu và khôi phục bản trước nếu bản mới không khởi động được.

### Chọn bản tải về

- **Windows 10/11:** `BizStudio-windows-amd64.zip`
- **Mac Apple Silicon:** `BizStudio-macos-arm64.dmg`
- **Mac Intel:** `BizStudio-macos-amd64.dmg`
- **Linux:** chọn gói `amd64` hoặc `arm64` phù hợp với máy

Sau khi cập nhật, mở **Cấu hình & API → Thiết lập đầy đủ & nhận file QR** để kiểm tra lại máy. Đăng nhập Claude vẫn thực hiện riêng bằng `claude auth login`.
