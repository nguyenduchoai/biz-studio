# Phiên AI và kiểm tra đầu ra

## Kết nối

- Mặc định `claudeBackend` rỗng hoặc `cli`: Claude CLI, không truyền model.
- `sdk`: official Python `claude-agent-sdk==0.2.152`, venv riêng `agent-sdk/venv`; không cần thêm Node.js.
- SDK chỉ dùng `anthropicApiKey` được cấu hình chủ động. Không tự chuyển sang tài khoản Claude khi thiếu key hoặc gặp lỗi.
- `claudeSdkBudgetUsd`: 0 dùng mặc định 2 USD mỗi lần Start/Resume; giá trị cấu hình hợp lệ 0.1–100 USD. SDK giới hạn 24 lượt AI. Ngân sách SDK không thay thế hạn mức chi tiêu phía nhà cung cấp.
- Backend, key và ngân sách được chụp một lần khi bắt đầu; thay cấu hình không đổi lượt đang chạy. Phiên cũ không được resume bằng backend khác.

## An toàn runtime

- SDK nhận thông tin yêu cầu qua stdin, không truyền key trong command line hay ghi key vào script. API settings trả key đã che; lưu trên máy theo cơ chế bảo vệ dữ liệu của store, không phải kho mã hóa riêng.
- Không đưa cloud credentials môi trường của app sang tiến trình AI. SDK chỉ nhận key đã chọn, cấu hình phiên riêng; tắt nạp settings của user/project và dùng safe-mode, giới hạn tool như CLI.
- Không ghi stderr thô của SDK vào lịch sử; bridge chỉ phát lỗi đã làm sạch. Chẩn đoán runtime chỉ import/thử `--version`, không gọi AI hay chứng minh API key còn hạn mức.
- Khóa đọc/ghi runtime ở cả process và hệ điều hành: có phiên SDK thì không cài đè venv, đang cài thì không mở phiên. CLI setup ở process khác cũng phải lấy khóa.
- Toàn bộ lượt có deadline 2 giờ; nút Dừng hủy nhóm tiến trình. Không tự tiếp tục phiên trả phí sau lỗi/khởi động lại.

## Điều kiện hoàn tất

1. Tiến trình kết thúc thành công, có result thành công, không bị hủy.
2. `meta.json` hợp lệ, `status=done`, output nằm trong `outputs/` của dự án, là MP4 thường, không rỗng và không qua symlink.
3. Nội dung SHA-256 khác các video trước lượt chạy; đổi tên/chạm mtime file cũ không đủ.
4. Có stream video thật, kích thước và thời lượng hợp lệ; giải mã toàn bộ video/audio thành công. File không được thay đổi trong khi kiểm tra.

Không đạt thì phiên/dự án báo lỗi, không thay `OutputFile` bằng kết quả chưa xác minh. Lịch sử/Claude session ID còn để người dùng tiếp tục. Mỗi lần tiếp tục kiểm tra lại đầu ra cũ trước khi chạy.

## Hàng đợi và xuất bản

- Cùng dự án: một tác vụ/phiên AI xử lý tại một thời điểm. Dự án khác không bị mắc kẹt sau một dự án đang bận. Giữ giới hạn tổng worker và hàng đợi hữu hạn. Khóa áp dụng cho dự án chính của tác vụ; chưa bao gồm thao tác CRUD đồng bộ, tài nguyên phụ thuộc dự án khác hoặc thư mục tải xuống tùy chỉnh trỏ vào dự án.
- Final/timeline dựng vào file tạm riêng, kiểm tra xong mới thay bản trước.
- Publish chạy QC mới trước metadata/provider hoặc ghi đè gói. Báo cáo gắn SHA-256 với video được đóng gói; không tin `qc.json` cũ.
- Lỗi file/giải mã/phân tích QC chặn xuất bản. Khung đen, đứng hình, im lặng và âm lượng bất thường là cảnh báo cần người dùng xem, vì có thể là nội dung chủ ý.
- Gói mới được dựng riêng rồi thay thế; lỗi thông thường có rollback. Đây không phải nhật ký giao dịch chống mất điện tuyệt đối.

## Kiểm chứng

Test tự động dùng tiến trình và dữ liệu giả, cùng video FFmpeg thật; không gọi AI trả phí. CI Windows/macOS cài thử SDK và kiểm tra bundled CLI nhưng không xác minh chất lượng dựng AI hoặc tài khoản thật.

Tham khảo: [Agent SDK](https://code.claude.com/docs/en/agent-sdk), [Python reference](https://code.claude.com/docs/en/agent-sdk/python). Theo hướng dẫn Anthropic, sản phẩm bên thứ ba dùng xác thực API key trừ khi đã được họ chấp thuận cách khác.
