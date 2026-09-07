<!-- release: v2.15.0-rc.2 -->

## Biên tập AI và xuất bản đáng tin cậy hơn

Bản RC để kiểm thử trên Windows và macOS trước khi phát hành ổn định.

### Cải thiện

- Thêm Claude Agent SDK tùy chọn cho phiên biên tập AI. Claude CLI vẫn là mặc định, không gắn cố định model.
- Chỉ báo hoàn tất khi có video mới vượt qua kiểm tra; giữ kết quả trước nếu lượt dựng mới thất bại.
- Xếp các tác vụ xử lý cùng dự án chạy lần lượt, giảm xung đột khi biên tập và tạo phụ đề.
- Kiểm tra lại chất lượng video trước khi đóng gói xuất bản; không dùng báo cáo cũ cho video đã thay đổi.
- Dừng phiên AI và chờ tiến trình thoát trước khi đóng hoặc cập nhật ứng dụng.

Agent SDK cần **Anthropic API key riêng**, tính phí API và không dùng hạn mức thuê bao Claude. Chỉ bật khi có nhu cầu; mặc định không tự cài hoặc chuyển sang SDK.

### Chọn bản tải về

- **Windows 10/11:** `BizStudio-windows-amd64.zip`
- **Mac Apple Silicon:** `BizStudio-macos-arm64.dmg`
- **Mac Intel:** `BizStudio-macos-amd64.dmg`
- **Linux:** chọn gói `amd64` hoặc `arm64` phù hợp với máy

Sau khi cập nhật, mở **Cấu hình & API → Thiết lập đầy đủ & nhận file QR** để kiểm tra lại máy. Đăng nhập Claude CLI vẫn thực hiện riêng bằng `claude auth login`.

Bản ổn định không tự chuyển sang RC. Nên sao lưu dự án quan trọng trước khi thử bản này.
