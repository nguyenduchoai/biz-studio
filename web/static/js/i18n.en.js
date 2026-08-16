/* ============================================================
   Biz Studio — English dictionary.

   Keys are the Vietnamese source strings. Anything missing falls back to
   Vietnamese, so a partial dictionary degrades gracefully instead of breaking.

   To add another language, copy this file, translate the right-hand side, and
   add one <script> line to index.html.
   ============================================================ */
(function () {
  'use strict';
  if (!window.I18N) return;

  window.I18N.add({
    // ---------- sidebar ----------
    'BẢNG ĐIỀU KHIỂN': 'DASHBOARD',
    'MODULE SÁNG TẠO': 'CREATIVE MODULES',
    'HỆ THỐNG': 'SYSTEM',
    'Tổng quan': 'Overview',
    'Xưởng làm sẵn': 'Preset Workshop',
    'Tải Video': 'Downloader',
    'OCR / ASR': 'OCR / ASR',
    'Dịch thuật': 'Translate',
    'TTS / Giọng đọc': 'TTS / Voices',
    'Bài viết → Video': 'Article → Video',
    'Vox-Director': 'Vox-Director',
    'HTML Video': 'HTML Video',
    'Text → Video': 'Text → Video',
    'Style Kit': 'Style Kit',
    'Nhân vật': 'Characters',
    'Ý tưởng & Hàng đợi': 'Ideas & Queue',
    'Diện mạo': 'Look',
    'Veo — Sinh video AI': 'Veo — AI Video',
    'Avatar nói': 'Talking Avatar',
    'Phim → Kể chuyện': 'Film → Narration',
    'Studio Editor': 'Studio Editor',
    'Dự án': 'Projects',
    'Cấu hình & API': 'Settings & API',
    'Nhật ký': 'Logs',
    'Video AI Studio': 'AI Video Studio',

    // ---------- page subtitles ----------
    'Toàn cảnh studio: dự án, tác vụ và công cụ': 'Studio at a glance: projects, jobs and tools',
    'Chọn khuôn theo lĩnh vực, chuẩn hoá cho từng nền tảng, nhạc nền theo tone':
      'Pick a template by niche, normalise per platform, background music by mood',
    'Render video từ HTML/CSS — prompt, bài viết hoặc repo GitHub thành MP4':
      'Render video from HTML/CSS — a prompt, an article or a GitHub repo into MP4',
    'Văn bản hoặc link bài viết → kịch bản đọc → giọng đọc → video. Phiên lưu lại, sửa tiếp bất cứ lúc nào.':
      'Text or article link → script → voice-over → video. Sessions are saved; edit any time.',
    'Chỉnh màu, tiếng động và font tiếng Việt dùng chung cho mọi video':
      'Colour grading, sound effects and a shared Vietnamese font for every video',
    'Ba chế độ: Dubbing nhanh (văn bản → giọng đọc), Dubbing chất lượng (lồng tiếng video theo phụ đề) và Clone voice (nhân bản giọng từ clip mẫu)':
      'Three modes: quick dubbing (text → voice), quality dubbing (video dubbed to subtitles), and voice cloning from a sample clip',
    'Toàn bộ dự án, sắp xếp theo lần cập nhật gần nhất.': 'All projects, most recently updated first.',
    'Giữ nhân vật giống nhau ở mọi cảnh — cả tên lẫn ngoại hình được chèn tự động vào prompt sinh ảnh':
      'Keep characters consistent across scenes — name and appearance are injected into image prompts automatically',

    // ---------- common actions ----------
    'Lưu': 'Save', 'Huỷ': 'Cancel', 'Hủy': 'Cancel', 'Xoá': 'Delete', 'Xóa': 'Delete',
    'Sửa': 'Edit', 'Mở': 'Open', 'Đóng': 'Close', 'Thêm': 'Add', 'Tạo': 'Create',
    'Chạy': 'Run', 'Dừng': 'Stop', 'Tải về': 'Download', 'Tải lên': 'Upload',
    'Xem thử': 'Preview', 'Áp dụng': 'Apply', 'Làm lại': 'Retry', 'Thử lại': 'Retry',
    'Chọn file…': 'Choose file…', 'Chọn file...': 'Choose file...',
    'Thu gọn': 'Collapse', 'Mở rộng': 'Expand', 'Quay lại': 'Back', 'Tiếp tục': 'Continue',
    'Sao chép': 'Copy', 'Đã chép': 'Copied', 'Nhân bản': 'Duplicate',
    '＋ Tạo dự án': '＋ New project', '＋ Phiên mới': '＋ New session',
    '＋ Thêm nhân vật': '＋ Add character', '＋ Thêm cảnh': '＋ Add scene',

    // ---------- statuses ----------
    'Hoàn thành': 'Done', 'Đang chạy': 'Running', 'Đang chờ': 'Queued',
    'Lỗi': 'Error', 'Nháp': 'Draft', 'Đã dừng': 'Stopped', 'Chưa chạy': 'Not started',
    'Đang tải…': 'Loading…', 'Đang tải...': 'Loading...', 'Đang xử lý…': 'Working…',
    'Chưa có dữ liệu': 'Nothing here yet', 'Không có kết quả': 'No results',

    // ---------- dashboard ----------
    'Tổng số dự án trong studio': 'Total projects in the studio',
    'Job nền đang xử lý': 'Background jobs in flight',
    'Công cụ': 'Tools',
    'Dự án gần đây': 'Recent projects',
    'Tác vụ đang chạy': 'Jobs running',
    'Tổng số dự án': 'Total projects',
    'Nguồn nội dung': 'Content source',
    'Danh sách cảnh': 'Scenes',
    'Cấu hình render': 'Render settings',
    'Phiên làm việc': 'Sessions',
    'Video / âm thanh cần xử lý': 'Video / audio to process',
    'Chỉnh màu — 14 kiểu': 'Colour grading — 14 presets',
    'Tiếng động — 10 hiệu ứng': 'Sound effects — 10 presets',
    'Nguồn nội dung ': 'Content source ',
    'Chọn giọng đọc': 'Choose a voice',
    'Nhân vật dùng để làm gì?': 'What are characters for?',
    'Tác vụ gần đây': 'Recent jobs',
    'Chưa có dự án nào': 'No projects yet',
    'Chưa có tác vụ nào': 'No jobs yet',

    // ---------- preset workshop ----------
    'Khuôn theo lĩnh vực': 'Templates by niche',
    'Rút clip ngắn': 'Long → short',
    'Ghép tư liệu': 'B-roll',
    'Nền tảng': 'Platforms',
    'Nhạc nền': 'Music',
    'Giọng theo ngôn ngữ': 'Voices by language',
    'Dùng khuôn này': 'Use this template',
    'Xem công thức': 'Show recipe',
    'Quảng cáo & bán hàng': 'Ads & selling',
    'Review & đánh giá': 'Reviews',
    'Kiến thức & giáo dục': 'Knowledge & education',
    'Kể chuyện': 'Storytelling',
    'Mạng xã hội': 'Social',
    'Doanh nghiệp': 'Business',
    'Giải trí': 'Entertainment',
    'Video nguồn': 'Source video',
    'Thời lượng clip (giây)': 'Clip length (seconds)',
    'Nền tảng đích': 'Target platform',
    'Ngưỡng điểm giữ lại': 'Keep-score threshold',
    'Clip nhắm vào ý gì (tuỳ chọn)': 'What should the clip be about (optional)',
    'Rút clip': 'Extract clip',
    'Thư mục clip tư liệu': 'B-roll folder',
    'File lời đọc': 'Voice-over file',
    'Khung hình': 'Aspect ratio',
    'Mỗi mẩu tối đa (giây)': 'Max piece length (seconds)',
    'Xáo thứ tự mẩu': 'Shuffle pieces',
    'Ghép tư liệu ': 'Assemble b-roll ',
    'Chuẩn hoá': 'Normalise',
    'Chọn nền tảng': 'Choose a platform',
    '▶ Nghe': '▶ Play', '⏸ Dừng': '⏸ Pause',

    // ---------- settings ----------
    'Kiểm tra kết nối': 'Test connections',
    'Dọn file tạm': 'Clear temp files',
    'Đã lưu': 'Saved',
    'Chưa cấu hình': 'Not configured',
    'Giao diện': 'Appearance',

    // ---------- status bar ----------
    'Trạng thái': 'Status', 'Trạng thái [STUDIO1]:': 'Status [STUDIO1]:',
    'Backend hoạt động': 'Backend online',
    'Mất kết nối backend': 'Backend offline',
    'Đang chạy': 'Running', 'tác vụ': 'jobs', 'Sẵn sàng': 'Ready',
    'Máy': 'Host', 'Không kết nối được': 'Cannot connect',

    // ---------- header buttons ----------
    'Đổi giao diện sáng / tối': 'Toggle light / dark',
    'Cấu hình & API': 'Settings & API',

    // ---------- common toasts ----------
    'Đã lưu thay đổi.': 'Changes saved.',
    'Đã xếp hàng.': 'Queued.',
    'Chưa chọn file.': 'No file selected.',
    'Không tải được': 'Could not load',
    'Không chạy được': 'Could not run'
  });
})();
