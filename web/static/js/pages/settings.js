/* ============================================================
   Biz Studio — Trang Cấu hình & API (settings)
   Load sau app.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var THEME_KEY = 'nova-theme';

  // ---------- CSS nội bộ (tabs, gradient, kết quả kiểm tra) ----------

  function injectStyles() {
    if (document.getElementById('settings-page-style')) return;
    var css = [
      '.tabs{display:flex;gap:2px;border-bottom:1px solid var(--border);margin-bottom:16px;flex-wrap:wrap}',
      '.tab{padding:9px 14px;font-size:13px;font-weight:600;font-family:inherit;color:var(--muted);background:none;border:none;border-bottom:2px solid transparent;margin-bottom:-1px;cursor:pointer}',
      '.tab.active{color:var(--blue);border-bottom-color:var(--blue)}',
      '.tab:disabled{opacity:.45;cursor:not-allowed}',
      '.api-status-row{display:flex;align-items:center;gap:10px;margin-bottom:14px;flex-wrap:wrap}',
      '.api-check-line{display:flex;align-items:baseline;gap:8px;font-size:13px;padding:5px 0}',
      '.settings-grid > .card + .card{margin-top:0}',
      'body.gradient-bg{background:linear-gradient(120deg,#eef3ff,#f5f7fb,#eafaf3,#f2effe);background-size:300% 300%;animation:novaGradientMove 22s ease infinite}',
      '[data-theme=dark] body.gradient-bg{background:linear-gradient(120deg,#0b1220,#101c33,#0c1b26,#151229);background-size:300% 300%;animation:novaGradientMove 22s ease infinite}',
      '@keyframes novaGradientMove{0%{background-position:0% 50%}50%{background-position:100% 50%}100%{background-position:0% 50%}}'
    ].join('\n');
    document.head.appendChild(h('style', { id: 'settings-page-style' }, css));
  }

  // ---------- Áp dụng cài đặt giao diện ngay ----------

  function applyUiScale(scale) {
    document.documentElement.style.fontSize = (Number(scale) || 100) + '%';
  }

  function applyTheme(theme) {
    var t = theme === 'dark' ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', t);
    try { localStorage.setItem(THEME_KEY, t); } catch (e) { /* private mode */ }
    var btn = document.getElementById('themeToggle');
    if (btn) btn.textContent = t === 'dark' ? '☀️' : '🌙';
  }

  function applyGradient(on) {
    document.body.classList.toggle('gradient-bg', !!on);
  }

  // ---------- Modal xác nhận ----------

  function confirmModal(opts) {
    var m = UI.modal({
      title: opts.title,
      body: h('p', { class: 'muted', style: { margin: '0' } }, opts.message),
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn(opts.okLabel || 'Đồng ý', {
          variant: opts.variant || 'primary',
          onclick: function () { m.close(); opts.onok(); }
        })
      ]
    });
  }

  // ---------- Field helpers ----------

  function textField(label, st, key, placeholder) {
    return UI.field(label, UI.input({
      value: st[key] || '', placeholder: placeholder,
      oninput: function (e) { st[key] = e.target.value; }
    }));
  }

  // Ô nhập bí mật (API key) + nút hiện/ẩn 👁
  function passwordField(label, st, key, placeholder) {
    var input = UI.input({
      type: 'password', value: st[key] || '', placeholder: placeholder,
      oninput: function (e) { st[key] = e.target.value; }
    });
    input.style.flex = '1';
    input.style.minWidth = '0';
    var eyeBtn = h('button', {
      class: 'icon-btn', type: 'button', title: 'Hiện / ẩn API key',
      onclick: function () {
        input.type = input.type === 'password' ? 'text' : 'password';
        eyeBtn.textContent = input.type === 'password' ? '👁' : '🙈';
      }
    }, '👁');
    return UI.field(label, h('div', { style: { display: 'flex', gap: '8px', alignItems: 'center' } }, input, eyeBtn));
  }

  // Field + dòng ghi chú giải thích ngay dưới (gói thành 1 ô của grid)
  function withNote(fieldEl, note) {
    return h('div', null, fieldEl,
      h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '-8px' } }, note));
  }

  // ---------- Chọn model: nạp thẳng từ API thay vì gõ tay ----------
  //
  // Danh sách model của Google đổi liên tục — model mới ra, model preview bị
  // gỡ. Gõ tay hoặc để sẵn gợi ý trong mã nguồn thì chỉ đúng vào ngày viết,
  // sau đó người dùng chọn phải model không còn tồn tại và nhận lỗi khó hiểu.
  // Nút "Nạp danh sách" hỏi thẳng API bằng khoá đang khai (lệnh đọc, miễn phí).

  var MODEL_CACHE = null;      // kết quả nạp gần nhất, dùng chung cho mọi ô
  var MODEL_FIELDS = [];       // các ô đang chờ danh sách

  // Ô model: mặc định là ô gõ tay (để vẫn dùng được khi chưa có khoá / mất
  // mạng), nạp xong thì biến thành danh sách chọn.
  function modelField(label, st, key, placeholder, group) {
    var input = UI.input({
      value: st[key] || '', placeholder: placeholder,
      oninput: function (e) { st[key] = e.target.value; }
    });
    var host = h('div', null, input);
    var note = h('div', { class: 'muted', style: { fontSize: '11.5px', marginTop: '4px' } },
      'Đang gõ tay — bấm “Nạp danh sách model” ở trên để chọn từ API.');

    var field = { key: key, group: group, host: host, note: note, st: st };
    MODEL_FIELDS.push(field);
    if (MODEL_CACHE) applyModels(field, MODEL_CACHE);
    return UI.field(label, h('div', null, host, note));
  }

  // applyModels đổi ô gõ tay thành ô chọn, giữ nguyên giá trị đang có.
  function applyModels(field, groups) {
    var list = (groups && groups[field.group]) || [];
    if (!list.length) {
      field.note.textContent = 'API không trả về model nào cho mục này — giữ ô gõ tay.';
      return;
    }
    var cur = field.st[field.key] || '';
    var opts = list.map(function (m) {
      return { value: m.id, label: m.id + (m.name && m.name !== m.id ? ' — ' + m.name : '') };
    });
    // Model đang đặt mà không còn trong danh sách: vẫn giữ lại nhưng nói rõ,
    // KHÔNG âm thầm đổi sang model khác — đó là cấu hình của người dùng.
    var known = list.some(function (m) { return m.id === cur; });
    if (cur && !known) {
      opts.unshift({ value: cur, label: cur + ' — ⚠ không còn trong danh sách API' });
    }
    if (!cur) cur = list[0].id;
    field.st[field.key] = cur;

    var sel = UI.select(null, opts, cur, function (v) { field.st[field.key] = v; });
    field.host.innerHTML = '';
    field.host.appendChild(sel);
    field.note.textContent = cur && !known
      ? '⚠ Model đang đặt không còn trong danh sách API — nên chọn lại.'
      : 'Đã nạp ' + list.length + ' model từ API.';
    field.note.style.color = (cur && !known) ? 'var(--red)' : '';
  }

  // loadModelsBtn — nút nạp dùng chung cho tất cả các ô model.
  function loadModelsBtn() {
    var btn = UI.btn('↻ Nạp danh sách model từ API', {
      variant: 'ghost',
      onclick: function () {
        btn.disabled = true;
        var old = btn.textContent;
        btn.textContent = 'Đang hỏi Google…';
        API.get('/api/tools/models').then(function (g) {
          MODEL_CACHE = g;
          MODEL_FIELDS.forEach(function (f) { applyModels(f, g); });
          UI.toast('Đã nạp ' + g.total + ' model từ API.');
        }).catch(function (err) {
          UI.toast('Không nạp được danh sách model: ' + err.message, 'error');
        }).finally(function () { btn.disabled = false; btn.textContent = old; });
      }
    });
    return h('div', { style: { marginBottom: '12px' } }, btn,
      h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '6px' } },
        'Lấy đúng danh sách model khoá của bạn dùng được — đây là lệnh đọc, không tốn tiền. ' +
        'Gõ tay dễ chọn nhầm model đã bị Google gỡ và nhận lỗi khó hiểu.'));
  }

  // ---------- Lưới card cài đặt giao diện ----------

  function uiCards(st, container) {
    var scaleSel = UI.select(null,
      [{ value: '90', label: '90%' }, { value: '100', label: '100% (chuẩn)' },
       { value: '110', label: '110%' }, { value: '125', label: '125%' }],
      String(st.uiScale || 100),
      function (v) { st.uiScale = Number(v); applyUiScale(st.uiScale); });

    var themeSel = UI.select(null,
      [{ value: 'light', label: 'Sáng' }, { value: 'dark', label: 'Tối' }],
      st.theme === 'dark' ? 'dark' : 'light',
      function (v) { st.theme = v; applyTheme(v); });

    var perfSel = UI.select(null,
      [{ value: 'auto', label: 'Tự động (khuyên dùng)' },
       { value: 'eco', label: 'Tiết kiệm' },
       { value: 'high', label: 'Hiệu năng cao' }],
      st.perfMode || 'auto',
      function (v) { st.perfMode = v; });

    var cleanBtn = UI.btn('🗑 Chọn dọn...', {
      variant: 'ghost',
      onclick: function () {
        confirmModal({
          title: 'Dọn file tạm',
          message: 'Xóa toàn bộ file tạm trong data/tmp và thư mục tmp của các dự án. Thao tác không ảnh hưởng asset hay video đã xuất. Tiếp tục?',
          okLabel: '🗑 Dọn ngay', variant: 'danger',
          onok: function () {
            cleanBtn.disabled = true;
            API.post('/api/settings/cleanup').then(function (res) {
              var mb = res && res.freedMB !== undefined ? Math.round(Number(res.freedMB) * 10) / 10 : 0;
              UI.toast('Đã giải phóng ' + mb + ' MB');
            }).catch(function (err) {
              UI.toast('Dọn file tạm thất bại: ' + err.message, 'error');
            }).finally(function () { cleanBtn.disabled = false; });
          }
        });
      }
    });

    container.appendChild(h('div', { class: 'grid-3 settings-grid' },
      UI.card({ title: 'Kích thước giao diện', icon: '🖥️', desc: 'Phóng to / thu nhỏ chữ toàn giao diện, áp dụng ngay.', body: scaleSel }),
      UI.card({
        title: 'Chế độ giao diện', icon: '🎨',
        desc: 'Biz là giao diện mặc định: nhẹ, rực rỡ, đồng nhất sáng/tối.',
        body: [h('div', { style: { marginBottom: '10px' } }, h('span', { class: 'badge badge-blue' }, 'BIZ')), themeSel]
      }),
      UI.card({
        title: 'Nền gradient chuyển động', icon: '🌈',
        body: UI.toggle('Bật hiệu ứng gradient', 'Nền chuyển màu nhẹ nhàng phía sau vùng làm việc.', !!st.gradientBg,
          function (v) { st.gradientBg = v; applyGradient(v); })
      }),
      UI.card({ title: 'Chế độ hiệu năng', icon: '⚡', desc: 'Cân bằng tốc độ render và tài nguyên máy.', body: perfSel }),
      UI.card({ title: 'Dọn file tạm', icon: '🧹', desc: 'Giải phóng dung lượng bằng cách xóa file tạm của hệ thống và các dự án.', body: cleanBtn })
    ));

    container.appendChild(h('div', { class: 'card mt-16' },
      UI.toggle('Nhớ bản dịch', 'Lưu các bản dịch gần đây để tái xuất nhanh.', !!st.rememberTranslations,
        function (v) { st.rememberTranslations = v; }),
      UI.toggle('Cache TTS block', 'Lưu tạm block âm thanh để tăng tốc.', !!st.cacheTts,
        function (v) { st.cacheTts = v; })
    ));
  }

  // ---------- Panel API (3 tab: Server Chung / Trực Tiếp / Media Xu hướng) ----------

  // Nhãn đẹp cho từng key trong kết quả POST /api/settings/test; key lạ hiện nguyên.
  var TEST_LABELS = {
    gemini: 'Gemini API', claude: 'Claude CLI', ffmpeg: 'FFmpeg', ytdlp: 'yt-dlp',
    openai: 'API Trực Tiếp', pexels: 'Pexels', chrome: 'Chrome', vieneu: 'VieNeu-TTS',
    whisper: 'faster-whisper'
  };

  // Tab 1 — API Server Chung
  function serverFields(st) {
    var qualitySel = UI.select('Chất lượng mặc định',
      [{ value: 'best', label: 'Tốt nhất (best)' }, { value: '1080p', label: '1080p' },
       { value: '720p', label: '720p' }, { value: '480p', label: '480p' },
       { value: 'audio', label: 'Chỉ âm thanh' }],
      st.quality || 'best',
      function (v) { st.quality = v; });

    var threadsSlider = UI.slider('Luồng tải (song song)', {
      min: 1, max: 8, step: 1, value: st.threads || 3,
      oninput: function (v) { st.threads = v; }
    });

    return h('div', { class: 'mt-8' }, loadModelsBtn(), h('div', { class: 'grid-2' },
      textField('Gemini Base', st, 'geminiBase', 'https://generativelanguage.googleapis.com'),
      passwordField('API Key', st, 'geminiApiKey', 'Dán API key của bạn…'),
      withNote(
        modelField('Model văn bản', st, 'geminiModel', 'vd: gemini-flash-latest', 'text'),
        'Dùng cho tách cảnh, dịch thuật, viết kịch bản.'),
      withNote(
        modelField('Model sinh ảnh', st, 'geminiImageModel', 'rỗng = gemini-2.5-flash-image', 'image'),
        'Dùng cho storyboard và thumbnail. Model văn bản KHÔNG sinh được ảnh nên phải chọn riêng.'),
      withNote(
        modelField('Model đọc giọng (Gemini TTS)', st, 'geminiTtsModel', 'rỗng = gemini-2.5-flash-preview-tts', 'tts'),
        'Chỉ dùng khi chọn engine Gemini; giọng VieNeu trên máy không cần model này.'),
      withNote(
        passwordField('Khoá Veo — sinh video AI (TRẢ PHÍ)', st, 'veoApiKey',
          'Để trống = dùng chung khoá Gemini ở trên'),
        '⚠️ Veo tính tiền theo GIÂY video và không có bậc miễn phí — dự án Google phải bật thanh toán. ' +
        'Clip 8 giây tốn khoảng $0.40 (lite) đến $3.20 (chuẩn). Chi phí luôn hiện trước khi bấm tạo.'),
      withNote(
        UI.select('Avatar nói — engine LongCat', [
          { value: '', label: 'Tắt' },
          { value: 'local', label: 'Chạy trên máy này (cần GPU NVIDIA)' },
          { value: 'remote', label: 'Đẩy sang máy GPU khác' }
        ], st.longcatMode || '', function (v) { st.longcatMode = v; }),
        'LongCat-Video-Avatar là model 13,6 tỉ tham số, BẮT BUỘC GPU NVIDIA — không có bản cho Apple Silicon hay CPU. ' +
        'Máy thường vẫn dùng được bằng chế độ "remote": cài trên máy GPU bằng ./scripts/setup-longcat.sh rồi chạy scripts/longcat-worker.py ở đó.'),
      withNote(
        textField('Địa chỉ máy GPU (chế độ remote)', st, 'longcatWorkerUrl', 'http://192.168.1.50:7070'),
        'Địa chỉ máy đang chạy longcat-worker.py.'),
      textField('LongCat — thư mục mã nguồn', st, 'longcatRepo', 'vd: data/longcat/LongCat-Video'),
      textField('LongCat — thư mục trọng số', st, 'longcatCheckpoint', 'vd: …/weights/LongCat-Video-Avatar-1.5'),
      textField('LongCat — python', st, 'longcatPython', 'rỗng = dò venv cạnh mã nguồn'),
      withNote(
        modelField('Model Veo', st, 'veoModel', 'rỗng = veo-3.1-fast-generate-preview', 'video'),
        'Bấm nạp danh sách để lấy đúng model Veo đang phục vụ — Google đã gỡ hẳn các model Veo 3.0 cũ.'),
      textField('Claude bin', st, 'claudeBin', 'claude'),
      textField('Claude model', st, 'claudeModel', 'Mặc định: claude-opus-5'),
      textField('yt-dlp bin', st, 'ytdlpBin', 'yt-dlp'),
      textField('Thư mục tải về', st, 'downloadDir', 'data/downloads'),
      textField('File Cookies', st, 'cookiesFile', 'Đường dẫn file cookies.txt (tùy chọn)'),
      textField('Chrome bin (render HTML Video)', st, 'chromeBin', 'tự dò Google Chrome'),
      textField('VieNeu-TTS python (giọng đọc Việt on-device)', st, 'vieneuPython', 'tự dò data/vieneu/venv — rỗng là được, bấm Cài ở mục Công cụ trên máy'),
      withNote(
        textField('faster-whisper python (bóc băng offline, mốc từng từ)', st, 'whisperPython',
          'tự dò data/whisper/venv — rỗng là được, bấm Cài ở mục Công cụ trên máy'),
        'Bóc băng ngay trên máy, không tốn API key — và cho mốc từng từ để cắt khoảng lặng an toàn, làm phụ đề karaoke.'),
      withNote(
        UI.select('Model whisper', [
          { value: 'small', label: 'small — nhanh, nhẹ (mặc định)' },
          { value: 'medium', label: 'medium — cân bằng' },
          { value: 'large-v3', label: 'large-v3 — chính xác nhất' }
        ], st.whisperModel || 'small', function (v) { st.whisperModel = v; }),
        'Lớn hơn = chính xác hơn nhưng chậm và nặng hơn (large-v3 tải về vài GB).'),
      withNote(
        UI.select('Compute', [
          { value: 'auto', label: 'auto — để máy tự chọn (mặc định)' },
          { value: 'int8', label: 'int8 — nhẹ RAM, chạy tốt trên CPU' },
          { value: 'float16', label: 'float16 — nhanh hơn khi có GPU' }
        ], st.whisperCompute || 'auto', function (v) { st.whisperCompute = v; }),
        'Đổi model hoặc compute xong nhớ Lưu cấu hình rồi bấm “Kiểm tra kết nối”.'),
      qualitySel,
      threadsSlider));
  }

  // Tab 2 — API Trực Tiếp (OpenAI-compatible)
  function directFields(st) {
    return h('div', { class: 'mt-8' },
      h('p', { class: 'muted', style: { margin: '0 0 12px' } },
        'Endpoint OpenAI-compatible — dùng cho Dịch thuật & phân tích cảnh: OpenAI, OpenRouter, LM Studio, Ollama (http://localhost:11434/v1)…'),
      h('div', { class: 'grid-2' },
        textField('Base URL', st, 'openaiBase', 'https://api.openai.com/v1'),
        passwordField('API Key', st, 'openaiKey', 'Dán API key của bạn…'),
        textField('Model', st, 'openaiModel', 'vd: gpt-4o-mini, llama3.1, qwen2.5')));
  }

  // Tab 3 — Media Xu hướng (Pexels)
  function mediaFields(st) {
    return h('div', { class: 'mt-8' },
      h('p', { class: 'muted', style: { margin: '0 0 12px' } },
        'Kho media stock theo từ khóa — tự chèn ảnh cho cảnh Vox/HTML Video khi có MediaKeyword.'),
      h('div', { class: 'grid-2' },
        passwordField('Pexels API Key', st, 'pexelsKey', 'Dán Pexels API key…')),
      h('p', { style: { margin: '10px 0 0', fontSize: '13px' } },
        h('a', { href: 'https://www.pexels.com/api/', target: '_blank', rel: 'noopener' },
          'Lấy key miễn phí tại pexels.com/api')));
  }

  function apiPanel(st) {
    var statusDot = h('span', { class: 'muted' }, '●');
    var statusText = h('span', { class: 'muted' }, 'Chưa kiểm tra');
    var resultsBox = h('div');

    function setStatus(cls, text) {
      statusDot.className = cls;
      statusText.className = cls;
      statusText.textContent = text;
    }

    // Render ĐỘNG mọi key server trả về — không hardcode danh sách.
    function renderResults(res) {
      resultsBox.innerHTML = '';
      var keys = Object.keys(res || {});
      if (!keys.length) {
        resultsBox.appendChild(h('div', { class: 'api-check-line text-red' }, '✗ không có dữ liệu trả về'));
        return false;
      }
      var allOk = true;
      keys.forEach(function (k) {
        var r = res[k] || {};
        if (!r.ok) allOk = false;
        resultsBox.appendChild(h('div', { class: 'api-check-line' },
          h('span', { class: r.ok ? 'text-green' : 'text-red' }, r.ok ? '✓' : '✗'),
          h('strong', null, TEST_LABELS[k] || k),
          h('span', { class: 'muted', style: { wordBreak: 'break-word' } }, r.detail || '')));
      });
      return allOk;
    }

    var testBtn = UI.btn('🔄 Kiểm tra kết nối', {
      variant: 'ghost', small: true,
      onclick: function () {
        testBtn.disabled = true;
        setStatus('text-amber', 'Đang kiểm tra…');
        resultsBox.innerHTML = '';
        resultsBox.appendChild(h('div', { class: 'api-check-line muted' }, UI.spinner(), ' Đang kiểm tra kết nối…'));
        API.post('/api/settings/test').then(function (res) {
          var ok = renderResults(res);
          setStatus(ok ? 'text-green' : 'text-red', ok ? 'API Online' : 'Có mục lỗi — xem chi tiết bên dưới');
        }).catch(function (err) {
          resultsBox.innerHTML = '';
          setStatus('text-red', 'Kiểm tra thất bại');
          resultsBox.appendChild(h('div', { class: 'api-check-line text-red' }, '✗ ' + err.message));
        }).finally(function () { testBtn.disabled = false; });
      }
    });

    // 3 tab thật — chuyển tab chỉ đổi phần fields; mọi giá trị chung 1 state st.
    var panels = [serverFields(st), directFields(st), mediaFields(st)];
    var tabBtns = [];

    function showTab(idx) {
      tabBtns.forEach(function (b, i) { b.classList.toggle('active', i === idx); });
      panels.forEach(function (p, i) { p.style.display = i === idx ? '' : 'none'; });
    }

    ['API Server Chung', 'API Trực Tiếp', 'Media Xu hướng'].forEach(function (name, i) {
      tabBtns.push(h('button', {
        class: i === 0 ? 'tab active' : 'tab', type: 'button',
        onclick: function () { showTab(i); }
      }, name));
    });
    panels.forEach(function (p, i) { if (i > 0) p.style.display = 'none'; });

    return h('div', { class: 'card mt-16' },
      h('div', { class: 'tabs' }, tabBtns),
      h('div', { class: 'api-status-row' }, statusDot, statusText, testBtn),
      resultsBox,
      panels);
  }

  // ---------- Hàng hành động ----------

  function actionRow(st, el) {
    var saveBtn = UI.btn('💾 Lưu cấu hình', {
      onclick: function () {
        st.uiScale = Number(st.uiScale) || 100;
        st.threads = Number(st.threads) || 3;
        saveBtn.disabled = true;
        API.put('/api/settings', st).then(function () {
          UI.toast('Đã lưu cấu hình');
        }).catch(function (err) {
          UI.toast('Lưu cấu hình thất bại: ' + err.message, 'error');
        }).finally(function () { saveBtn.disabled = false; });
      }
    });

    var resetBtn = UI.btn('↺ Đặt lại về mặc định', {
      variant: 'ghost',
      onclick: function () {
        confirmModal({
          title: 'Đặt lại cấu hình',
          message: 'Toàn bộ cấu hình (bao gồm API key) sẽ được đưa về giá trị mặc định. Tiếp tục?',
          okLabel: '↺ Đặt lại', variant: 'danger',
          onok: function () {
            resetBtn.disabled = true;
            API.put('/api/settings', {}).then(function () {
              UI.toast('Đã đặt lại cấu hình mặc định');
              load(el, true);
            }).catch(function (err) {
              UI.toast('Đặt lại thất bại: ' + err.message, 'error');
              resetBtn.disabled = false;
            });
          }
        });
      }
    });

    return h('div', { class: 'row mt-16', style: { justifyContent: 'flex-end' } }, resetBtn, saveBtn);
  }

  // ---------- Load + build ----------

  function buildForm(el, st) {
    uiCards(st, el);
    // Đặt trên panel API: công cụ thiếu là thứ chặn người dùng ngay từ thao tác
    // đầu tiên, còn API key thì họ mới dán xong ở bước cài đặt ban đầu.
    if (window.SettingsTools) el.appendChild(SettingsTools.card());
    el.appendChild(apiPanel(st));
    el.appendChild(actionRow(st, el));
  }

  function load(el, applyThemeToo) {
    el.innerHTML = '';
    el.appendChild(h('div', { class: 'empty' }, UI.spinner(), h('span', null, 'Đang tải cấu hình…')));
    API.get('/api/settings').then(function (st) {
      st = st || {};
      el.innerHTML = '';
      buildForm(el, st);
      applyUiScale(st.uiScale || 100);
      applyGradient(st.gradientBg);
      if (applyThemeToo) applyTheme(st.theme);
    }).catch(function (err) {
      el.innerHTML = '';
      el.appendChild(UI.card({
        title: 'Không tải được cấu hình', icon: '❌',
        body: h('div', { class: 'text-red' }, err.message),
        foot: UI.btn('Thử lại', { variant: 'ghost', small: true, onclick: function () { load(el, applyThemeToo); } })
      }));
    });
  }

  App.pages['settings'] = {
    title: 'Cấu hình & API',
    subtitle: 'Cài đặt giao diện, hiệu năng và kết nối API của studio',
    render: function (el) {
      injectStyles();
      load(el, false);
    }
  };
})();
