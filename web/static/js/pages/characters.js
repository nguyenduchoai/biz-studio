/* ============================================================
   Biz Studio — Trang Nhân vật (nhân vật nhất quán xuyên suốt video)
   Load sau app.js. Tự đăng ký App.pages['characters'].
   Không framework / CDN / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var AV_COLORS = ['#7C3AED', '#2563EB', '#0EA5E9', '#10B981',
    '#F59E0B', '#EF4444', '#EC4899', '#6366F1'];

  var LOOK_EXAMPLE = 'Vietnamese woman, 28 years old, long straight black hair, round glasses, ' +
    'beige linen blazer over a white shirt, slim build, warm friendly smile';

  // ---------- CSS nội bộ ----------

  function injectStyles() {
    if (document.getElementById('characters-page-style')) return;
    var css = [
      '.ch-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:16px}',
      '.ch-grid .card{margin-top:0;display:flex;flex-direction:column}',
      '.ch-head{display:flex;gap:12px;align-items:center}',
      '.ch-av{position:relative;width:56px;height:56px;flex:none;border-radius:14px;',
      '  overflow:hidden;border:1px solid var(--border);background:var(--bg)}',
      '.ch-av img{width:100%;height:100%;object-fit:cover;display:block}',
      '.ch-av-fb{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;',
      '  color:#fff;font-size:22px;font-weight:700}',
      '.ch-id{min-width:0;flex:1}',
      '.ch-name{font-size:14.5px;font-weight:700;margin:0;word-break:break-word}',
      '.ch-role{font-size:12px;color:var(--muted);margin-top:2px;word-break:break-word}',
      '.ch-look{font-size:12.5px;color:var(--muted);line-height:1.55;word-break:break-word;',
      '  display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;margin:10px 0 0}',
      '.ch-thumb{width:100%;max-height:220px;object-fit:cover;border-radius:12px;',
      '  margin-top:12px;display:block;background:var(--bg)}'
    ].join('\n');
    document.head.appendChild(h('style', { id: 'characters-page-style' }, css));
  }

  // ---------- Helpers ----------

  function dataUrl(p) { return '/data/' + String(p).split('/').map(encodeURIComponent).join('/'); }

  // dataURL — đường dẫn tương đối trong data/ → URL phục vụ file.
  function dataURL(rel) {
    return '/data/' + String(rel).split('/').map(encodeURIComponent).join('/');
  }

  function previewUrl(id, bust) {
    var u = '/data/characters/' + encodeURIComponent(id + '-preview.png');
    return bust ? (u + '?t=' + bust) : u;
  }

  function hashColor(s) {
    s = String(s || '');
    var n = 0;
    for (var i = 0; i < s.length; i++) n = (n * 31 + s.charCodeAt(i)) % 100000;
    return AV_COLORS[n % AV_COLORS.length];
  }

  function initial(name) {
    var s = String(name || '').trim();
    return s ? s.charAt(0).toUpperCase() : '?';
  }

  function redAlert(msg) {
    return h('div', {
      class: 'text-red',
      style: {
        background: 'rgba(239,68,68,.08)', border: '1px solid var(--red)', borderRadius: '10px',
        padding: '9px 11px', fontSize: '12.5px', fontWeight: '600'
      }
    }, '⚠ ' + msg);
  }

  function amberWarn(msg) {
    return h('div', {
      style: {
        background: 'rgba(245,158,11,.09)', border: '1px solid var(--amber)',
        borderRadius: '10px', padding: '10px 12px', marginBottom: '16px'
      }
    },
      h('div', { class: 'row' },
        h('span', null, '⚠️'),
        h('span', { class: 'text-amber', style: { fontWeight: '600', flex: '1', minWidth: '160px' } }, msg),
        h('a', { href: '#/settings', class: 'btn btn-ghost btn-sm' }, 'Mở Cấu hình & API →')));
  }

  // ---------- Avatar (ảnh tham chiếu → ảnh xem thử → chữ cái đầu) ----------

  function avatarEl(c) {
    var fb = h('div', { class: 'ch-av-fb', style: { background: hashColor(c.name || c.id) } },
      initial(c.name));
    var img = h('img', { alt: 'Ảnh nhân vật ' + (c.name || ''), style: { display: 'none' } });
    var wrap = h('div', { class: 'ch-av' }, fb, img);

    var cands = [], idx = 0, bust = 0;
    if (c.refImage) cands.push(dataUrl(c.refImage));
    cands.push(previewUrl(c.id));

    function tryNext() {
      if (idx >= cands.length) { img.style.display = 'none'; fb.style.display = ''; return; }
      var u = cands[idx++];
      img.src = bust ? (u + (u.indexOf('?') >= 0 ? '&' : '?') + 't=' + bust) : u;
    }
    img.onload = function () { img.style.display = ''; fb.style.display = 'none'; };
    img.onerror = function () { tryNext(); };
    tryNext();

    wrap.reload = function () { idx = 0; bust = Date.now(); tryNext(); };
    return wrap;
  }

  // ---------- Form tạo / sửa ----------

  function editorBody(nameInput, roleInput, lookTa) {
    return h('div', null,
      UI.field('Tên nhân vật *', nameInput),
      h('div', { class: 'muted', style: { fontSize: '12px', margin: '-8px 0 14px' } },
        '💡 Tên ngắn gọn để nhận ra khi gán nhân vật cho từng cảnh trong Storyboard.'),
      UI.field('Vai trò', roleInput),
      UI.field('Mô tả ngoại hình', lookTa),
      h('div', { class: 'muted', style: { fontSize: '12px', margin: '-8px 0 8px', lineHeight: '1.6' } },
        '💡 Viết bằng tiếng Anh, càng cụ thể càng nhất quán: tuổi, tóc, trang phục, dáng người…'),
      h('div', {
        class: 'muted',
        style: {
          fontSize: '12px', lineHeight: '1.6', background: 'var(--bg)', border: '1px solid var(--border)',
          borderRadius: '10px', padding: '8px 10px', wordBreak: 'break-word'
        }
      }, 'Ví dụ: ' + LOOK_EXAMPLE));
  }

  function openEditor(ch, onSaved) {
    var isEdit = !!ch;
    var nameInput = UI.input({ value: isEdit ? (ch.name || '') : '', placeholder: 'VD: Arin' });
    var roleInput = UI.input({
      value: isEdit ? (ch.role || '') : '',
      placeholder: 'VD: Người dẫn chuyện — vui vẻ, gần gũi'
    });
    var lookTa = UI.textarea({ value: isEdit ? (ch.look || '') : '', rows: 6, placeholder: LOOK_EXAMPLE });
    lookTa.style.minHeight = '150px';

    var saveBtn = UI.btn(isEdit ? '💾 Lưu thay đổi' : '＋ Tạo nhân vật', {
      onclick: function () {
        var name = nameInput.value.trim();
        if (!name) { UI.toast('Vui lòng nhập tên nhân vật', 'error'); return; }
        var payload = { name: name, role: roleInput.value.trim(), look: lookTa.value.trim() };
        saveBtn.disabled = true;
        var req = isEdit
          ? API.put('/api/characters/' + encodeURIComponent(ch.id), payload)
          : API.post('/api/characters', payload);
        req.then(function () {
          m.close();
          UI.toast(isEdit ? 'Đã cập nhật nhân vật' : 'Đã tạo nhân vật mới');
          onSaved();
        }).catch(function (err) {
          UI.toast('Lưu nhân vật thất bại: ' + err.message, 'error');
        }).finally(function () { saveBtn.disabled = false; });
      }
    });

    var m = UI.modal({
      title: isEdit ? 'Sửa nhân vật' : 'Thêm nhân vật mới',
      body: editorBody(nameInput, roleInput, lookTa),
      actions: [UI.btn('Huỷ', { variant: 'ghost', onclick: function () { m.close(); } }), saveBtn]
    });
  }

  function confirmDelete(ch, onDeleted) {
    var m = UI.modal({
      title: 'Xoá nhân vật?',
      body: h('div', null,
        h('p', { style: { margin: '0 0 6px' } },
          'Nhân vật "' + (ch.name || ch.id) + '" sẽ bị xoá vĩnh viễn. Không thể hoàn tác.'),
        h('p', { class: 'muted', style: { margin: '0', fontSize: '12.5px' } },
          'Nhân vật cũng bị gỡ khỏi mọi cảnh đang dùng — các cảnh đó vẫn giữ ảnh cũ, nhưng lần ' +
          'tạo ảnh sau sẽ không còn mô tả ngoại hình của nhân vật này.')),
      actions: [
        UI.btn('Huỷ', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('🗑 Xoá nhân vật', {
          variant: 'danger',
          onclick: function () {
            m.close();
            API.del('/api/characters/' + encodeURIComponent(ch.id)).then(function () {
              UI.toast('Đã xoá nhân vật');
              onDeleted();
            }).catch(function (err) {
              UI.toast('Xoá nhân vật thất bại: ' + err.message, 'error');
            });
          }
        })
      ]
    });
  }

  // ---------- Card một nhân vật ----------

  // bibleBlock — tóm tắt hồ sơ đầy đủ ngay trên thẻ nhân vật; chưa dựng thì ẩn.
  function bibleBlock(c) {
    var p = c.persona, v = c.voiceSpec;
    if (!p && !v) return h('span');
    var rows = [];
    if (p) {
      var head = [p.gender, p.ageRange, p.identity].filter(Boolean).join(' · ');
      if (head) rows.push(h('div', { style: { fontWeight: '600' } }, head));
      if (p.personality && p.personality.length) {
        rows.push(h('div', null, (p.personality || []).map(function (t) {
          return h('span', { class: 'badge badge-blue', style: { marginRight: '4px' } }, t);
        })));
      }
      if (p.temperament) rows.push(h('div', { class: 'muted' }, p.temperament));
      if (p.motivation) rows.push(h('div', { class: 'muted' }, '🎯 ' + p.motivation));
    }
    if (v) {
      var vs = [v.timbre, v.pitch, v.pace, v.accent].filter(Boolean).join(' · ');
      if (vs) rows.push(h('div', { class: 'muted' }, '🎙 ' + vs));
      if (v.voiceId) {
        rows.push(h('div', null, h('span', { class: 'badge badge-blue' }, 'giọng: ' + v.voiceId)));
      }
      if (v.note) {
        rows.push(h('div', { style: { color: 'var(--red)', fontSize: '12px' } }, '⚠ ' + v.note));
      }
    }
    if (c.sheetImage) {
      rows.push(h('a', { href: dataURL(c.sheetImage), target: '_blank' }, '📐 Xem bản ba góc nhìn'));
    }
    return h('div', {
      style: {
        fontSize: '12.5px', lineHeight: '1.6', margin: '8px 0',
        padding: '8px 10px', borderRadius: '8px', background: 'var(--bg)'
      }
    }, rows);
  }

  // watchJob theo dõi một job tới khi xong rồi gọi lại.
  function watchJob(id, card, done) {
    var t = setInterval(function () {
      API.get('/api/jobs/' + id).then(function (j) {
        card.setJob(j);
        if (j.status === 'done') { clearInterval(t); done(); }
        else if (j.status === 'error') { clearInterval(t); }
      }).catch(function () { clearInterval(t); });
    }, 3000);
  }

  function charCard(c, ctx) {
    var av = avatarEl(c);

    var thumb = h('img', {
      class: 'ch-thumb', alt: 'Ảnh xem thử nhân vật ' + (c.name || ''), style: { display: 'none' }
    });
    thumb.onload = function () { thumb.style.display = ''; };
    thumb.onerror = function () { thumb.style.display = 'none'; };
    thumb.src = previewUrl(c.id); // chưa có ảnh thì onerror tự ẩn

    var statusHost = h('div', { class: 'mt-8' });
    var fileIn = h('input', { type: 'file', accept: 'image/*', style: { display: 'none' } });

    var refBtn = UI.btn('📷 Ảnh tham chiếu', {
      variant: 'ghost', small: true, onclick: function () { fileIn.click(); }
    });
    var previewBtn = UI.btn('🖼 Xem thử', {
      variant: 'ghost', small: true, onclick: function () { runPreview(c, card, ctx); }
    });
    var editBtn = UI.btn('✏️ Sửa', {
      variant: 'ghost', small: true,
      onclick: function () { openEditor(c, function () { load(ctx); }); }
    });
    var delBtn = UI.btn('🗑 Xoá', {
      variant: 'danger', small: true,
      onclick: function () { confirmDelete(c, function () { load(ctx); }); }
    });
    var bibleBtn = UI.btn('📖 Dựng hồ sơ', {
      variant: 'ghost', small: true,
      onclick: function () {
        bibleBtn.disabled = true;
        card.setJob({ status: 'running', detail: 'AI đang dựng hồ sơ nhân vật…' });
        API.post('/api/characters/' + encodeURIComponent(c.id) + '/bible', {})
          .then(function (fresh) {
            card.setJob(null);
            var v = fresh.voiceSpec && fresh.voiceSpec.voiceId;
            UI.toast('Đã dựng hồ sơ' + (v ? ' · ghép giọng ' + v : ''));
            load(ctx);
          })
          .catch(function (err) {
            card.setJob({ status: 'error', error: err.message });
          })
          .finally(function () { bibleBtn.disabled = false; });
      }
    });
    var sheetBtn = UI.btn('📐 Bản ba góc nhìn', {
      variant: 'ghost', small: true,
      onclick: function () {
        sheetBtn.disabled = true;
        API.post('/api/characters/' + encodeURIComponent(c.id) + '/sheet', { setAsRef: true })
          .then(function (job) {
            card.setJob(job);
            watchJob(job.id, card, function () { load(ctx); });
          })
          .catch(function (err) { card.setJob({ status: 'error', error: err.message }); })
          .finally(function () { sheetBtn.disabled = false; });
      }
    });
    var btns = [bibleBtn, sheetBtn, refBtn, previewBtn, editBtn, delBtn];

    fileIn.onchange = function () {
      var f = fileIn.files && fileIn.files[0];
      fileIn.value = '';
      if (!f) return;
      card.setJob({ status: 'running', detail: 'Đang tải ảnh tham chiếu…' });
      API.upload('/api/characters/' + encodeURIComponent(c.id) + '/ref', [f]).then(function () {
        UI.toast('Đã cập nhật ảnh tham chiếu cho "' + (c.name || c.id) + '"');
        load(ctx);
      }).catch(function (err) {
        card.setJob({ status: 'error', error: 'Không tải được ảnh tham chiếu: ' + err.message });
      });
    };

    var card = h('div', { class: 'card' },
      h('div', { class: 'ch-head' }, av,
        h('div', { class: 'ch-id' },
          h('div', { class: 'ch-name' }, c.name || '(chưa đặt tên)'),
          h('div', { class: 'ch-role' }, c.role || 'Chưa ghi vai trò'))),
      bibleBlock(c),
      h('div', { class: 'ch-look', title: c.look || '' },
        c.look || '(chưa có mô tả ngoại hình — nhân vật sẽ khó giống nhau giữa các cảnh)'),
      thumb,
      h('div', { class: 'row', style: { marginTop: '12px' } }, btns, fileIn),
      statusHost);

    // Trạng thái riêng của card: tải ảnh tham chiếu / job xem thử.
    card.setJob = function (j) {
      statusHost.innerHTML = '';
      var busy = !!j && (j.status === 'running' || j.status === 'queued');
      btns.forEach(function (b) { b.disabled = busy; });
      if (!j) return;
      if (j.status === 'error') {
        statusHost.appendChild(redAlert(j.error || 'Không tạo được ảnh xem thử'));
        return;
      }
      if (j.status === 'done') {
        thumb.src = previewUrl(c.id, Date.now());
        av.reload();
        statusHost.appendChild(h('div', { class: 'text-green', style: { fontSize: '12.5px', fontWeight: '600' } },
          '✓ Đã tạo ảnh xem thử'));
        return;
      }
      statusHost.appendChild(h('div', { class: 'row muted', style: { fontSize: '12.5px' } },
        UI.spinner(), h('span', null, j.detail || 'Đang xử lý…')));
    };
    return card;
  }

  function runPreview(c, card, ctx) {
    card.setJob({ status: 'running', detail: 'Đang gửi yêu cầu…' });
    API.post('/api/characters/' + encodeURIComponent(c.id) + '/preview', {}).then(function (job) {
      if (!job || !job.id) {
        card.setJob({ status: 'error', error: 'Máy chủ không trả về thông tin tác vụ' });
        return;
      }
      ctx.jobChar[job.id] = c.id;
      card.setJob(job);
      UI.toast('Đang tạo ảnh xem thử cho "' + (c.name || c.id) + '"');
    }).catch(function (err) {
      card.setJob({ status: 'error', error: err.message });
    });
  }

  // ---------- Danh sách ----------

  function emptyBox(ctx) {
    return UI.card({
      body: h('div', { style: { textAlign: 'center', padding: '10px 0' } },
        UI.empty('Chưa có nhân vật nào. Tạo nhân vật đầu tiên — ví dụ người dẫn chuyện của kênh — ' +
          'rồi gán vào từng cảnh trong Storyboard để mọi cảnh cùng một gương mặt.', '🧑‍🎤'),
        h('div', { style: { marginTop: '12px' } },
          UI.btn('＋ Thêm nhân vật', {
            onclick: function () { openEditor(null, function () { load(ctx); }); }
          })))
    });
  }

  function load(ctx) {
    ctx.cards = {};
    ctx.listHost.innerHTML = '';
    ctx.listHost.appendChild(h('div', { class: 'empty' },
      UI.spinner(), h('span', null, 'Đang tải danh sách nhân vật…')));

    API.get('/api/characters').then(function (list) {
      list = Array.isArray(list) ? list : [];
      ctx.listHost.innerHTML = '';
      if (!list.length) { ctx.listHost.appendChild(emptyBox(ctx)); return; }
      var grid = h('div', { class: 'ch-grid' });
      list.forEach(function (c) {
        if (!c || !c.id) return;
        var card = charCard(c, ctx);
        ctx.cards[c.id] = card;
        grid.appendChild(card);
      });
      ctx.listHost.appendChild(grid);
    }).catch(function (err) {
      ctx.listHost.innerHTML = '';
      ctx.listHost.appendChild(UI.card({
        title: 'Không tải được danh sách nhân vật', icon: '❌',
        body: [redAlert(err.message),
          h('div', { class: 'muted mt-8', style: { fontSize: '12.5px' } },
            'Kiểm tra máy chủ đã bật module Nhân vật (endpoint /api/characters) chưa.')],
        foot: UI.btn('Thử lại', { variant: 'ghost', small: true, onclick: function () { load(ctx); } })
      }));
    });
  }

  // ---------- Trang ----------

  App.pages.characters = {
    title: 'Nhân vật',
    subtitle: 'Giữ nhân vật giống nhau ở mọi cảnh — mô tả ngoại hình được chèn tự động vào prompt sinh ảnh',
    render: function (el) {
      injectStyles();

      var ctx = { listHost: h('div'), cards: {}, jobChar: {} };

      el.appendChild(UI.card({
        title: 'Nhân vật dùng để làm gì?', icon: '🧑‍🎤',
        body: h('div', { style: { fontSize: '13px', lineHeight: '1.65' } },
          h('p', { class: 'muted', style: { margin: '0 0 6px' } },
            'Nhân vật giúp người trong các cảnh trông giống nhau xuyên suốt video. ' +
            'Mô tả ngoại hình sẽ được tự động chèn vào prompt sinh ảnh của mọi cảnh có nhân vật đó.'),
          h('p', { class: 'muted', style: { margin: '0' } },
            'Gán nhân vật cho từng cảnh ở phần ', h('strong', null, 'Storyboard'), ' của ',
            h('a', { href: '#/text2video' }, 'Text → Video'), '.'))
      }));

      var warnHost = h('div', { style: { marginTop: '16px' } });
      function renderWarn() {
        warnHost.innerHTML = '';
        var s = App.state;
        if (s && s.tools && !s.tools.geminiKey) {
          warnHost.appendChild(amberWarn('Chưa có Gemini API key — chưa sinh hoặc xem thử được ảnh nhân vật. ' +
            'Bạn vẫn tạo được nhân vật và tự tải ảnh tham chiếu.'));
        }
      }
      renderWarn();
      el.appendChild(warnHost);

      el.appendChild(h('div', { class: 'row-between', style: { margin: '16px 0' } },
        h('div', { class: 'muted', style: { fontSize: '12.5px' } },
          'Một cảnh có thể có nhiều nhân vật — chọn bằng chip trong Storyboard.'),
        UI.btn('＋ Thêm nhân vật', {
          onclick: function () { openEditor(null, function () { load(ctx); }); }
        })));

      el.appendChild(ctx.listHost);
      load(ctx);

      // ---------- SSE + dọn dẹp ----------
      var onJob = function (j) {
        if (!j || !j.id) return;
        var charId = ctx.jobChar[j.id];
        if (!charId) return;
        var card = ctx.cards[charId];
        if (!card) return;
        card.setJob(j);
        if (j.status === 'done' || j.status === 'error') delete ctx.jobChar[j.id];
      };
      var onState = function () { renderWarn(); };
      Bus.on('job', onJob);
      Bus.on('state', onState);

      App._cleanup = function () {
        Bus.off('job', onJob);
        Bus.off('state', onState);
      };
    }
  };
})();
