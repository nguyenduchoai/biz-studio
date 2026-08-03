/* ============================================================
   Biz Studio — Storyboard cho trang "Text → Video"
   Mỗi đoạn kịch bản có ảnh riêng: sửa prompt tạo ảnh, tạo lại đúng
   một cảnh, hoặc tự tải ảnh thay thế.

   Export global T2VStoryboard. text2video.js gọi LÚC RENDER
   (không phải lúc load) nên thứ tự script trong index.html không quan trọng.

     T2VStoryboard.render(host, ctx) -> function dọn dẹp

   ctx do text2video.js truyền sang:
     { mountId, session, busy, inlineHost,
       startAll(), reload(), saveSegments(),
       fetchSession(), applyServer(s), watchJob(job, host, onDone) }
   ============================================================ */
(function () {
  'use strict';

  var SRC = {
    ai:     { label: 'AI',      cls: 'badge-blue' },
    stock:  { label: 'Stock',   cls: 'badge-green' },
    upload: { label: 'Tải lên', cls: 'badge-amber' }
  };

  // Trạng thái riêng của module — KHÔNG đăng ký listener toàn cục.
  var st = {
    mountId: '',   // đổi khi mở lại trang → xoá tiến trình cũ
    host: null,
    ctx: null,
    warnHost: null, // ô cảnh báo thiếu API key (vẽ lại khi /api/state về)
    pending: {},   // idx -> job đang chạy cho cảnh đó
    bust: {},      // idx -> timestamp phá cache ảnh
    chars: [],     // danh sách nhân vật (GET /api/characters), nạp 1 lần mỗi lần mở trang
    charsLoaded: false
  };

  // ---------- CSS nội bộ ----------

  function ensureCss() {
    if (document.getElementById('t2v-sb-style')) return;
    var el = document.createElement('style');
    el.id = 't2v-sb-style';
    el.textContent =
      '.t2v-sb-row{display:flex;gap:12px;align-items:flex-start;border:1px solid var(--border);' +
        'border-radius:12px;padding:10px;margin-bottom:10px;background:var(--card)}' +
      '.t2v-sb-thumb{position:relative;width:140px;height:92px;flex:none;border-radius:10px;overflow:hidden;' +
        'background:var(--bg);border:1px solid var(--border);display:flex;align-items:center;justify-content:center}' +
      '.t2v-sb-thumb img{width:100%;height:100%;object-fit:cover;display:block}' +
      '.t2v-sb-ph{display:flex;flex-direction:column;align-items:center;gap:2px;color:var(--muted);font-size:11px}' +
      '.t2v-sb-ph .ic{font-size:22px;opacity:.65}' +
      '.t2v-sb-src{position:absolute;left:5px;top:5px;font-size:10px;padding:2px 7px}' +
      '.t2v-sb-busy{position:absolute;inset:0;background:rgba(15,17,40,.62);color:#fff;display:flex;' +
        'flex-direction:column;align-items:center;justify-content:center;gap:6px;font-size:11.5px;' +
        'font-weight:600;text-align:center;padding:6px}' +
      '.t2v-sb-busy .spinner{border-color:rgba(255,255,255,.35);border-top-color:#fff}' +
      '.t2v-sb-main{flex:1;min-width:0}' +
      '.t2v-sb-text{font-size:13px;line-height:1.5;display:-webkit-box;-webkit-line-clamp:2;' +
        '-webkit-box-orient:vertical;overflow:hidden;word-break:break-word}' +
      '.t2v-sb-lab{font-size:11.5px;font-weight:600;color:var(--muted);margin:8px 0 4px}' +
      '.t2v-sb-row .textarea{min-height:44px;font-size:12.5px;padding:7px 10px}' +
      '.t2v-sb-btns{display:flex;gap:6px;flex-wrap:wrap;margin-top:8px}' +
      '.t2v-sb-chips{display:flex;gap:6px;flex-wrap:wrap}' +
      '.t2v-sb-chip{display:inline-flex;align-items:center;gap:5px;font-family:inherit;font-size:12px;' +
        'font-weight:600;line-height:1.3;padding:4px 11px;border-radius:999px;cursor:pointer;' +
        'background:var(--bg);color:var(--muted);border:1px solid var(--border);' +
        'transition:background .12s ease,color .12s ease,border-color .12s ease}' +
      '.t2v-sb-chip:hover{border-color:var(--blue);color:var(--text)}' +
      '.t2v-sb-chip.on{background:var(--blue);color:#fff;border-color:var(--blue)}' +
      '.t2v-sb-chip .tick{display:none;font-size:11px}' +
      '.t2v-sb-chip.on .tick{display:inline}' +
      '@media(max-width:700px){.t2v-sb-row{flex-direction:column}.t2v-sb-thumb{width:100%;height:160px}}';
    document.head.appendChild(el);
  }

  // ---------- Helpers ----------

  function dataUrl(p) { return '/data/' + String(p).split('/').map(encodeURIComponent).join('/'); }

  function shotUrl(seg, i) {
    var u = dataUrl(seg.imagePath);
    return st.bust[i] ? (u + '?t=' + st.bust[i]) : u;
  }

  function redAlert(msg) {
    return h('div', {
      class: 'text-red',
      style: { background: 'rgba(239,68,68,.08)', border: '1px solid var(--red)', borderRadius: '10px',
        padding: '9px 11px', fontSize: '12.5px', fontWeight: '600', marginTop: '8px' }
    }, '⚠ ' + msg);
  }

  function amberWarn(msg) {
    return h('div', {
      style: { background: 'rgba(245,158,11,.09)', border: '1px solid var(--amber)',
        borderRadius: '10px', padding: '10px 12px', marginTop: '12px' }
    },
      h('div', { class: 'row' },
        h('span', null, '⚠️'),
        h('span', { class: 'text-amber', style: { fontWeight: '600', flex: '1', minWidth: '160px', fontSize: '12.5px' } }, msg),
        h('a', { href: '#/settings', class: 'btn btn-ghost btn-sm' }, 'Mở Cấu hình & API →')));
  }

  function redraw() {
    if (st.host && st.ctx) draw(st.host, st.ctx);
  }

  // Chỉ nhận lại phần ảnh từ máy chủ — KHÔNG đè prompt người dùng đang gõ.
  function mergeImages(local, fresh) {
    var a = (local && local.segments) || [];
    var b = (fresh && fresh.segments) || [];
    for (var i = 0; i < a.length && i < b.length; i++) {
      a[i].imagePath = b[i].imagePath || '';
      a[i].imageSource = b[i].imageSource || '';
      if (!a[i].imagePrompt) a[i].imagePrompt = b[i].imagePrompt || '';
    }
  }

  function refresh(ctx) {
    if (!ctx.fetchSession) { redraw(); return; }
    ctx.fetchSession().then(function (s) {
      if (!s || !s.id) return;
      mergeImages(ctx.session, s);
      if (ctx.applyServer) ctx.applyServer(s);
    }).catch(function (e) {
      UI.toast('Không tải lại được thông tin cảnh: ' + e.message, 'error');
    }).then(function () { redraw(); });
  }

  // ---------- Nhân vật của cảnh ----------

  // Nạp danh sách nhân vật 1 lần cho mỗi lần mở trang. Backend chưa có endpoint
  // này thì coi như chưa có nhân vật — KHÔNG làm hỏng phần còn lại của Storyboard.
  function loadChars() {
    if (st.charsLoaded) return;
    st.charsLoaded = true;
    API.get('/api/characters').then(function (list) {
      st.chars = Array.isArray(list) ? list.filter(function (c) { return c && c.id; }) : [];
    }).catch(function () {
      st.chars = [];
    }).then(function () {
      // Danh sách rỗng → bản vẽ đầu đã đúng, khỏi vẽ lại (tránh cắt ngang người đang gõ prompt).
      if (st.chars.length) redraw();
    });
  }

  function charIds(seg) {
    if (!Array.isArray(seg.characterIds)) seg.characterIds = [];
    return seg.characterIds;
  }

  function charChip(ctx, seg, c) {
    var on = charIds(seg).indexOf(c.id) >= 0;
    var name = c.name || '(chưa đặt tên)';
    var chip = h('button', {
      type: 'button', class: 't2v-sb-chip' + (on ? ' on' : ''),
      title: c.look || c.role || ('Nhân vật ' + name)
    }, h('span', { class: 'tick' }, '✓'), h('span', null, name));

    chip.onclick = function () {
      var ids = charIds(seg);
      var i = ids.indexOf(c.id);
      if (i >= 0) ids.splice(i, 1); else ids.push(c.id);
      chip.className = 't2v-sb-chip' + (i >= 0 ? '' : ' on');
      if (ctx.saveSegments) ctx.saveSegments();
    };
    return chip;
  }

  function charBlock(ctx, seg) {
    var box = h('div', null, h('div', { class: 't2v-sb-lab' }, '🧑 Nhân vật trong cảnh:'));
    if (!st.chars.length) {
      box.appendChild(h('div', { class: 'muted', style: { fontSize: '11.5px' } },
        'Chưa có nhân vật — tạo ở trang ', h('a', { href: '#/characters' }, 'Nhân vật'), '.'));
      return box;
    }
    var chips = h('div', { class: 't2v-sb-chips' });
    st.chars.forEach(function (c) { chips.appendChild(charChip(ctx, seg, c)); });
    box.appendChild(chips);
    return box;
  }

  // ---------- Xem ảnh lớn ----------

  function viewLarge(seg, i) {
    UI.modal({
      title: 'Cảnh #' + (i + 1),
      body: h('div', null,
        h('img', {
          src: shotUrl(seg, i), alt: 'Ảnh cảnh ' + (i + 1),
          style: { width: '100%', borderRadius: '12px', display: 'block', background: 'var(--bg)' }
        }),
        seg.imagePrompt
          ? h('p', { class: 'muted', style: { fontSize: '12.5px', marginTop: '10px', wordBreak: 'break-word' } },
            '🎨 ' + seg.imagePrompt)
          : null),
      actions: []
    });
  }

  // ---------- Một cảnh ----------

  function segRow(ctx, seg, i) {
    var hasImg = !!seg.imagePath;
    var src = SRC[seg.imageSource] || (hasImg ? { label: 'Ảnh', cls: 'badge-gray' } : null);

    var overlay = h('div', {
      class: 't2v-sb-busy', style: { display: 'none' },
      onclick: function (e) { e.stopPropagation(); }
    });
    var thumbInner = hasImg
      ? h('img', { src: shotUrl(seg, i), alt: 'Ảnh cảnh ' + (i + 1) })
      : h('div', { class: 't2v-sb-ph' }, h('span', { class: 'ic' }, '🖼'), h('span', null, 'Chưa có ảnh'));

    var thumb = h('div', {
      class: 't2v-sb-thumb', title: hasImg ? 'Bấm để xem ảnh lớn' : 'Cảnh này chưa có ảnh',
      style: hasImg ? { cursor: 'zoom-in' } : null,
      onclick: hasImg ? function () { viewLarge(seg, i); } : null
    }, thumbInner, src ? h('span', { class: 'badge ' + src.cls + ' t2v-sb-src' }, src.label) : null, overlay);

    var meta = [];
    if (seg.seconds > 0) meta.push(UI.fmtDur(seg.seconds) + ' (thật)');
    meta.push((seg.text || '').length + ' ký tự');

    var promptTa = UI.textarea({
      rows: 2, value: seg.imagePrompt || '',
      placeholder: 'Mô tả ảnh cho cảnh này (tiếng Anh cho model hiểu tốt hơn). Để trống → AI tự nghĩ khi tạo ảnh.',
      oninput: function () {
        seg.imagePrompt = promptTa.value;
        if (ctx.saveSegments) ctx.saveSegments();
      }
    });

    var errHost = h('div');
    var upBtn, genBtn, viewBtn;

    function setDisabled(v) {
      upBtn.disabled = !!v;
      genBtn.disabled = !!v;
      viewBtn.disabled = !!v || !hasImg;
    }

    function setBusy(text) {
      errHost.innerHTML = '';
      overlay.innerHTML = '';
      overlay.appendChild(UI.spinner());
      overlay.appendChild(h('span', null, text || 'Đang xử lý…'));
      overlay.style.display = '';
      setDisabled(true);
    }

    function setError(msg) {
      overlay.style.display = 'none';
      overlay.innerHTML = '';
      setDisabled(false);
      errHost.innerHTML = '';
      if (msg) errHost.appendChild(redAlert(msg));
    }

    // --- Tải ảnh lên ---
    var fileIn = h('input', { type: 'file', accept: 'image/*', style: { display: 'none' } });
    fileIn.onchange = function () {
      var f = fileIn.files && fileIn.files[0];
      fileIn.value = '';
      if (!f) return;
      setBusy('Đang tải ảnh lên…');
      API.upload(segPath(ctx, i) + '/image/upload', [f]).then(function (s) {
        if (!s || !s.id) throw new Error('Máy chủ không trả về phiên sau khi tải ảnh');
        st.bust[i] = Date.now();
        mergeImages(ctx.session, s);
        if (ctx.applyServer) ctx.applyServer(s);
        UI.toast('Đã thay ảnh cảnh #' + (i + 1));
        redraw();
      }).catch(function (e) {
        setError('Không tải được ảnh lên: ' + e.message);
        UI.toast('Không tải được ảnh lên: ' + e.message, 'error');
      });
    };

    // --- Tạo lại cảnh ---
    function regenerate() {
      setBusy('Đang gửi yêu cầu…');
      API.post(segPath(ctx, i) + '/image', { prompt: (promptTa.value || '').trim() }).then(function (job) {
        if (!job || !job.id) throw new Error('Máy chủ không trả về tác vụ');
        st.pending[i] = job;
        setBusy('Đang tạo ảnh…');
        bindShotJob(ctx, i, job, overlay, setError);
      }).catch(function (e) {
        delete st.pending[i];
        setError('Không tạo được ảnh: ' + e.message);
        UI.toast('Không tạo được ảnh cảnh #' + (i + 1) + ': ' + e.message, 'error');
      });
    }

    upBtn = UI.btn('📁 Tải ảnh lên', {
      variant: 'ghost', small: true, onclick: function () { fileIn.click(); }
    });
    genBtn = UI.btn('🔄 Tạo lại cảnh', { variant: 'ghost', small: true, onclick: regenerate });
    viewBtn = UI.btn('👁 Xem lớn', {
      variant: 'ghost', small: true, disabled: !hasImg,
      onclick: function () { viewLarge(seg, i); }
    });

    var row = h('div', { class: 't2v-sb-row' }, thumb,
      h('div', { class: 't2v-sb-main' },
        h('div', { class: 'row-between', style: { alignItems: 'flex-start' } },
          h('b', { style: { fontSize: '12.5px', flex: 'none' } }, '#' + (i + 1)),
          h('span', { class: 'muted', style: { fontSize: '11.5px' } }, meta.join(' · '))),
        h('div', { class: 't2v-sb-text', title: seg.text || '' }, seg.text || '(đoạn trống)'),
        h('div', { class: 't2v-sb-lab' }, '🎨 Prompt tạo ảnh:'),
        promptTa,
        charBlock(ctx, seg),
        h('div', { class: 't2v-sb-btns' }, upBtn, genBtn, viewBtn, fileIn),
        errHost));

    // Job của cảnh này còn đang chạy (render lại giữa chừng) → gắn lại tiến độ.
    if (st.pending[i]) {
      setBusy('Đang tạo ảnh…');
      bindShotJob(ctx, i, st.pending[i], overlay, setError);
    }
    return row;
  }

  function segPath(ctx, i) {
    return '/api/t2v/sessions/' + encodeURIComponent(ctx.session.id) + '/segments/' + i;
  }

  // Theo dõi job tạo ảnh của MỘT cảnh qua ctx.watchJob (listener do text2video.js giữ).
  function bindShotJob(ctx, i, job, overlay, setError) {
    if (!ctx.watchJob) return;
    ctx.watchJob(job, overlay, function (j) {
      delete st.pending[i];
      if (j && j.status === 'done') {
        st.bust[i] = Date.now();
        UI.toast('Đã tạo ảnh mới cho cảnh #' + (i + 1));
        refresh(ctx);
        return;
      }
      var msg = (j && j.error) || 'Không tạo được ảnh cho cảnh này';
      setError(msg);
      UI.toast('Cảnh #' + (i + 1) + ' thất bại: ' + msg, 'error');
    });
  }

  // ---------- Khối đầu card ----------

  function headBlock(ctx, segs) {
    var withImg = 0;
    segs.forEach(function (s) { if (s.imagePath) withImg++; });

    var genBtn = UI.btn('🖼 Sinh ảnh cho tất cả cảnh', {
      disabled: !!ctx.busy || !segs.length,
      onclick: function () {
        if (!segs.length) { UI.toast('Chưa có kịch bản — hãy viết kịch bản trước.', 'error'); return; }
        if (ctx.startAll) ctx.startAll();
      }
    });

    var box = h('div', null,
      h('div', { class: 'muted', style: { fontSize: '12.5px' } },
        'Mỗi đoạn kịch bản một ảnh riêng — video sẽ là ảnh thật thay vì chữ trên nền màu.'),
      h('div', { class: 'row mt-8' }, genBtn,
        h('span', { class: 'badge ' + (withImg === segs.length && segs.length ? 'badge-green' : 'badge-gray') },
          withImg + '/' + segs.length + ' cảnh đã có ảnh'),
        h('span', { class: 'muted', style: { fontSize: '12px' } },
          'Chỉ sinh cho cảnh chưa có ảnh — ảnh bạn tự tải lên sẽ không bị ghi đè.')));

    if (ctx.inlineHost) box.appendChild(ctx.inlineHost);

    st.warnHost = h('div');
    box.appendChild(st.warnHost);
    syncWarn();
    return box;
  }

  // /api/state về sau lần vẽ đầu → text2video.js gọi lại hàm này.
  function syncWarn() {
    if (!st.warnHost) return;
    st.warnHost.innerHTML = '';
    var tools = App.state && App.state.tools;
    if (tools && !tools.geminiKey && !tools.pexelsKey) {
      st.warnHost.appendChild(amberWarn('Chưa có Gemini hoặc Pexels API key — chưa sinh được ảnh; ' +
        'bạn vẫn có thể tự tải ảnh lên cho từng cảnh.'));
    }
  }

  // ---------- Vẽ ----------

  function draw(host, ctx) {
    st.host = host;
    st.ctx = ctx;
    host.innerHTML = '';

    var segs = (ctx.session && ctx.session.segments) || [];
    host.appendChild(headBlock(ctx, segs));

    if (!segs.length) {
      host.appendChild(UI.empty(
        'Chưa có kịch bản — viết kịch bản ở bước 2 rồi quay lại đây để sinh ảnh cho từng cảnh.', '🖼️'));
      return;
    }
    var list = h('div', { class: 'mt-16' });
    segs.forEach(function (seg, i) { list.appendChild(segRow(ctx, seg, i)); });
    host.appendChild(list);
  }

  // ---------- API công khai ----------

  function render(host, ctx) {
    ensureCss();
    if (!host) return function () {};
    if (!ctx || !ctx.session || !ctx.session.id) {
      host.innerHTML = '';
      host.appendChild(redAlert('Thiếu dữ liệu phiên — không dựng được Storyboard.'));
      return function () {};
    }
    // Mở lại trang (mount mới) → bỏ tiến trình cũ để không hiện spinner treo,
    // đồng thời nạp lại danh sách nhân vật (người dùng có thể vừa thêm nhân vật mới).
    if (st.mountId !== ctx.mountId) {
      st.mountId = ctx.mountId || '';
      st.pending = {};
      st.bust = {};
      st.charsLoaded = false;
      st.chars = [];
    }
    draw(host, ctx);
    loadChars();

    return function () {
      if (st.host === host) { st.host = null; st.ctx = null; st.warnHost = null; }
    };
  }

  window.T2VStoryboard = {
    render: render,
    syncWarn: syncWarn,
    dispose: function () {
      st.host = null; st.ctx = null; st.warnHost = null;
      st.pending = {}; st.bust = {}; st.mountId = '';
      st.chars = []; st.charsLoaded = false;
    }
  };
})();
