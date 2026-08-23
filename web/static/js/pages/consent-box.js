/* ============================================================
   Biz Studio — khối xác nhận quyền trước khi nhân bản mặt/giọng người thật.

   Dùng chung cho trang Clone giọng và trang Avatar để lời hỏi giống hệt nhau ở
   cả hai chỗ. Hỏi hai kiểu khác nhau cho cùng một việc thì người dùng không
   biết mình đang cam kết cái gì.

   Đây chỉ là lớp giao diện. Backend chặn độc lập (internal/consent) — gọi API
   bằng curl vẫn phải đi qua cổng đó.

   Nạp sau ui.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var TEXTS = {
    voice: {
      title: '🔒 Xác nhận quyền nhân bản giọng',
      what: 'giọng nói',
      rights: 'Tôi có quyền sử dụng clip giọng này',
      adult: 'Người nói trong clip đủ 18 tuổi',
      permitted: 'Người đó đồng ý cho nhân bản giọng của mình',
      subject: 'Giọng của ai (để sau còn tra lại)'
    },
    face: {
      title: '🔒 Xác nhận quyền dùng khuôn mặt',
      what: 'khuôn mặt',
      rights: 'Tôi có quyền sử dụng ảnh này',
      adult: 'Người trong ảnh đủ 18 tuổi',
      permitted: 'Người đó đồng ý cho ảnh của mình nói theo lời đọc',
      subject: 'Ảnh của ai (để sau còn tra lại)'
    }
  };

  function injectStyles() {
    if (document.getElementById('consent-style')) return;
    document.head.appendChild(h('style', { id: 'consent-style' },
      '.consent-box{border:1px solid var(--amber,#e0a800);border-radius:8px;padding:12px;' +
      'background:rgba(224,168,0,.06);margin-top:12px}' +
      '.consent-box .consent-title{font-weight:700;font-size:13px;margin-bottom:4px}' +
      '.consent-box .consent-why{font-size:12px;color:var(--muted);line-height:1.55;margin-bottom:8px}' +
      '.consent-box label.toggle{padding:4px 0}'));
  }

  // make dựng khối xác nhận. kind = 'voice' | 'face'.
  // Trả { el, values(), missing() } — missing() trả danh sách ô còn thiếu để
  // nơi gọi báo cho đúng chỗ thay vì chỉ nói "chưa xác nhận".
  function make(kind) {
    injectStyles();
    var t = TEXTS[kind] || TEXTS.voice;
    var v = { rights: false, adult: false, permitted: false, subject: '' };

    var subjectInput = UI.input({
      placeholder: 'vd: Chị Lan — đã ký giấy đồng ý',
      oninput: function (e) { v.subject = e.target.value; }
    });

    var el = h('div', { class: 'consent-box' },
      h('div', { class: 'consent-title' }, t.title),
      h('div', { class: 'consent-why' },
        'Nhân bản ' + t.what + ' của người khác khi chưa được phép là việc bạn phải tự chịu trách nhiệm. ' +
        'Biz Studio không kiểm tra hộ được — chỉ bạn mới biết — nên bắt buộc hỏi và ghi lại câu trả lời vào nhật ký.'),
      UI.toggle(t.rights, '', false, function (c) { v.rights = c; }),
      UI.toggle(t.adult, '', false, function (c) { v.adult = c; }),
      UI.toggle(t.permitted, '', false, function (c) { v.permitted = c; }),
      h('div', { class: 'mt-8' }, UI.field(t.subject, subjectInput)));

    function missing() {
      var out = [];
      if (!v.rights) out.push(t.rights);
      if (!v.adult) out.push(t.adult);
      if (!v.permitted) out.push(t.permitted);
      return out;
    }

    return {
      el: el,
      values: function () { return { rights: v.rights, adult: v.adult, permitted: v.permitted, subject: v.subject }; },
      missing: missing,
      // check báo lỗi qua toast và trả false nếu còn thiếu.
      check: function () {
        var m = missing();
        if (!m.length) return true;
        UI.toast('Còn thiếu xác nhận: ' + m.join('; '), 'error');
        return false;
      }
    };
  }

  window.ConsentBox = { make: make };
})();
