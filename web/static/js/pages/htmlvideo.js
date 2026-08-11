/* ============================================================
   Biz Studio — Trang "HTML Video"
   Video-as-code: prompt / bài viết / repo GitHub → cảnh HTML → MP4.
   Đăng ký App.pages['htmlvideo']. Load sau api.js/ui.js/app.js.
   ============================================================ */
(function () {
  'use strict';

  // Khuôn chọn ở trang Xưởng được nạp vào đây: điền sẵn khung hình và ghép
  // hướng viết kịch bản vào ô prompt. Dùng MỘT LẦN rồi xoá — lần sau vào trang
  // này mà không chọn khuôn thì phải là trang trắng, không dính khuôn cũ.
  var PREFILL_KEY = 'biz-template-prefill';

  function takePrefill() {
    try {
      var raw = localStorage.getItem(PREFILL_KEY);
      if (!raw) return null;
      localStorage.removeItem(PREFILL_KEY);
      return JSON.parse(raw);
    } catch (e) { return null; }
  }

  // applyPrefill điền khuôn vào state; trả về khuôn đã dùng (hoặc null).
  function applyPrefill(st) {
    var t = takePrefill();
    if (!t) return null;
    if (t.aspect) st.cfg.aspect = t.aspect;
    if (t.seconds) st.targetSeconds = t.seconds;
    var guide = [t.script, t.hook && ('Mở đầu: ' + t.hook), t.body && ('Thân: ' + t.body), t.cta && ('Chốt: ' + t.cta)]
      .filter(Boolean).join('\n');
    st.templateGuide = guide;
    st.templateName = t.name;
    return t;
  }

  var TEMPLATES = [
    { value: 'hero', label: 'Mở đầu (hero)' },
    { value: 'bullets', label: 'Ý chính (bullets)' },
    { value: 'code', label: 'Code' },
    { value: 'chart', label: 'Biểu đồ (chart)' },
    { value: 'product', label: 'Sản phẩm / Ảnh' },
    { value: 'quote', label: 'Trích dẫn' },
    { value: 'outro', label: 'Kết / CTA' }
  ];
  var STYLES = ['Giới thiệu công nghệ', 'Explainer', 'Sản phẩm', 'Số liệu', 'Social ngắn']
    .map(function (s) { return { value: s, label: s }; });

  function tplLabel(v) {
    for (var i = 0; i < TEMPLATES.length; i++) if (TEMPLATES[i].value === v) return TEMPLATES[i].label;
    return v;
  }

  function dataURL(rel) {
    return '/data/' + String(rel).split('/').map(encodeURIComponent).join('/');
  }

  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(
        function () { UI.toast('Đã sao chép đường dẫn.'); },
        function () { UI.toast('Không sao chép được đường dẫn.', 'error'); });
    } else {
      UI.toast('Trình duyệt không hỗ trợ sao chép tự động.', 'error');
    }
  }

  function chartToText(arr) {
    return (arr || []).map(function (c) { return (c.label || '') + ': ' + (Number(c.value) || 0); }).join('\n');
  }

  function parseChart(text) {
    var out = [];
    String(text || '').split('\n').forEach(function (line) {
      line = line.trim();
      if (!line) return;
      var i = line.lastIndexOf(':');
      if (i < 0) { out.push({ label: line, value: 0 }); return; }
      var v = parseFloat(line.slice(i + 1).trim().replace(',', '.'));
      out.push({ label: line.slice(0, i).trim(), value: isNaN(v) ? 0 : v });
    });
    return out;
  }

  function normScene(s) {
    s = s || {};
    var tpl = String(s.template || 'hero');
    if (!TEMPLATES.some(function (t) { return t.value === tpl; })) tpl = 'hero';
    return {
      template: tpl,
      title: s.title || '',
      subtitle: s.subtitle || '',
      bullets: Array.isArray(s.bullets) ? s.bullets.map(String) : [],
      code: s.code || '',
      image: s.image || '',
      chart: Array.isArray(s.chart) ? s.chart.map(function (c) {
        return { label: String((c && c.label) || ''), value: Number(c && c.value) || 0 };
      }) : [],
      voiceText: s.voiceText || '',
      duration: Number(s.duration) > 0 ? Number(s.duration) : 5,
      accent: s.accent || '',
      _open: false
    };
  }

  App.pages['htmlvideo'] = {
    title: 'HTML Video',
    subtitle: 'Render video từ HTML/CSS — prompt, bài viết hoặc repo GitHub thành MP4',
    render: render
  };

  function render(root) {
    var st = {
      tab: 'prompt', prompt: '', content: '', repoUrl: '',
      count: 6, style: STYLES[0].value,
      scenes: [], projectId: '', lastProjectId: '', jobHandler: null,
      cfg: { aspect: '9:16', theme: 'vivid', fps: 24, narration: true, voice: '', engine: '', bgmPath: '', bgmVolume: 25, burnSub: false, transition: 'none', motion: 'basic', reveal: 'none' }
    };
    App._cleanup = function () {
      if (st.jobHandler) { Bus.off('job', st.jobHandler); st.jobHandler = null; }
    };

    var tpl = applyPrefill(st);
    if (tpl) {
      root.appendChild(h('div', {
        class: 'card',
        style: { borderLeft: '3px solid var(--blue)' }
      },
        h('div', { class: 'card-title' }, '🧰 Đang dùng khuôn: ' + tpl.icon + ' ' + tpl.name),
        h('div', { class: 'muted', style: { fontSize: '12.5px', lineHeight: '1.6', whiteSpace: 'pre-line' } },
          st.templateGuide),
        h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } },
          'Khung hình đã đặt ' + st.cfg.aspect + (tpl.seconds ? ' · nhắm ' + tpl.seconds + ' giây' : '') +
          (tpl.musicMood ? ' · tone nhạc gợi ý: ' + tpl.musicMood : '') +
          '. Hướng dẫn trên sẽ được ghép vào yêu cầu khi AI tách cảnh.'),
        // Style của khuôn là tên một bộ Style Kit (dùng cho sinh ẢNH ở Text →
        // Video), không phải ô "Phong cách" của trang này — hai từ vựng khác
        // nhau. Nói thẳng ra thay vì gán bừa vào ô sai.
        tpl.style ? h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '4px' } },
          'Nếu dựng bằng ảnh AI ở Text → Video, khuôn này hợp bộ Style Kit "' + tpl.style + '".') : null));
    }

    // ---------- Ghi chú đầu trang ----------
    var note = h('div', null,
      h('span', { class: 'badge badge-blue' }, 'ℹ️ Cần Google Chrome trên máy để render HTML Video'));

    // ---------- Card 1: Nguồn nội dung ----------
    var tabDefs = [
      { id: 'prompt', label: '✍️ Prompt' },
      { id: 'content', label: '📰 Bài viết' },
      { id: 'repo', label: '🐙 Repo GitHub' }
    ];
    var tabBtns = {};
    var tabBar = h('div', { class: 'row' }, tabDefs.map(function (t) {
      var b = h('button', {
        class: 'btn btn-sm ' + (st.tab === t.id ? 'btn-primary' : 'btn-ghost'), type: 'button',
        onclick: function () { st.tab = t.id; refreshTabs(); }
      }, t.label);
      tabBtns[t.id] = b;
      return b;
    }));
    var tabBody = h('div', { class: 'mt-8' });

    function refreshTabs() {
      tabDefs.forEach(function (t) {
        tabBtns[t.id].className = 'btn btn-sm ' + (st.tab === t.id ? 'btn-primary' : 'btn-ghost');
      });
      tabBody.innerHTML = '';
      if (st.tab === 'prompt') {
        tabBody.appendChild(UI.field('Mô tả video', UI.textarea({
          value: st.prompt, rows: 6, placeholder: 'Mô tả video bạn muốn…',
          oninput: function () { st.prompt = this.value; }
        })));
      } else if (st.tab === 'content') {
        tabBody.appendChild(UI.field('Nội dung bài viết', UI.textarea({
          value: st.content, rows: 6, placeholder: 'Dán nội dung bài viết vào đây…',
          oninput: function () { st.content = this.value; }
        })));
      } else {
        tabBody.appendChild(UI.field('URL repo GitHub', UI.input({
          value: st.repoUrl, placeholder: 'https://github.com/owner/repo',
          oninput: function () { st.repoUrl = this.value.trim(); }
        })));
        tabBody.appendChild(h('div', { class: 'muted mt-8', style: { fontSize: '12px' } },
          'Tự đọc README của repo để phân tích thành cảnh.'));
      }
    }
    refreshTabs();

    var planSpin = h('span', { style: { display: 'none' } }, UI.spinner());
    var planErr = h('div', { class: 'text-red mt-8', style: { display: 'none' } });
    var planBtn = UI.btn('🧠 Phân tích thành cảnh', { variant: 'primary', onclick: analyze });
    var srcCard = UI.card({
      title: 'Nguồn nội dung', icon: '🧩',
      desc: 'Chọn nguồn: mô tả trực tiếp, dán bài viết hoặc nhập repo GitHub — LLM sẽ tách thành các cảnh HTML.',
      body: [
        tabBar, tabBody,
        h('div', { class: 'grid-3 mt-16' },
          UI.slider('Số cảnh', { min: 3, max: 12, step: 1, value: st.count, oninput: function (v) { st.count = v; } }),
          UI.select('Phong cách', STYLES, st.style, function (v) { st.style = v; }),
          UI.field(' ', h('div', { class: 'row' }, planBtn, planSpin))),
        planErr
      ]
    });

    async function analyze() {
      planErr.style.display = 'none';
      var body = { count: st.count, style: st.style };
      if (st.tab === 'prompt') {
        if (!st.prompt.trim()) { UI.toast('Vui lòng nhập mô tả video trước.', 'error'); return; }
        body.prompt = st.prompt.trim();
      } else if (st.tab === 'content') {
        if (!st.content.trim()) { UI.toast('Vui lòng dán nội dung bài viết trước.', 'error'); return; }
        body.content = st.content.trim();
      } else {
        if (!st.repoUrl || st.repoUrl.indexOf('github.com/') < 0) {
          UI.toast('Vui lòng nhập URL repo GitHub hợp lệ (github.com/owner/repo).', 'error'); return;
        }
        body.repoUrl = st.repoUrl;
      }
      // Khuôn chỉ ĐI KÈM chứ không thay lời người dùng: gửi thành trường riêng
      // để AI coi là hướng dẫn dựng, còn prompt vẫn nguyên văn họ gõ.
      if (st.templateGuide) body.templateGuide = st.templateGuide;
      planBtn.disabled = true;
      planSpin.style.display = '';
      try {
        var res = await API.post('/api/tools/htmlvideo/plan', body);
        var scenes = (res && res.scenes) ? res.scenes : [];
        if (!scenes.length) throw new Error('LLM không trả về cảnh nào.');
        st.scenes = scenes.map(normScene);
        renderScenes();
        UI.toast('Đã phân tích thành ' + st.scenes.length + ' cảnh.');
      } catch (err) {
        planErr.textContent = '❌ Phân tích thất bại: ' + err.message;
        planErr.style.display = '';
        UI.toast('Phân tích thất bại: ' + err.message, 'error');
      } finally {
        planBtn.disabled = false;
        planSpin.style.display = 'none';
      }
    }

    // ---------- Card 2: Danh sách cảnh ----------
    var summary = h('span', { class: 'muted', style: { fontSize: '12px' } }, '0 cảnh · tổng ~0 giây');
    var listHost = h('div', { class: 'mt-8' });
    var scenesCard = h('div', { class: 'card' },
      h('div', { class: 'row-between' }, h('div', { class: 'card-title' }, '🎞️ Danh sách cảnh'), summary),
      listHost,
      h('div', { class: 'mt-8' }, UI.btn('+ Thêm cảnh', {
        variant: 'ghost', small: true,
        onclick: function () {
          var sc = normScene({ title: 'Cảnh ' + (st.scenes.length + 1) });
          sc._open = true;
          st.scenes.push(sc);
          renderScenes();
        }
      })));

    function updateSummary() {
      var total = 0;
      st.scenes.forEach(function (s) { total += Number(s.duration) || 0; });
      summary.textContent = st.scenes.length + ' cảnh · tổng ~' + Math.round(total) + ' giây';
    }

    function smallInput(scene, key, opts) {
      opts = opts || {};
      return h('input', {
        class: 'input', type: opts.type || 'text', value: scene[key],
        min: opts.min, step: opts.step, placeholder: opts.placeholder,
        oninput: function () {
          scene[key] = opts.type === 'number' ? (Number(this.value) || 0) : this.value;
          if (key === 'duration') updateSummary();
        }
      });
    }

    function areaField(label, placeholder, rowsN, getVal, setVal) {
      var ta = UI.textarea({ rows: rowsN, placeholder: placeholder, oninput: function () { setVal(this.value); } });
      ta.value = getVal();
      return h('div', { class: 'mt-8' }, UI.field(label, ta));
    }

    function sceneDetail(sc) {
      var rows = [h('div', { class: 'grid-3' },
        UI.field('Template', UI.select(null, TEMPLATES, sc.template, function (v) { sc.template = v; renderScenes(); })),
        UI.field('Thời lượng (giây)', smallInput(sc, 'duration', { type: 'number', min: 1, step: 0.5 })),
        UI.field('Ảnh (đường dẫn hoặc từ khóa stock)',
          sc.template === 'product' ? smallInput(sc, 'image', { placeholder: 'vd: assets/anh.png hoặc "laptop"' }) : h('div', { class: 'muted' }, '—'))
      ), h('div', { class: 'grid-2 mt-8' },
        UI.field('Title', smallInput(sc, 'title', { placeholder: 'Tiêu đề cảnh' })),
        UI.field('Subtitle', smallInput(sc, 'subtitle', { placeholder: 'Phụ đề / mô tả ngắn' })))];

      if (sc.template === 'bullets') {
        rows.push(areaField('Bullets (mỗi dòng 1 ý)', 'Mỗi dòng một ý chính…', 4,
          function () { return (sc.bullets || []).join('\n'); },
          function (v) { sc.bullets = v.split('\n').map(function (x) { return x.trim(); }).filter(Boolean); }));
      }
      if (sc.template === 'code') {
        rows.push(areaField('Code', 'Dán đoạn code cần hiển thị…', 5,
          function () { return sc.code || ''; },
          function (v) { sc.code = v; }));
      }
      if (sc.template === 'chart') {
        rows.push(areaField('Dữ liệu biểu đồ (Nhãn: giá trị)', 'Mỗi dòng dạng "Nhãn: giá trị", vd:\n2023: 120\n2024: 250', 4,
          function () { return chartToText(sc.chart); },
          function (v) { sc.chart = parseChart(v); }));
      }
      rows.push(areaField('Lời đọc (voiceText)', 'Lời đọc cho cảnh này (dùng khi bật lồng tiếng)…', 2,
        function () { return sc.voiceText || ''; },
        function (v) { sc.voiceText = v; }));
      return h('div', { class: 'mt-8' }, rows);
    }

    function sceneRow(sc, i) {
      var head = h('div', { class: 'row-between', style: { cursor: 'pointer' } },
        h('div', { class: 'row' },
          h('span', null, sc._open ? '▾' : '▸'),
          h('b', null, '#' + (i + 1)),
          h('span', { class: 'badge badge-gray' }, tplLabel(sc.template)),
          h('span', { class: 'muted' }, sc.title || '(chưa có tiêu đề)'),
          h('span', { class: 'muted', style: { fontSize: '12px' } }, '· ' + (Number(sc.duration) || 0) + 's')),
        h('div', { class: 'row' },
          h('button', {
            class: 'btn btn-ghost btn-sm', title: 'Chuyển lên', disabled: i === 0,
            onclick: function (e) { e.stopPropagation(); swap(i, i - 1); }
          }, '↑'),
          h('button', {
            class: 'btn btn-ghost btn-sm', title: 'Chuyển xuống', disabled: i === st.scenes.length - 1,
            onclick: function (e) { e.stopPropagation(); swap(i, i + 1); }
          }, '↓'),
          h('button', {
            class: 'btn btn-ghost btn-sm', title: 'Xóa cảnh',
            onclick: function (e) { e.stopPropagation(); st.scenes.splice(i, 1); renderScenes(); }
          }, '🗑')));
      head.onclick = function () { sc._open = !sc._open; renderScenes(); };
      return h('div', { style: { border: '1px solid var(--border)', borderRadius: '10px', padding: '10px 12px', marginBottom: '8px' } },
        head, sc._open ? sceneDetail(sc) : null);
    }

    function swap(a, b) {
      var t = st.scenes[a]; st.scenes[a] = st.scenes[b]; st.scenes[b] = t;
      renderScenes();
    }

    function renderScenes() {
      updateSummary();
      listHost.innerHTML = '';
      if (!st.scenes.length) {
        listHost.appendChild(UI.empty('Chưa có cảnh nào — hãy "Phân tích thành cảnh" hoặc thêm thủ công.', '🎬'));
        return;
      }
      st.scenes.forEach(function (sc, i) { listHost.appendChild(sceneRow(sc, i)); });
    }

    // ---------- Card 3: Cấu hình render ----------
    var voiceSel = UI.select(null, [{ value: '', label: 'Đang tải giọng đọc…' }], '');
    voiceSel.onchange = function () { st.cfg.voice = voiceSel.value; };
    var engineSel = UI.select(null, [
      { value: '', label: 'Tự động (ưu tiên VieNeu)' },
        { value: 'vieneu', label: 'VieNeu-TTS (tiếng Việt tự nhiên)' },
      { value: 'say', label: 'macOS say' },
      { value: 'gemini', label: 'Gemini TTS' }
    ], '', function (v) { st.cfg.engine = v; });
    var projectSel = UI.select(null, [{ value: '', label: 'Không — thư mục tạm' }], '',
      function (v) { st.projectId = v; });

    function syncNarration() {
      voiceSel.disabled = !st.cfg.narration;
      engineSel.disabled = !st.cfg.narration;
    }

    async function loadVoices() {
      try {
        var voices = (await API.get('/api/tools/voices')) || [];
        voices.sort(function (a, b) {
          var av = String(a.lang || '').toLowerCase().indexOf('vi') === 0 ? 0 : 1;
          var bv = String(b.lang || '').toLowerCase().indexOf('vi') === 0 ? 0 : 1;
          return av - bv || String(a.name || '').localeCompare(String(b.name || ''));
        });
        voiceSel.innerHTML = '';
        if (!voices.length) {
          voiceSel.appendChild(h('option', { value: '' }, 'Không tìm thấy giọng đọc'));
          return;
        }
        voices.forEach(function (v) {
          voiceSel.appendChild(h('option', { value: v.id },
            (v.name || v.id) + ' — ' + (v.lang || '?') + (v.engine ? ' (' + v.engine + ')' : '')));
        });
        voiceSel.value = voices[0].id;
        st.cfg.voice = voices[0].id;
      } catch (err) {
        voiceSel.innerHTML = '';
        voiceSel.appendChild(h('option', { value: '' }, 'Lỗi tải giọng đọc'));
        UI.toast('Không tải được danh sách giọng đọc: ' + err.message, 'error');
      }
    }

    async function loadProjects() {
      try {
        var projects = (await API.get('/api/projects')) || [];
        projects.forEach(function (p) {
          projectSel.appendChild(h('option', { value: p.id }, p.name || p.id));
        });
      } catch (err) {
        UI.toast('Không tải được danh sách dự án: ' + err.message, 'error');
      }
    }
    loadVoices();
    loadProjects();
    syncNarration();

    var resultHost = h('div', { class: 'mt-16' });
    var renderBtn = UI.btn('🎬 Render HTML Video', { variant: 'primary', large: true, onclick: startRender });
    var cfgCard = UI.card({
      title: 'Cấu hình render', icon: '⚙️',
      body: [
        h('div', { class: 'grid-3' },
          UI.select('Khung hình', [
            { value: '9:16', label: '9:16 — Dọc (Shorts/Reels)' },
            { value: '3:4', label: '3:4 — Dọc kiểu trang giấy (truyện tranh, nhật ký)' },
            { value: '16:9', label: '16:9 — Ngang (YouTube)' },
            { value: '1:1', label: '1:1 — Vuông' }
          ], st.cfg.aspect, function (v) { st.cfg.aspect = v; }),
          UI.select('Theme', [
            { value: 'vivid', label: 'Rực rỡ (vivid)' },
            { value: 'dark', label: 'Tối (dark)' },
            { value: 'light', label: 'Sáng (light)' }
          ], st.cfg.theme, function (v) { st.cfg.theme = v; }),
          UI.select('FPS', [
            { value: '24', label: '24 fps' },
            { value: '30', label: '30 fps' }
          ], String(st.cfg.fps), function (v) { st.cfg.fps = Number(v) || 24; })),
        h('div', { class: 'grid-2 mt-8' },
          UI.select('Chuyển cảnh', [
            { value: 'none', label: 'Cắt thẳng' },
            { value: 'fade', label: 'Chớm tối ở mối nối' },
            { value: 'dip', label: 'Tối hẳn giữa hai cảnh' },
            { value: 'page', label: 'Lật trang — như mở một cuốn sách' }
          ], st.cfg.transition, function (v) { st.cfg.transition = v; }),
          UI.select('Chuyển động', [
            { value: 'basic', label: 'Phóng nhẹ (mặc định)' },
            { value: 'cinematic', label: 'Điện ảnh — trôi đổi hướng, có chiều sâu' }
          ], st.cfg.motion, function (v) { st.cfg.motion = v; })),
        h('div', { class: 'grid-2 mt-8' },
          UI.select('Cách ảnh hiện ra', [
            { value: 'none', label: 'Hiện thẳng (mặc định)' },
            { value: 'draw', label: 'Vẽ ra — nét đen trắng, tô bóng, rồi lên màu' }
          ], st.cfg.reveal, function (v) { st.cfg.reveal = v; })),
        h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '6px' } },
          '"Vẽ ra" chỉ có tác dụng ở cảnh CÓ ảnh — dùng được cả ảnh AI sinh lẫn ảnh bạn tự vẽ đưa vào.'),
        h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '6px' } },
          'Chuyển cảnh nằm trong thời lượng của chính cảnh nên tổng thời lượng video không đổi — hình vẫn bám đúng giọng đọc.'),
        h('div', { class: 'mt-8' },
          UI.toggle('Lồng tiếng AI (narration)', 'Đọc voiceText của từng cảnh bằng TTS', st.cfg.narration,
            function (v) { st.cfg.narration = v; syncNarration(); })),
        h('div', { class: 'grid-3 mt-8' },
          UI.field('Giọng đọc', voiceSel),
          UI.field('Engine giọng', engineSel),
          UI.field('Lưu vào dự án', projectSel)),
        h('div', { class: 'grid-2 mt-8' },
          UI.field('Nhạc nền (tùy chọn)', UI.input({
            value: st.cfg.bgmPath, placeholder: 'Đường dẫn file nhạc, vd: downloads/bgm.mp3',
            oninput: function () { st.cfg.bgmPath = this.value.trim(); }
          })),
          UI.slider('Âm lượng nhạc (%)', { min: 0, max: 100, step: 1, value: st.cfg.bgmVolume, oninput: function (v) { st.cfg.bgmVolume = v; } })),
        UI.toggle('Ghi phụ đề vào video', 'Burn phụ đề trực tiếp lên khung hình (burnSub)', st.cfg.burnSub,
          function (v) { st.cfg.burnSub = v; }),
        h('div', { class: 'mt-16' }, renderBtn),
        resultHost
      ]
    });

    // ---------- Render + theo dõi Job ----------
    function cleanScene(s) {
      var out = {
        template: s.template, title: s.title, subtitle: s.subtitle,
        bullets: s.bullets || [], code: s.code, image: s.image,
        chart: s.chart || [], voiceText: s.voiceText,
        duration: Number(s.duration) > 0 ? Number(s.duration) : 5
      };
      if (s.accent) out.accent = s.accent;
      return out;
    }

    async function startRender() {
      if (!st.scenes.length) { UI.toast('Chưa có cảnh nào — hãy phân tích nội dung hoặc thêm cảnh trước.', 'error'); return; }
      var body = {
        scenes: st.scenes.map(cleanScene),
        config: {
          aspect: st.cfg.aspect, theme: st.cfg.theme, fps: st.cfg.fps, reveal: st.cfg.reveal,
          narration: st.cfg.narration, voice: st.cfg.voice, engine: st.cfg.engine,
          bgmPath: st.cfg.bgmPath, bgmVolume: st.cfg.bgmVolume / 100, burnSub: st.cfg.burnSub
        }
      };
      if (st.projectId) body.projectId = st.projectId;
      st.lastProjectId = st.projectId || '';
      renderBtn.disabled = true;
      try {
        var job = await API.post('/api/tools/htmlvideo', body);
        trackJob(job);
      } catch (err) {
        renderBtn.disabled = false;
        UI.toast('Không tạo được video: ' + err.message, 'error');
      }
    }

    function trackJob(job) {
      var prog = UI.progress(job.progress || 0);
      var detail = h('div', { class: 'muted mt-8' }, job.detail || 'Đang chuẩn bị…');
      resultHost.innerHTML = '';
      resultHost.appendChild(h('div', null,
        h('div', { class: 'row' }, UI.spinner(), h('b', null, 'Đang render HTML Video…')),
        h('div', { class: 'mt-8' }, prog), detail));
      st.jobHandler = Bus.on('job', function (j) {
        if (!j || j.id !== job.id) return;
        prog.set(j.progress || 0);
        if (j.detail) detail.textContent = j.detail;
        if (j.status === 'done') {
          Bus.off('job', st.jobHandler); st.jobHandler = null;
          renderBtn.disabled = false;
          showDone(j);
        } else if (j.status === 'error') {
          Bus.off('job', st.jobHandler); st.jobHandler = null;
          renderBtn.disabled = false;
          resultHost.innerHTML = '';
          resultHost.appendChild(h('div', { class: 'text-red' },
            '❌ Render thất bại: ' + (j.error || 'lỗi không xác định')));
          UI.toast('Render thất bại: ' + (j.error || 'lỗi không xác định'), 'error');
        }
      });
    }

    function showDone(j) {
      resultHost.innerHTML = '';
      UI.toast('Render HTML Video hoàn tất!');
      var openBtn = st.lastProjectId ? UI.btn('📁 Mở dự án', {
        variant: 'ghost',
        onclick: function () { App.navigate('projects/' + st.lastProjectId); }
      }) : null;
      if (j.output && j.output.charAt(0) !== '/') {
        var url = dataURL(j.output);
        resultHost.appendChild(h('div', null,
          h('video', { controls: 'controls', src: url, style: { width: '100%', maxHeight: '440px', borderRadius: '12px', background: '#000' } }),
          h('div', { class: 'row mt-8' },
            h('a', { class: 'btn btn-ghost', href: url, download: '' }, '⬇ Tải video'),
            openBtn,
            h('span', { class: 'muted', style: { fontSize: '12px' } }, j.output))));
      } else {
        resultHost.appendChild(h('div', { class: 'row' },
          h('span', null, '✅ Video đã lưu tại:'),
          h('code', { style: { fontSize: '12px', wordBreak: 'break-all' } }, j.output || '(không rõ)'),
          UI.btn('📋 Sao chép', { variant: 'ghost', small: true, onclick: function () { copyText(j.output || ''); } }),
          openBtn));
      }
    }

    // ---------- Lắp trang ----------
    root.appendChild(note);
    root.appendChild(h('div', { class: 'mt-8' }, srcCard));
    root.appendChild(h('div', { class: 'mt-16' }, scenesCard));
    root.appendChild(h('div', { class: 'mt-16' }, cfgCard));
    renderScenes();
  }
})();
