/* ============================================================
   Biz Studio — Trang chi tiết dự án (project-detail)
   Được projects.js ủy quyền render khi hash = #/projects/<id>.
   ============================================================ */
(function () {
  'use strict';

  var STEPS = ['Phân tích', 'Dựng scene', 'Render draft', 'Lắp draft', 'Render final', 'Hoàn thành'];
  var P_STATUS = {
    done: ['Hoàn thành', 'badge-green'], running: ['Đang chạy', 'badge-amber'],
    draft: ['Nháp', 'badge-gray'], error: ['Lỗi', 'badge-red']
  };
  var S_STATUS = {
    running: ['Đang chạy', 'badge-amber pd-pulse'], done: ['Hoàn thành', 'badge-green'],
    error: ['Lỗi', 'badge-red'], stopped: ['Đã dừng', 'badge-gray']
  };
  var ASSET_ICON = { video: '🎞', image: '🖼', audio: '🎵' };
  var ASSET_GROUPS = [['video', 'VIDEO'], ['image', 'ẢNH'], ['audio', 'ÂM THANH']];

  function ensureCss() {
    if (document.getElementById('pd-style')) return;
    var st = document.createElement('style');
    st.id = 'pd-style';
    st.textContent =
      '.pd-grid{display:grid;grid-template-columns:320px minmax(0,1fr) 340px 360px;gap:16px;align-items:start;margin-top:16px}' +
      '@media(max-width:1500px){.pd-grid{grid-template-columns:320px minmax(0,1fr) 340px}.pd-ai{grid-column:1/-1;min-height:auto}}' +
      '@media(max-width:1100px){.pd-grid{grid-template-columns:1fr 1fr}}' +
      '@media(max-width:800px){.pd-grid{grid-template-columns:1fr}}' +
      '.pd-ai{background:var(--blue-soft);display:flex;flex-direction:column;min-height:72vh}' +
      '.pd-asset{border:1px solid var(--border);border-radius:10px;padding:8px 10px;margin-bottom:8px;background:var(--card)}' +
      '.pd-asset-head{display:flex;align-items:center;gap:8px;cursor:pointer;min-width:0}' +
      '.pd-asset-name{flex:1;min-width:0;font-size:12.5px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}' +
      '.pd-group-label{font-size:11px;font-weight:700;color:var(--muted);letter-spacing:.05em;margin:12px 0 6px}' +
      '.pd-log{flex:1;overflow-y:auto;max-height:55vh;display:flex;flex-direction:column;gap:8px;padding:4px 2px;margin:10px 0}' +
      '.pd-ev-init{font-size:12px;color:var(--muted)}' +
      '.pd-bubble{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:8px 10px;font-size:13px;white-space:pre-wrap;word-break:break-word;max-width:94%;align-self:flex-start}' +
      '.pd-bubble.user{align-self:flex-end;background:var(--blue-soft);border-color:var(--blue)}' +
      '.pd-tool{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:11.5px;color:var(--muted);word-break:break-all}' +
      '.pd-toolres{font-size:11.5px;color:var(--muted);white-space:pre-wrap;word-break:break-word}' +
      '.pd-result{border:1.5px solid var(--green);border-radius:10px;padding:8px 10px;font-size:13px;white-space:pre-wrap;word-break:break-word;background:var(--card)}' +
      '.pd-result-title{color:var(--green);font-weight:700;font-size:12.5px;margin-bottom:4px}' +
      '@keyframes pd-pulse{0%,100%{opacity:1}50%{opacity:.4}}.pd-pulse{animation:pd-pulse 1.2s ease infinite}' +
      '.pd-video{width:100%;border-radius:10px;background:#000;display:block}' +
      '.pd-thumb-img{width:100%;border-radius:10px;display:block;border:1px solid var(--border)}';
    document.head.appendChild(st);
  }

  // ---------- Helpers ----------

  function trunc(s, n) { s = String(s === undefined || s === null ? '' : s); return s.length > n ? s.slice(0, n) + '…' : s; }
  function fileName(p) { return String(p || '').split('/').pop(); }
  function badge(map, s) { var m = map[s] || [s || '—', 'badge-gray']; return h('span', { class: 'badge ' + m[1] }, m[0]); }
  function sub(ctx, ev, fn) { Bus.on(ev, fn); ctx.unsubs.push({ ev: ev, fn: fn }); }
  function dataHref(out) {
    if (!out) return null;
    if (out.indexOf('/data/') === 0) return out;
    if (out.charAt(0) === '/') { var i = out.indexOf('/data/'); return i >= 0 ? out.slice(i) : null; }
    return '/data/' + out;
  }
  function copyText(t, msg) {
    function fallback() {
      var ta = document.createElement('textarea');
      ta.value = t; document.body.appendChild(ta); ta.select();
      try { document.execCommand('copy'); UI.toast(msg); } catch (e) { UI.toast('Không sao chép được: ' + e.message, 'error'); }
      ta.remove();
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(t).then(function () { UI.toast(msg); }, fallback);
    } else fallback();
  }
  function setPageHeader(p) {
    App.setHeader(p.name, p.width + '×' + p.height + ' · ' + p.fps + 'fps · cập nhật ' + UI.timeAgo(p.updatedAt));
  }
  function saveProject(ctx, patch, okMsg) {
    var body = Object.assign({}, ctx.project, patch);
    return API.put('/api/projects/' + encodeURIComponent(ctx.project.id), body).then(function (np) {
      ctx.project = np || body;
      if (okMsg) UI.toast(okMsg);
      renderHeader(ctx);
      setPageHeader(ctx.project);
      return ctx.project;
    }).catch(function (e) { UI.toast('Không lưu được: ' + e.message, 'error'); throw e; });
  }
  function refreshProject(ctx) {
    return API.get('/api/projects/' + encodeURIComponent(ctx.project.id)).then(function (d) {
      ctx.project = d.project;
      renderHeader(ctx); setPageHeader(ctx.project); renderOutMedia(ctx);
    }).catch(function (e) { UI.toast('Không tải lại được dự án: ' + e.message, 'error'); });
  }

  // Theo dõi 1 job: progress bar trong host → gọi onDone khi xong.
  function watchJob(ctx, job, host, onDone) {
    host.innerHTML = '';
    var txt = h('div', { class: 'muted', style: { fontSize: '12px', marginBottom: '4px' } });
    var bar = UI.progress(job.progress);
    var wrap = h('div', { style: { marginTop: '10px' } }, txt, bar);
    host.appendChild(wrap);
    function line(j) {
      txt.textContent = (j.detail || 'Đang xử lý…') + ' · ' + Math.round(j.progress || 0) + '%';
    }
    line(job);
    var fn = Bus.on('job', function (j) {
      if (j.id !== job.id) return;
      if (j.status === 'done') {
        Bus.off('job', fn); wrap.remove();
        if (onDone) onDone(j);
        return;
      }
      if (j.status === 'error') {
        Bus.off('job', fn); bar.remove();
        txt.className = 'text-red'; txt.style.fontSize = '12px';
        txt.textContent = '⚠ Lỗi: ' + (j.error || 'không rõ');
        return;
      }
      line(j); bar.set(j.progress);
    });
    ctx.unsubs.push({ ev: 'job', fn: fn });
  }

  // ---------- Header: stepper + badges + hành động ----------

  function stepperEl(progress) {
    var row = h('div', { class: 'stepper', style: { flex: '1', minWidth: '380px', overflowX: 'auto', paddingBottom: '4px' } });
    STEPS.forEach(function (label, i) {
      if (i > 0) row.appendChild(h('div', { class: 'step-line' + (progress > i ? ' done' : '') }));
      var cls = 'step' + (progress > i ? ' done' : (progress === i ? ' active' : ''));
      row.appendChild(h('div', { class: cls },
        h('div', { class: 'step-dot' }, progress > i ? '✓' : String(i + 1)),
        h('div', { class: 'step-label' }, label)));
    });
    return row;
  }

  function tagAdder(ctx) {
    var wrap = h('span');
    function showBtn() {
      wrap.innerHTML = '';
      wrap.appendChild(h('button', { class: 'btn btn-ghost btn-sm', onclick: showInput }, '+ Tag'));
    }
    function showInput() {
      var inp = h('input', { class: 'input', placeholder: 'Tên tag…', style: { width: '140px', height: '30px', padding: '4px 10px', fontSize: '12.5px' } });
      inp.onkeydown = function (e) {
        if (e.key === 'Escape') { showBtn(); return; }
        if (e.key !== 'Enter') return;
        var v = inp.value.trim();
        if (!v) { showBtn(); return; }
        var tags = (ctx.project.tags || []).slice();
        if (tags.indexOf(v) < 0) tags.push(v);
        saveProject(ctx, { tags: tags }, 'Đã thêm tag "' + v + '"');
      };
      inp.onblur = function () { setTimeout(showBtn, 150); };
      wrap.innerHTML = ''; wrap.appendChild(inp); inp.focus();
    }
    showBtn();
    return wrap;
  }

  function renderHeader(ctx) {
    var p = ctx.project;
    ctx.headerHost.innerHTML = '';
    var actions = h('div', { class: 'row', style: { flexWrap: 'nowrap', flex: 'none' } },
      UI.btn('▶ Render final', {
        variant: 'danger',
        onclick: function () {
          API.post('/api/projects/' + encodeURIComponent(p.id) + '/render-final').then(function (job) {
            UI.toast('Đã bắt đầu render final');
            watchJob(ctx, job, ctx.headerJobHost, function () { UI.toast('Render final hoàn tất!'); refreshProject(ctx); });
          }).catch(function (e) { UI.toast('Không render được: ' + e.message, 'error'); });
        }
      }),
      UI.btn('Nhân bản', {
        variant: 'ghost',
        onclick: function () {
          API.post('/api/projects/' + encodeURIComponent(p.id) + '/duplicate').then(function (np) {
            UI.toast('Đã nhân bản dự án'); App.navigate('projects/' + np.id);
          }).catch(function (e) { UI.toast('Không nhân bản được: ' + e.message, 'error'); });
        }
      }),
      UI.btn('🗑 Xóa', {
        variant: 'ghost',
        onclick: function () {
          var m = UI.modal({
            title: 'Xóa dự án',
            body: h('p', null, 'Xóa vĩnh viễn dự án "', h('b', null, p.name), '" cùng toàn bộ dữ liệu? Không thể hoàn tác.'),
            actions: [
              UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
              UI.btn('Xóa vĩnh viễn', {
                variant: 'danger',
                onclick: function () {
                  API.del('/api/projects/' + encodeURIComponent(p.id)).then(function () {
                    m.close(); UI.toast('Đã xóa dự án'); App.navigate('projects');
                  }).catch(function (e) { UI.toast('Không xóa được: ' + e.message, 'error'); });
                }
              })]
          });
        }
      }));

    var badges = h('div', { class: 'row', style: { marginTop: '10px' } },
      badge(P_STATUS, p.status),
      h('span', { class: 'badge badge-gray' }, 'ID: ' + p.id),
      (p.tags || []).map(function (t) {
        return UI.chip(t, {
          onremove: function () {
            saveProject(ctx, { tags: (p.tags || []).filter(function (x) { return x !== t; }) }, 'Đã bỏ tag "' + t + '"');
          }
        });
      }),
      tagAdder(ctx));

    ctx.headerJobHost = h('div');
    ctx.headerHost.appendChild(UI.card({
      body: [h('div', { class: 'row-between', style: { alignItems: 'flex-start', gap: '16px', flexWrap: 'wrap' } },
        stepperEl(p.progress || 0), actions), badges, ctx.headerJobHost]
    }));
  }

  // ---------- Cột 1: Nguồn & Asset ----------

  function assetItem(ctx, a) {
    var open = false;
    var detail = h('div', { style: { display: 'none', marginTop: '8px' } });
    var ta = UI.textarea({ value: a.desc || '', rows: 2, placeholder: 'Mô tả asset này cho AI hiểu…' });
    detail.appendChild(ta);
    detail.appendChild(h('div', { class: 'row-between', style: { marginTop: '6px' } },
      UI.btn('Lưu', {
        small: true,
        onclick: function () {
          API.put('/api/assets/' + encodeURIComponent(a.id), { desc: ta.value.trim(), order: a.order || 0 }).then(function (na) {
            for (var i = 0; i < ctx.assets.length; i++) if (ctx.assets[i].id === a.id) ctx.assets[i] = na || Object.assign({}, a, { desc: ta.value.trim() });
            UI.toast('Đã lưu mô tả asset'); renderAssetsCol(ctx);
          }).catch(function (e) { UI.toast('Không lưu được: ' + e.message, 'error'); });
        }
      }),
      UI.btn('Xóa', {
        variant: 'danger', small: true,
        onclick: function () {
          API.del('/api/assets/' + encodeURIComponent(a.id)).then(function () {
            ctx.assets = ctx.assets.filter(function (x) { return x.id !== a.id; });
            UI.toast('Đã xóa asset "' + a.name + '"'); renderAssetsCol(ctx);
          }).catch(function (e) { UI.toast('Không xóa được: ' + e.message, 'error'); });
        }
      })));

    var caret = h('span', { class: 'muted', style: { flex: 'none', fontSize: '11px' } }, '▾');
    var head = h('div', {
      class: 'pd-asset-head',
      onclick: function () { open = !open; detail.style.display = open ? 'block' : 'none'; caret.textContent = open ? '▴' : '▾'; }
    },
      h('span', { style: { flex: 'none' } }, ASSET_ICON[a.kind] || '📄'),
      h('span', { class: 'pd-asset-name', title: a.name }, a.name),
      h('span', { class: 'muted', style: { flex: 'none', fontSize: '11px' } }, UI.fmtBytes(a.size)),
      !a.desc ? h('span', { class: 'badge badge-amber', style: { flex: 'none' } }, 'Chưa mô tả') : null,
      caret);
    return h('div', { class: 'pd-asset' }, head, detail);
  }

  function qrModal(ctx) {
    var m = UI.modal({
      title: '📱 Kết nối điện thoại',
      body: h('div', { style: { textAlign: 'center' } },
        h('img', {
          src: '/api/qr.png?project=' + encodeURIComponent(ctx.project.id), alt: 'QR kết nối',
          style: { width: '220px', height: '220px', borderRadius: '12px', border: '1px solid var(--border)' }
        }),
        h('p', { class: 'muted', style: { marginTop: '10px' } },
          'Quét QR bằng camera điện thoại (cùng Wi-Fi) để gửi file thẳng vào dự án.')),
      actions: [UI.btn('Đóng', { variant: 'ghost', onclick: function () { m.close(); } })]
    });
  }

  function pickUpload(ctx) {
    var inp = h('input', { type: 'file', multiple: 'multiple', style: { display: 'none' } });
    document.body.appendChild(inp);
    inp.onchange = function () {
      var files = Array.prototype.slice.call(inp.files || []);
      inp.remove();
      if (!files.length) return;
      UI.toast('Đang tải lên ' + files.length + ' file…');
      API.upload('/api/projects/' + encodeURIComponent(ctx.project.id) + '/assets', files).then(function () {
        UI.toast('Đã tải lên ' + files.length + ' file');
        return API.get('/api/projects/' + encodeURIComponent(ctx.project.id));
      }).then(function (d) {
        if (d) { ctx.assets = d.assets || []; renderAssetsCol(ctx); }
      }).catch(function (e) { UI.toast('Tải lên thất bại: ' + e.message, 'error'); });
    };
    inp.click();
  }

  function renderAssetsCol(ctx) {
    ctx.colAssets.innerHTML = '';
    var body = [h('div', { class: 'row', style: { marginBottom: '4px' } },
      UI.btn('📱 Kết nối điện thoại', { variant: 'ghost', small: true, onclick: function () { qrModal(ctx); } }),
      UI.btn('⬆ Tải lên', { variant: 'ghost', small: true, onclick: function () { pickUpload(ctx); } }))];
    var main = ctx.assets.filter(function (a) { return a.kind !== 'other'; });
    var others = ctx.assets.filter(function (a) { return a.kind === 'other'; });
    if (!main.length) {
      body.push(UI.empty('Chưa có asset — tải lên hoặc quét QR từ điện thoại.', '🗂'));
    } else {
      ASSET_GROUPS.forEach(function (g) {
        var items = main.filter(function (a) { return a.kind === g[0]; });
        if (!items.length) return;
        body.push(h('div', { class: 'pd-group-label' }, g[1] + ' · ' + items.length));
        items.forEach(function (a) { body.push(assetItem(ctx, a)); });
      });
    }
    ctx.colAssets.appendChild(UI.card({ title: 'Nguồn & Asset', icon: '🗂', body: body }));
    if (others.length) {
      ctx.colAssets.appendChild(UI.card({
        title: 'Khác', icon: '📦',
        body: others.map(function (a) { return assetItem(ctx, a); })
      }));
    }
  }

  // ---------- Cột 2: Kịch bản edit (Brief) ----------

  function renderBriefCol(ctx) {
    var p = ctx.project;
    ctx.colBrief.innerHTML = '';
    var briefTa = UI.textarea({ value: p.briefDesc || '', rows: 3, placeholder: 'Video gốc quay gì, bối cảnh, nhân vật, điểm nhấn…' });
    var promptTa = UI.textarea({ value: p.editPrompt || '', rows: 8, placeholder: 'Mô tả cách bạn muốn AI edit video này…' });

    var promptSel = h('select', { class: 'select' }, h('option', { value: '' }, 'Dùng prompt mẫu…'));
    var promptMap = {};
    API.get('/api/prompts').then(function (list) {
      (list || []).forEach(function (pt) {
        promptMap[pt.id] = pt.body;
        promptSel.appendChild(h('option', { value: pt.id }, pt.name));
      });
    }).catch(function (e) {
      promptSel.appendChild(h('option', { value: '', disabled: 'disabled' }, 'Không tải được prompt mẫu: ' + e.message));
    });
    promptSel.onchange = function () {
      if (promptSel.value && promptMap[promptSel.value] !== undefined) promptTa.value = promptMap[promptSel.value];
    };

    var tgCut = UI.toggle('Tự động cắt ngắn video', 'AI cắt bỏ khoảng lặng, đoạn thừa', p.autoCut);
    var tgSub = UI.toggle('Tạo phụ đề', 'Phụ đề karaoke theo giọng nói', p.autoSub);
    var tgKey = UI.toggle('Làm nổi bật key chính', 'AI chọn keyword để highlight', p.autoKey);

    var kw = (p.keywords || []).slice();
    var kwHost = h('div');
    function renderKw() {
      kwHost.innerHTML = '';
      kw.forEach(function (k, i) {
        kwHost.appendChild(UI.chip(k, { onremove: function () { kw.splice(i, 1); renderKw(); } }));
      });
    }
    renderKw();
    var kwIn = UI.input({
      placeholder: 'Nhập keyword rồi nhấn Enter…',
      onkeydown: function (e) {
        if (e.key !== 'Enter') return;
        e.preventDefault();
        var v = kwIn.value.trim();
        if (v && kw.indexOf(v) < 0) { kw.push(v); renderKw(); }
        kwIn.value = '';
      }
    });

    ctx.colBrief.appendChild(UI.card({
      title: 'Kịch bản edit (Brief)', icon: '📝',
      body: [
        UI.field('Mô tả video gốc', briefTa),
        h('div', { class: 'field' },
          h('label', { class: 'field-label' }, 'Yêu cầu edit (prompt)'),
          h('div', { class: 'row', style: { flexWrap: 'nowrap', marginBottom: '6px' } },
            h('div', { style: { flex: '1', minWidth: 0 } }, promptSel),
            h('a', { href: '#/prompts', class: 'muted', style: { fontSize: '12.5px', whiteSpace: 'nowrap' } }, 'Quản lý prompt')),
          promptTa),
        tgCut, tgSub, tgKey,
        h('div', { class: 'field', style: { marginTop: '8px' } },
          h('label', { class: 'field-label' }, 'Keyword chỉ định thêm (tùy chọn)'),
          kwHost, kwIn)
      ],
      foot: UI.btn('💾 Lưu kịch bản', {
        onclick: function () {
          saveProject(ctx, {
            briefDesc: briefTa.value, editPrompt: promptTa.value,
            autoCut: tgCut.input.checked, autoSub: tgSub.input.checked, autoKey: tgKey.input.checked,
            keywords: kw.slice()
          }, 'Đã lưu kịch bản edit');
        }
      })
    }));
  }

  // ---------- Cột 3: Output / Thumbnail / QC / Gói xuất bản ----------

  function renderOutMedia(ctx) {
    var p = ctx.project;
    ctx.outHost.innerHTML = '';
    if (p.outputFile) {
      ctx.outHost.appendChild(h('video', { class: 'pd-video', controls: 'controls', src: '/data/' + p.outputFile }));
      ctx.outHost.appendChild(h('div', { class: 'row-between mt-8' },
        h('span', { class: 'muted', title: p.outputFile, style: { fontSize: '12px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }, fileName(p.outputFile)),
        UI.btn('Mở file', {
          variant: 'ghost', small: true,
          onclick: function () { copyText(location.origin + '/data/' + p.outputFile, 'Đã sao chép đường dẫn video'); }
        })));
    } else {
      ctx.outHost.appendChild(UI.empty('Chưa có video output', '🎬'));
    }
    ctx.thumbHost.innerHTML = '';
    if (p.thumbFile) {
      ctx.thumbHost.appendChild(h('img', { class: 'pd-thumb-img', src: '/data/' + p.thumbFile + '?v=' + Date.now(), alt: 'Thumbnail' }));
    } else {
      ctx.thumbHost.appendChild(UI.empty('Chưa có thumbnail', '🖼'));
    }
  }

  function thumbModal(ctx) {
    var modeSel = UI.select(null, [
      { value: 'frame', label: 'Từ frame video' },
      { value: 'ai', label: 'Bằng AI (Gemini)' }
    ], 'frame', function (v) {
      frameField.style.display = v === 'frame' ? '' : 'none';
      aiField.style.display = v === 'ai' ? '' : 'none';
    });
    var tIn = UI.input({ type: 'number', value: 1, min: 0, step: 0.5 });
    var promptIn = UI.textarea({ rows: 3, placeholder: 'Mô tả thumbnail muốn AI vẽ…' });
    var frameField = UI.field('Lấy tại giây thứ', tIn);
    var aiField = UI.field('Prompt tạo ảnh', promptIn);
    aiField.style.display = 'none';
    var m = UI.modal({
      title: 'Tạo thumbnail',
      body: [UI.field('Cách tạo', modeSel), frameField, aiField],
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('Tạo thumbnail', {
          onclick: function () {
            var mode = modeSel.value;
            if (mode === 'ai' && !promptIn.value.trim()) { UI.toast('Vui lòng nhập prompt tạo ảnh', 'error'); return; }
            API.post('/api/projects/' + encodeURIComponent(ctx.project.id) + '/thumbnail', {
              mode: mode, t: Number(tIn.value) || 0, prompt: promptIn.value.trim()
            }).then(function (job) {
              m.close(); UI.toast('Đang tạo thumbnail…');
              watchJob(ctx, job, ctx.thumbJobHost, function () { UI.toast('Đã tạo thumbnail'); refreshProject(ctx); });
            }).catch(function (e) { UI.toast('Không tạo được thumbnail: ' + e.message, 'error'); });
          }
        })]
    });
  }

  function spanChips(label, spans) {
    if (!spans || !spans.length) return null;
    return h('div', { class: 'mt-8' },
      h('div', { style: { fontWeight: 600, fontSize: '12.5px', marginBottom: '4px' } }, label + ' (' + spans.length + ')'),
      h('div', null, spans.map(function (s) {
        return h('span', { class: 'badge badge-gray', style: { margin: '0 4px 4px 0' } }, UI.fmtDur(s.start) + ' – ' + UI.fmtDur(s.end));
      })));
  }

  function renderQcReport(host, r) {
    host.innerHTML = '';
    host.appendChild(h('div', { class: 'row mt-8', style: { fontSize: '12.5px' } },
      h('span', null, h('b', null, 'Âm lượng: '), (typeof r.loudnessLufs === 'number' ? r.loudnessLufs.toFixed(1) : '—') + ' LUFS'),
      h('span', { class: 'muted' }, '· ' + UI.fmtDur(r.durationS || 0) + ' · ' + (r.width || 0) + '×' + (r.height || 0))));
    var b = spanChips('Frame đen', r.blackSpans), f = spanChips('Đứng hình', r.freezeSpans), s = spanChips('Khoảng lặng', r.silenceSpans);
    if (b) host.appendChild(b);
    if (f) host.appendChild(f);
    if (s) host.appendChild(s);
    var warns = r.warnings || [];
    if (!warns.length) {
      host.appendChild(h('div', { class: 'text-green mt-8', style: { fontWeight: 700 } }, '✓ Đạt — không có cảnh báo'));
    } else {
      warns.forEach(function (w) {
        var cls = /lỗi|error|hỏng|fail/i.test(w) ? 'text-red' : 'text-amber';
        host.appendChild(h('div', { class: cls + ' mt-8', style: { fontSize: '12.5px' } }, '⚠ ' + w));
      });
    }
  }

  function loadQcReport(ctx) {
    API.get('/data/projects/' + encodeURIComponent(ctx.project.id) + '/qc.json')
      .then(function (r) { if (r) renderQcReport(ctx.qcHost, r); })
      .catch(function (e) { ctx.qcHost.innerHTML = ''; ctx.qcHost.appendChild(h('div', { class: 'text-red mt-8', style: { fontSize: '12px' } }, 'Không đọc được báo cáo QC: ' + e.message)); });
  }

  function showPublishLink(ctx, job) {
    ctx.pubHost.innerHTML = '';
    var href = dataHref(job.output);
    if (href) {
      ctx.pubHost.appendChild(h('a', { href: href, download: '', class: 'btn btn-ghost btn-sm', style: { marginTop: '10px', textDecoration: 'none' } }, '⬇ Tải gói xuất bản (.zip)'));
    } else {
      ctx.pubHost.appendChild(h('div', { class: 'row mt-8', style: { fontSize: '12px' } },
        h('span', { class: 'muted' }, trunc(job.output, 60)),
        UI.btn('Copy', { variant: 'ghost', small: true, onclick: function () { copyText(job.output, 'Đã sao chép đường dẫn'); } })));
    }
  }

  function renderOutputCol(ctx) {
    ctx.colOut.innerHTML = '';
    ctx.outHost = h('div'); ctx.thumbHost = h('div');
    ctx.thumbJobHost = h('div'); ctx.qcHost = h('div'); ctx.qcJobHost = h('div'); ctx.pubHost = h('div'); ctx.pubJobHost = h('div');
    ctx.colOut.appendChild(UI.card({ title: 'Video output', icon: '🎬', body: ctx.outHost }));
    ctx.colOut.appendChild(UI.card({
      title: 'Thumbnail', icon: '🖼',
      body: [ctx.thumbHost, ctx.thumbJobHost],
      foot: UI.btn('Tạo thumbnail', { variant: 'ghost', small: true, onclick: function () { thumbModal(ctx); } })
    }));
    ctx.colOut.appendChild(UI.card({
      title: 'QC tự động', icon: '🛡',
      desc: 'QC đo âm lượng, tìm frame đen, đoạn đứng hình, khoảng lặng…',
      body: [ctx.qcHost, ctx.qcJobHost],
      foot: UI.btn('🛡 Chạy QC', {
        variant: 'ghost', small: true,
        onclick: function () {
          API.post('/api/projects/' + encodeURIComponent(ctx.project.id) + '/qc').then(function (job) {
            UI.toast('Đang chạy QC…');
            watchJob(ctx, job, ctx.qcJobHost, function () { loadQcReport(ctx); });
          }).catch(function (e) { UI.toast('Không chạy được QC: ' + e.message, 'error'); });
        }
      })
    }));
    ctx.colOut.appendChild(UI.card({
      title: 'Gói xuất bản', icon: '📦',
      desc: 'Sinh phụ đề .srt/.vtt, tiêu đề, mô tả, hashtag, thumbnail.',
      body: [ctx.pubHost, ctx.pubJobHost],
      foot: UI.btn('📦 Tạo gói xuất bản', {
        variant: 'ghost', small: true,
        onclick: function () {
          API.post('/api/projects/' + encodeURIComponent(ctx.project.id) + '/publish').then(function (job) {
            UI.toast('Đang tạo gói xuất bản…');
            watchJob(ctx, job, ctx.pubJobHost, function (j) { UI.toast('Gói xuất bản đã sẵn sàng'); showPublishLink(ctx, j); });
          }).catch(function (e) { UI.toast('Không tạo được gói: ' + e.message, 'error'); });
        }
      })
    }));
    renderOutMedia(ctx);
    // Kết quả sẵn có từ các job đã xong trước đó
    var doneQc = null, donePub = null;
    (ctx.jobs || []).forEach(function (j) {
      if (j.status !== 'done') return;
      if (j.kind === 'qc') doneQc = j;
      if (j.kind === 'publish') donePub = j;
    });
    if (doneQc) loadQcReport(ctx);
    if (donePub) showPublishLink(ctx, donePub);
  }

  // Gắn lại progress cho job đang chạy khi mở trang.
  function attachRunningJobs(ctx) {
    var hostByKind = { qc: ctx.qcJobHost, thumbnail: ctx.thumbJobHost, publish: ctx.pubJobHost, render: ctx.headerJobHost };
    var doneByKind = {
      qc: function () { loadQcReport(ctx); },
      thumbnail: function () { refreshProject(ctx); },
      publish: function (j) { showPublishLink(ctx, j); },
      render: function () { refreshProject(ctx); }
    };
    (ctx.jobs || []).forEach(function (j) {
      if (j.status !== 'running' && j.status !== 'queued') return;
      var host = hostByKind[j.kind];
      if (host) watchJob(ctx, j, host, doneByKind[j.kind]);
    });
  }

  // ---------- Cột 4: Panel AI ----------

  function eventEl(ev) {
    switch (ev.type) {
      case 'init': return h('div', { class: 'pd-ev-init' }, '⚙ Khởi tạo · ' + (ev.payload || ''));
      case 'text': return h('div', { class: 'pd-bubble' }, ev.payload || '');
      case 'tool': return h('div', { class: 'pd-tool' }, '🔧 ' + (ev.name || 'tool') + ' ' + trunc(ev.payload, 120));
      case 'tool_result': return h('div', { class: 'pd-toolres' }, trunc(ev.payload, 200));
      case 'result': return h('div', { class: 'pd-result' }, h('div', { class: 'pd-result-title' }, '✓ Hoàn thành'), h('div', null, ev.payload || ''));
      case 'user': return h('div', { class: 'pd-bubble user' }, ev.payload || '');
      case 'error': return h('div', { class: 'text-red', style: { fontSize: '12.5px' } }, '⚠ ' + (ev.payload || 'Lỗi không rõ'));
      default: return h('div', { class: 'pd-ev-init' }, (ev.type || '?') + ' · ' + trunc(ev.payload, 120));
    }
  }

  function sessionLabel(s) {
    var st = S_STATUS[s.status] ? S_STATUS[s.status][0] : (s.status || '—');
    return (s.title || s.id) + ' · ' + st + ' · ' + UI.timeAgo(s.startedAt);
  }

  function renderAI(ctx) {
    var panel = ctx.aiPanel;
    panel.innerHTML = '';
    var logHost = h('div', { class: 'pd-log' });
    var statusHost = h('div', { class: 'mt-8' });
    var sessSel = h('select', { class: 'select', style: { marginTop: '10px' } });
    var elapsedSpan = null;

    function cur() {
      for (var i = 0; i < ctx.sessions.length; i++) if (ctx.sessions[i].id === ctx.selSid) return ctx.sessions[i];
      return null;
    }
    function elapsedText(s) {
      var start = new Date(s.startedAt).getTime();
      var end = s.status === 'running' ? Date.now() : new Date(s.endedAt || s.startedAt).getTime();
      return 'chạy ' + UI.fmtDur(Math.max(0, (end - start) / 1000));
    }
    function scrollBottom() { logHost.scrollTop = logHost.scrollHeight; }
    function appendEvent(ev) { logHost.appendChild(eventEl(ev)); scrollBottom(); }

    ctx.refreshSessionOptions = function () {
      sessSel.innerHTML = '';
      var list = ctx.sessions.slice().sort(function (a, b) { return new Date(b.startedAt) - new Date(a.startedAt); });
      if (!list.length) {
        sessSel.appendChild(h('option', { value: '' }, 'Chưa có phiên nào'));
        sessSel.disabled = true;
        return;
      }
      sessSel.disabled = false;
      list.forEach(function (s) { sessSel.appendChild(h('option', { value: s.id }, sessionLabel(s))); });
      if (ctx.selSid) sessSel.value = ctx.selSid;
    };

    ctx.renderSessionStatus = function () {
      statusHost.innerHTML = '';
      var s = cur();
      elapsedSpan = null;
      if (!s) {
        statusHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px' } }, 'Chọn hoặc tạo phiên để bắt đầu.'));
        return;
      }
      elapsedSpan = h('span', { class: 'muted', style: { fontSize: '12px' } }, elapsedText(s));
      statusHost.appendChild(h('div', { class: 'row-between' },
        h('div', { class: 'row' }, badge(S_STATUS, s.status), elapsedSpan,
          s.costUsd ? h('span', { class: 'muted', style: { fontSize: '11.5px' } }, '$' + s.costUsd.toFixed(3)) : null),
        s.status === 'running' ? UI.btn('⏹ Dừng', {
          variant: 'danger', small: true,
          onclick: function () {
            API.post('/api/sessions/' + encodeURIComponent(s.id) + '/stop').then(function () {
              UI.toast('Đã gửi yêu cầu dừng phiên');
            }).catch(function (e) { UI.toast('Không dừng được: ' + e.message, 'error'); });
          }
        }) : null));
    };

    function selectSession(sid) {
      ctx.selSid = sid || null;
      ctx.renderSessionStatus();
      logHost.innerHTML = '';
      if (!ctx.selSid) return;
      logHost.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải log phiên…'));
      API.get('/api/sessions/' + encodeURIComponent(sid) + '/events').then(function (evts) {
        if (ctx.selSid !== sid) return;
        logHost.innerHTML = '';
        (evts || []).forEach(function (ev) { logHost.appendChild(eventEl(ev)); });
        scrollBottom();
      }).catch(function (e) {
        logHost.innerHTML = '';
        logHost.appendChild(h('div', { class: 'text-red', style: { fontSize: '12px' } }, 'Không tải được log: ' + e.message));
      });
    }
    ctx.selectSession = selectSession;
    ctx.appendSessionEvent = appendEvent;
    sessSel.onchange = function () { selectSession(sessSel.value); };

    var msgIn = UI.input({
      placeholder: 'Dặn dò thêm cho AI…',
      onkeydown: function (e) { if (e.key === 'Enter') send(); }
    });
    function send() {
      var t = msgIn.value.trim();
      if (!t) return;
      if (!ctx.selSid) { UI.toast('Hãy chọn hoặc tạo phiên AI trước', 'error'); return; }
      msgIn.value = '';
      ctx.lastUserMsg = { text: t, ts: Date.now() };
      appendEvent({ type: 'user', payload: t });
      API.post('/api/sessions/' + encodeURIComponent(ctx.selSid) + '/message', { text: t })
        .catch(function (e) { UI.toast('Không gửi được: ' + e.message, 'error'); });
    }

    panel.appendChild(h('div', { class: 'card-title' }, '🤖 AI của project'));
    panel.appendChild(UI.btn('✨ Bắt đầu Edit bằng AI với phiên mới', {
      onclick: function () {
        API.post('/api/projects/' + encodeURIComponent(ctx.project.id) + '/sessions', { extra: '' }).then(function (s) {
          ctx.sessions.unshift(s);
          ctx.refreshSessionOptions();
          sessSel.value = s.id;
          selectSession(s.id);
          UI.toast('Đã tạo phiên AI mới');
        }).catch(function (e) { UI.toast('Không tạo được phiên: ' + e.message, 'error'); });
      }
    }));
    panel.lastChild.style.width = '100%';
    panel.appendChild(sessSel);
    panel.appendChild(statusHost);
    panel.appendChild(logHost);
    panel.appendChild(h('div', { class: 'row', style: { flexWrap: 'nowrap' } },
      h('div', { style: { flex: '1', minWidth: 0 } }, msgIn),
      UI.btn('Gửi', { onclick: send })));

    // Đồng hồ thời gian chạy (chỉ cập nhật text, 1s/lần)
    ctx.timers.push(setInterval(function () {
      var s = cur();
      if (s && s.status === 'running' && elapsedSpan) elapsedSpan.textContent = elapsedText(s);
    }, 1000));

    ctx.refreshSessionOptions();
    var newest = ctx.sessions.slice().sort(function (a, b) { return new Date(b.startedAt) - new Date(a.startedAt); })[0];
    if (newest) { sessSel.value = newest.id; selectSession(newest.id); }
    else ctx.renderSessionStatus();
  }

  // ---------- Realtime ----------

  function bindRealtime(ctx) {
    sub(ctx, 'session_event', function (d) {
      if (!d || !d.event || d.sessionId !== ctx.selSid) return;
      var ev = d.event;
      // tránh lặp bubble user đã hiển thị ngay khi gửi
      if (ev.type === 'user' && ctx.lastUserMsg && ev.payload === ctx.lastUserMsg.text &&
        Date.now() - ctx.lastUserMsg.ts < 8000) return;
      ctx.appendSessionEvent(ev);
    });
    sub(ctx, 'session', function (s) {
      if (!s || s.projectId !== ctx.project.id) return;
      var found = false;
      for (var i = 0; i < ctx.sessions.length; i++) {
        if (ctx.sessions[i].id === s.id) { ctx.sessions[i] = s; found = true; break; }
      }
      if (!found) ctx.sessions.unshift(s);
      ctx.refreshSessionOptions();
      if (s.id === ctx.selSid) ctx.renderSessionStatus();
    });
    sub(ctx, 'job', function (j) {
      if (!j || j.projectId !== ctx.project.id) return;
      var found = false;
      for (var i = 0; i < ctx.jobs.length; i++) {
        if (ctx.jobs[i].id === j.id) { ctx.jobs[i] = j; found = true; break; }
      }
      if (!found) ctx.jobs.unshift(j);
    });
    sub(ctx, 'project', function (p) {
      if (!p || p.id !== ctx.project.id) return;
      var outChanged = p.outputFile !== ctx.project.outputFile || p.thumbFile !== ctx.project.thumbFile;
      ctx.project = Object.assign({}, ctx.project, {
        status: p.status, progress: p.progress, tags: p.tags,
        outputFile: p.outputFile, thumbFile: p.thumbFile, updatedAt: p.updatedAt
      });
      renderHeader(ctx);
      setPageHeader(ctx.project);
      if (outChanged) renderOutMedia(ctx);
    });
  }

  // ---------- Render chính ----------

  function build(ctx) {
    setPageHeader(ctx.project);
    ctx.headerHost = h('div');
    ctx.colAssets = h('div', { class: 'pd-col' });
    ctx.colBrief = h('div', { class: 'pd-col' });
    ctx.colOut = h('div', { class: 'pd-col' });
    ctx.aiPanel = h('div', { class: 'card pd-ai' });
    ctx.el.appendChild(ctx.headerHost);
    ctx.el.appendChild(h('div', { class: 'pd-grid' }, ctx.colAssets, ctx.colBrief, ctx.colOut, ctx.aiPanel));
    renderHeader(ctx);
    renderAssetsCol(ctx);
    renderBriefCol(ctx);
    renderOutputCol(ctx);
    renderAI(ctx);
    bindRealtime(ctx);
    attachRunningJobs(ctx);
    App._cleanup = function () {
      ctx.unsubs.forEach(function (u) { Bus.off(u.ev, u.fn); });
      ctx.timers.forEach(clearInterval);
    };
  }

  App.pages['project-detail'] = {
    title: 'Dự án',
    subtitle: 'Đang tải…',
    render: function (el, projectId) {
      ensureCss();
      el.innerHTML = '';
      if (!projectId) {
        el.appendChild(UI.empty('Thiếu mã dự án trong đường dẫn', '❓'));
        return;
      }
      el.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải dự án…'));
      API.get('/api/projects/' + encodeURIComponent(projectId)).then(function (d) {
        el.innerHTML = '';
        if (!d || !d.project) throw new Error('Phản hồi không hợp lệ từ máy chủ');
        build({
          el: el, project: d.project, assets: d.assets || [], sessions: d.sessions || [],
          jobs: d.jobs || [], unsubs: [], timers: [], selSid: null, lastUserMsg: null
        });
      }).catch(function (e) {
        el.innerHTML = '';
        App.setHeader('Dự án', 'Không tải được dự án');
        el.appendChild(UI.card({
          title: 'Không tải được dự án', icon: '❌',
          body: [h('p', { class: 'text-red' }, e.message),
            UI.btn('← Về danh sách dự án', { variant: 'ghost', onclick: function () { App.navigate('projects'); } })]
        }));
      });
    }
  };
})();
