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

  // Ô nhập model + datalist gợi ý
  function modelField(label, st, key, placeholder, listId, suggestions) {
    var input = UI.input({
      value: st[key] || '', placeholder: placeholder,
      oninput: function (e) { st[key] = e.target.value; }
    });
    input.setAttribute('list', listId);
    var list = h('datalist', { id: listId }, suggestions.map(function (m) {
      return h('option', { value: m });
    }));
    return UI.field(label, h('div', null, input, list));
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
    openai: 'API Trực Tiếp', pexels: 'Pexels', chrome: 'Chrome'
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

    return h('div', { class: 'grid-2 mt-8' },
      textField('Gemini Base', st, 'geminiBase', 'https://generativelanguage.googleapis.com'),
      passwordField('API Key', st, 'geminiApiKey', 'Dán API key của bạn…'),
      modelField('Model', st, 'geminiModel', 'gemini-2.5-flash', 'gemini-model-list',
        ['gemini-2.5-flash', 'gemini-2.5-pro', 'gemini-3-flash-preview']),
      textField('Claude bin', st, 'claudeBin', 'claude'),
      textField('Claude model (tùy chọn)', st, 'claudeModel', 'VD: claude-sonnet-4-5'),
      textField('yt-dlp bin', st, 'ytdlpBin', 'yt-dlp'),
      textField('Thư mục tải về', st, 'downloadDir', 'data/downloads'),
      textField('File Cookies', st, 'cookiesFile', 'Đường dẫn file cookies.txt (tùy chọn)'),
      textField('Chrome bin (render HTML Video)', st, 'chromeBin', 'tự dò Google Chrome'),
      qualitySel,
      threadsSlider);
  }

  // Tab 2 — API Trực Tiếp (OpenAI-compatible)
  function directFields(st) {
    return h('div', { class: 'mt-8' },
      h('p', { class: 'muted', style: { margin: '0 0 12px' } },
        'Endpoint OpenAI-compatible — dùng cho Dịch thuật & phân tích cảnh: OpenAI, OpenRouter, LM Studio, Ollama (http://localhost:11434/v1)…'),
      h('div', { class: 'grid-2' },
        textField('Base URL', st, 'openaiBase', 'https://api.openai.com/v1'),
        passwordField('API Key', st, 'openaiKey', 'Dán API key của bạn…'),
        modelField('Model', st, 'openaiModel', 'gpt-4o-mini', 'openai-model-list',
          ['gpt-4o-mini', 'gpt-4o', 'llama3.1', 'qwen2.5'])));
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
