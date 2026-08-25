/* ============================================================
   Biz Studio — Trang Mẫu thiết kế (Style Kit đầy đủ)
   Điều khiển toàn bộ giao diện video: phong cách, cỡ chữ, màu,
   phông chữ, logo/tên kênh, tư liệu nền — kèm khung xem trước sống.
   Load sau app.js. Tự đăng ký App.pages['stylekit'].
   Không framework / CDN / ES modules.
   ============================================================ */
(function () {
  'use strict';

  // ---------- Hằng số ----------
  var FONT_MODERN = '-apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif';
  var FONT_IMPACT = '"Arial Black", "Helvetica Neue", Impact, sans-serif';
  var FONT_SERIF = 'Georgia, "Times New Roman", "Songti SC", serif';
  var FONT_ROUND = '"Avenir Next", "Trebuchet MS", "Segoe UI", sans-serif';
  var FONT_MONO = 'ui-monospace, "SF Mono", Menlo, Consolas, monospace';
  var FONT_CUSTOM = '__custom__';
  var FONT_OPTS = [
    { value: FONT_MODERN, label: 'Hiện đại (mặc định)' }, { value: FONT_IMPACT, label: 'Đậm thể thao' },
    { value: FONT_SERIF, label: 'Serif sang trọng' }, { value: FONT_ROUND, label: 'Tròn thân thiện' },
    { value: FONT_MONO, label: 'Mono kỹ thuật' }, { value: FONT_CUSTOM, label: 'Tự nhập…' }
  ];
  var COLOR_PRESETS = [
    { name: 'Sáng', bgDeep: '#F1F5F9', textMain: '#0F172A', accent: '#2563EB' },
    { name: 'Tối', bgDeep: '#07070F', textMain: '#F5F5F7', accent: '#F59E0B' },
    { name: 'Cyberpunk Neon', bgDeep: '#05060F', textMain: '#E0F2FE', accent: '#22D3EE' },
    { name: 'Hồng Tím Neon', bgDeep: '#140B2E', textMain: '#F8FAFC', accent: '#EC4899' }
  ];
  var THEME_OPTS = [{ value: 'vivid', label: 'Rực rỡ (vivid)' }, { value: 'dark', label: 'Tối (dark)' },
    { value: 'light', label: 'Sáng (light)' }];
  var TEMPLATES = [
    { id: 'hero', label: 'Mở đầu', title: 'Ba cách tăng doanh thu', sub: 'Xem đến cuối để nhận mẫu miễn phí' },
    { id: 'bullets', label: 'Ý chính', title: 'Ba điều cần nhớ', sub: 'Tóm tắt nhanh trong 30 giây' },
    { id: 'photo', label: 'Ảnh', title: 'Khoảnh khắc quyết định', sub: 'Ảnh nền tràn khung' },
    { id: 'outro', label: 'Kết', title: 'Cảm ơn đã xem', sub: 'Đăng ký kênh để không bỏ lỡ' }
  ];
  var GROUPS = [
    { icon: '🎞', label: 'Phong cách chuyển động', desc: 'Nền tảng, prompt sinh ảnh' },
    { icon: '📐', label: 'Khung hình & giới hạn', desc: 'Cỡ chữ, độ dài kịch bản' },
    { icon: '🎨', label: 'Màu sắc & phông chữ', desc: 'Preset, mã màu, font' },
    { icon: '🏷', label: 'Nhận diện & stock', desc: 'Tên kênh, logo, tư liệu' }
  ];
  var VIDEO_EXT = ['mp4', 'mov', 'webm', 'mkv', 'avi', 'm4v'];
  var MAX_COLORS = 12, FALLBACK_HEX = '#7C3AED', SYNC_DELAY = 400;
  // Khung xem trước dựng đúng khổ video dọc rồi thu nhỏ bằng transform;
  // seek(t) đưa cảnh về giữa hiệu ứng vào để không thấy khung trắng.
  var STAGE_W = 1080, STAGE_H = 1920, PREVIEW_T = 2.5;

  var S = null; // trạng thái trang — tạo lại mỗi lần vào trang
  // alive — callback của phiên cũ (đã rời trang) phải tự im lặng.
  function alive(st) { return !!st && st === S && !st.dead; }

  // ---------- CSS nội bộ ----------
  function injectStyles() {
    if (document.getElementById('stylekit-page-style')) return;
    var BD = 'border:1px solid var(--border)';
    var ON = 'background:var(--grad-soft);border-color:var(--blue);color:var(--blue)';
    var BTN = BD + ';background:var(--card);color:var(--text);cursor:pointer;font-family:inherit;font-weight:600';
    var COL = 'display:flex;flex-direction:column;align-items:center;justify-content:center';
    var css = [
      '.sk-main{display:grid;grid-template-columns:minmax(0,1fr) 400px;gap:16px;align-items:start;margin-top:16px}',
      '@media(max-width:1260px){.sk-main{grid-template-columns:minmax(0,1fr)}}',
      '.sk-left{display:grid;grid-template-columns:186px minmax(0,1fr);gap:14px;align-items:start}',
      '@media(max-width:860px){.sk-left{grid-template-columns:minmax(0,1fr)}}',
      '.sk-rail{display:flex;flex-direction:column;gap:6px} .sk-seg{display:flex;gap:8px;flex-wrap:wrap}',
      '.sk-tab{display:block;width:100%;text-align:left;border-radius:10px;padding:9px 11px;font-size:13px;' + BTN + '}',
      '.sk-tab:hover,.sk-segbtn:hover{background:var(--bg)} .sk-tab.active,.sk-segbtn.active{' + ON + '}',
      '.sk-tab small{display:block;font-weight:500;font-size:11px;color:var(--muted);margin-top:2px}',
      '.sk-tab.active small{color:var(--blue);opacity:.85}',
      '.sk-segbtn{border-radius:9px;padding:7px 13px;font-size:12.5px;' + BTN + '}',
      '.sk-presets{display:grid;grid-template-columns:repeat(auto-fill,minmax(128px,1fr));gap:10px;margin-bottom:16px}',
      '.sk-preset{border-radius:12px;padding:8px;' + BTN + '} .sk-preset:hover{border-color:var(--blue)}',
      '.sk-bars{display:flex;height:34px;border-radius:8px;overflow:hidden;' + BD + '} .sk-bars span{flex:1}',
      '.sk-preset-name{font-size:12px;font-weight:600;margin-top:6px;text-align:center}'
        + '.sk-code{font-family:' + FONT_MONO + ';font-size:12px;min-height:200px}',
      '.sk-color-row{display:flex;align-items:center;gap:8px;margin-bottom:8px} .sk-color-row .input{flex:1;min-width:0}',
      '.sk-color-pick{width:40px;height:38px;flex:none;padding:2px;border-radius:var(--r-input);' + BTN + '}',
      '.sk-hint{font-size:12px;color:var(--muted);line-height:1.65;margin:-8px 0 14px}',
      '.sk-hint code{background:var(--bg);' + BD + ';border-radius:5px;padding:1px 5px;font-size:11.5px}',
      '.sk-frame{height:min(60vh,560px);aspect-ratio:9/16;position:relative;border-radius:14px;overflow:hidden;'
        + BD + ';background:#0B0B12} .sk-frame.big{height:min(76vh,800px)}',
      '.sk-frame iframe{position:absolute;left:0;top:0;width:' + STAGE_W + 'px;height:' + STAGE_H + 'px;border:0;'
        + 'display:block;transform-origin:0 0;background:#0B0B12}',
      '.sk-off{position:absolute;inset:0;' + COL + ';gap:8px;padding:22px;text-align:center;background:var(--bg);'
        + 'color:var(--muted);font-size:12.5px;line-height:1.6} .sk-center{display:flex;justify-content:center}',
      '.sk-stock{display:grid;grid-template-columns:repeat(auto-fill,minmax(94px,1fr));gap:10px;margin-top:10px}',
      '.sk-stock-it{position:relative;border-radius:10px;overflow:hidden;background:var(--bg);aspect-ratio:1;' + BD + '}'
        + '.sk-stock-it img{width:100%;height:100%;object-fit:cover;display:block}',
      '.sk-stock-vid{height:100%;gap:4px;font-size:10.5px;color:var(--muted);padding:6px;text-align:center;'
        + 'word-break:break-all;' + COL + '}',
      '.sk-stock-x{position:absolute;top:4px;right:4px;width:22px;height:22px;border:0;border-radius:6px;'
        + 'background:rgba(11,11,18,.72);color:#fff;cursor:pointer;font-size:13px;line-height:1;' + COL + '}',
      '.sk-logo{max-width:100%;max-height:96px;border-radius:10px;background:var(--bg);display:block;padding:6px;' + BD + '}'
        + '.sk-sample{width:100%;max-height:220px;object-fit:cover;border-radius:12px;margin-top:10px;display:block}'
    ].join('\n');
    document.head.appendChild(h('style', { id: 'stylekit-page-style' }, css));
  }

  // ---------- Helpers ----------
  function dataUrl(p) { return '/data/' + String(p).split('/').map(encodeURIComponent).join('/'); }
  function baseName(p) { var a = String(p || '').split('/'); return a[a.length - 1] || p; }
  function isHex(v) { return /^#[0-9a-fA-F]{6}$/.test(String(v || '').trim()); }
  function safeHex(v, fb) { return isHex(v) ? String(v).trim() : (fb || FALLBACK_HEX); }
  function samplePath(id) { return '/data/styles/' + encodeURIComponent(id + '-preview.png'); }
  function label(text, mt) { return h('div', { class: 'field-label', style: { margin: (mt || 0) + 'px 0 8px' } }, text); }

  function isVideo(p) {
    var a = String(p || '').toLowerCase().split('.');
    return a.length > 1 && VIDEO_EXT.indexOf(a[a.length - 1]) >= 0;
  }

  function hint() { // hint('chữ', node, 'chữ'…)
    var el = h('div', { class: 'sk-hint' }), c;
    for (var i = 0; i < arguments.length; i++) {
      c = arguments[i];
      el.appendChild(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return el;
  }

  function amberWarn(msg) {
    return h('div', { style: { background: 'rgba(245,158,11,.09)', border: '1px solid var(--amber)',
      borderRadius: '10px', padding: '10px 12px', marginTop: '16px' } },
      h('div', { class: 'row' }, h('span', null, '⚠️'),
      h('span', { class: 'text-amber', style: { fontWeight: '600', flex: '1', minWidth: '160px' } }, msg),
      h('a', { href: '#/settings', class: 'btn btn-ghost btn-sm' }, 'Mở Cấu hình & API →')));
  }

  // bind — gõ tới đâu ghi vào bộ mẫu tới đó rồi hẹn giờ lưu + xem trước.
  function bind(el, key) {
    el.oninput = function () { S.cur[key] = el.value; touch(); };
    return el;
  }

  function segmented(items, value, onpick) {
    var wrap = h('div', { class: 'sk-seg' });
    items.forEach(function (it) {
      var b = h('button', { class: 'sk-segbtn' + (it.value === value ? ' active' : ''), type: 'button' }, it.label);
      b.onclick = function () {
        Array.prototype.forEach.call(wrap.children, function (c) { c.classList.remove('active'); });
        b.classList.add('active');
        onpick(it.value);
      };
      wrap.appendChild(b);
    });
    return wrap;
  }

  function filePicker(accept, multiple, onfiles) {
    var inp = h('input', { type: 'file', accept: accept, multiple: multiple ? 'multiple' : null, style: { display: 'none' } });
    inp.onchange = function () {
      var fs = inp.files ? Array.prototype.slice.call(inp.files) : [];
      inp.value = '';
      if (fs.length) onfiles(fs);
    };
    return inp; // gắn vào DOM cùng nút bấm để .click() chạy được
  }

  // ---------- Bộ mẫu: chuẩn hoá & payload ----------
  // Backend cũ chưa trả field v1.9 → bù mặc định giống applyStyleDefaults của store.
  function normKit(k) {
    var o = JSON.parse(JSON.stringify(k || {}));
    o.palette = Array.isArray(o.palette) ? o.palette : [];
    o.stockPaths = Array.isArray(o.stockPaths) ? o.stockPaths : [];
    o.theme = (o.theme === 'dark' || o.theme === 'light') ? o.theme : 'vivid';
    o.logoPos = (o.logoPos === 'center' || o.logoPos === 'right') ? o.logoPos : 'left';
    o.baseTemplate = o.baseTemplate === 'custom' ? 'custom' : 'builtin';
    o.accent = o.accent || o.palette[0] || '#F59E0B';
    var str = { bgDeep: '#0F0A1E', textMain: '#F8FAFC', fontHead: FONT_MODERN, fontBody: FONT_MODERN,
      customHtml: '', channelName: '', logoPath: '', stylePrompt: '', negative: '' };
    var num = { sizeTitle: 48, sizeBig: 150, sizeBody: 22, maxVoiceChars: 180, maxImageChars: 200 };
    Object.keys(str).forEach(function (k2) { o[k2] = o[k2] || str[k2]; });
    Object.keys(num).forEach(function (k2) { o[k2] = Number(o[k2]) || num[k2]; });
    return o;
  }

  function payload() {
    var k = S.cur;
    return {
      name: (k.name || '').trim() || 'Bộ mẫu không tên',
      stylePrompt: k.stylePrompt, negative: k.negative, palette: k.palette, theme: k.theme,
      bgDeep: k.bgDeep, textMain: k.textMain, accent: k.accent, fontHead: k.fontHead, fontBody: k.fontBody,
      sizeTitle: Number(k.sizeTitle) || 48, sizeBig: Number(k.sizeBig) || 150, sizeBody: Number(k.sizeBody) || 22,
      channelName: k.channelName, logoPath: k.logoPath, logoPos: k.logoPos, stockPaths: k.stockPaths,
      maxVoiceChars: Number(k.maxVoiceChars) || 0, maxImageChars: Number(k.maxImageChars) || 0,
      baseTemplate: k.baseTemplate, customHtml: k.customHtml, isDefault: !!k.isDefault
    };
  }

  // ---------- Đồng bộ với máy chủ ----------
  function setSaveInfo(text, cls) {
    if (!S.saveInfo) return;
    S.saveInfo.className = 'muted' + (cls ? ' ' + cls : '');
    S.saveInfo.textContent = text;
  }

  function save(silent) {
    var st = S;
    if (!st.cur || !st.cur.id) return Promise.resolve();
    setSaveInfo('Đang lưu…');
    return API.put('/api/styles/' + encodeURIComponent(st.cur.id), payload()).then(function (res) {
      if (!alive(st)) return;
      st.dirty = false;
      if (res && res.id) {
        st.cur.isDefault = !!res.isDefault;
        if (res.logoPath !== undefined) st.cur.logoPath = res.logoPath || '';
        if (Array.isArray(res.stockPaths)) st.cur.stockPaths = res.stockPaths;
      }
      setSaveInfo('✓ Đã lưu lúc ' + new Date().toLocaleTimeString('vi-VN'), 'text-green');
      if (!silent) UI.toast('Đã lưu cài đặt mẫu thiết kế');
    }).catch(function (err) {
      if (!alive(st)) return;
      setSaveInfo('⚠ Chưa lưu được: ' + err.message, 'text-red');
      if (!silent) UI.toast('Lưu thất bại: ' + err.message, 'error');
    });
  }

  // touch — đổi cài đặt nào cũng lưu tạm rồi nạp lại khung xem trước
  // (preview.html dựng từ dữ liệu trong store nên phải lưu trước).
  function touch() {
    var st = S;
    st.dirty = true; setSaveInfo('Có thay đổi chưa lưu…');
    if (st.timer) clearTimeout(st.timer);
    st.timer = setTimeout(function () {
      st.timer = null;
      if (alive(st)) save(true).then(function () { if (alive(st)) loadPreview(); });
    }, SYNC_DELAY);
  }

  function flush() { // lưu ngay thay đổi đang chờ (trước khi đổi bộ / xoá)
    if (S.timer) { clearTimeout(S.timer); S.timer = null; }
    return S.dirty ? save(true) : Promise.resolve();
  }

  // ---------- Khung xem trước ----------
  function frameBox(big) {
	// allow-scripts để animation window.seek vẫn chạy; cố ý KHÔNG
	// allow-same-origin để CustomHTML không đọc key hay gọi control API.
	var frame = h('iframe', { title: 'Xem trước mẫu thiết kế', src: 'about:blank', sandbox: 'allow-scripts' }), off = h('div', { class: 'sk-off' });
    var box = h('div', { class: 'sk-frame' + (big ? ' big' : '') }, frame, off);
    function overlay() { off.style.display = ''; off.innerHTML = ''; return off; }
    box.fit = function () { frame.style.transform = 'scale(' + ((box.clientWidth || STAGE_W) / STAGE_W) + ')'; };
    frame.onload = function () {
      box.fit();
      // Template dựng cảnh theo seek(t) — không gọi thì chữ còn ở trạng thái chưa hiện.
      try {
        var w = frame.contentWindow;
        if (w && typeof w.seek === 'function') w.seek(PREVIEW_T);
      } catch (e) { /* iframe chưa sẵn sàng — bỏ qua */ }
    };
    box.setUrl = function (u) { off.style.display = 'none'; box.fit(); frame.src = u; };
    box.setLoading = function () {
      overlay().appendChild(UI.spinner());
      off.appendChild(h('span', null, 'Đang dựng bản xem trước…'));
    };
    box.setOff = function (msg) {
      frame.src = 'about:blank';
      overlay().appendChild(h('div', { style: { fontSize: '26px' } }, '🖼'));
      off.appendChild(h('div', null, msg));
      off.appendChild(h('div', { style: { fontSize: '11.5px', opacity: '.85' } },
        'Cài đặt vẫn được lưu bình thường — mở lại trang khi máy chủ sẵn sàng.'));
    };
    box.setOff('Chưa có bản xem trước');
    return box; // khung xám khi chưa có bản dựng
  }

  function curTpl() {
    for (var i = 0; i < TEMPLATES.length; i++) if (TEMPLATES[i].id === S.tpl) return TEMPLATES[i];
    return TEMPLATES[0];
  }

  function loadPreview() {
    var st = S;
    // Khung của modal đã đóng thì gỡ khỏi danh sách, khỏi nạp lại vô ích.
    st.frames = st.frames.filter(function (f) { return document.body.contains(f); });
    if (!st.cur || !st.cur.id || !st.frames.length) return;
    var t = curTpl();
	var url = '/api/styles/' + encodeURIComponent(st.cur.id) + '/preview.html?template=' + encodeURIComponent(t.id)
      + '&title=' + encodeURIComponent(t.title) + '&subtitle=' + encodeURIComponent(t.sub)
	  + '&aspect=9:16&previewAt=' + PREVIEW_T + '&t=' + Date.now();
    st.frames.forEach(function (f) { f.setLoading(); });
    fetch(url).then(function (res) {
      if (res.ok) return null;
      return res.text().then(function (txt) {
        var msg = '';
        try { var j = JSON.parse(txt); msg = (j && j.error) ? j.error : ''; } catch (e) { msg = ''; }
        throw new Error(msg || ('máy chủ trả lỗi ' + res.status));
      });
    }).then(function () {
      if (alive(st)) st.frames.forEach(function (f) { f.setUrl(url); });
    }).catch(function (err) {
      if (!alive(st)) return;
      var m = (err && err.message) ? err.message : 'không kết nối được máy chủ';
      st.frames.forEach(function (f) { f.setOff('Chưa xem trước được: ' + m); });
    });
  }

  function openZoom() {
    var big = frameBox(true);
    S.frames.push(big);
    var m = UI.modal({
      title: 'Xem trước — ' + curTpl().label,
      body: h('div', { class: 'sk-center', style: { marginTop: '4px' } }, big),
      actions: [UI.btn('Đóng', { variant: 'ghost', onclick: function () { m.close(); } })]
    });
    m.el.style.width = 'min(560px, calc(100vw - 40px))';
    loadPreview();
  }

  function previewPanel() {
    var box = frameBox(false); S.frames = [box];
    return UI.card({
      title: 'XEM TRƯỚC', icon: '👁',
      body: h('div', null,
        h('div', { class: 'row-between', style: { marginBottom: '12px' } },
          segmented(TEMPLATES.map(function (t) { return { value: t.id, label: t.label }; }), S.tpl,
            function (v) { S.tpl = v; loadPreview(); }),
          UI.btn('⤢ Phóng to', { variant: 'ghost', small: true, onclick: openZoom })),
        h('div', { class: 'sk-center' }, box),
        h('div', { class: 'sk-hint', style: { margin: '12px 0 0' } },
          'Bản xem trước dùng đúng template lúc render video. ' +
          'Hiệu ứng chuyển động sẽ được kết xuất đầy đủ khi xuất video.'))
    });
  }

  // ---------- Nhóm 1: Phong cách chuyển động ----------
  // Ảnh thử chất hình do AI sinh (dùng lại job style_preview có sẵn).
  function samplePreview() {
    var st = S;
    var img = h('img', { class: 'sk-sample', alt: 'Ảnh thử phong cách', src: samplePath(st.cur.id), style: { display: 'none' } });
    img.onload = function () { img.style.display = ''; };
    img.onerror = function () { img.style.display = 'none'; };
    var status = h('span', { class: 'muted', style: { fontSize: '12px' } });
    S.sample = { img: img, status: status };
    return h('div', null, h('div', { class: 'row' },
      UI.btn('🖼 Sinh ảnh thử (AI)', {
        variant: 'ghost', small: true,
        onclick: function () {
          status.textContent = 'Đang gửi yêu cầu…';
          API.post('/api/styles/' + encodeURIComponent(st.cur.id) + '/preview', {}).then(function (job) {
            if (alive(st) && job) st.sampleJobId = job.id;
          }).catch(function (err) { status.textContent = '⚠ ' + err.message; });
        }
      }), status), img);
  }

  function groupMotion() {
    var customTa = bind(UI.textarea({
      value: S.cur.customHtml, rows: 12,
      placeholder: '<div class="scene">{{TITLE}}</div>\n<script>window.seek=function(t){ /* vẽ theo giây t */ };<\/script>'
    }), 'customHtml');
    customTa.classList.add('sk-code');
    var customHost = h('div', { style: { marginTop: '14px', display: S.cur.baseTemplate === 'custom' ? '' : 'none' } },
      UI.field('Mã HTML/CSS của bạn', customTa),
      hint('Các biến được thay trước khi render: ', h('code', null, '{{TITLE}}'), ' ',
        h('code', null, '{{SUBTITLE}}'), ' ', h('code', null, '{{CHANNEL_NAME}}'), ' ',
        h('code', null, '{{ACCENT}}'), ' ', h('code', null, '{{BG_DEEP}}'), ' ',
        h('code', null, '{{TEXT_MAIN}}'), ' ', h('code', null, '{{IMAGE}}'), '.', h('br', null),
        'Bắt buộc tự định nghĩa ', h('code', null, 'window.seek(t)'),
        ' — hàm nhận thời điểm (giây) để vẽ đúng khung hình. Thiếu hàm này video vẫn dựng được nhưng đứng yên.'));

    var promptTa = bind(UI.textarea({
      value: S.cur.stylePrompt, rows: 6,
      placeholder: 'cinematic photography, warm natural light, shallow depth of field, film grain…'
    }), 'stylePrompt');
    promptTa.style.minHeight = '140px';
    return h('div', null,
      label('Nền tảng giao diện'),
      segmented([{ value: 'builtin', label: '🧩 Dựng sẵn' }, { value: 'custom', label: '💻 Custom HTML' }],
        S.cur.baseTemplate, function (v) {
          S.cur.baseTemplate = v;
          customHost.style.display = v === 'custom' ? '' : 'none';
          touch();
        }),
      customHost,
      h('div', { style: { marginTop: '18px' } },
        UI.field('Style prompt (ghép vào MỌI prompt sinh ảnh)', promptTa),
        hint('💡 Viết bằng tiếng Anh cho model hiểu tốt hơn. Mô tả chất hình: chất liệu, ánh sáng, nét vẽ, bố cục…'),
        UI.field('Tránh (negative)', bind(UI.textarea({
          value: S.cur.negative, rows: 3, placeholder: 'text, watermark, deformed hands, cluttered background…'
        }), 'negative')),
        hint('💡 Những thứ không muốn xuất hiện — chữ, watermark, tay biến dạng…'),
        samplePreview()));
  }

  // ---------- Nhóm 2: Khung hình & giới hạn ----------
  function sizeSlider(text, key, min, max) {
    return UI.slider(text, {
      min: min, max: max, step: 1, value: S.cur[key],
      oninput: function (v) { S.cur[key] = Number(v) || min; touch(); }
    });
  }

  function groupFrame() {
    return h('div', null,
      label('Cỡ chữ trong khung video (px)'),
      sizeSlider('Tiêu đề', 'sizeTitle', 24, 96),
      sizeSlider('Số lớn / con số nổi bật', 'sizeBig', 60, 260),
      sizeSlider('Nội dung & phụ đề', 'sizeBody', 14, 48),
      hint('💡 Video dọc 1080×1920: tiêu đề 44–64px, số lớn 140–200px, nội dung 20–28px là dễ đọc nhất.'),
      label('Giới hạn độ dài do AI viết (số ký tự)', 18),
      sizeSlider('Thoại mỗi đoạn', 'maxVoiceChars', 60, 400),
      sizeSlider('Mô tả hình mỗi cảnh', 'maxImageChars', 60, 400),
      hint('💡 Giới hạn được đưa vào prompt khi AI viết kịch bản, mô tả cảnh và cắt hậu kiểm — ' +
        'giữ mỗi đoạn ngắn gọn để chữ không tràn khung.'));
  }

  // ---------- Nhóm 3: Màu sắc & phông chữ ----------
  function colorRow(text, key) {
    var input = UI.input({ value: S.cur[key] || '', placeholder: '#RRGGBB' });
    var pick = h('input', { type: 'color', class: 'sk-color-pick', value: safeHex(S.cur[key], '#000000') });
    input.oninput = function () {
      S.cur[key] = input.value.trim();
      if (isHex(input.value)) pick.value = input.value.trim();
      touch();
    };
    pick.oninput = function () { input.value = pick.value; S.cur[key] = pick.value; touch(); };
    S.colorInputs[key] = { text: input, pick: pick };
    return UI.field(text, h('div', { class: 'sk-color-row' }, pick, input));
  }

  function presetCards() {
    var grid = h('div', { class: 'sk-presets' });
    COLOR_PRESETS.forEach(function (p) {
      grid.appendChild(h('div', {
        class: 'sk-preset', title: 'Áp dụng bộ màu ' + p.name,
        onclick: function () {
          ['bgDeep', 'textMain', 'accent'].forEach(function (k) {
            S.cur[k] = p[k];
            var r = S.colorInputs[k];
            if (r) { r.text.value = p[k]; r.pick.value = p[k]; }
          });
          UI.toast('Đã áp bộ màu ' + p.name);
          touch();
        }
      }, h('div', { class: 'sk-bars' }, h('span', { style: { background: p.bgDeep } }),
        h('span', { style: { background: p.textMain } }), h('span', { style: { background: p.accent } })),
        h('div', { class: 'sk-preset-name' }, p.name)));
    });
    return grid;
  }

  function paletteEditor() {
    var host = h('div');
    function sync() {
      S.cur.palette = Array.prototype.slice.call(host.querySelectorAll('.sk-pal-hex'))
        .map(function (i) { return i.value.trim(); }).filter(function (v) { return !!v; });
      touch();
    }
    function addRow(hex) {
      if (host.children.length >= MAX_COLORS) { UI.toast('Tối đa ' + MAX_COLORS + ' màu cho một bộ mẫu', 'error'); return; }
      var input = UI.input({ value: hex || FALLBACK_HEX, placeholder: '#RRGGBB' });
      input.classList.add('sk-pal-hex');
      var pick = h('input', { type: 'color', class: 'sk-color-pick', value: safeHex(hex) });
      input.oninput = function () { if (isHex(input.value)) pick.value = input.value.trim(); sync(); };
      pick.oninput = function () { input.value = pick.value; sync(); };
      var row = h('div', { class: 'sk-color-row' }, pick, input,
        UI.btn('✕', { variant: 'ghost', small: true, onclick: function () { row.remove(); sync(); } }));
      host.appendChild(row);
    }
    (S.cur.palette || []).forEach(function (c) { addRow(c); });
    return h('div', null, host,
      UI.btn('＋ Thêm màu', { variant: 'ghost', small: true, onclick: function () { addRow(''); sync(); } }));
  }

  function fontField(text, key) {
    var cur = S.cur[key] || FONT_MODERN, known = false;
    FONT_OPTS.forEach(function (o) { if (o.value === cur) known = true; });
    var custom = UI.input({ value: cur, placeholder: 'VD: Georgia, "Times New Roman", serif' });
    custom.style.marginTop = '8px'; custom.style.display = known ? 'none' : '';
    custom.oninput = function () { S.cur[key] = custom.value.trim() || FONT_MODERN; touch(); };
    var sel = UI.select(null, FONT_OPTS, known ? cur : FONT_CUSTOM, function (v) {
      custom.style.display = v === FONT_CUSTOM ? '' : 'none';
      if (v !== FONT_CUSTOM) custom.value = v;
      S.cur[key] = (v === FONT_CUSTOM ? custom.value.trim() : v) || FONT_MODERN;
      touch();
    });
    return UI.field(text, h('div', null, sel, custom));
  }

  function groupColor() {
    S.colorInputs = {};
    return h('div', null,
      label('Bộ màu dựng sẵn — bấm phát ăn ngay'), presetCards(),
      colorRow('Nền chính (bgDeep)', 'bgDeep'), colorRow('Chữ chính (textMain)', 'textMain'),
      colorRow('Màu nhấn (accent)', 'accent'),
      UI.select('Tông nền khung chữ', THEME_OPTS, S.cur.theme, function (v) { S.cur.theme = v; touch(); }),
      label('Bảng màu gợi ý cho AI sinh ảnh', 18), paletteEditor(),
      hint('💡 Bảng màu này chỉ gợi ý cho model sinh ảnh, không đổi màu chữ trong khung video.'),
      h('div', { style: { marginTop: '18px' } },
        fontField('Phông tiêu đề & số lớn', 'fontHead'), fontField('Phông nội dung & phụ đề', 'fontBody'),
        hint('💡 Chỉ dùng font có sẵn trên máy để render offline không lỗi chữ.')));
  }

  // ---------- Nhóm 4: Nhận diện & stock ----------
  function logoSection() {
    var st = S, host = h('div');
    var picker = filePicker('image/*', false, function (files) {
      UI.toast('Đang tải logo lên…');
      API.upload('/api/styles/' + encodeURIComponent(st.cur.id) + '/logo', [files[0]]).then(function (res) {
        if (!alive(st)) return;
        st.cur.logoPath = (res && res.logoPath) ? res.logoPath : ('styles/' + st.cur.id + '-logo.png');
        st.bust = Date.now(); UI.toast('Đã cập nhật logo');
        render(); loadPreview();
      }).catch(function (err) { UI.toast('Tải logo thất bại: ' + err.message, 'error'); });
    });

    function render() {
      host.innerHTML = '';
      host.appendChild(st.cur.logoPath
        ? h('img', { class: 'sk-logo', alt: 'Logo kênh', src: dataUrl(st.cur.logoPath) + '?t=' + st.bust })
        : h('div', { class: 'sk-hint', style: { margin: '0 0 8px' } },
          'Chưa có logo — khung video sẽ chỉ hiện tên kênh (nếu có).'));
      host.appendChild(h('div', { class: 'row', style: { marginTop: '8px' } },
        UI.btn(st.cur.logoPath ? '🖼 Đổi logo' : '⬆️ Tải logo lên',
          { variant: 'ghost', small: true, onclick: function () { picker.click(); } }),
        st.cur.logoPath ? UI.btn('🗑 Bỏ logo', { variant: 'ghost', small: true,
          onclick: function () { st.cur.logoPath = ''; render(); touch(); } }) : null,
        picker));
      host.appendChild(h('div', { style: { marginTop: '12px' } }, label('Vị trí logo & tên kênh'),
        segmented([{ value: 'left', label: 'Góc trái' }, { value: 'center', label: 'Giữa' },
          { value: 'right', label: 'Góc phải' }], st.cur.logoPos,
        function (v) { st.cur.logoPos = v; touch(); })));
    }
    render();
    return host;
  }

  function stockSection() {
    var st = S, host = h('div');
    var picker = filePicker('image/*,video/*', true, function (files) {
      UI.toast('Đang tải ' + files.length + ' tư liệu lên…');
      API.upload('/api/styles/' + encodeURIComponent(st.cur.id) + '/stock', files).then(function (res) {
        if (!alive(st)) return;
        if (res && Array.isArray(res.stockPaths)) st.cur.stockPaths = res.stockPaths;
        UI.toast('Đã thêm tư liệu nền');
        refreshServerFields(function () { render(); loadPreview(); });
      }).catch(function (err) { UI.toast('Tải tư liệu thất bại: ' + err.message, 'error'); });
    });

    function removeStock(p) {
      API.del('/api/styles/' + encodeURIComponent(st.cur.id) + '/stock?path=' + encodeURIComponent(p)).then(function () {
        if (!alive(st)) return;
        st.cur.stockPaths = (st.cur.stockPaths || []).filter(function (x) { return x !== p; });
        UI.toast('Đã gỡ tư liệu nền');
        render(); loadPreview();
      }).catch(function (err) { UI.toast('Gỡ tư liệu thất bại: ' + err.message, 'error'); });
    }

    function render() {
      host.innerHTML = ''; var list = st.cur.stockPaths || [];
      host.appendChild(h('div', { class: 'row' },
        UI.btn('＋ Thêm tư liệu nền', { variant: 'ghost', small: true, onclick: function () { picker.click(); } }),
        picker, h('span', { class: 'muted', style: { fontSize: '12px' } }, list.length + ' tệp')));
      if (!list.length) {
        host.appendChild(h('div', { class: 'sk-hint', style: { margin: '10px 0 0' } },
          'Chưa có tư liệu nào. Ảnh/video ở đây sẽ chạy làm nền dưới lớp chữ cho các cảnh không có ảnh riêng.'));
        return;
      }
      var grid = h('div', { class: 'sk-stock' });
      list.forEach(function (p) {
        grid.appendChild(h('div', { class: 'sk-stock-it', title: p },
          isVideo(p)
            ? h('div', { class: 'sk-stock-vid' }, h('span', { style: { fontSize: '20px' } }, '🎬'), h('span', null, baseName(p)))
            : h('img', { src: dataUrl(p), alt: baseName(p) }),
          h('button', { class: 'sk-stock-x', type: 'button', title: 'Gỡ tư liệu này',
            onclick: function () { removeStock(p); } }, '×')));
      });
      host.appendChild(grid);
    }
    render();
    return host;
  }

  function groupBrand() {
    return h('div', null,
      UI.field('Tên kênh hiển thị', bind(UI.input({ value: S.cur.channelName, placeholder: 'VD: Biz Studio' }), 'channelName')),
      hint('💡 Hiện ở đáy khung video cạnh logo. Trong Custom HTML chèn bằng biến ', h('code', null, '{{CHANNEL_NAME}}'), '.'),
      label('Logo kênh', 18), logoSection(),
      label('Thư viện tư liệu nền', 20), stockSection());
  }

  // ---------- Bố cục trang ----------
  function renderPanel() {
    S.panelHost.innerHTML = ''; S.sample = null;
    S.panelHost.appendChild([groupMotion, groupFrame, groupColor, groupBrand][S.tab]());
    Array.prototype.forEach.call(S.railHost.children, function (b, i) { b.classList.toggle('active', i === S.tab); });
  }

  function renderBody() {
    S.bodyHost.innerHTML = '';
    if (!S.cur) {
      S.bodyHost.appendChild(UI.card({
        body: h('div', { style: { textAlign: 'center', padding: '10px 0' } },
          UI.empty('Chưa có bộ mẫu nào. Tạo bộ đầu tiên để mọi video của bạn cùng một bộ mặt.', '🎨'),
          h('div', { style: { marginTop: '12px' } }, UI.btn('＋ Tạo mới', { onclick: createKit })))
      }));
      return;
    }
    S.railHost = h('div', { class: 'sk-rail' });
    GROUPS.forEach(function (g, i) {
      S.railHost.appendChild(h('button', {
        class: 'sk-tab' + (i === S.tab ? ' active' : ''), type: 'button',
        onclick: function () { S.tab = i; renderPanel(); }
      }, g.icon + '  ' + g.label, h('small', null, g.desc)));
    });
    S.panelHost = h('div');
    S.bodyHost.appendChild(h('div', { class: 'sk-main' },
      h('div', { class: 'sk-left' }, S.railHost, h('div', { class: 'card' }, S.panelHost)), previewPanel()));
    renderPanel(); loadPreview();
  }

  function renderBar() {
    S.barHost.innerHTML = '';
    if (!S.cur) return;
    var sel = h('select', { class: 'select' });
    S.list.forEach(function (k) {
      sel.appendChild(h('option', { value: k.id, selected: k.id === S.cur.id },
        (k.isDefault ? '★ ' : '') + (k.name || '(không tên)')));
    });
    sel.value = S.cur.id;
    sel.onchange = function () { selectKit(sel.value); };
    var nameInput = UI.input({ value: S.cur.name || '', placeholder: 'Tên bộ mẫu' });
    nameInput.oninput = function () {
      S.cur.name = nameInput.value;
      var opt = sel.options[sel.selectedIndex];
      if (opt) opt.textContent = (S.cur.isDefault ? '★ ' : '') + (nameInput.value || '(không tên)');
      touch();
    };
    S.saveInfo = h('span', { class: 'muted', style: { fontSize: '12px' } }, 'Tự lưu khi bạn chỉnh');
    S.barHost.appendChild(h('div', { class: 'card' },
      h('div', { class: 'row' },
        h('div', { style: { minWidth: '200px', flex: '1 1 200px' } }, label('Bộ mẫu đang sửa'), sel),
        h('div', { style: { minWidth: '200px', flex: '1 1 200px' } }, label('Tên bộ mẫu'), nameInput)),
      h('div', { class: 'row', style: { marginTop: '12px' } },
        UI.btn('💾 Lưu cài đặt', { small: true, onclick: function () {
          if (S.timer) { clearTimeout(S.timer); S.timer = null; }
          save(false).then(loadPreview);
        } }),
        UI.btn('＋ Tạo mới', { variant: 'ghost', small: true, onclick: createKit }),
        UI.btn('🗑 Xoá', { variant: 'danger', small: true, onclick: confirmDelete }),
        S.cur.isDefault
          ? h('span', { class: 'badge badge-green' }, '✓ Đang dùng cho toàn hệ thống')
          : UI.btn('✅ Đặt làm bộ đang dùng', { variant: 'ghost', small: true, onclick: setDefault }),
        S.saveInfo)));
  }

  // ---------- Thao tác với danh sách ----------
  function selectKit(id) {
    var st = S;
    flush().then(function () {
      if (!alive(st)) return;
      for (var i = 0; i < st.list.length; i++) if (st.list[i].id === id) { st.cur = normKit(st.list[i]); break; }
      st.dirty = false; st.tab = 0;
      renderBar(); renderBody();
    });
  }

  function loadList(keepId) {
    var st = S;
    st.bodyHost.innerHTML = '';
    st.bodyHost.appendChild(h('div', { class: 'empty' }, UI.spinner(), h('span', null, 'Đang tải bộ mẫu…')));
    return API.get('/api/styles').then(function (list) {
      if (!alive(st)) return;
      st.list = Array.isArray(list) ? list : []; var pick = null;
      st.list.forEach(function (k) { if (keepId && k.id === keepId) pick = k; });
      if (!pick) st.list.forEach(function (k) { if (!pick && k.isDefault) pick = k; });
      if (!pick && st.list.length) pick = st.list[0];
      st.cur = pick ? normKit(pick) : null; st.dirty = false;
      renderBar(); renderBody();
    }).catch(function (err) {
      if (!alive(st)) return;
      st.bodyHost.innerHTML = '';
      st.bodyHost.appendChild(UI.card({
        title: 'Không tải được danh sách bộ mẫu', icon: '❌',
        body: h('div', { class: 'text-red' }, err.message),
        foot: UI.btn('Thử lại', { variant: 'ghost', small: true, onclick: function () { loadList(keepId); } })
      }));
    });
  }

  // refreshServerFields — lấy lại field do máy chủ quản lý (logo, tư liệu, mặc định), không đè form.
  function refreshServerFields(then) {
    var st = S;
    API.get('/api/styles').then(function (list) {
      if (!alive(st) || !st.cur) return;
      st.list = Array.isArray(list) ? list : [];
      st.list.forEach(function (k) {
        if (k.id !== st.cur.id) return;
        st.cur.logoPath = k.logoPath || '';
        st.cur.stockPaths = Array.isArray(k.stockPaths) ? k.stockPaths : [];
        st.cur.isDefault = !!k.isDefault;
      });
      if (then) then();
    }).catch(function () { if (alive(st) && then) then(); });
  }

  function createKit() {
    var st = S;
    var body = {
      name: 'Bộ mẫu mới', theme: 'vivid', negative: 'text, watermark, cluttered background',
      stylePrompt: 'clean modern illustration, soft lighting, simple composition',
      palette: ['#7C3AED', '#EC4899', '#F8FAFC', '#111827'], bgDeep: '#0F0A1E', textMain: '#F8FAFC',
      accent: '#F59E0B', fontHead: FONT_MODERN, fontBody: FONT_MODERN, sizeTitle: 48, sizeBig: 150,
      sizeBody: 22, logoPos: 'left', baseTemplate: 'builtin', maxVoiceChars: 180, maxImageChars: 200
    };
    flush().then(function () { return API.post('/api/styles', body); }).then(function (k) {
      if (!alive(st)) return;
      UI.toast('Đã tạo bộ mẫu mới');
      loadList(k && k.id ? k.id : '');
    }).catch(function (err) { UI.toast('Tạo bộ mẫu thất bại: ' + err.message, 'error'); });
  }

  function setDefault() {
    var st = S;
    API.post('/api/styles/' + encodeURIComponent(st.cur.id) + '/default', {}).then(function () {
      if (!alive(st)) return;
      UI.toast('Đang dùng bộ mẫu: ' + (st.cur.name || st.cur.id));
      loadList(st.cur.id);
    }).catch(function (err) { UI.toast('Không đặt được bộ đang dùng: ' + err.message, 'error'); });
  }

  function confirmDelete() {
    var st = S, k = st.cur;
    var m = UI.modal({
      title: 'Xoá bộ mẫu?',
      body: h('div', null,
        h('p', { style: { margin: '0 0 6px' } },
          'Bộ mẫu "' + (k.name || k.id) + '" sẽ bị xoá vĩnh viễn. Không thể hoàn tác.'),
        k.isDefault ? h('p', { class: 'muted', style: { margin: '0', fontSize: '12.5px' } },
          'Đây đang là bộ dùng chung — hệ thống sẽ tự chuyển sang bộ mới nhất còn lại.') : null),
      actions: [UI.btn('Huỷ', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('🗑 Xoá bộ mẫu', { variant: 'danger', onclick: function () {
          m.close();
          if (st.timer) { clearTimeout(st.timer); st.timer = null; }
          st.dirty = false;
          API.del('/api/styles/' + encodeURIComponent(k.id)).then(function () {
            if (!alive(st)) return;
            UI.toast('Đã xoá bộ mẫu');
            loadList('');
          }).catch(function (err) { UI.toast('Xoá thất bại: ' + err.message, 'error'); });
        } })]
    });
  }

  function renderWarn() {
    if (!S || !S.warnHost) return;
    S.warnHost.innerHTML = '';
    var st = App.state;
    if (st && st.tools && !st.tools.geminiKey) {
      S.warnHost.appendChild(amberWarn('Chưa có Gemini API key — ảnh cảnh sẽ dùng card màu thay vì ảnh AI'));
    }
  }

  // ---------- Trang ----------
  App.pages.stylekit = {
    title: 'Mẫu thiết kế',
    subtitle: 'Bộ mẫu điều khiển toàn bộ giao diện video — màu, phông chữ, logo, tư liệu nền — chỉnh gì thấy nấy',
    render: function (el) {
      injectStyles();
      S = { list: [], cur: null, tab: 0, tpl: 'hero', frames: [], timer: null, dirty: false,
        dead: false, bust: Date.now(), colorInputs: {}, sample: null, sampleJobId: '' };

      el.appendChild(UI.card({
        title: 'Mẫu thiết kế là gì?', icon: '🎨',
        body: h('div', { class: 'muted', style: { fontSize: '13px', lineHeight: '1.65' } },
          h('p', { style: { margin: '0 0 6px' } },
            'Một bộ mẫu quy định cả chất ảnh AI lẫn bộ mặt khung video: màu nền, màu chữ, màu nhấn, phông chữ, ' +
            'cỡ chữ, logo & tên kênh, tư liệu nền và giới hạn độ dài kịch bản.'),
          h('p', { style: { margin: '0' } }, 'Bộ đang dùng áp cho ', h('strong', null, 'Vox-Director'), ', ',
            h('strong', null, 'HTML Video'), ' và ', h('strong', null, 'Text → Video'),
            '. Mọi chỉnh sửa được lưu tự động và hiện ngay ở khung xem trước.'))
      }));

      S.warnHost = h('div'); S.bodyHost = h('div');
      S.barHost = h('div', { style: { marginTop: '16px' } });
      el.appendChild(S.warnHost); el.appendChild(S.barHost); el.appendChild(S.bodyHost);
      renderWarn(); loadList('');

      var onState = function () { renderWarn(); };
      var onResize = function () { S.frames.forEach(function (f) { if (f.fit) f.fit(); }); };
      window.addEventListener('resize', onResize);
      var onJob = function (j) { // job ảnh thử phong cách
        if (!j || !j.id || !S.sample || j.id !== S.sampleJobId) return;
        S.sample.status.textContent = j.status === 'error' ? ('⚠ ' + (j.error || 'không tạo được ảnh thử'))
          : j.status === 'done' ? '✓ Đã tạo ảnh thử' : (j.detail || 'Đang sinh ảnh mẫu…');
        if (j.status === 'done') S.sample.img.src = samplePath(S.cur.id) + '?t=' + Date.now();
      };
      Bus.on('state', onState);
      Bus.on('job', onJob);

      App._cleanup = function () {
        S.dead = true;
        if (S.timer) { clearTimeout(S.timer); S.timer = null; }
        S.frames = [];
        window.removeEventListener('resize', onResize);
        Bus.off('state', onState); Bus.off('job', onJob);
      };
    }
  };
})();
