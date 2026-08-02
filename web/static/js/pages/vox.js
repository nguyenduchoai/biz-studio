/* ============================================================
   Biz Studio — Trang "Vox-Director"
   Như "Bài viết → Video" nhưng nâng cao: dự án đích, gán media
   từng cảnh, nhịp dựng, chất lượng. Đăng ký App.pages['vox'].
   ============================================================ */
(function () {
  'use strict';

  var THEMES = [
    { value: 'Tin nhanh', label: 'Tin nhanh' },
    { value: 'Cảm hứng', label: 'Cảm hứng' },
    { value: 'Tối Neon', label: 'Tối Neon' },
    { value: 'Sáng Pro', label: 'Sáng Pro' }
  ];
  var STEPS = ['Tạo video', 'Kịch bản', 'Media', 'Cấu hình', 'Dựng & Xuất bản'];

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

  function stepperEl(current) {
    var el = h('div', { class: 'stepper' });
    STEPS.forEach(function (lb, i) {
      var n = i + 1;
      var cls = 'step' + (n < current ? ' done' : n === current ? ' active' : '');
      el.appendChild(h('div', { class: cls },
        h('div', { class: 'step-dot' }, n < current ? '✓' : String(n)),
        h('div', { class: 'step-label' }, n + '. ' + lb)));
      if (i < STEPS.length - 1) {
        el.appendChild(h('div', { class: 'step-line' + (n < current ? ' done' : '') }));
      }
    });
    return el;
  }

  App.pages['vox'] = {
    title: 'Vox-Director',
    subtitle: 'Đạo diễn video Vox: kịch bản, media từng cảnh, dựng motion và xuất bản vào dự án',
    render: render
  };

  function render(root) {
    var st = {
      step: 1, title: '', content: '', count: 8, scenes: [], projectId: '', pace: 'balanced', jobHandler: null,
      cfg: { aspect: '9:16', voice: '', engine: '', theme: 'Tin nhanh', bgmPath: '', bgmVolume: 25, quality: 'standard', burnSub: false }
    };
    App._cleanup = function () {
      if (st.jobHandler) { Bus.off('job', st.jobHandler); st.jobHandler = null; }
    };

    // ---------- Workflow strip ----------
    var stepHost = h('div', { class: 'card', style: { padding: '14px 18px' } }, stepperEl(st.step));
    function setStep(n) { st.step = n; stepHost.innerHTML = ''; stepHost.appendChild(stepperEl(n)); }

    // ---------- Dự án đích ----------
    var projSel = UI.select(null, [{ value: '', label: 'Đang tải dự án…' }], '');
    projSel.onchange = function () { st.projectId = projSel.value; };
    loadProjects();

    async function loadProjects() {
      try {
        var projects = (await API.get('/api/projects')) || [];
        projSel.innerHTML = '';
        projSel.appendChild(h('option', { value: '' }, '— Không gắn dự án (chỉ dựng video) —'));
        projects.forEach(function (p) {
          projSel.appendChild(h('option', { value: p.id }, p.name + (p.kind ? ' · ' + p.kind : '')));
        });
      } catch (err) {
        projSel.innerHTML = '';
        projSel.appendChild(h('option', { value: '' }, 'Lỗi tải dự án'));
        UI.toast('Không tải được danh sách dự án: ' + err.message, 'error');
      }
    }

    // ---------- Trái: kịch bản ----------
    var charCount = h('div', { class: 'muted', style: { textAlign: 'right', fontSize: '12px', marginTop: '-8px' } }, '0 ký tự');
    var contentTA = UI.textarea({
      placeholder: 'Dán nội dung / kịch bản thô vào đây…', rows: 11,
      oninput: function () { st.content = this.value; charCount.textContent = this.value.length + ' ký tự'; }
    });
    var prepBtn = UI.btn('✨ Chuẩn bị nội dung', { variant: 'primary', onclick: prepare });
    var leftCard = UI.card({
      title: 'Kịch bản / Nội dung Vox', icon: '📝',
      desc: 'Chọn dự án đích để video lưu vào dự án và tận dụng media theo từ khóa.',
      body: [
        UI.field('Dự án đích', projSel),
        UI.field('Tiêu đề', UI.input({ placeholder: 'Tiêu đề video…', oninput: function () { st.title = this.value; } })),
        UI.field('Nội dung', contentTA),
        charCount,
        UI.slider('Số cảnh', { min: 4, max: 15, step: 1, value: 8, oninput: function (v) { st.count = v; } }),
        prepBtn
      ]
    });

    async function prepare() {
      if (!st.content.trim()) { UI.toast('Vui lòng nhập nội dung trước.', 'error'); return; }
      var content = (st.title.trim() ? 'Tiêu đề: ' + st.title.trim() + '\n\n' : '') + st.content;
      prepBtn.disabled = true;
      try {
        var res = await API.post('/api/tools/scenes', { content: content, count: st.count, style: st.cfg.theme });
        var scenes = (res && res.scenes) ? res.scenes : [];
        st.scenes = scenes.map(function (s) {
          return {
            title: s.title || '', voiceText: s.voiceText || '', mediaPath: s.mediaPath || '',
            mediaKeyword: s.mediaKeyword || '', duration: Number(s.duration) || 6
          };
        });
        if (!st.scenes.length) throw new Error('LLM không trả về cảnh nào.');
        renderScenes();
        setStep(3);
        UI.toast('Đã tạo ' + st.scenes.length + ' cảnh.');
      } catch (err) {
        UI.toast('Chuẩn bị nội dung thất bại: ' + err.message, 'error');
      } finally {
        prepBtn.disabled = false;
      }
    }

    // ---------- Phải: danh sách cảnh (kèm cột Media) ----------
    var summary = h('span', { class: 'muted', style: { fontSize: '12px' } }, '0 cảnh · 0 giây');
    var tableHost = h('div', { style: { overflowX: 'auto' } });
    var rightCard = h('div', { class: 'card' },
      h('div', { class: 'row-between' }, h('div', { class: 'card-title' }, '🎞️ Danh sách cảnh'), summary),
      tableHost,
      h('div', { class: 'mt-8' }, UI.btn('+ Thêm cảnh', {
        variant: 'ghost', small: true,
        onclick: function () {
          st.scenes.push({ title: 'Cảnh ' + (st.scenes.length + 1), voiceText: '', mediaKeyword: '', mediaPath: '', duration: 6 });
          renderScenes();
        }
      })));

    function updateSummary() {
      var total = 0;
      st.scenes.forEach(function (s) { total += Number(s.duration) || 0; });
      summary.textContent = st.scenes.length + ' cảnh · ' + Math.round(total) + ' giây';
    }

    function cellInput(scene, key, opts) {
      opts = opts || {};
      return h('input', {
        class: 'input', type: opts.type || 'text', value: scene[key],
        min: opts.min, step: opts.step, placeholder: opts.placeholder,
        style: { padding: '6px 8px', fontSize: '12.5px' },
        oninput: function () {
          scene[key] = opts.type === 'number' ? Number(this.value) : this.value;
          if (key === 'duration') updateSummary();
        }
      });
    }

    function cellArea(scene, key) {
      var el = h('textarea', {
        class: 'textarea', rows: 2,
        style: { padding: '6px 8px', fontSize: '12.5px', minHeight: '52px' },
        oninput: function () { scene[key] = this.value; }
      });
      el.value = scene[key] || '';
      return el;
    }

    function attachMedia(scene) {
      var p = window.prompt('Đường dẫn media cho cảnh này (tương đối data/ hoặc tuyệt đối):', scene.mediaPath || '');
      if (p === null) return;
      scene.mediaPath = p.trim();
      renderScenes();
    }

    function mediaCell(sc) {
      return h('td', null,
        h('div', { class: 'row', style: { gap: '6px', flexWrap: 'nowrap' } },
          cellInput(sc, 'mediaKeyword', { placeholder: 'từ khóa' }),
          h('button', {
            class: 'btn btn-ghost btn-sm', title: 'Gán đường dẫn media cụ thể',
            onclick: function () { attachMedia(sc); }
          }, '📎')),
        sc.mediaPath
          ? h('div', { class: 'muted', style: { fontSize: '11px', marginTop: '4px', wordBreak: 'break-all' } }, '📎 ' + sc.mediaPath)
          : null);
    }

    function renderScenes() {
      updateSummary();
      tableHost.innerHTML = '';
      if (!st.scenes.length) {
        tableHost.appendChild(UI.empty('Chưa có cảnh nào — hãy "Chuẩn bị nội dung" hoặc thêm thủ công.', '🎬'));
        return;
      }
      var tbody = h('tbody');
      st.scenes.forEach(function (sc, i) {
        tbody.appendChild(h('tr', null,
          h('td', { class: 'muted' }, String(i + 1)),
          h('td', null, cellInput(sc, 'title')),
          h('td', null, cellArea(sc, 'voiceText')),
          mediaCell(sc),
          h('td', null, cellInput(sc, 'duration', { type: 'number', min: 1, step: 1 })),
          h('td', null, h('button', {
            class: 'btn btn-ghost btn-sm', title: 'Xóa cảnh',
            onclick: function () { st.scenes.splice(i, 1); renderScenes(); }
          }, '🗑'))));
      });
      tableHost.appendChild(h('table', { class: 'table' },
        h('thead', null, h('tr', null,
          h('th', { style: { width: '30px' } }, '#'),
          h('th', { style: { width: '18%' } }, 'Tiêu đề'),
          h('th', null, 'Lời đọc'),
          h('th', { style: { width: '24%' } }, 'Media'),
          h('th', { style: { width: '66px' } }, 'Giây'),
          h('th', { style: { width: '44px' } }, ''))),
        tbody));
    }

    // ---------- Cấu hình & dựng ----------
    var voiceSel = UI.select(null, [{ value: '', label: 'Đang tải giọng đọc…' }], '');
    voiceSel.onchange = function () { st.cfg.voice = voiceSel.value; };
    loadVoices();

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

    var cfgGrid = h('div', { class: 'grid-3' },
      UI.select('Mẫu / Theme', THEMES, st.cfg.theme, function (v) { st.cfg.theme = v; }),
      UI.select('Kích thước', [
        { value: '9:16', label: '9:16 (1080×1920)' },
        { value: '16:9', label: '16:9 (1920×1080)' }
      ], st.cfg.aspect, function (v) { st.cfg.aspect = v; }),
      UI.field('Giọng đọc', voiceSel),
      UI.select('Engine giọng', [
        { value: '', label: 'Tự động (ưu tiên VieNeu)' },
        { value: 'vieneu', label: 'VieNeu-TTS (tiếng Việt tự nhiên)' },
        { value: 'say', label: 'macOS say' },
        { value: 'gemini', label: 'Gemini TTS' }
      ], st.cfg.engine, function (v) { st.cfg.engine = v; }),
      UI.select('Nhịp dựng', [
        { value: 'balanced', label: 'Cân bằng' },
        { value: 'fast', label: 'Nhanh' },
        { value: 'slow', label: 'Chậm' }
      ], st.pace, function (v) { st.pace = v; }),
      UI.select('Chất lượng', [
        { value: 'standard', label: 'Tiêu chuẩn' },
        { value: 'high', label: 'Cao' }
      ], st.cfg.quality, function (v) { st.cfg.quality = v; }),
      UI.field('Nhạc nền (tùy chọn)', UI.input({
        placeholder: 'Đường dẫn file nhạc, vd: downloads/bgm.mp3',
        oninput: function () { st.cfg.bgmPath = this.value.trim(); }
      })),
      UI.slider('Âm lượng nhạc (%)', { min: 0, max: 100, step: 1, value: 25, oninput: function (v) { st.cfg.bgmVolume = v; } }));

    var resultHost = h('div', { class: 'mt-16' });
    var renderBtn = UI.btn('🎥 Dựng Motion, tạo video', { variant: 'primary', large: true, onclick: startRender });
    var cfgCard = UI.card({
      title: 'Cấu hình & Dựng video', icon: '⚙️',
      body: [
        cfgGrid,
        UI.toggle('Ghi phụ đề vào video', 'Burn phụ đề trực tiếp lên khung hình (burnSub)', false,
          function (v) { st.cfg.burnSub = v; }),
        h('div', { class: 'mt-16' }, renderBtn),
        resultHost
      ]
    });

    async function startRender() {
      if (!st.scenes.length) { UI.toast('Chưa có cảnh nào — hãy chuẩn bị nội dung trước.', 'error'); return; }
      var body = {
        scenes: st.scenes.map(function (s) {
          return {
            title: s.title, voiceText: s.voiceText, mediaPath: s.mediaPath || '',
            mediaKeyword: s.mediaKeyword, duration: Number(s.duration) || 6
          };
        }),
        config: {
          aspect: st.cfg.aspect, voice: st.cfg.voice, engine: st.cfg.engine, theme: st.cfg.theme,
          bgmPath: st.cfg.bgmPath, bgmVolume: st.cfg.bgmVolume / 100,
          quality: st.cfg.quality, burnSub: st.cfg.burnSub
        }
      };
      if (st.projectId) body.projectId = st.projectId;
      renderBtn.disabled = true;
      setStep(5);
      try {
        var job = await API.post('/api/tools/vox', body);
        trackJob(job);
      } catch (err) {
        renderBtn.disabled = false;
        UI.toast('Không dựng được video: ' + err.message, 'error');
      }
    }

    function trackJob(job) {
      var prog = UI.progress(job.progress || 0);
      var detail = h('div', { class: 'muted mt-8' }, job.detail || 'Đang chuẩn bị…');
      resultHost.innerHTML = '';
      resultHost.appendChild(h('div', null,
        h('div', { class: 'row' }, UI.spinner(), h('b', null, 'Đang dựng motion từng cảnh…')),
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
            '❌ Dựng video thất bại: ' + (j.error || 'lỗi không xác định')));
          UI.toast('Dựng video thất bại: ' + (j.error || 'lỗi không xác định'), 'error');
        }
      });
    }

    function showDone(j) {
      setStep(STEPS.length + 1);
      resultHost.innerHTML = '';
      UI.toast('Dựng video hoàn tất!');
      var actions = h('div', { class: 'row mt-8' });
      if (j.output && j.output.charAt(0) !== '/') {
        var url = dataURL(j.output);
        resultHost.appendChild(h('video', {
          controls: 'controls', src: url,
          style: { width: '100%', maxHeight: '440px', borderRadius: '12px', background: '#000' }
        }));
        actions.appendChild(h('a', { class: 'btn btn-ghost', href: url, download: '' }, '⬇ Tải video'));
        actions.appendChild(h('span', { class: 'muted', style: { fontSize: '12px' } }, j.output));
      } else {
        actions.appendChild(h('span', null, '✅ Video đã lưu tại:'));
        actions.appendChild(h('code', { style: { fontSize: '12px', wordBreak: 'break-all' } }, j.output || '(không rõ)'));
        actions.appendChild(UI.btn('📋 Sao chép', { variant: 'ghost', small: true, onclick: function () { copyText(j.output || ''); } }));
      }
      if (st.projectId) {
        actions.appendChild(UI.btn('📂 Mở dự án', {
          variant: 'primary',
          onclick: function () { location.hash = '#/projects/' + st.projectId; }
        }));
      }
      resultHost.appendChild(actions);
    }

    // ---------- Lắp trang ----------
    var grid = h('div', { class: 'grid-2 mt-16' }, h('div', null, leftCard), h('div', null, rightCard));
    root.appendChild(stepHost);
    root.appendChild(grid);
    root.appendChild(h('div', { class: 'mt-16' }, cfgCard));
    renderScenes();
  }
})();
