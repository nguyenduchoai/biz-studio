/* ============================================================
   Biz Studio — lớp đa ngôn ngữ.

   Giao diện có ~1500 chuỗi rải khắp 24 trang. Sửa từng chỗ gọi là một bản vá
   khổng lồ và lần sau thêm trang mới lại quên. Thay vào đó chặn ở ĐÚNG MỘT
   CHỖ: mọi chữ lên màn hình đều đi qua appendChild() trong ui.js, và vài thuộc
   tính chữ (placeholder, title, alt) đi qua h().

   Tra cứu theo CHÍNH CHUỖI TIẾNG VIỆT, không theo mã khoá. Lý do: không phải
   đụng vào 24 file để rắc khoá, và chuỗi nào chưa dịch thì tự rơi về tiếng
   Việt — thiếu bản dịch chỉ là thiếu, không phải vỡ giao diện hay hiện ra
   "missing.key.42".

   Đánh đổi phải biết: sửa chữ tiếng Việt trong mã mà quên sửa từ điển thì chỗ
   đó lặng lẽ quay về tiếng Việt. Chấp nhận được — rơi về tiếng Việt vẫn đọc
   được, còn hơn màn hình trắng.

   Nạp TRƯỚC ui.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var KEY = 'biz-lang';
  var DICT = {};          // chuỗi tiếng Việt → chuỗi tiếng Anh
  var lang = 'vi';

  try { lang = localStorage.getItem(KEY) || 'vi'; } catch (e) { /* chế độ riêng tư */ }

  /* t dịch một chuỗi. Không phải chuỗi, đang ở tiếng Việt, hoặc chưa có bản
     dịch → trả nguyên xi. */
  function t(s) {
    if (lang === 'vi' || typeof s !== 'string' || !s) return s;
    var hit = DICT[s];
    if (hit) return hit;
    // Chuỗi hay bị bọc khoảng trắng ở hai đầu khi ghép chuỗi động — thử lại
    // sau khi cắt, rồi trả về đúng dạng cũ kèm khoảng trắng.
    var trimmed = s.trim();
    if (trimmed !== s && DICT[trimmed]) {
      return s.replace(trimmed, DICT[trimmed]);
    }
    // Tiêu đề thẻ hay dán emoji liền chữ ("📁Dự án gần đây"). Tách emoji đầu
    // chuỗi ra tra lại rồi gắn về, thay vì phải nhân đôi mọi khoá trong từ điển
    // — có emoji một bản, không emoji một bản.
    var m = /^([^\p{L}\p{N}]+)(.*)$/u.exec(trimmed);
    if (m && m[2] && DICT[m[2]]) {
      return s.replace(trimmed, m[1] + DICT[m[2]]);
    }
    return s;
  }

  function get() { return lang; }

  function set(v) {
    lang = (v === 'en') ? 'en' : 'vi';
    try { localStorage.setItem(KEY, lang); } catch (e) { /* chế độ riêng tư */ }
    // Vẽ lại toàn bộ là cách chắc chắn nhất: giao diện dựng bằng hàm thuần,
    // không có ràng buộc dữ liệu hai chiều để mà cập nhật tại chỗ.
    location.reload();
  }

  /* add nạp thêm từ điển. Tách khỏi lõi để file dịch để riêng, ai muốn thêm
     ngôn ngữ chỉ cần thêm một file. */
  function add(map) {
    Object.keys(map).forEach(function (k) { DICT[k] = map[k]; });
  }

  /* Số chuỗi đã dịch — dùng cho thông báo độ phủ, không giấu chuyện còn thiếu. */
  function size() { return Object.keys(DICT).length; }

  window.I18N = { t: t, get: get, set: set, add: add, size: size };
})();
