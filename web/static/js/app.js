/* ============================================================
   Biz Studio — App shell: nav, router, theme, status
   Load sau api.js + ui.js, trước js/pages/*.js.
   ============================================================ */
(function () {
  'use strict';

  var NAV = [
    { group: 'BẢNG ĐIỀU KHIỂN', items: [
      { id: 'dashboard', label: 'Tổng quan', icon: '📊' }
    ]},
    { group: 'MODULE SÁNG TẠO', items: [
      { id: 'download', label: 'Tải Video', icon: '⬇️' },
      { id: 'ocr', label: 'OCR / ASR', icon: '📝' },
      { id: 'translate', label: 'Dịch thuật', icon: '🌐' },
      { id: 'tts', label: 'TTS / Giọng đọc', icon: '🎙️' },
      { id: 'article', label: 'Bài viết → Video', icon: '📰' },
      { id: 'vox', label: 'Vox-Director', icon: '🎬' },
      { id: 'htmlvideo', label: 'HTML Video', icon: '🧩' },
      { id: 'text2video', label: 'Text → Video', icon: '📜' },
      { id: 'stylekit', label: 'Style Kit', icon: '🎨' },
      { id: 'characters', label: 'Nhân vật', icon: '🧑‍🎤' },
      { id: 'ideas', label: 'Ý tưởng & Hàng đợi', icon: '💡' },
      { id: 'look', label: 'Diện mạo', icon: '🌈' },
      { id: 'veo', label: 'Veo — Sinh video AI', icon: '🎥' },
      { id: 'avatar', label: 'Avatar nói', icon: '🗣️' },
      { id: 'editor', label: 'Studio Editor', icon: '✂️' }
    ]},
    { group: 'HỆ THỐNG', items: [
      { id: 'projects', label: 'Dự án', icon: '📁' },
      { id: 'settings', label: 'Cấu hình & API', icon: '⚙️' },
      { id: 'logs', label: 'Nhật ký', icon: '🧾' }
    ]}
  ];

  var THEME_KEY = 'nova-theme';

  window.App = {
    pages: {},          // pages tự đăng ký: App.pages['id'] = {title, subtitle, render(el, param)}
    state: null,        // cache /api/state gần nhất
    _cleanup: null,     // page có thể gán hàm dọn dẹp, gọi khi rời trang
    navigate: function (p) { location.hash = '#/' + p; },
    setHeader: setHeader
  };

  function $(id) { return document.getElementById(id); }

  // ---------- Sidebar nav ----------

  function renderNav() {
    var host = $('navGroups');
    host.innerHTML = '';
    NAV.forEach(function (g) {
      host.appendChild(h('div', { class: 'nav-group-label' }, g.group));
      g.items.forEach(function (it) {
        host.appendChild(h('div', {
          class: 'nav-item', dataset: { nav: it.id },
          onclick: function () { App.navigate(it.id); }
        }, h('span', { class: 'nav-icon' }, it.icon), h('span', null, it.label)));
      });
    });
  }

  function setActiveNav(id) {
    document.querySelectorAll('.nav-item').forEach(function (el) {
      el.classList.toggle('active', el.dataset.nav === id);
    });
  }

  // ---------- Header ----------

  function setHeader(title, subtitle) {
    $('pageTitle').textContent = title || '';
    $('pageSubtitle').textContent = subtitle || '';
  }

  // ---------- Router ----------

  function parseHash() {
    var raw = location.hash.replace(/^#\/?/, '');
    if (!raw) return { id: 'dashboard', param: '' };
    var i = raw.indexOf('/');
    if (i < 0) return { id: raw, param: '' };
    return { id: raw.slice(0, i), param: decodeURIComponent(raw.slice(i + 1)) };
  }

  function route() {
    var r = parseHash();
    if (typeof App._cleanup === 'function') {
      try { App._cleanup(); } catch (e) { console.error('Lỗi cleanup trang cũ:', e); }
    }
    App._cleanup = null;

    var ws = $('workspace');
    ws.innerHTML = '';
    ws.scrollTop = 0;
    setActiveNav(r.id);

    var def = App.pages[r.id];
    if (!def) {
      setHeader('Trang đang phát triển', 'Chức năng "' + r.id + '" sẽ sớm có mặt.');
      ws.appendChild(UI.empty('Trang đang phát triển — quay lại sau nhé!', '🚧'));
      return;
    }
    setHeader(def.title || r.id, def.subtitle || '');
    try {
      def.render(ws, r.param);
    } catch (err) {
      console.error('Lỗi render trang [' + r.id + ']:', err);
      ws.innerHTML = '';
      ws.appendChild(UI.card({
        title: 'Đã xảy ra lỗi khi hiển thị trang',
        icon: '❌',
        body: h('div', { class: 'text-red' }, String(err && err.message ? err.message : err))
      }));
    }
  }

  // ---------- Theme ----------

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    var btn = $('themeToggle');
    if (btn) btn.textContent = theme === 'dark' ? '☀️' : '🌙';
  }

  function initTheme() {
    var saved = 'light';
    try { saved = localStorage.getItem(THEME_KEY) || 'light'; } catch (e) { /* private mode */ }
    applyTheme(saved);
    $('themeToggle').onclick = function () {
      var cur = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
      try { localStorage.setItem(THEME_KEY, cur); } catch (e) { /* bỏ qua */ }
      applyTheme(cur);
    };
  }

  // ---------- Status bar + sidebar disk (poll /api/state 5s) ----------

  function pct(v) { return (v === undefined || v === null) ? '--' : Math.round(Number(v)); }

  function renderState(st) {
    App.state = st;
    var host = st.host || {};
    var counts = st.counts || {};

    // Trạng thái studio
    var dot = $('studioDot'), txt = $('studioStatus');
    var running = counts.jobsRunning || 0;
    if (running > 0) {
      dot.className = 'status-dot busy';
      txt.className = 'status-text busy';
      txt.textContent = 'Đang chạy ' + running + ' tác vụ';
    } else {
      dot.className = 'status-dot';
      txt.className = 'status-text';
      txt.textContent = 'Sẵn sàng';
    }

    // Thông số máy
    $('hostStats').textContent = 'Máy CPU: ' + pct(host.cpuPct) + '% | GPU: -- | RAM: ' +
      pct(host.ramPct) + '% | Disk: ' + pct(host.diskPct) + '%';
    $('ramUsed').textContent = 'RAM: ' + (host.ramUsedMB || 0) + ' MB';

    // Backend OK
    $('backendDot').className = 'backend-dot';
    $('backendText').textContent = 'Backend hoạt động';

    // Sidebar: dung lượng + version
    var dp = Number(host.diskPct) || 0;
    $('diskPctLabel').textContent = pct(host.diskPct) + '%';
    var fill = $('diskFill');
    fill.style.width = Math.max(0, Math.min(100, dp)) + '%';
    fill.className = 'progress-fill' + (dp >= 90 ? ' red' : '');
    if (st.app) $('appVersion').textContent = (st.app.name || 'Biz Studio') + ' v' + (st.app.version || '');
  }

  function renderStateError() {
    $('backendDot').className = 'backend-dot err';
    $('backendText').textContent = 'Mất kết nối backend';
    var dot = $('studioDot'), txt = $('studioStatus');
    dot.className = 'status-dot err';
    txt.className = 'status-text err';
    txt.textContent = 'Không kết nối được';
  }

  function pollState() {
    API.get('/api/state').then(function (st) {
      renderState(st);
      Bus.emit('state', st);
    }).catch(function (err) {
      console.error('Lỗi lấy /api/state:', err.message);
      renderStateError();
    });
  }

  // ---------- Init ----------

  document.addEventListener('DOMContentLoaded', function () {
    renderNav();
    initTheme();
    $('settingsBtn').onclick = function () { App.navigate('settings'); };
    window.addEventListener('hashchange', route);
    if (!location.hash) {
      location.hash = '#/dashboard'; // hashchange sẽ gọi route()
    } else {
      route();
    }
    pollState();
    setInterval(pollState, 5000);
  });
})();
