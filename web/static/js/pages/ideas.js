/* ============================================================
   Biz Studio — Trang "Ý tưởng & Hàng đợi"
   AI đề xuất hàng loạt ý tưởng video → người duyệt → hàng đợi
   tự sản xuất tuần tự (kịch bản → giọng đọc → storyboard → dựng).
   Đăng ký App.pages['ideas']. Load sau app.js. Không framework.
   ============================================================ */
(function () {
  'use strict';

  var STATUS = {
    proposed:  ['Chờ duyệt',      'badge-gray'],
    approved:  ['Đã duyệt',       'badge-blue'],
    queued:    ['Trong hàng đợi', 'badge-amber'],
    producing: ['Đang sản xuất',  'badge-amber ib-blink'],
    done:      ['Hoàn thành',     'badge-green'],
    error:     ['Lỗi',            'badge-red'],
    rejected:  ['Đã bỏ',          'badge-gray']
  };

  // Chip lọc: '' = tất cả, còn lại khớp Idea.status
  var FILTERS = [''].concat(Object.keys(STATUS)).map(function (v) {
    return { value: v, label: v ? STATUS[v][0] : 'Tất cả' };
  });

  var TONE_OPTS = ['Gần gũi', 'Chuyên gia', 'Truyền cảm hứng', 'Hài hước', 'Nghiêm túc'];

  var SIZE_PRESETS = [
    { value: '1080x1920', label: '1080×1920 — Dọc (TikTok / Shorts)' },
    { value: '1920x1080', label: '1920×1080 — Ngang (YouTube)' },
    { value: '1080x1080', label: '1080×1080 — Vuông' }
  ];

  var FPS_OPTS = [{ value: '30', label: '30 fps' }, { value: '60', label: '60 fps' }];
  var QUEUE_POLL_MS = 5000;
  var PRODUCING_HINT = 'Đang sản xuất — kịch bản → giọng đọc → storyboard → dựng video…';
  var EMPTY_HINT = 'Chưa có ý tưởng nào. Nhập chủ đề kênh ở trên rồi bấm "✨ Sinh ý tưởng" ' +
    'để AI đề xuất hàng loạt.';

  // ---------- CSS nội bộ ----------

  function injectStyles() {
    if (document.getElementById('ideas-page-style')) return;
    var css = [
      '.ib-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(320px,1fr));gap:16px}',
      '.ib-grid .card{margin-top:0;display:flex;flex-direction:column}.ib-grid .card .row{gap:7px}',
      '.ib-head{display:flex;align-items:flex-start;justify-content:space-between;gap:10px}',
      '.ib-title{font-size:14.5px;font-weight:700;line-height:1.45;word-break:break-word;min-width:0}',
      '.ib-angle{font-size:12.5px;color:var(--muted);line-height:1.55;margin:10px 0 0;word-break:break-word}',
      '.ib-hook{font-size:12.5px;font-style:italic;line-height:1.55;margin:9px 0 0;padding-left:9px;',
      '  border-left:2px solid var(--border);word-break:break-word}',
      '.ib-kw{display:flex;gap:6px;flex-wrap:wrap;margin-top:10px}',
      '.ib-meta{font-size:11.5px;color:var(--muted);margin-top:10px}',
      '.ib-filters{display:flex;gap:8px;flex-wrap:wrap;margin:18px 0 14px}',
      '.ib-fchip{border:1px solid var(--border);background:var(--card);color:var(--muted);border-radius:999px;',
      '  padding:6px 13px;font-size:12.5px;font-weight:600;cursor:pointer;line-height:1.25}',
      '.ib-fchip:hover{border-color:var(--blue)}',
      '.ib-fchip.on{border-color:var(--blue);background:var(--blue-soft);color:var(--blue)}',
      '.ib-fchip .n{opacity:.72;margin-left:6px;font-weight:700}',
      '.ib-qline{display:flex;align-items:center;gap:9px;flex-wrap:wrap;font-size:13px}',
      '.ib-dot{font-size:10px;vertical-align:middle}',
      '.ib-alert{background:rgba(239,68,68,.08);border:1px solid var(--red);border-radius:10px;',
      '  padding:9px 11px;font-size:12.5px;font-weight:600;word-break:break-word}',
      '.ib-warn{background:rgba(245,158,11,.09);border:1px solid var(--amber);',
      '  border-radius:10px;padding:10px 12px}',
      '.ib-video{width:100%;max-height:62vh;border-radius:12px;background:#000}',
      '.ib-blink{animation:ib-blink 1.3s ease-in-out infinite}',
      '@keyframes ib-blink{0%,100%{opacity:1}50%{opacity:.45}}'
    ].join('\n');
    document.head.appendChild(h('style', { id: 'ideas-page-style' }, css));
  }

  // ---------- Helpers ----------

  function dataUrl(p) { return '/data/' + String(p).split('/').map(encodeURIComponent).join('/'); }
  function fileName(p) { return String(p || '').split('/').pop(); }
  function idPath(id) { return '/api/ideas/' + encodeURIComponent(id); }

  function sizeText(x) {
    return (x.width || 1080) + '×' + (x.height || 1920) + ' · ' + (x.fps || 30) + 'fps';
  }

  function statusBadge(s) {
    var m = STATUS[s] || [s || '—', 'badge-gray'];
    return h('span', { class: 'badge ' + m[1], style: { flex: 'none' } }, m[0]);
  }

  function redAlert(msg) { return h('div', { class: 'text-red ib-alert' }, '⚠ ' + msg); }

  function amberWarn(msg) {
    return h('div', { class: 'ib-warn row' },
      h('span', null, '⚠️'),
      h('span', { class: 'text-amber', style: { fontWeight: '600', flex: '1', minWidth: '160px' } }, msg),
      h('a', { href: '#/settings', class: 'btn btn-ghost btn-sm' }, 'Mở Cấu hình & API →'));
  }

  function parseKeywords(raw) {
    return String(raw || '').split(/[,\n]/).map(function (s) { return s.trim(); })
      .filter(function (s) { return !!s; });
  }

  function splitSize(v) {
    var a = String(v || '').split('x');
    return { w: Number(a[0]) || 1080, h: Number(a[1]) || 1920 };
  }

  function indexOfIdea(list, id) {
    for (var i = 0; i < (list || []).length; i++) if (list[i].id === id) return i;
    return -1;
  }

  // ---------- Modal: xem video ----------

  function openVideo(idea) {
    var url = dataUrl(idea.outputPath);
    UI.modal({
      title: idea.title || 'Video đã dựng',
      body: h('div', null,
        h('video', { controls: 'controls', src: url, class: 'ib-video' }),
        h('div', { class: 'muted mt-8', style: { fontSize: '11.5px', wordBreak: 'break-all' } },
          idea.outputPath)),
      actions: [h('a', { class: 'btn btn-ghost', href: url, download: fileName(idea.outputPath) }, '⬇ Tải về')]
    });
  }

  // ---------- Modal: thêm / sửa ý tưởng ----------

  function openEditor(idea, onSaved) {
    var isEdit = !!idea;
    var src = idea || {};
    var titleIn = UI.input({ value: src.title || '',
      placeholder: 'VD: 5 công cụ AI giúp bạn tan làm sớm 1 tiếng' });
    var angleTa = UI.textarea({ value: src.angle || '', rows: 3,
      placeholder: 'Góc tiếp cận / tóm tắt nội dung sẽ nói trong video' });
    var hookTa = UI.textarea({ value: src.hook || '', rows: 2,
      placeholder: 'Câu mở đầu giữ chân người xem trong 3 giây đầu' });
    var kwIn = UI.input({ value: (src.keywords || []).join(', '),
      placeholder: 'công cụ ai, năng suất, dân văn phòng' });

    var size = ((isEdit ? idea.width : 0) || 1080) + 'x' + ((isEdit ? idea.height : 0) || 1920);
    var sizeSel = UI.select('Khung hình', SIZE_PRESETS, size, function (v) { size = v; });
    var fps = String((isEdit ? idea.fps : 0) || 30);
    var fpsSel = UI.select('Tốc độ khung hình', FPS_OPTS, fps, function (v) { fps = v; });

    var saveBtn = UI.btn(isEdit ? '💾 Lưu thay đổi' : '＋ Thêm ý tưởng', {
      onclick: function () {
        var title = titleIn.value.trim();
        if (!title) { UI.toast('Vui lòng nhập tiêu đề ý tưởng', 'error'); titleIn.focus(); return; }
        var wh = splitSize(size);
        var payload = {
          title: title, angle: angleTa.value.trim(), hook: hookTa.value.trim(),
          keywords: parseKeywords(kwIn.value), width: wh.w, height: wh.h, fps: Number(fps) || 30
        };
        if (isEdit) payload.status = idea.status;
        saveBtn.disabled = true;
        var req = isEdit ? API.put(idPath(idea.id), payload) : API.post('/api/ideas', payload);
        req.then(function (res) {
          m.close();
          UI.toast(isEdit ? 'Đã cập nhật ý tưởng' : 'Đã thêm ý tưởng mới');
          onSaved(res);
        }).catch(function (err) {
          UI.toast('Lưu ý tưởng thất bại: ' + err.message, 'error');
        }).finally(function () { saveBtn.disabled = false; });
      }
    });

    var m = UI.modal({
      title: isEdit ? 'Sửa ý tưởng' : 'Thêm ý tưởng thủ công',
      body: h('div', null,
        UI.field('Tiêu đề *', titleIn),
        UI.field('Góc tiếp cận', angleTa),
        UI.field('Hook mở đầu', hookTa),
        UI.field('Từ khoá (cách nhau bằng dấu phẩy)', kwIn),
        h('div', { class: 'grid-2' }, sizeSel, fpsSel),
        isEdit ? null : h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '6px' } },
          '💡 Ý tưởng thêm tay được đánh dấu "Đã duyệt" ngay, chỉ cần đưa vào hàng đợi là sản xuất.')),
      actions: [UI.btn('Huỷ', { variant: 'ghost', onclick: function () { m.close(); } }), saveBtn]
    });
  }

  function confirmDelete(idea, onDeleted) {
    var m = UI.modal({
      title: 'Xoá ý tưởng?',
      body: h('div', null,
        h('p', { style: { margin: '0 0 6px' } },
          'Ý tưởng "' + (idea.title || idea.id) + '" sẽ bị xoá vĩnh viễn. Không thể hoàn tác.'),
        idea.t2vSessionId
          ? h('p', { class: 'muted', style: { margin: '0', fontSize: '12.5px' } },
            'Phiên Text → Video đã tạo từ ý tưởng này vẫn được giữ lại.') : null),
      actions: [
        UI.btn('Huỷ', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('🗑 Xoá ý tưởng', {
          variant: 'danger',
          onclick: function () {
            m.close();
            API.del(idPath(idea.id))
              .then(function () { UI.toast('Đã xoá ý tưởng'); onDeleted(); })
              .catch(function (err) { UI.toast('Xoá ý tưởng thất bại: ' + err.message, 'error'); });
          }
        })
      ]
    });
  }

  // ---------- Card một ý tưởng ----------

  function ideaCard(idea, ctx) {
    var statusHost = h('div', { class: 'mt-8' });
    var btns = h('div', { class: 'row', style: { marginTop: '12px' } });
    var actionBtns = [];
    function addBtn(label, opts) {
      var b = UI.btn(label, opts);
      actionBtns.push(b);
      btns.appendChild(b);
    }
    function busy(v) { actionBtns.forEach(function (b) { b.disabled = v; }); }

    function act(path, okMsg) {
      busy(true);
      API.post(idPath(idea.id) + '/' + path, {}).then(function (res) {
        UI.toast(okMsg);
        if (res && res.id) applyIdea(ctx, res); else load(ctx, true);
        pollQueue(ctx);
      }).catch(function (err) {
        busy(false);
        statusHost.innerHTML = '';
        statusHost.appendChild(redAlert(err.message));
      });
    }

    if (idea.status === 'proposed') {
      addBtn('✓ Duyệt', { small: true, onclick: function () { act('approve', 'Đã duyệt ý tưởng'); } });
      addBtn('✗ Bỏ', { variant: 'ghost', small: true, onclick: function () { act('reject', 'Đã bỏ ý tưởng'); } });
    }
    if (idea.status === 'approved' || idea.status === 'rejected' || idea.status === 'error') {
      addBtn('▶ Đưa vào hàng đợi',
        { small: true, onclick: function () { act('queue', 'Đã đưa vào hàng đợi sản xuất'); } });
    }
    if (idea.status === 'queued') {
      btns.appendChild(h('span', { class: 'muted', style: { fontSize: '12.5px', fontWeight: '600' } },
        '⏳ Đang chờ trong hàng đợi'));
    }
    if (idea.t2vSessionId) {
      addBtn('📜 Mở phiên', { variant: 'ghost', small: true,
        onclick: function () { App.navigate('text2video/' + idea.t2vSessionId); } });
    }
    if (idea.status === 'done' && idea.outputPath) {
      addBtn('▶ Xem video', { small: true, onclick: function () { openVideo(idea); } });
      btns.appendChild(h('a', { class: 'btn btn-ghost btn-sm', href: dataUrl(idea.outputPath),
        download: fileName(idea.outputPath) }, '⬇ Tải về'));
    }
    // Ý tưởng hàng đợi ĐANG chạy thì máy chủ chặn sửa/xoá — khoá nút cho rõ ràng.
    var busyNow = !!(ctx.queue && ctx.queue.currentIdeaId === idea.id);
    addBtn('✏️ Sửa', { variant: 'ghost', small: true, disabled: busyNow,
      onclick: function () {
        openEditor(idea, function (res) {
          if (res && res.id) applyIdea(ctx, res); else load(ctx, true);
        });
      } });
    addBtn('🗑 Xoá', { variant: 'danger', small: true, disabled: busyNow,
      onclick: function () { confirmDelete(idea, function () { load(ctx, true); }); } });
    if (busyNow) btns.title = 'Đang sản xuất — bấm Dừng hàng đợi trước khi sửa hoặc xoá';
    var kw = h('div', { class: 'ib-kw' });
    (idea.keywords || []).forEach(function (k) { if (k) kw.appendChild(UI.chip(k)); });

    var card = h('div', { class: 'card' },
      h('div', { class: 'ib-head' },
        h('div', { class: 'ib-title', title: idea.title || '' }, idea.title || '(chưa có tiêu đề)'),
        statusBadge(idea.status)),
      idea.angle ? h('div', { class: 'ib-angle' }, idea.angle) : null,
      idea.hook ? h('div', { class: 'ib-hook' }, '“' + idea.hook + '”') : null, kw,
      h('div', { class: 'ib-meta' }, sizeText(idea) + ' · tạo ' + (UI.timeAgo(idea.createdAt) || '—')),
      btns, statusHost);

    // Tiến trình sản xuất của riêng card này (job t2v_* do runner phát ra).
    card.setJob = function (j) {
      statusHost.innerHTML = '';
      if (!j) return;
      if (j.status === 'error') { statusHost.appendChild(redAlert(j.error || 'Sản xuất thất bại')); return; }
      statusHost.appendChild(h('div', { class: 'row muted', style: { fontSize: '12.5px' } },
        UI.spinner(), h('span', null, j.detail || 'Đang sản xuất…')));
    };

    if (idea.status === 'error' && idea.error) statusHost.appendChild(redAlert(idea.error));
    else if (idea.status === 'producing') card.setJob({ status: 'running', detail: PRODUCING_HINT });
    return card;
  }

  // ---------- Danh sách + bộ lọc ----------

  function matchFilter(ctx, idea) { return !ctx.filter || idea.status === ctx.filter; }

  function updateCounts(ctx) {
    var counts = {};
    (ctx.list || []).forEach(function (x) { counts[x.status] = (counts[x.status] || 0) + 1; });
    FILTERS.forEach(function (f) {
      var el = ctx.chipNums[f.value];
      if (el) el.textContent = String(f.value ? (counts[f.value] || 0) : (ctx.list || []).length);
    });
  }

  function filterBar(ctx) {
    var bar = h('div', { class: 'ib-filters' });
    ctx.chipNums = {};
    FILTERS.forEach(function (f) {
      var num = h('span', { class: 'n' }, '0');
      var chip = h('button', {
        class: 'ib-fchip' + (ctx.filter === f.value ? ' on' : ''), type: 'button',
        onclick: function () {
          ctx.filter = f.value;
          bar.querySelectorAll('.ib-fchip').forEach(function (c) { c.classList.remove('on'); });
          chip.classList.add('on');
          renderList(ctx);
        }
      }, f.label, num);
      ctx.chipNums[f.value] = num;
      bar.appendChild(chip);
    });
    return bar;
  }

  function renderList(ctx) {
    ctx.cards = {};
    ctx.listHost.innerHTML = '';
    updateCounts(ctx);
    var list = (ctx.list || []).filter(function (x) { return matchFilter(ctx, x); });
    if (!list.length) {
      ctx.listHost.appendChild(UI.card({
        body: h('div', { style: { textAlign: 'center', padding: '10px 0' } },
          UI.empty((ctx.list || []).length ? 'Không có ý tưởng nào ở trạng thái này.' : EMPTY_HINT, '💡'))
      }));
      return;
    }
    var grid = h('div', { class: 'ib-grid' });
    list.forEach(function (x) {
      ctx.cards[x.id] = ideaCard(x, ctx);
      grid.appendChild(ctx.cards[x.id]);
    });
    ctx.listHost.appendChild(grid);
  }

  // Cập nhật 1 ý tưởng tại chỗ — không vẽ lại cả trang để tránh giật khi đang thao tác.
  function applyIdea(ctx, idea) {
    if (!idea || !idea.id) return;
    var i = indexOfIdea(ctx.list, idea.id);
    if (i < 0) { ctx.list.unshift(idea); renderList(ctx); return; }
    ctx.list[i] = idea;
    updateCounts(ctx);
    var old = ctx.cards[idea.id];
    // Đổi trạng thái làm ý tưởng rớt khỏi bộ lọc đang chọn → vẽ lại danh sách.
    if (!matchFilter(ctx, idea) || !old || !old.parentNode) { renderList(ctx); return; }
    var fresh = ideaCard(idea, ctx);
    ctx.cards[idea.id] = fresh;
    old.parentNode.replaceChild(fresh, old);
  }

  function load(ctx, silent) {
    if (ctx.loading) return;
    ctx.loading = true;
    if (!silent) {
      ctx.listHost.innerHTML = '';
      ctx.listHost.appendChild(h('div', { class: 'empty' },
        UI.spinner(), h('span', null, 'Đang tải danh sách ý tưởng…')));
    }
    API.get('/api/ideas').then(function (list) {
      ctx.list = (Array.isArray(list) ? list : [])
        .filter(function (x) { return x && x.id; })
        .sort(function (a, b) { return new Date(b.createdAt || 0) - new Date(a.createdAt || 0); });
      renderList(ctx);
    }).catch(function (err) {
      if (silent) return; // giữ nguyên danh sách cũ, lần poll sau thử lại
      ctx.listHost.innerHTML = '';
      ctx.listHost.appendChild(UI.card({
        title: 'Không tải được danh sách ý tưởng', icon: '❌',
        body: [redAlert(err.message), h('div', { class: 'muted mt-8', style: { fontSize: '12.5px' } },
          'Kiểm tra máy chủ đã bật module Ý tưởng (endpoint /api/ideas) chưa.')],
        foot: UI.btn('Thử lại', { variant: 'ghost', small: true, onclick: function () { load(ctx); } })
      }));
    }).finally(function () { ctx.loading = false; });
  }

  // ---------- Card "Sinh ý tưởng" ----------

  function genCard(ctx) {
    var topicIn = UI.input({ placeholder: 'VD: Kênh review công cụ AI cho dân văn phòng' });
    var audienceIn = UI.input({ placeholder: 'VD: người trẻ 22–30 làm văn phòng' });
    var count = 8;
    var countField = UI.slider('Số lượng ý tưởng (3–20)', {
      min: 3, max: 20, step: 1, value: 8,
      oninput: function (v) { count = Math.max(3, Math.min(20, v || 8)); }
    });
    var tone = TONE_OPTS[0];
    var toneSel = UI.select('Tông giọng', TONE_OPTS, tone, function (v) { tone = v; });
    var size = SIZE_PRESETS[0].value;
    var sizeSel = UI.select('Khung hình', SIZE_PRESETS, size, function (v) { size = v; });
    var fps = '30';
    var fpsSel = UI.select('Tốc độ khung hình', FPS_OPTS, fps, function (v) { fps = v; });

    var statusHost = h('div', { class: 'mt-8' });
    var genBtn = UI.btn('✨ Sinh ý tưởng', { onclick: function () { submit(); } });
    ctx.setGenJob = function (j) {
      statusHost.innerHTML = '';
      genBtn.disabled = !!j && (j.status === 'running' || j.status === 'queued');
      if (!j) return;
      if (j.status === 'error') {
        statusHost.appendChild(redAlert(j.error || 'Không sinh được ý tưởng'));
      } else if (j.status === 'done') {
        statusHost.appendChild(h('div', { class: 'text-green', style: { fontSize: '12.5px', fontWeight: '600' } },
          '✓ Đã sinh xong — kéo xuống duyệt các ý tưởng mới'));
      } else {
        statusHost.appendChild(h('div', { class: 'row muted', style: { fontSize: '12.5px' } },
          UI.spinner(), h('span', null, j.detail || 'Đang hỏi AI…')));
      }
    };

    function submit() {
      var topic = topicIn.value.trim();
      if (!topic) { UI.toast('Vui lòng nhập chủ đề / kênh', 'error'); topicIn.focus(); return; }
      var wh = splitSize(size);
      ctx.setGenJob({ status: 'running', detail: 'Đang gửi yêu cầu…' });
      API.post('/api/ideas/generate', {
        topic: topic, count: count, audience: audienceIn.value.trim(), tone: tone,
        width: wh.w, height: wh.h, fps: Number(fps) || 30
      }).then(function (job) {
        if (!job || !job.id) {
          ctx.setGenJob({ status: 'error', error: 'Máy chủ không trả về thông tin tác vụ' });
          return;
        }
        ctx.genJobId = job.id;
        ctx.setGenJob(job);
        UI.toast('Đang sinh ' + count + ' ý tưởng cho "' + topic + '"');
      }).catch(function (err) {
        ctx.setGenJob({ status: 'error', error: err.message });
      });
    }

    var addBtn = UI.btn('＋ Thêm thủ công', {
      variant: 'ghost',
      onclick: function () {
        openEditor(null, function (res) {
          if (res && res.id) applyIdea(ctx, res); else load(ctx, true);
        });
      }
    });

    return UI.card({
      title: 'Sinh ý tưởng', icon: '✨',
      desc: 'AI đề xuất hàng loạt ý tưởng cho kênh của bạn — bạn chỉ việc duyệt.',
      body: h('div', null,
        UI.field('Chủ đề / kênh *', topicIn), countField,
        UI.field('Đối tượng xem', audienceIn),
        h('div', { class: 'grid-2' }, toneSel, fpsSel), sizeSel,
        h('div', { class: 'row', style: { marginTop: '14px' } }, genBtn, addBtn),
        statusHost)
    });
  }

  // ---------- Card "Hàng đợi sản xuất" ----------

  function queueCard(ctx) {
    var line = h('div', { class: 'ib-qline' });
    var startBtn = UI.btn('▶ Bắt đầu', { small: true, onclick: function () { act('start'); } });
    var stopBtn = UI.btn('⏹ Dừng', { variant: 'ghost', small: true, onclick: function () { act('stop'); } });

    function act(what) {
      startBtn.disabled = true;
      stopBtn.disabled = true;
      API.post('/api/ideas/queue/' + what, {}).then(function () {
        UI.toast(what === 'start' ? 'Đã bật hàng đợi sản xuất' : 'Đã dừng nhận ý tưởng mới');
      }).catch(function (err) {
        UI.toast('Không đổi được trạng thái hàng đợi: ' + err.message, 'error');
      }).finally(function () { pollQueue(ctx); });
    }

    ctx.renderQueue = function (q, errMsg) {
      line.innerHTML = '';
      if (errMsg) {
        startBtn.disabled = stopBtn.disabled = false;
        line.appendChild(h('span', { class: 'muted', style: { fontSize: '12.5px' } },
          '⚠ Chưa đọc được trạng thái hàng đợi: ' + errMsg));
        return;
      }
      q = q || {};
      var running = !!q.running;
      startBtn.disabled = running;
      stopBtn.disabled = !running;
      line.appendChild(h('span', { class: 'ib-dot' + (running ? ' ib-blink' : ''),
        style: { color: running ? 'var(--green)' : 'var(--muted)' } }, '●'));
      line.appendChild(h('span', { class: running ? 'text-green' : 'muted', style: { fontWeight: '700' } },
        running ? 'Đang chạy' : 'Đang dừng'));

      if (q.currentIdeaId) {
        var i = indexOfIdea(ctx.list, q.currentIdeaId);
        line.appendChild(h('span', { class: 'muted' }, '·'));
        line.appendChild(h('span', { style: { minWidth: '0', wordBreak: 'break-word' } }, 'đang làm: ',
          h('strong', null, i >= 0 ? (ctx.list[i].title || q.currentIdeaId) : q.currentIdeaId)));
      }
      line.appendChild(h('span', { class: 'muted' }, '·'));
      line.appendChild(h('span', { class: 'muted' }, (q.queued || 0) + ' chờ sản xuất'));
    };

    ctx.renderQueue(null, null);

    return UI.card({
      title: 'Hàng đợi sản xuất', icon: '🏭',
      desc: 'Hàng đợi chạy tuần tự từng ý tưởng một. Bấm Dừng thì ý tưởng đang chạy vẫn được làm cho xong.',
      body: h('div', null, line,
        h('div', { class: 'row', style: { marginTop: '14px' } }, startBtn, stopBtn),
        h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '12px', lineHeight: '1.6' } },
          'Mỗi ý tưởng đi qua: viết kịch bản → giọng đọc → storyboard → dựng video. ' +
          'Ý tưởng lỗi sẽ được bỏ qua để chạy tiếp ý tưởng sau.'))
    });
  }

  function pollQueue(ctx) {
    return API.get('/api/ideas/queue').then(function (q) {
      q = q || {};
      var sig = [!!q.running, q.currentIdeaId || '', q.queued || 0, q.producing || 0].join('|');
      var changed = ctx.queueSig !== null && ctx.queueSig !== sig;
      ctx.queueSig = sig;
      ctx.queue = q;
      if (ctx.renderQueue) ctx.renderQueue(q, null);
      // Runner đổi trạng thái → đồng bộ lại card (phòng khi bản tin 'idea' không tới).
      if (changed) load(ctx, true);
    }).catch(function (err) {
      ctx.queue = null;
      if (ctx.renderQueue) ctx.renderQueue(null, err.message);
    });
  }

  // ---------- Trang ----------

  App.pages.ideas = {
    title: 'Ý tưởng & Hàng đợi',
    subtitle: 'AI đề xuất ý tưởng — bạn duyệt — hệ thống tự sản xuất video tuần tự',
    render: function (el) {
      injectStyles();

      var ctx = {
        list: [], cards: {}, chipNums: {}, filter: '',
        listHost: h('div'), genJobId: null, queue: null, queueSig: null, loading: false
      };

      var warnHost = h('div', { style: { marginBottom: '16px' } });
      function renderWarn() {
        warnHost.innerHTML = '';
        var t = App.state && App.state.tools;
        if (t && !t.claude && !t.geminiKey && !t.openaiKey) {
          warnHost.appendChild(amberWarn('Chưa có engine LLM khả dụng — không sinh được ý tưởng và kịch bản'));
        }
      }
      renderWarn();
      el.appendChild(warnHost);

      el.appendChild(h('div', { class: 'grid-2' }, genCard(ctx), queueCard(ctx)));
      el.appendChild(filterBar(ctx));
      el.appendChild(ctx.listHost);

      load(ctx);
      pollQueue(ctx);
      var timer = setInterval(function () { pollQueue(ctx); }, QUEUE_POLL_MS);

      // ---------- Realtime ----------
      var onIdea = function (x) {
        if (!x || !x.id) return;
        applyIdea(ctx, x);
        if (ctx.renderQueue && ctx.queue) ctx.renderQueue(ctx.queue, null);
      };

      var onJob = function (j) {
        if (!j || !j.id) return;
        if (ctx.genJobId && j.id === ctx.genJobId) {
          ctx.setGenJob(j);
          if (j.status === 'done') { UI.toast('Đã sinh xong ý tưởng mới'); load(ctx, true); }
          if (j.status === 'done' || j.status === 'error') ctx.genJobId = null;
          return;
        }
        // Job idea_gen từ nơi khác (hoặc sau khi SSE nối lại) → nạp lại danh sách.
        if (j.kind === 'idea_gen' && j.status === 'done') { load(ctx, true); return; }
        // Tiến trình sản xuất của ý tưởng đang chạy trong hàng đợi.
        if (String(j.kind || '').indexOf('t2v_') === 0 && ctx.queue && ctx.queue.currentIdeaId) {
          var card = ctx.cards[ctx.queue.currentIdeaId];
          if (card && card.setJob) card.setJob(j.status === 'error' ? j : { status: 'running', detail: j.detail });
        }
      };
      var onState = function () { renderWarn(); };

      Bus.on('idea', onIdea);
      Bus.on('job', onJob);
      Bus.on('state', onState);

      App._cleanup = function () {
        clearInterval(timer);
        Bus.off('idea', onIdea);
        Bus.off('job', onJob);
        Bus.off('state', onState);
      };
    }
  };
})();
