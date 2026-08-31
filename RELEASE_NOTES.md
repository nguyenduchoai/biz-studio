<!-- release: v2.14.1 -->

## Biz Studio ổn định hơn trên Windows

Bản này sửa lỗi bộ cài Full dừng sau khi xác nhận Windows Firewall.

### Cải thiện

- Biz Studio nhận đúng kết quả Firewall trên nhiều cấu hình Windows 10/11.
- Bộ cài tiếp tục cài các công cụ còn thiếu sau khi xác nhận UAC.
- Không cần cài lại Python hoặc tắt Windows Firewall.
- Kết nối QR vẫn chỉ hoạt động trên mạng Private/Domain và đúng ứng dụng Biz Studio.

### Chọn bản tải về

- **Windows 10/11:** `BizStudio-windows-amd64.zip`
- **Mac Apple Silicon:** `BizStudio-macos-arm64.dmg`
- **Mac Intel:** `BizStudio-macos-amd64.dmg`
- **Linux:** chọn gói `amd64` hoặc `arm64` phù hợp với máy

Người đang dùng `v2.14.0` nên cập nhật lên bản này trước khi chạy lại bộ cài Full.
