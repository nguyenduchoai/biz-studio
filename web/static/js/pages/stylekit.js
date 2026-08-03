/* ============================================================
   Biz Studio — Trang Style Kit (bộ style hình ảnh dùng chung)
   Load sau app.js. Tự đăng ký App.pages['stylekit'].
   Không framework / CDN / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var THEME_OPTS = [
    { value: 'vivid', label: 'Rực rỡ (vivid)' },
    { value: 'dark', label: 'Tối (dark)' },
    { value: 'light', label: 'Sáng (light)' }
  ];
  var THEME_LABELS = { vivid: 'Rực rỡ', dark: 'Tối', light: 'Sáng' };
  var MAX_COLORS = 12;
  var FALLBACK_HEX = '#7C3AED';

  // ---------- CSS nội bộ ----------

  function injectStyles() {
    if (document.getElementById('stylekit-page-style')) return;
    var css = [
      '.sk-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:16px}',
      '.sk-grid .card{margin-top:0;display:flex;flex-direction:column}',
      '.sk-name{font-size:14.5px;font-weight:700;margin:0;word-break:break-word}',
      '.sk-prompt{font-size:12.5px;color:var(--muted);line-height:1.55;word-break:break-word;',
      '  display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;margin:10px 0 0}',
      '.sk-swatches{display:flex;gap:6px;flex-wrap:wrap;margin-top:10px}',
      '.sk-swatch{width:26px;height:26px;border-radius:7px;border:1px solid var(--border);flex:none}',
      '.sk-thumb{width:100%;max-height:200px;object-fit:cover;border-radius:12px;',
      '  margin-top:12px;display:block;background:var(--bg)}',
      '.sk-color-row{display:flex;align-items:center;gap:8px;margin-bottom:8px}',
      '.sk-color-row .input{flex:1;min-width:0}',
      '.sk-color-pick{width:40px;height:38px;flex:none;padding:2px;cursor:pointer;',
      '  border:1px solid var(--border);border-radius:var(--r-input);background:var(--card)}'
    ].join('\n');
    document.head.appendChild(h('style', { id: 'stylekit-page-style' }, css));
  }

  // ---------- Helpers ----------

  function normTheme(v) {
    v = String(v || '').toLowerCase();
    return (v === 'dark' || v === 'light') ? v : 'vivid';
  }

  function safeHex(v) {
    v = String(v || '').trim();
    return /^#[0-9a-fA-F]{6}$/.test(v) ? v : FALLBACK_HEX;
  }

  function previewUrl(id, bust) {
    var u = '/data/styles/' + encodeURIComponent(id + '-preview.png');
    return bust ? (u + '?t=' + Date.now()) : u;
  }

  function redAlert(msg) {
    return h('div', {
      class: 'text-red',
      style: {
        background: 'rgba(239,68,68,.08)', border: '1px solid var(--red)',
        borderRadius: '10px', padding: '10px 12px', fontSize: '13px', fontWeight: '600'
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

  // ---------- Bảng màu (form) ----------

  function paletteEditor(initial) {
    var rowsHost = h('div');
    var rows = [];

    function render() {
      rowsHost.innerHTML = '';
      if (!rows.length) {
        rowsHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12px', marginBottom: '8px' } },
          'Chưa có màu nào — để trống cũng được, khi đó prompt không kèm bảng màu.'));
      }
      rows.forEach(function (row) { rowsHost.appendChild(row.el); });
    }

    function addRow(hex) {
      if (rows.length >= MAX_COLORS) {
        UI.toast('Tối đa ' + MAX_COLORS + ' màu cho một bộ style', 'error');
        return;
      }
      var text = UI.input({ value: hex || FALLBACK_HEX, placeholder: '#RRGGBB' });
      var pick = h('input', { type: 'color', class: 'sk-color-pick', value: safeHex(hex || FALLBACK_HEX) });
      text.oninput = function () {
        var v = text.value.trim();
        if (/^#[0-9a-fA-F]{6}$/.test(v)) pick.value = v;
      };
      pick.oninput = function () { text.value = pick.value; };

      var row = { input: text };
      row.el = h('div', { class: 'sk-color-row' }, pick, text,
        UI.btn('✕', {
          variant: 'ghost', small: true,
          onclick: function () {
            var i = rows.indexOf(row);
            if (i >= 0) rows.splice(i, 1);
            render();
          }
        }));
      rows.push(row);
      render();
    }

    (initial || []).forEach(function (c) { addRow(c); });
    render();

    var wrap = h('div', null, rowsHost,
      UI.btn('＋ Thêm màu', { variant: 'ghost', small: true, onclick: function () { addRow(''); } }));
    wrap.values = function () {
      return rows.map(function (r) { return r.input.value.trim(); })
        .filter(function (v) { return !!v; });
    };
    return wrap;
  }

  // ---------- Form tạo / sửa ----------

  function openEditor(kit, onSaved) {
    var isEdit = !!kit;
    var nameInput = UI.input({
      value: isEdit ? (kit.name || '') : '',
      placeholder: 'VD: 🎬 Điện ảnh ấm'
    });

    var promptTa = UI.textarea({
      value: isEdit ? (kit.stylePrompt || '') : '',
      rows: 6,
      placeholder: 'cinematic photography, warm natural light, shallow depth of field, film grain, muted color grading…'
    });
    promptTa.style.minHeight = '150px';

    var negTa = UI.textarea({
      value: isEdit ? (kit.negative || '') : '',
      rows: 3,
      placeholder: 'text, watermark, deformed hands, cluttered background…'
    });

    var theme = normTheme(isEdit ? kit.theme : 'vivid');
    var themeSel = UI.select(null, THEME_OPTS, theme, function (v) { theme = v; });

    var palette = paletteEditor(isEdit && kit.palette ? kit.palette : ['#7C3AED', '#EC4899', '#F8FAFC', '#111827']);

    var lockDefault = isEdit && !!kit.isDefault;
    var defToggle = UI.toggle(
      'Đặt làm bộ đang dùng',
      lockDefault
        ? 'Đây đang là bộ mặc định — muốn đổi thì bấm "Dùng bộ này" ở một bộ khác'
        : 'Mọi prompt sinh ảnh (Vox-Director, HTML Video, Text → Video) sẽ dùng bộ này',
      lockDefault, null);
    if (lockDefault) defToggle.input.disabled = true;

    var saveBtn = UI.btn(isEdit ? '💾 Lưu thay đổi' : '＋ Tạo bộ style', {
      onclick: function () {
        var name = nameInput.value.trim();
        var sp = promptTa.value.trim();
        if (!name) { UI.toast('Vui lòng nhập tên bộ style', 'error'); return; }
        if (!sp) { UI.toast('Vui lòng nhập style prompt — đây là phần quyết định chất hình', 'error'); return; }

        var payload = {
          name: name,
          stylePrompt: sp,
          negative: negTa.value.trim(),
          palette: palette.values(),
          theme: theme,
          isDefault: !!defToggle.input.checked
        };
        saveBtn.disabled = true;
        var req = isEdit
          ? API.put('/api/styles/' + encodeURIComponent(kit.id), payload)
          : API.post('/api/styles', payload);
        req.then(function () {
          m.close();
          UI.toast(isEdit ? 'Đã cập nhật bộ style' : 'Đã tạo bộ style mới');
          onSaved();
        }).catch(function (err) {
          UI.toast('Lưu bộ style thất bại: ' + err.message, 'error');
        }).finally(function () { saveBtn.disabled = false; });
      }
    });

    var m = UI.modal({
      title: isEdit ? 'Sửa bộ style' : 'Tạo bộ style mới',
      body: h('div', null,
        UI.field('Tên bộ style *', nameInput),
        UI.field('Style prompt *', promptTa),
        h('div', { class: 'muted', style: { fontSize: '12px', margin: '-8px 0 14px' } },
          '💡 Viết bằng tiếng Anh cho model hiểu tốt hơn. Mô tả chất hình: chất liệu, ánh sáng, nét vẽ, bố cục…'),
        UI.field('Tránh (negative)', negTa),
        h('div', { class: 'muted', style: { fontSize: '12px', margin: '-8px 0 14px' } },
          '💡 Những thứ không muốn xuất hiện — chữ, watermark, tay biến dạng…'),
        UI.field('Bảng màu', palette),
        h('div', { style: { marginTop: '14px' } }, UI.field('Theme khung chữ', themeSel)),
        defToggle),
      actions: [
        UI.btn('Huỷ', { variant: 'ghost', onclick: function () { m.close(); } }),
        saveBtn
      ]
    });
  }

  function confirmDelete(kit, onDeleted) {
    var m = UI.modal({
      title: 'Xoá bộ style?',
      body: h('div', null,
        h('p', { style: { margin: '0 0 6px' } },
          'Bộ style "' + (kit.name || kit.id) + '" sẽ bị xoá vĩnh viễn. Không thể hoàn tác.'),
        kit.isDefault
          ? h('p', { class: 'muted', style: { margin: '0', fontSize: '12.5px' } },
            'Đây đang là bộ mặc định — hệ thống sẽ tự chuyển sang bộ mới nhất còn lại.')
          : null),
      actions: [
        UI.btn('Huỷ', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('🗑 Xoá bộ style', {
          variant: 'danger',
          onclick: function () {
            m.close();
            API.del('/api/styles/' + encodeURIComponent(kit.id)).then(function () {
              UI.toast('Đã xoá bộ style');
              onDeleted();
            }).catch(function (err) {
              UI.toast('Xoá bộ style thất bại: ' + err.message, 'error');
            });
          }
        })
      ]
    });
  }

  // ---------- Card một bộ style ----------

  function styleCard(k, ctx) {
    var img = h('img', {
      class: 'sk-thumb', alt: 'Ảnh xem thử bộ style ' + (k.name || ''),
      style: { display: 'none' }
    });
    img.onload = function () { img.style.display = ''; };
    img.onerror = function () { img.style.display = 'none'; };
    img.src = previewUrl(k.id); // thử tải; chưa có ảnh thì onerror ẩn đi

    var swatches = h('div', { class: 'sk-swatches' });
    (k.palette || []).forEach(function (c) {
      swatches.appendChild(h('span', {
        class: 'sk-swatch', title: c, style: { background: c }
      }));
    });

    var statusHost = h('div', { class: 'mt-8' });
    var previewBtn = UI.btn('🖼 Xem thử', {
      variant: 'ghost', small: true,
      onclick: function () { runPreview(k, card, ctx); }
    });

    var btns = h('div', { class: 'row', style: { marginTop: '12px' } });
    if (!k.isDefault) {
      btns.appendChild(UI.btn('✅ Dùng bộ này', {
        small: true,
        onclick: function () {
          API.post('/api/styles/' + encodeURIComponent(k.id) + '/default', {}).then(function () {
            UI.toast('Đang dùng bộ style: ' + (k.name || k.id));
            load(ctx);
          }).catch(function (err) {
            UI.toast('Không đặt được bộ mặc định: ' + err.message, 'error');
          });
        }
      }));
    }
    btns.appendChild(previewBtn);
    btns.appendChild(UI.btn('✏️ Sửa', {
      variant: 'ghost', small: true,
      onclick: function () { openEditor(k, function () { load(ctx); }); }
    }));
    btns.appendChild(UI.btn('🗑 Xoá', {
      variant: 'danger', small: true,
      onclick: function () { confirmDelete(k, function () { load(ctx); }); }
    }));

    var card = h('div', { class: 'card' },
      h('div', { class: 'row-between' },
        h('div', { class: 'sk-name' }, k.name || '(không tên)'),
        k.isDefault
          ? h('span', { class: 'badge badge-green' }, 'Đang dùng')
          : h('span', { class: 'badge badge-gray' }, THEME_LABELS[normTheme(k.theme)])),
      swatches,
      h('div', { class: 'sk-prompt', title: k.stylePrompt || '' }, k.stylePrompt || '(chưa có style prompt)'),
      img, btns, statusHost);

    // Cập nhật trạng thái job xem thử của riêng card này.
    card.setJob = function (j) {
      statusHost.innerHTML = '';
      if (j.status === 'error') {
        previewBtn.disabled = false;
        statusHost.appendChild(redAlert(j.error || 'Không tạo được ảnh xem thử'));
        return;
      }
      if (j.status === 'done') {
        previewBtn.disabled = false;
        img.src = previewUrl(k.id, true);
        statusHost.appendChild(h('div', { class: 'text-green', style: { fontSize: '12.5px', fontWeight: '600' } },
          '✓ Đã tạo ảnh xem thử'));
        return;
      }
      previewBtn.disabled = true;
      statusHost.appendChild(h('div', { class: 'row muted', style: { fontSize: '12.5px' } },
        UI.spinner(), h('span', null, j.detail || 'Đang tạo ảnh xem thử…')));
    };
    return card;
  }

  function runPreview(k, card, ctx) {
    card.setJob({ status: 'running', detail: 'Đang gửi yêu cầu…' });
    API.post('/api/styles/' + encodeURIComponent(k.id) + '/preview', {}).then(function (job) {
      if (!job || !job.id) {
        card.setJob({ status: 'error', error: 'Máy chủ không trả về thông tin tác vụ' });
        return;
      }
      ctx.jobKit[job.id] = k.id;
      card.setJob(job);
      UI.toast('Đang tạo ảnh xem thử cho bộ "' + (k.name || k.id) + '"');
    }).catch(function (err) {
      card.setJob({ status: 'error', error: err.message });
    });
  }

  // ---------- Danh sách ----------

  function load(ctx) {
    ctx.cards = {};
    ctx.listHost.innerHTML = '';
    ctx.listHost.appendChild(h('div', { class: 'empty' },
      UI.spinner(), h('span', null, 'Đang tải bộ style…')));

    API.get('/api/styles').then(function (list) {
      list = Array.isArray(list) ? list : [];
      ctx.listHost.innerHTML = '';
      if (!list.length) {
        ctx.listHost.appendChild(UI.card({
          body: h('div', { style: { textAlign: 'center', padding: '10px 0' } },
            UI.empty('Chưa có bộ style nào. Tạo bộ đầu tiên để mọi cảnh trong video của bạn cùng một chất hình.', '🎨'),
            h('div', { style: { marginTop: '12px' } },
              UI.btn('＋ Tạo bộ style', { onclick: function () { openEditor(null, function () { load(ctx); }); } })))
        }));
        return;
      }
      var grid = h('div', { class: 'sk-grid' });
      list.forEach(function (k) {
        var card = styleCard(k, ctx);
        ctx.cards[k.id] = card;
        grid.appendChild(card);
      });
      ctx.listHost.appendChild(grid);
    }).catch(function (err) {
      ctx.listHost.innerHTML = '';
      ctx.listHost.appendChild(UI.card({
        title: 'Không tải được danh sách bộ style', icon: '❌',
        body: h('div', { class: 'text-red' }, err.message),
        foot: UI.btn('Thử lại', { variant: 'ghost', small: true, onclick: function () { load(ctx); } })
      }));
    });
  }

  // ---------- Trang ----------

  App.pages.stylekit = {
    title: 'Style Kit',
    subtitle: 'Bộ style hình ảnh dùng chung — mọi cảnh trong một video trông như cùng một bộ phim',
    render: function (el) {
      injectStyles();

      var ctx = { listHost: h('div'), cards: {}, jobKit: {} };

      el.appendChild(UI.card({
        title: 'Style Kit là gì?', icon: '🎨',
        body: h('div', { style: { fontSize: '13px', lineHeight: '1.65' } },
          h('p', { class: 'muted', style: { margin: '0 0 6px' } },
            'Style Kit là câu mô tả phong cách được ghép vào MỌI prompt sinh ảnh, giúp các cảnh trong cùng một video trông đồng nhất.'),
          h('p', { class: 'muted', style: { margin: '0' } },
            'Bộ đang chọn áp cho ',
            h('strong', null, 'Vox-Director'), ', ',
            h('strong', null, 'HTML Video'), ' và ',
            h('strong', null, 'Text → Video'), '.'))
      }));

      var warnHost = h('div', { style: { marginTop: '16px' } });
      function renderWarn() {
        warnHost.innerHTML = '';
        var st = App.state;
        if (st && st.tools && !st.tools.geminiKey) {
          warnHost.appendChild(amberWarn('Chưa có Gemini API key — ảnh sẽ dùng card màu thay vì ảnh AI'));
        }
      }
      renderWarn();
      el.appendChild(warnHost);

      el.appendChild(h('div', { class: 'row-between', style: { margin: '16px 0' } },
        h('div', { class: 'muted', style: { fontSize: '12.5px' } },
          'Bấm "Dùng bộ này" để đổi bộ style đang áp dụng cho toàn hệ thống.'),
        UI.btn('＋ Tạo bộ style', {
          onclick: function () { openEditor(null, function () { load(ctx); }); }
        })));

      el.appendChild(ctx.listHost);
      load(ctx);

      // ---------- SSE + dọn dẹp ----------
      var onJob = function (j) {
        if (!j || !j.id) return;
        var kitId = ctx.jobKit[j.id];
        if (!kitId) return;
        var card = ctx.cards[kitId];
        if (!card) return;
        card.setJob(j);
        if (j.status === 'done' || j.status === 'error') delete ctx.jobKit[j.id];
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
