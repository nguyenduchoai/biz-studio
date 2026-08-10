/* ============================================================
   Biz Studio — Trang "Text → Video"
   Phiên làm việc 5 bước: Nguồn → Kịch bản → Giọng đọc → Cấu hình → Dựng video.
   Phiên lưu trong store, quay lại sửa tiếp bất cứ lúc nào.
   Đăng ký App.pages['text2video']. Load sau api.js / ui.js / app.js.
   ============================================================ */
(function () {
  'use strict';

  var STEPS = ['Nguồn', 'Kịch bản', 'Giọng đọc', 'Cấu hình', 'Dựng video'];

  var STATUS = {
    draft:    ['Nháp', 'badge-gray'],
    script:   ['Có kịch bản', 'badge-blue'],
    voice:    ['Có giọng đọc', 'badge-blue'],
    building: ['Đang dựng', 'badge-amber'],
    done:     ['Hoàn thành', 'badge-green'],
    error:    ['Lỗi', 'badge-red']
  };

  var MODEL_GROUPS = [
    { group: 'Claude subscription (không cần API key)', items: [
      { value: 'claude:',       label: 'Claude — model mặc định (theo Cấu hình & API)' },
      { value: 'claude:opus',   label: 'Claude Opus — viết hay nhất, chậm hơn' },
      { value: 'claude:sonnet', label: 'Claude Sonnet — cân bằng (khuyên dùng)' },
      { value: 'claude:haiku',  label: 'Claude Haiku — nhanh, tiết kiệm' }
    ]},
    { group: 'Engine khác', items: [
      { value: 'gemini:', label: 'Gemini (cần Gemini API key)' },
      { value: 'openai:', label: 'API Trực Tiếp (OpenAI-compatible)' }
    ]}
  ];

  var STYLE_OPTS = [
    { value: 'tu_nhien', label: 'Tự nhiên (hội thoại)' },
    { value: 'tin_tuc', label: 'Tin tức' },
    { value: 'doc_truyen', label: 'Đọc truyện' }
  ];

  var SIZE_PRESETS = [
    { value: '1080x1920', label: '1080×1920 — Dọc (TikTok / Shorts)', w: 1080, h: 1920 },
    { value: '1920x1080', label: '1920×1080 — Ngang (YouTube)',       w: 1920, h: 1080 },
    { value: '1080x1080', label: '1080×1080 — Vuông',                 w: 1080, h: 1080 }
  ];

  var ENGINE_LABELS = { vieneu: 'VieNeu', say: 'macOS', gemini: 'Gemini', clone: 'Nhân bản' };
  var BUSY_BY_KIND = {
    t2v_script: 'script', t2v_storyboard: 'storyboard', t2v_voice: 'voice', t2v_build: 'build'
  };
  var JOB_LABEL = {
    t2v_script: '✂ Viết kịch bản', t2v_storyboard: '🖼 Sinh ảnh storyboard',
    t2v_shot: '🖼 Tạo lại ảnh cảnh', t2v_voice: '🎙 Tạo giọng đọc', t2v_build: '🎬 Dựng video'
  };
  var LOG_RE = /t2v|text2video|tts|vieneu|htmlvideo|agent/i;
  var VIDEO_EXT = ['.mp4', '.mov', '.mkv', '.webm', '.m4v'];

  // ---------- CSS nội bộ ----------

  function ensureCss() {
    if (document.getElementById('t2v-style')) return;
    var st = document.createElement('style');
    st.id = 't2v-style';
    st.textContent =
      '.t2v-grid{display:grid;grid-template-columns:minmax(0,1fr) 340px;gap:16px;align-items:start;margin-top:16px}' +
      '@media(max-width:1100px){.t2v-grid{grid-template-columns:minmax(0,1fr)}}' +
      '.t2v-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;flex-wrap:wrap}' +
      '.t2v-title{font-size:17px;font-weight:700;cursor:pointer;border-bottom:1px dashed transparent;display:inline-block}' +
      '.t2v-title:hover{border-bottom-color:var(--blue);color:var(--blue)}' +
      '.t2v-name-input{max-width:420px;height:34px}' +
      '.t2v-sec-head{display:flex;align-items:center;justify-content:space-between;gap:10px}' +
      '.t2v-num{width:24px;height:24px;border-radius:50%;background:var(--grad);color:#fff;font-size:12px;' +
        'font-weight:700;display:flex;align-items:center;justify-content:center;flex:none}' +
      '.t2v-seg{border:1px solid var(--border);border-radius:10px;padding:8px 10px;margin-bottom:8px;background:var(--card)}' +
      '.t2v-seg-head{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:6px}' +
      '.t2v-seg-foot{font-size:11.5px;color:var(--muted);margin-top:4px}' +
      '.t2v-seg .textarea{min-height:56px}' +
      '.t2v-vgrid{display:grid;grid-template-columns:repeat(auto-fill,minmax(132px,1fr));gap:8px;' +
        'max-height:280px;overflow-y:auto;padding:2px}' +
      '.t2v-voice{border:1px solid var(--border);border-radius:10px;padding:9px 6px;text-align:center;' +
        'cursor:pointer;background:var(--card);transition:border-color .12s ease}' +
      '.t2v-voice:hover{border-color:var(--blue)}' +
      '.t2v-voice.sel{border-color:var(--blue);box-shadow:var(--focus-ring)}' +
      '.t2v-mode{border:1.5px solid var(--border);border-radius:12px;padding:12px 14px;cursor:pointer;' +
        'flex:1;min-width:230px;background:var(--card)}' +
      '.t2v-mode:hover{border-color:var(--blue)}' +
      '.t2v-mode.sel{border-color:var(--blue);background:var(--blue-soft)}' +
      '.t2v-mode-t{font-weight:700;font-size:13.5px}' +
      '.t2v-mode-d{font-size:12px;color:var(--muted);margin-top:3px}' +
      '.t2v-row{display:flex;align-items:center;justify-content:space-between;gap:12px;border:1px solid var(--border);' +
        'border-radius:12px;padding:11px 14px;margin-bottom:10px;background:var(--card);transition:border-color .12s ease}' +
      '.t2v-row:hover{border-color:var(--blue)}' +
      '.t2v-row-name{font-weight:600;font-size:14px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}' +
      '.t2v-log{display:flex;flex-direction:column;gap:5px;max-height:46vh;overflow-y:auto;margin-top:10px}' +
      '.t2v-logline{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:11px;color:var(--muted);' +
        'word-break:break-word;border-bottom:1px dashed var(--border);padding-bottom:4px}' +
      '.t2v-logline.warn{color:var(--amber)}.t2v-logline.error{color:var(--red)}' +
      '.t2v-dot{font-size:9px;vertical-align:middle}' +
      '.t2v-path{font-size:11.5px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;cursor:pointer;' +
        'word-break:break-all}';
    document.head.appendChild(st);
  }

  // ---------- Helpers ----------

  function isRel(p) { return !!p && String(p).charAt(0) !== '/'; }
  function dataUrl(p) { return '/data/' + String(p).split('/').map(encodeURIComponent).join('/'); }
  function fileName(p) { return String(p || '').split('/').pop(); }

  function isVideoPath(p) {
    var low = String(p || '').toLowerCase();
    for (var i = 0; i < VIDEO_EXT.length; i++) {
      if (low.slice(-VIDEO_EXT[i].length) === VIDEO_EXT[i]) return true;
    }
    return false;
  }

  function copyText(text, okMsg) {
    function fallback() {
      var ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand('copy'); UI.toast(okMsg || 'Đã sao chép đường dẫn'); }
      catch (e) { UI.toast('Không sao chép được — hãy copy thủ công', 'error'); }
      ta.remove();
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () { UI.toast(okMsg || 'Đã sao chép đường dẫn'); }, fallback);
    } else fallback();
  }

  function statusBadge(s) {
    var m = STATUS[s] || [s || '—', 'badge-gray'];
    return h('span', { class: 'badge ' + m[1] }, m[0]);
  }

  function redAlert(msg) {
    return h('div', {
      class: 'text-red',
      style: { background: 'rgba(239,68,68,.08)', border: '1px solid var(--red)', borderRadius: '10px',
        padding: '10px 12px', fontSize: '13px', fontWeight: '600' }
    }, '⚠ ' + msg);
  }

  function voiceColor(s) {
    var hsh = 0;
    s = String(s || '?');
    for (var i = 0; i < s.length; i++) hsh = (hsh * 31 + s.charCodeAt(i)) % 360;
    return 'hsl(' + hsh + ', 62%, 45%)';
  }

  function avatar(name, size) {
    var px = (size || 34) + 'px';
    return h('div', {
      style: { width: px, height: px, borderRadius: '50%', flex: 'none', margin: '0 auto 6px',
        background: voiceColor(name), color: '#fff', display: 'flex', alignItems: 'center',
        justifyContent: 'center', fontWeight: '700', fontSize: '14px' }
    }, String(name || '?').charAt(0).toUpperCase());
  }

  function engineBadgeCls(e) {
    if (e === 'vieneu') return 'badge badge-green';
    if (e === 'gemini') return 'badge badge-blue';
    if (e === 'clone') return 'badge badge-amber';
    return 'badge badge-gray';
  }

  function sizeText(s) { return (s.width || 0) + '×' + (s.height || 0) + ' · ' + (s.fps || 0) + 'fps'; }
  function baseVoiceId(v) { return String(v || '').split('@')[0]; }
  function segCount(s) { return (s.segments || []).length; }

  function voiceNameOf(voices, id) {
    var base = baseVoiceId(id);
    if (!base) return '';
    for (var i = 0; i < (voices || []).length; i++) {
      if (voices[i].id === base) return voices[i].name || voices[i].id;
    }
    return base;
  }

  function sortVoices(list) {
    return (list || []).slice().sort(function (a, b) {
      var av = String(a.lang || '').toLowerCase().indexOf('vi') === 0 ? 0 : 1;
      var bv = String(b.lang || '').toLowerCase().indexOf('vi') === 0 ? 0 : 1;
      return av - bv || String(a.name || '').localeCompare(String(b.name || ''));
    });
  }

  function stepperEl(step) {
    var row = h('div', { class: 'stepper', style: { flex: '1', minWidth: '340px', overflowX: 'auto', paddingBottom: '4px' } });
    STEPS.forEach(function (label, i) {
      if (i > 0) row.appendChild(h('div', { class: 'step-line' + (step > i ? ' done' : '') }));
      var cls = 'step' + (step > i ? ' done' : (step === i ? ' active' : ''));
      row.appendChild(h('div', { class: cls },
        h('div', { class: 'step-dot' }, step > i ? '✓' : String(i + 1)),
        h('div', { class: 'step-label' }, label)));
    });
    return row;
  }

  // ============================================================
  // A. DANH SÁCH PHIÊN
  // ============================================================

  // Khoá localStorage do trang Xưởng ghi khi bấm "Dùng khuôn này".
  var PREFILL_KEY = 'biz-template-prefill-t2v';

  // takePrefill đọc RỒI XOÁ: khuôn chỉ dùng cho đúng phiên vừa tạo. Không xoá
  // thì mọi phiên mới sau đó đều dính khuôn cũ mà người dùng không hiểu vì sao.
  function takePrefill() {
    try {
      var raw = localStorage.getItem(PREFILL_KEY);
      if (!raw) return null;
      localStorage.removeItem(PREFILL_KEY);
      return JSON.parse(raw);
    } catch (e) { return null; }
  }

  function createSession(btn) {
    if (btn) btn.disabled = true;
    var tpl = takePrefill();
    var body = tpl ? { templateId: tpl.id } : {};
    API.post('/api/t2v/sessions', body).then(function (s) {
      if (!s || !s.id) throw new Error('Máy chủ không trả về phiên mới');
      UI.toast('Đã tạo phiên "' + (s.name || s.id) + '"');
      App.navigate('text2video/' + s.id);
    }).catch(function (e) {
      if (btn) btn.disabled = false;
      UI.toast('Không tạo được phiên: ' + e.message, 'error');
    });
  }

  function confirmDelete(s, onDone) {
    var m = UI.modal({
      title: 'Xoá phiên Text → Video',
      body: h('p', null, 'Xoá vĩnh viễn phiên "', h('b', null, s.name || s.id),
        '" cùng kịch bản, giọng đọc và video đã dựng? Hành động này không thể hoàn tác.'),
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('Xoá vĩnh viễn', {
          variant: 'danger',
          onclick: function () {
            API.del('/api/t2v/sessions/' + encodeURIComponent(s.id)).then(function () {
              m.close();
              UI.toast('Đã xoá phiên');
              if (onDone) onDone();
            }).catch(function (e) { UI.toast('Không xoá được: ' + e.message, 'error'); });
          }
        })]
    });
  }

  function sessionRow(s, reloadList) {
    var meta = [sizeText(s), segCount(s) + ' đoạn'];
    if (s.voiceSeconds > 0) meta.push('giọng ' + UI.fmtDur(s.voiceSeconds));
    meta.push('Sửa cuối: ' + (UI.timeAgo(s.updatedAt) || '—'));
    return h('div', { class: 't2v-row' },
      h('div', { style: { minWidth: 0, flex: '1' } },
        h('div', { class: 'row', style: { flexWrap: 'nowrap', minWidth: 0 } },
          h('div', { class: 't2v-row-name', title: s.name }, s.name || '(chưa đặt tên)'),
          statusBadge(s.status)),
        h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '3px' } }, meta.join(' · '))),
      h('div', { class: 'row', style: { flexWrap: 'nowrap', flex: 'none' } },
        UI.btn('Mở', { variant: 'ghost', small: true, onclick: function () { App.navigate('text2video/' + s.id); } }),
        UI.btn('Xoá', { variant: 'danger', small: true, onclick: function () { confirmDelete(s, reloadList); } })));
  }

  function renderList(root) {
    var host = h('div', { class: 'mt-16' });
    var newBtn = UI.btn('＋ Phiên mới', { onclick: function () { createSession(newBtn); } });
    root.appendChild(h('div', { class: 'row-between', style: { flexWrap: 'wrap' } },
      h('span', { class: 'muted' },
        'Mỗi phiên lưu lại nguồn, kịch bản, giọng đọc và cấu hình — mở lại để sửa tiếp bất cứ lúc nào.'),
      newBtn));
    root.appendChild(host);

    function load() {
      host.innerHTML = '';
      host.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải danh sách phiên…'));
      API.get('/api/t2v/sessions').then(function (list) {
        list = (list || []).slice().sort(function (a, b) {
          return new Date(b.updatedAt || 0) - new Date(a.updatedAt || 0);
        });
        host.innerHTML = '';
        if (!list.length) {
          host.appendChild(UI.card({
            body: h('div', { class: 'empty' },
              h('span', { class: 'empty-icon' }, '📜'),
              h('div', { style: { fontWeight: '700', fontSize: '15px', color: 'var(--text)' } }, 'Chưa có phiên nào'),
              h('div', null, 'Tạo phiên đầu tiên: dán văn bản hoặc link bài viết, AI viết kịch bản đọc rồi dựng thành video.'),
              h('div', { class: 'mt-8' }, UI.btn('＋ Phiên mới', { onclick: function () { createSession(null); } })))
          }));
          return;
        }
        var box = h('div');
        list.forEach(function (s) { box.appendChild(sessionRow(s, load)); });
        host.appendChild(UI.card({ title: 'Phiên làm việc', icon: '📜', body: box }));
      }).catch(function (e) {
        host.innerHTML = '';
        host.appendChild(UI.card({
          title: 'Không tải được danh sách phiên', icon: '❌',
          body: [redAlert(e.message),
            h('div', { class: 'muted mt-8', style: { fontSize: '12.5px' } },
              'Kiểm tra máy chủ đã bật module Text → Video (endpoint /api/t2v/sessions) chưa.'),
            h('div', { class: 'mt-8' }, UI.btn('Thử lại', { variant: 'ghost', onclick: load }))]
        }));
      });
    }
    load();
  }

  // ============================================================
  // B. CHI TIẾT PHIÊN
  // ============================================================

  function sub(ctx, ev, fn) { Bus.on(ev, fn); ctx.unsubs.push({ ev: ev, fn: fn }); }

  function setPageHeader(ctx) {
    App.setHeader('Text → Video',
      (ctx.sess.name || 'Phiên') + ' · ' + sizeText(ctx.sess) + ' · cập nhật ' + UI.timeAgo(ctx.sess.updatedAt));
  }

  // Chỉ nhận field máy chủ tính lại — KHÔNG ghi đè phần người dùng đang gõ.
  function applyServer(ctx, s) {
    if (!s) return;
    ['status', 'step', 'voicePath', 'transcriptPath', 'outputPath', 'voiceSeconds', 'projectId', 'updatedAt']
      .forEach(function (k) { if (s[k] !== undefined) ctx.sess[k] = s[k]; });
    renderChips(ctx);
    renderStepper(ctx);
    setPageHeader(ctx);
  }

  function putSession(ctx, patch) {
    if (patch) Object.keys(patch).forEach(function (k) { ctx.sess[k] = patch[k]; });
    return API.put('/api/t2v/sessions/' + encodeURIComponent(ctx.sess.id), ctx.sess)
      .then(function (s) { applyServer(ctx, s); return s; })
      .catch(function (e) { UI.toast('Không lưu được phiên: ' + e.message, 'error'); return null; });
  }

  function queueSave(ctx) {
    if (ctx.saveTimer) clearTimeout(ctx.saveTimer);
    ctx.saveTimer = setTimeout(function () { ctx.saveTimer = null; putSession(ctx, null); }, 600);
  }

  // Card 3 (Storyboard) chen giữa Kịch bản và Giọng đọc nên số card (1..6) không
  // còn trùng bước của máy chủ (0..5) — CARD_STAGE quy đổi để biết card nào nên
  // thu gọn khi mở lại phiên đang dở.
  var CARD_STAGE = [0, 1, 2, 3, 3, 4, 5];

  function initOpen(step) {
    var o = {};
    for (var k = 1; k <= 6; k++) o[k] = CARD_STAGE[k] > step;
    if (step >= 5) o[6] = true;
    return o;
  }

  function reloadSession(ctx) {
    return API.get('/api/t2v/sessions/' + encodeURIComponent(ctx.sess.id)).then(function (s) {
      if (!s || !s.id) return;
      ctx.sess = s;
      ctx.open = initOpen(Number(s.step) || 0);
      renderHead(ctx); renderStepper(ctx); renderChips(ctx); renderLeft(ctx);
      setPageHeader(ctx);
    }).catch(function (e) { UI.toast('Không tải lại được phiên: ' + e.message, 'error'); });
  }

  // ---------- Header / stepper / chips ----------

  function renderHead(ctx) {
    var s = ctx.sess;
    ctx.headHost.innerHTML = '';

    var nameEl = h('div', { class: 't2v-title', title: 'Bấm để đổi tên phiên' }, s.name || '(chưa đặt tên)');
    nameEl.onclick = function () {
      var inp = h('input', { class: 'input t2v-name-input', value: s.name || '' });
      function done(save) {
        inp.onblur = null;
        var v = inp.value.trim();
        if (save && v && v !== s.name) {
          putSession(ctx, { name: v }).then(function () { renderHead(ctx); setPageHeader(ctx); });
        } else renderHead(ctx);
      }
      inp.onkeydown = function (e) {
        if (e.key === 'Enter') done(true);
        else if (e.key === 'Escape') done(false);
      };
      inp.onblur = function () { done(true); };
      nameEl.parentNode.replaceChild(inp, nameEl);
      inp.focus();
      inp.select();
    };

    ctx.headHost.appendChild(h('div', { class: 't2v-head' },
      h('div', { style: { minWidth: 0 } },
        nameEl,
        h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '2px' } }, sizeText(s))),
      h('div', { class: 'row', style: { flexWrap: 'nowrap', flex: 'none' } },
        UI.btn('← Danh sách phiên', { variant: 'ghost', onclick: function () { App.navigate('text2video'); } }),
        UI.btn('🗑 Xoá', {
          variant: 'danger',
          onclick: function () { confirmDelete(s, function () { App.navigate('text2video'); }); }
        }))));
  }

  function renderStepper(ctx) {
    ctx.stepHost.innerHTML = '';
    ctx.stepHost.appendChild(stepperEl(Number(ctx.sess.step) || 0));
  }

  function renderChips(ctx) {
    var s = ctx.sess;
    ctx.chipHost.innerHTML = '';
    var vname = voiceNameOf(ctx.voices, s.voiceId);
    ctx.chipHost.appendChild(h('div', { class: 'row', style: { marginTop: '10px' } },
      statusBadge(s.status),
      h('span', { class: 'badge badge-gray' }, s.sourceKind === 'link' ? '🔗 Link bài viết' : '📋 Dán văn bản'),
      h('span', { class: 'badge badge-gray' }, segCount(s) + ' đoạn'),
      h('span', { class: 'badge badge-gray' }, vname ? '🎙 ' + vname : '🎙 chưa chọn giọng'),
      h('span', { class: 'badge ' + (s.voiceSeconds > 0 ? 'badge-green' : 'badge-gray') },
        s.voiceSeconds > 0 ? 'thật ' + UI.fmtDur(s.voiceSeconds) : 'chưa có giọng đọc'),
      h('span', { class: 'muted', style: { fontSize: '12px' } },
        'Sửa cuối: ' + (UI.timeAgo(s.updatedAt) || '—'))));
    if (s.templateId) renderTemplateChip(ctx);
  }

  // Khuôn lặng lẽ nắn kịch bản AI viết ra. Không hiện thì lần sau người dùng mở
  // lại phiên, thấy kịch bản ra khác hẳn phiên khác mà không hiểu vì sao — nên
  // vừa hiện tên vừa cho gỡ ngay tại đây.
  function renderTemplateChip(ctx) {
    var s = ctx.sess;
    API.get('/api/studio/templates').then(function (d) {
      var tpl = null, list = (d && d.templates) || [];
      for (var i = 0; i < list.length; i++) if (list[i].id === s.templateId) { tpl = list[i]; break; }
      if (!tpl || ctx.sess.templateId !== s.templateId) return;
      var off = UI.btn('Gỡ khuôn', {
        variant: 'ghost', small: true,
        onclick: function () {
          off.disabled = true;
          putSession(ctx, { templateId: '' })
            .then(function () { renderChips(ctx); UI.toast('Đã gỡ khuôn — kịch bản sau sẽ viết tự do.'); })
            .catch(function (e) { off.disabled = false; UI.toast('Không gỡ được: ' + e.message, 'error'); });
        }
      });
      ctx.chipHost.appendChild(h('div', {
        class: 'row', style: {
          marginTop: '8px', gap: '8px', padding: '8px 10px', borderRadius: '8px',
          background: 'var(--surface-2, rgba(124,58,237,.06))', border: '1px solid var(--border)'
        }
      },
        h('span', { style: { fontSize: '13px' } }, '🧰 Khuôn: ', h('b', null, tpl.icon + ' ' + tpl.name)),
        h('span', { class: 'muted', style: { fontSize: '12px', flex: '1', minWidth: '0' } },
          'AI viết kịch bản theo nhịp của khuôn này.'),
        off));
    }).catch(function () { /* mất mạng thì thôi, không chặn phần còn lại */ });
  }

  // ---------- Card thu gọn được ----------

  function section(ctx, key, title, icon, bodyEl) {
    var open = ctx.open[key] !== false;
    var body = h('div', { style: { display: open ? '' : 'none', marginTop: '12px' } }, bodyEl);
    var toggle = UI.btn(open ? 'Thu gọn' : 'Mở rộng', { variant: 'ghost', small: true });
    toggle.onclick = function () {
      open = !open;
      ctx.open[key] = open;
      body.style.display = open ? '' : 'none';
      toggle.lastChild.textContent = open ? 'Thu gọn' : 'Mở rộng';
    };
    return h('div', { class: 'card' },
      h('div', { class: 't2v-sec-head' },
        h('div', { class: 'row', style: { flexWrap: 'nowrap', minWidth: 0 } },
          h('div', { class: 't2v-num' }, String(key)),
          h('div', { class: 'card-title', style: { margin: '0' } }, icon + ' ' + title)),
        toggle),
      body);
  }

  // ---------- 1. Nguồn ----------

  function secSource(ctx) {
    var s = ctx.sess;
    var kind = s.sourceKind === 'link' ? 'link' : 'text';
    var counter = h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '6px' } });
    var ta = UI.textarea({
      rows: 10, value: s.sourceText || '',
      placeholder: 'Dán nội dung bài viết / bản thảo vào đây…',
      oninput: function () { s.sourceText = ta.value; updateCounter(); }
    });
    ta.onblur = function () { putSession(ctx, { sourceText: ta.value }); };

    function updateCounter() {
      counter.textContent = 'Sửa thoải mái trước khi viết kịch bản — AI chỉ đọc đúng phần chữ trong ô này · ' +
        (ta.value || '').length + ' ký tự';
    }
    updateCounter();

    var urlIn = UI.input({
      value: s.sourceUrl || '', placeholder: 'https://vnexpress.net/bai-viet…',
      oninput: function () { s.sourceUrl = urlIn.value.trim(); }
    });
    var fetchErr = h('div', { class: 'mt-8', style: { display: 'none' } });
    var fetchSpin = h('span', { style: { display: 'none' } }, UI.spinner());
    var fetchBtn = UI.btn('⬇ Lấy nội dung', { onclick: doFetch });

    function doFetch() {
      var url = (urlIn.value || '').trim();
      fetchErr.style.display = 'none';
      if (!url || url.indexOf('http') !== 0) {
        UI.toast('Vui lòng nhập link bài viết hợp lệ (bắt đầu bằng http).', 'error');
        return;
      }
      fetchBtn.disabled = true;
      fetchSpin.style.display = '';
      API.post('/api/t2v/sessions/' + encodeURIComponent(s.id) + '/fetch', { url: url }).then(function (ns) {
        if (!ns) throw new Error('Máy chủ không trả về nội dung');
        s.sourceUrl = ns.sourceUrl || url;
        s.sourceKind = 'link';
        s.sourceText = ns.sourceText || '';
        ta.value = s.sourceText;
        updateCounter();
        applyServer(ctx, ns);
        UI.toast('Đã lấy nội dung bài viết (' + s.sourceText.length + ' ký tự)');
      }).catch(function (e) {
        fetchErr.innerHTML = '';
        fetchErr.appendChild(redAlert('Không lấy được nội dung: ' + e.message));
        fetchErr.style.display = '';
        UI.toast('Không lấy được nội dung: ' + e.message, 'error');
      }).then(function () {
        fetchBtn.disabled = false;
        fetchSpin.style.display = 'none';
      });
    }

    var linkPane = h('div', { style: { display: kind === 'link' ? '' : 'none' } },
      h('div', { class: 'row', style: { flexWrap: 'nowrap', alignItems: 'flex-end' } },
        h('div', { style: { flex: '1', minWidth: '0' } }, UI.field('Link bài viết', urlIn)),
        h('div', { style: { flex: 'none', paddingBottom: '14px' } }, h('div', { class: 'row' }, fetchBtn, fetchSpin))),
      fetchErr);

    var tabDefs = [{ id: 'link', label: '🔗 Link bài viết' }, { id: 'text', label: '📋 Dán văn bản' }];
    var tabBtns = {};
    var tabRow = h('div', { class: 'row' }, tabDefs.map(function (t) {
      var b = h('button', {
        class: 'btn btn-sm ' + (kind === t.id ? 'btn-primary' : 'btn-ghost'), type: 'button',
        onclick: function () { switchTab(t.id); }
      }, t.label);
      tabBtns[t.id] = b;
      return b;
    }));

    function switchTab(id) {
      if (kind === id) return;
      kind = id;
      tabDefs.forEach(function (t) {
        tabBtns[t.id].className = 'btn btn-sm ' + (kind === t.id ? 'btn-primary' : 'btn-ghost');
      });
      linkPane.style.display = kind === 'link' ? '' : 'none';
      putSession(ctx, { sourceKind: kind, sourceText: ta.value, sourceUrl: (urlIn.value || '').trim() });
    }

    return section(ctx, 1, 'Nguồn nội dung', '📥',
      h('div', null, tabRow, h('div', { class: 'mt-8' }, linkPane), ta, counter));
  }

  // ---------- 2. Kịch bản đọc ----------

  function modelValue(s) {
    var v = (s.scriptEngine || 'claude') + ':' + (s.scriptModel || '');
    var ok = false;
    MODEL_GROUPS.forEach(function (g) {
      g.items.forEach(function (it) { if (it.value === v) ok = true; });
    });
    return ok ? v : 'claude:';
  }

  function modelSelect(value, onchange) {
    var sel = h('select', { class: 'select' });
    MODEL_GROUPS.forEach(function (g) {
      var og = h('optgroup', { label: g.group });
      g.items.forEach(function (it) {
        og.appendChild(h('option', { value: it.value, selected: it.value === value }, it.label));
      });
      sel.appendChild(og);
    });
    sel.value = value;
    sel.onchange = function () { onchange(sel.value); };
    return sel;
  }

  function engineStatusLine(engine) {
    var tools = (App.state && App.state.tools) || {};
    var ok, msg, hint = null;
    if (engine === 'gemini') {
      ok = !!tools.geminiKey;
      msg = ok ? 'Đã cấu hình Gemini API key' : 'Chưa cấu hình Gemini API key';
      if (!ok) hint = 'Mở Cấu hình & API →';
    } else if (engine === 'openai') {
      ok = true;
      msg = 'Dùng endpoint OpenAI-compatible đã cấu hình';
      hint = 'Kiểm tra Cấu hình & API →';
    } else {
      ok = !!tools.claude;
      msg = ok ? 'Đã kết nối subscription' : 'Chưa thấy Claude CLI trên máy';
      if (!ok) hint = 'Cài Claude Code hoặc chọn engine khác →';
    }
    return h('div', { class: 'row', style: { fontSize: '12.5px', minHeight: '38px' } },
      h('span', { class: 't2v-dot', style: { color: ok ? 'var(--green)' : 'var(--red)' } }, '●'),
      h('span', { style: { fontWeight: '600', color: ok ? 'var(--green)' : 'var(--red)' } }, msg),
      hint ? h('a', { href: '#/settings', class: 'muted', style: { fontSize: '12px' } }, hint) : null);
  }

  function segRow(ctx, seg, i, rerender) {
    var segs = ctx.sess.segments || [];
    var foot = h('div', { class: 't2v-seg-foot' });
    function updateFoot() {
      var n = (seg.text || '').length;
      foot.textContent = n + ' ký tự · ' + (seg.seconds > 0 ? UI.fmtDur(seg.seconds) + ' (thật)' : 'chưa đo');
    }
    var ta = UI.textarea({
      rows: 2, value: seg.text || '', placeholder: 'Nội dung đoạn đọc…',
      oninput: function () {
        seg.text = ta.value;
        seg.chars = ta.value.length;
        updateFoot();
        queueSave(ctx);
      }
    });
    updateFoot();

    function move(to) {
      if (to < 0 || to >= segs.length) return;
      var t = segs[i]; segs[i] = segs[to]; segs[to] = t;
      queueSave(ctx);
      rerender();
    }
    return h('div', { class: 't2v-seg' },
      h('div', { class: 't2v-seg-head' },
        h('b', { style: { fontSize: '12.5px' } }, '#' + (i + 1)),
        h('div', { class: 'row', style: { flexWrap: 'nowrap' } },
          h('button', { class: 'btn btn-ghost btn-sm', type: 'button', title: 'Chuyển lên', disabled: i === 0,
            onclick: function () { move(i - 1); } }, '↑'),
          h('button', { class: 'btn btn-ghost btn-sm', type: 'button', title: 'Chuyển xuống', disabled: i === segs.length - 1,
            onclick: function () { move(i + 1); } }, '↓'),
          h('button', { class: 'btn btn-ghost btn-sm', type: 'button', title: 'Xoá đoạn',
            onclick: function () { segs.splice(i, 1); queueSave(ctx); rerender(); } }, '×'))),
      ta, foot);
  }

  function secScript(ctx) {
    var s = ctx.sess;
    var engineHost = h('div');
    function renderEngine() {
      engineHost.innerHTML = '';
      engineHost.appendChild(engineStatusLine(s.scriptEngine || 'claude'));
    }
    renderEngine();

    var sel = modelSelect(modelValue(s), function (v) {
      var parts = String(v).split(':');
      putSession(ctx, { scriptEngine: parts[0] || 'claude', scriptModel: parts[1] || '' });
      renderEngine();
    });
    var secIn = UI.input({
      type: 'number', min: 0, step: 5, placeholder: 'Tự động',
      value: s.targetSeconds > 0 ? s.targetSeconds : '',
      onchange: function () { putSession(ctx, { targetSeconds: Math.max(0, Number(secIn.value) || 0) }); }
    });

    var segHost = h('div', { class: 'mt-8' });
    function renderSegs() {
      segHost.innerHTML = '';
      var segs = s.segments || [];
      if (!segs.length) {
        segHost.appendChild(UI.empty('Chưa có đoạn nào — bấm "Viết lại" để AI tạo kịch bản, hoặc thêm đoạn thủ công.', '✍️'));
        return;
      }
      segs.forEach(function (seg, i) { segHost.appendChild(segRow(ctx, seg, i, renderSegs)); });
    }
    renderSegs();

    var writeBtn = UI.btn('✂ Viết lại', {
      disabled: !!ctx.busy,
      onclick: function () {
        if (!(s.sourceText || '').trim()) {
          UI.toast('Chưa có nội dung nguồn — hãy dán văn bản hoặc lấy nội dung từ link trước.', 'error');
          return;
        }
        if ((s.segments || []).length) {
          var m = UI.modal({
            title: 'Viết lại kịch bản',
            body: h('p', null, 'Kịch bản hiện có ', h('b', null, String(s.segments.length)),
              ' đoạn sẽ bị thay bằng bản mới. Tiếp tục?'),
            actions: [
              UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
              UI.btn('Viết lại', { onclick: function () { m.close(); startJob(ctx, 'script'); } })]
          });
          return;
        }
        startJob(ctx, 'script');
      }
    });

    ctx.inline.script = h('div');

    return section(ctx, 2, 'Kịch bản đọc', '✍️', h('div', null,
      h('div', { class: 'grid-3' },
        UI.field('Model viết kịch bản', sel),
        UI.field('Độ dài mong muốn (giây)', secIn),
        UI.field('Trạng thái engine', engineHost)),
      h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '-4px' } },
        'Chạy bằng chính tài khoản Claude Code đã đăng nhập trên máy này — không cần API key. ' +
        'Model chọn ở đây được lưu cho lần sau.'),
      h('div', { class: 'row mt-16' }, writeBtn,
        h('span', { class: 'muted', style: { fontSize: '12px' } },
          'AI viết văn nói tự nhiên, chia thành các đoạn 1–3 câu — bạn sửa tay thoải mái bên dưới.')),
      ctx.inline.script,
      segHost,
      h('div', { class: 'mt-8' }, UI.btn('+ Thêm đoạn', {
        variant: 'ghost', small: true,
        onclick: function () {
          if (!s.segments) s.segments = [];
          s.segments.push({ text: '', chars: 0, seconds: 0, audioPath: '' });
          queueSave(ctx);
          renderSegs();
        }
      }))));
  }

  // ---------- 3. Storyboard (file text2video-storyboard.js) ----------

  // ctx rút gọn cho module storyboard — chỉ những gì nó cần, không lộ nội bộ trang.
  function storyboardCtx(ctx) {
    return {
      mountId: ctx.mountId,
      session: ctx.sess,
      busy: ctx.busy,
      inlineHost: ctx.inline.storyboard,
      startAll: function () { startJob(ctx, 'storyboard'); },
      reload: function () { return reloadSession(ctx); },
      saveSegments: function () { queueSave(ctx); },
      fetchSession: function () { return API.get('/api/t2v/sessions/' + encodeURIComponent(ctx.sess.id)); },
      applyServer: function (s) { applyServer(ctx, s); },
      watchJob: function (job, host, onDone) { watchJob(ctx, job, host, onDone); }
    };
  }

  function secStoryboard(ctx) {
    ctx.inline.storyboard = h('div');
    var host = h('div');

    if (window.T2VStoryboard && typeof window.T2VStoryboard.render === 'function') {
      try {
        ctx.sbCleanup = window.T2VStoryboard.render(host, storyboardCtx(ctx));
      } catch (e) {
        host.innerHTML = '';
        host.appendChild(redAlert('Không dựng được phần Storyboard: ' + (e && e.message ? e.message : e)));
      }
    } else {
      host.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px' } },
        'Chưa nạp được module Storyboard (js/pages/text2video-storyboard.js) — tải lại trang để thử lại.'));
      host.appendChild(ctx.inline.storyboard);
    }
    return section(ctx, 3, 'Storyboard', '🖼️', host);
  }

  // ---------- 4. Giọng đọc ----------

  function secVoice(ctx) {
    var s = ctx.sess;
    var query = '';
    var gridHost = h('div');

    function needStyle() {
      var e = s.voiceEngine || '';
      return e === 'vieneu' || e === 'clone';
    }

    var styleField = UI.field('Phong cách đọc (VieNeu / giọng nhân bản)',
      UI.select(null, STYLE_OPTS, s.voiceStyle || 'tu_nhien', function (v) { putSession(ctx, { voiceStyle: v }); }));
    styleField.style.display = needStyle() ? '' : 'none';

    function voiceCard(v) {
      var isSel = baseVoiceId(s.voiceId) === v.id;
      var name = v.name || v.id;
      return h('div', {
        class: 't2v-voice' + (isSel ? ' sel' : ''),
        onclick: function () {
          putSession(ctx, { voiceId: v.id, voiceEngine: v.engine || '' });
          styleField.style.display = needStyle() ? '' : 'none';
          renderGrid();
          renderChips(ctx);
        }
      },
        avatar(name, 34),
        h('div', { style: { fontWeight: '600', fontSize: '12.5px', wordBreak: 'break-word' } }, name),
        h('div', { class: 'muted', style: { fontSize: '11px', margin: '2px 0 5px' } }, v.lang || '—'),
        h('span', { class: engineBadgeCls(v.engine) }, ENGINE_LABELS[v.engine] || v.engine || '?'));
    }

    function renderGrid() {
      gridHost.innerHTML = '';
      if (ctx.voicesErr) { gridHost.appendChild(redAlert('Không tải được danh sách giọng: ' + ctx.voicesErr)); return; }
      if (!ctx.voices) {
        gridHost.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải danh sách giọng…'));
        return;
      }
      var list = ctx.voices.filter(function (v) {
        return !query || String(v.name || v.id).toLowerCase().indexOf(query) >= 0;
      });
      if (!list.length) { gridHost.appendChild(UI.empty('Không có giọng nào khớp từ khoá', '🎙️')); return; }
      var grid = h('div', { class: 't2v-vgrid' });
      list.forEach(function (v) { grid.appendChild(voiceCard(v)); });
      gridHost.appendChild(grid);
    }
    renderGrid();
    ctx.renderVoiceGrid = renderGrid;

    var searchIn = UI.input({
      placeholder: '🔍 Tìm giọng theo tên…',
      oninput: function () { query = searchIn.value.trim().toLowerCase(); renderGrid(); }
    });

    var makeBtn = UI.btn('🎙 Tạo giọng đọc', {
      disabled: !!ctx.busy,
      onclick: function () {
        if (!(s.segments || []).length) { UI.toast('Chưa có kịch bản — hãy viết kịch bản trước.', 'error'); return; }
        if (!s.voiceId) { UI.toast('Hãy chọn giọng đọc trước.', 'error'); return; }
        startJob(ctx, 'voice');
      }
    });

    ctx.inline.voice = h('div');

    var resultHost = h('div', { class: 'mt-16' });
    if (s.voicePath) {
      resultHost.appendChild(h('audio', { controls: 'controls', src: dataUrl(s.voicePath), style: { width: '100%' } }));
      resultHost.appendChild(h('div', { class: 'row mt-8', style: { fontSize: '12.5px' } },
        h('span', { class: 'text-green', style: { fontWeight: '600' } },
          '✓ Tổng thời lượng thật: ' + UI.fmtDur(s.voiceSeconds || 0)),
        h('a', { class: 'btn btn-ghost btn-sm', href: dataUrl(s.voicePath), download: fileName(s.voicePath) },
          '⬇ Tải ' + fileName(s.voicePath)),
        s.transcriptPath
          ? h('a', { class: 'btn btn-ghost btn-sm', href: dataUrl(s.transcriptPath), target: '_blank' }, '📄 transcript.json')
          : null));
    } else {
      resultHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px' } },
        'Chưa có giọng đọc. Sau khi tạo, thời lượng THẬT của từng đoạn sẽ được đo và dùng để canh cảnh video.'));
    }

    return section(ctx, 4, 'Giọng đọc', '🎙️', h('div', null,
      searchIn,
      h('div', { class: 'mt-8' }, gridHost),
      h('div', { class: 'mt-16' }, styleField),
      h('div', { class: 'row' }, makeBtn,
        h('span', { class: 'muted', style: { fontSize: '12px' } }, 'Đọc từng đoạn rồi ghép thành voice.wav.')),
      ctx.inline.voice,
      resultHost));
  }

  // ---------- 5. Cấu hình video ----------

  function secConfig(ctx) {
    var s = ctx.sess;
    var cur = (s.width || 1080) + 'x' + (s.height || 1920);
    var sizeSel = UI.select('Kích thước khung hình', SIZE_PRESETS, cur, function (v) {
      var pr = SIZE_PRESETS[0];
      SIZE_PRESETS.forEach(function (x) { if (x.value === v) pr = x; });
      putSession(ctx, { width: pr.w, height: pr.h }).then(function () { renderHead(ctx); });
    });
    var fpsSel = UI.select('Tốc độ khung hình (fps)',
      [{ value: '30', label: '30 fps' }, { value: '60', label: '60 fps' }],
      String(s.fps || 30), function (v) {
        putSession(ctx, { fps: Number(v) || 30 }).then(function () { renderHead(ctx); });
      });
    return section(ctx, 5, 'Cấu hình video', '⚙️', h('div', null,
      h('div', { class: 'grid-2' }, sizeSel, fpsSel),
      h('div', { class: 'muted', style: { fontSize: '12px' } },
        'Kích thước áp dụng cho cả chế độ dựng bằng AI lẫn HTML Video.')));
  }

  // ---------- 6. Dựng video ----------

  function subStepper(mode, progress) {
    var labels = mode === 'html'
      ? ['Dựng cảnh HTML', 'Render video', 'Ghép giọng đọc']
      : ['Tạo dự án', 'Chép giọng đọc', 'Khởi động phiên AI'];
    var p = Math.max(0, Math.min(100, Number(progress) || 0));
    var done = Math.min(labels.length, Math.floor(p * labels.length / 100));
    var row = h('div', { class: 'stepper', style: { marginTop: '10px', overflowX: 'auto' } });
    labels.forEach(function (label, i) {
      if (i > 0) row.appendChild(h('div', { class: 'step-line' + (done > i ? ' done' : '') }));
      var cls = 'step' + (done > i ? ' done' : (done === i ? ' active' : ''));
      row.appendChild(h('div', { class: cls },
        h('div', { class: 'step-dot' }, done > i ? '✓' : String(i + 1)),
        h('div', { class: 'step-label' }, label)));
    });
    return row;
  }

  function pathChip(label, p) {
    if (!p) return null;
    return h('span', {
      class: 'badge badge-gray t2v-path', title: 'Bấm để sao chép đường dẫn',
      onclick: function () { copyText(p, 'Đã sao chép: ' + p); }
    }, label + ': ' + p);
  }

  function secBuild(ctx) {
    var s = ctx.sess;
    var mode = s.buildMode === 'html' ? 'html' : 'ai';
    var modeHost = h('div', { class: 'row', style: { alignItems: 'stretch' } });

    function renderModes() {
      modeHost.innerHTML = '';
      [
        { id: 'ai', title: 'Dựng bằng AI (Claude)', icon: '🤖',
          desc: 'Tạo dự án + phiên AI: Claude chọn media, dựng cảnh và xuất video. Cần Claude CLI còn hạn mức.' },
        { id: 'html', title: 'Dựng bằng HTML Video', icon: '🧩',
          desc: 'Render local bằng Chrome + ffmpeg, không cần Claude — dùng được cả khi Claude hết credit.' }
      ].forEach(function (m) {
        modeHost.appendChild(h('div', {
          class: 't2v-mode' + (mode === m.id ? ' sel' : ''),
          onclick: function () { mode = m.id; putSession(ctx, { buildMode: m.id }); renderModes(); }
        },
          h('div', { class: 't2v-mode-t' }, (mode === m.id ? '◉ ' : '○ ') + m.icon + ' ' + m.title),
          h('div', { class: 't2v-mode-d' }, m.desc)));
      });
    }
    renderModes();

    var buildBtn = UI.btn('🎬 Dựng video', {
      large: true, disabled: !!ctx.busy,
      onclick: function () {
        if (!(s.segments || []).length) { UI.toast('Chưa có kịch bản — hãy viết kịch bản trước.', 'error'); return; }
        if (!s.voicePath) {
          UI.toast('Chưa có giọng đọc — hãy tạo giọng đọc để đo thời lượng thật trước khi dựng.', 'error');
          return;
        }
        startJob(ctx, 'build', { mode: mode });
      }
    });

    ctx.inline.build = h('div');

    var resultHost = h('div', { class: 'mt-16' });
    if (s.outputPath) {
      if (isRel(s.outputPath) && isVideoPath(s.outputPath)) {
        resultHost.appendChild(h('video', {
          controls: 'controls', src: dataUrl(s.outputPath),
          style: { width: '100%', maxHeight: '420px', borderRadius: '12px', background: '#000' }
        }));
      }
      resultHost.appendChild(h('div', { class: 'row mt-8' },
        isRel(s.outputPath)
          ? h('a', { class: 'btn btn-primary', href: dataUrl(s.outputPath), download: fileName(s.outputPath) }, '⬇ Tải video về')
          : null,
        s.projectId ? UI.btn('📁 Mở project (render lại, QC, xuất bản…)', {
          variant: 'ghost', onclick: function () { App.navigate('projects/' + s.projectId); }
        }) : null,
        h('span', {
          class: 'muted t2v-path', title: 'Bấm để sao chép đường dẫn',
          onclick: function () { copyText(s.outputPath, 'Đã sao chép: ' + s.outputPath); }
        }, s.outputPath)));
    } else if (s.projectId) {
      resultHost.appendChild(h('div', { class: 'row' },
        h('span', { class: 'muted', style: { fontSize: '12.5px' } },
          'Đã tạo dự án cho phiên AI — mở project để theo dõi Claude dựng video.'),
        UI.btn('📁 Mở project', {
          variant: 'ghost', small: true,
          onclick: function () { App.navigate('projects/' + s.projectId); }
        })));
    } else {
      resultHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px' } },
        'Chưa có video. Chọn chế độ dựng rồi bấm "Dựng video".'));
    }

    return section(ctx, 6, 'Dựng video', '🎬', h('div', null,
      modeHost,
      h('div', { class: 'mt-16' }, buildBtn),
      ctx.inline.build,
      resultHost,
      h('div', { class: 'row mt-8' }, pathChip('File giọng đọc', s.voicePath), pathChip('Transcript', s.transcriptPath))));
  }

  // ---------- Cột trái ----------

  function renderLeft(ctx) {
    ctx.inline = {};
    ctx.renderVoiceGrid = null;
    ctx.left.innerHTML = '';
    ctx.left.appendChild(secSource(ctx));
    ctx.left.appendChild(secScript(ctx));
    ctx.left.appendChild(secStoryboard(ctx));
    ctx.left.appendChild(secVoice(ctx));
    ctx.left.appendChild(secConfig(ctx));
    ctx.left.appendChild(secBuild(ctx));
    renderJob(ctx);
  }

  // ---------- Job ----------

  function startJob(ctx, which, body) {
    ctx.busy = which;
    ctx.jobErr = '';
    ctx.errStep = '';
    ctx.job = null;
    renderLeft(ctx);
    API.post('/api/t2v/sessions/' + encodeURIComponent(ctx.sess.id) + '/' + which, body || {}).then(function (job) {
      if (!job || !job.id) throw new Error('Máy chủ không trả về tác vụ');
      ctx.jobId = job.id;
      ctx.job = job;
      renderJob(ctx);
    }).catch(function (e) {
      ctx.busy = '';
      ctx.jobErr = e.message;
      ctx.errStep = which;
      UI.toast('Không chạy được bước này: ' + e.message, 'error');
      renderLeft(ctx);
    });
  }

  // Job lẻ (ảnh của MỘT cảnh storyboard): module storyboard đăng ký ở đây thay vì
  // tự mở listener riêng — dọn dẹp gọn theo vòng đời trang.
  function watchJob(ctx, job, host, onDone) {
    if (!job || !job.id) return;
    ctx.watchers[job.id] = { host: host, onDone: onDone };
    renderWatch(ctx, job);
  }

  function renderWatch(ctx, j) {
    var w = ctx.watchers[j.id];
    if (!w) return;
    if (j.status === 'running' || j.status === 'queued') {
      if (w.host) {
        w.host.innerHTML = '';
        w.host.appendChild(UI.spinner());
        w.host.appendChild(h('span', null, Math.round(Number(j.progress) || 0) + '%'));
        if (j.detail) {
          w.host.appendChild(h('span', { style: { fontWeight: '400', opacity: '.85' } }, j.detail));
        }
      }
      return;
    }
    delete ctx.watchers[j.id];
    if (w.host) w.host.innerHTML = '';
    if (typeof w.onDone === 'function') {
      try { w.onDone(j); } catch (e) { console.error('Lỗi xử lý job storyboard:', e); }
    }
  }

  function progressBlock(j) {
    var pct = Math.round(Number(j.progress) || 0);
    return h('div', { style: { marginTop: '10px' } },
      h('div', { class: 'row-between', style: { fontSize: '12.5px' } },
        h('div', { class: 'row' }, UI.spinner(), h('b', null, JOB_LABEL[j.kind] || j.kind || 'Tác vụ')),
        h('span', { class: 'muted', style: { fontWeight: '600' } }, pct + '%')),
      h('div', { class: 'muted', style: { fontSize: '12px', margin: '4px 0' } }, j.detail || 'Đang chuẩn bị…'),
      UI.progress(pct));
  }

  // Vẽ job hiện tại vào panel nhật ký + ô inline của bước đang chạy.
  function renderJob(ctx) {
    var j = ctx.job;
    var running = !!j && (j.status === 'running' || j.status === 'queued');
    if (ctx.jobHost) {
      ctx.jobHost.innerHTML = '';
      if (running) {
        ctx.jobHost.appendChild(progressBlock(j));
      } else if (ctx.jobErr) {
        ctx.jobHost.appendChild(redAlert(ctx.jobErr));
      } else if (j && j.status === 'done') {
        ctx.jobHost.appendChild(h('div', { class: 'text-green', style: { fontSize: '12.5px', fontWeight: '600' } },
          '✓ ' + (JOB_LABEL[j.kind] || j.kind) + ' hoàn tất'));
      } else {
        ctx.jobHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px' } },
          'Chưa có tác vụ nào đang chạy — nhật ký sẽ hiện ở đây khi bạn chạy các bước.'));
      }
    }
    var host = ctx.busy ? ctx.inline[ctx.busy] : null;
    if (host) {
      host.innerHTML = '';
      if (running) {
        if (ctx.busy === 'build') host.appendChild(subStepper(ctx.sess.buildMode || 'ai', j.progress));
        host.appendChild(progressBlock(j));
      } else {
        host.appendChild(h('div', { class: 'row muted mt-8' }, UI.spinner(), 'Đang gửi yêu cầu…'));
      }
    }
    if (!ctx.busy && ctx.jobErr) {
      var errHost = ctx.inline[ctx.errStep] || ctx.inline.build;
      if (errHost) {
        errHost.innerHTML = '';
        errHost.appendChild(h('div', { class: 'mt-8' }, redAlert(ctx.jobErr)));
      }
    }
  }

  // ---------- Cột phải: Nhật ký ----------

  function logLine(l) {
    var t = '';
    try { t = new Date(l.createdAt).toLocaleTimeString('vi-VN'); } catch (e) { t = ''; }
    var cls = 't2v-logline' + (l.level === 'error' ? ' error' : (l.level === 'warn' ? ' warn' : ''));
    return h('div', { class: cls }, t + ' [' + (l.module || '?') + '] ' + (l.message || ''));
  }

  function renderAiLink(ctx) {
    if (!ctx.aiHost) return;
    ctx.aiHost.innerHTML = '';
    var s = ctx.sess;
    if ((s.buildMode || 'ai') !== 'ai' || !s.projectId) return;
    var btn = UI.btn('🤖 Xem phiên AI trong project', {
      onclick: function () { App.navigate('projects/' + s.projectId); }
    });
    btn.style.width = '100%';
    ctx.aiHost.appendChild(h('div', { class: 'mt-8' }, btn,
      h('div', { class: 'muted', style: { fontSize: '11.5px', marginTop: '4px' } },
        'Log chi tiết của Claude (tool call, tiến độ dựng) hiển thị trong trang dự án.')));
  }

  function renderRight(ctx) {
    ctx.jobHost = h('div');
    ctx.aiHost = h('div');
    ctx.logHost = h('div', { class: 't2v-log' });
    ctx.right.innerHTML = '';
    ctx.right.appendChild(UI.card({
      title: 'Nhật ký', icon: '🧾',
      desc: 'Tiến trình các bước của phiên này và log hệ thống liên quan.',
      body: [ctx.jobHost, ctx.aiHost, ctx.logHost]
    }));
    renderAiLink(ctx);
    renderJob(ctx);

    API.get('/api/logs?limit=120').then(function (list) {
      var rows = (list || []).filter(function (l) { return LOG_RE.test(String(l.module || '')); }).slice(0, 40);
      ctx.logHost.innerHTML = '';
      if (!rows.length) {
        ctx.logHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12px' } }, 'Chưa có log liên quan.'));
        return;
      }
      rows.forEach(function (l) { ctx.logHost.appendChild(logLine(l)); });
    }).catch(function () {
      ctx.logHost.innerHTML = '';
      ctx.logHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12px' } },
        'Không tải được log cũ — log mới vẫn hiện theo thời gian thực.'));
    });
  }

  // ---------- Realtime ----------

  function bindRealtime(ctx) {
    sub(ctx, 'job', function (j) {
      if (!j) return;
      if (ctx.watchers[j.id]) { renderWatch(ctx, j); return; }  // ảnh của 1 cảnh — storyboard tự lo
      if (j.kind === 't2v_shot') return;                        // cảnh lẻ không thuộc luồng chính
      if (j.id !== ctx.jobId && String(j.kind || '').indexOf('t2v_') !== 0) return;
      ctx.job = j;
      if (j.status === 'running' || j.status === 'queued') {
        ctx.jobId = j.id;
        var b = BUSY_BY_KIND[j.kind] || ctx.busy;
        if (b && ctx.busy !== b) { ctx.busy = b; renderLeft(ctx); return; }
        renderJob(ctx);
        return;
      }
      if (j.id !== ctx.jobId) { renderJob(ctx); return; }
      ctx.jobId = null;
      var failedStep = BUSY_BY_KIND[j.kind] || ctx.busy;
      ctx.busy = '';
      if (j.status === 'done') {
        ctx.jobErr = '';
        ctx.errStep = '';
        UI.toast((JOB_LABEL[j.kind] || 'Tác vụ') + ' hoàn tất');
        reloadSession(ctx).then(function () { renderAiLink(ctx); renderJob(ctx); });
      } else {
        ctx.jobErr = j.error || 'Lỗi không xác định';
        ctx.errStep = failedStep;
        UI.toast('Thất bại: ' + ctx.jobErr, 'error');
        renderLeft(ctx);
      }
    });

    // /api/state về sau lần vẽ đầu — cập nhật cảnh báo thiếu API key của Storyboard
    // mà không dựng lại cả cột trái (tránh mất phần người dùng đang gõ).
    sub(ctx, 'state', function () {
      if (window.T2VStoryboard && typeof window.T2VStoryboard.syncWarn === 'function') {
        window.T2VStoryboard.syncWarn();
      }
    });

    sub(ctx, 'log', function (l) {
      if (!l || !ctx.logHost || !LOG_RE.test(String(l.module || ''))) return;
      var first = ctx.logHost.firstChild;
      if (first && first.className && first.className.indexOf('t2v-logline') < 0) ctx.logHost.innerHTML = '';
      ctx.logHost.insertBefore(logLine(l), ctx.logHost.firstChild);
      while (ctx.logHost.childNodes.length > 60) ctx.logHost.removeChild(ctx.logHost.lastChild);
    });
  }

  // Gắn lại job đang chạy khi mở lại trang giữa chừng.
  function attachRunningJob(ctx) {
    API.get('/api/jobs').then(function (jobs) {
      for (var i = 0; i < (jobs || []).length; i++) {
        var j = jobs[i];
        if (!BUSY_BY_KIND[j.kind]) continue;
        if (j.status !== 'running' && j.status !== 'queued') continue;
        ctx.jobId = j.id;
        ctx.job = j;
        ctx.busy = BUSY_BY_KIND[j.kind];
        renderLeft(ctx);
        return;
      }
    }).catch(function () { /* không có /api/jobs cũng không sao */ });
  }

  // ---------- Dựng trang chi tiết ----------

  function loadVoices(ctx) {
    API.get('/api/tools/voices').then(function (list) {
      ctx.voices = sortVoices(list);
      if (!ctx.sess.voiceId && ctx.voices.length) {
        ctx.sess.voiceId = ctx.voices[0].id;
        ctx.sess.voiceEngine = ctx.voices[0].engine || '';
      }
      renderChips(ctx);
      if (ctx.renderVoiceGrid) ctx.renderVoiceGrid();
      else renderLeft(ctx);
    }).catch(function (e) {
      ctx.voices = [];
      ctx.voicesErr = e.message;
      if (ctx.renderVoiceGrid) ctx.renderVoiceGrid();
    });
  }

  function build(ctx) {
    ctx.headHost = h('div');
    ctx.stepHost = h('div', { class: 'mt-16' });
    ctx.chipHost = h('div');
    ctx.left = h('div');
    ctx.right = h('div');

    ctx.el.appendChild(UI.card({ body: [ctx.headHost, ctx.stepHost, ctx.chipHost] }));
    ctx.el.appendChild(h('div', { class: 't2v-grid' }, ctx.left, ctx.right));

    renderHead(ctx);
    renderStepper(ctx);
    renderChips(ctx);
    renderRight(ctx);
    renderLeft(ctx);
    bindRealtime(ctx);
    attachRunningJob(ctx);
    setPageHeader(ctx);
    loadVoices(ctx);

    ctx.timers.push(setInterval(function () { renderChips(ctx); }, 30000));

    App._cleanup = function () {
      ctx.unsubs.forEach(function (u) { Bus.off(u.ev, u.fn); });
      ctx.timers.forEach(clearInterval);
      if (ctx.saveTimer) { clearTimeout(ctx.saveTimer); ctx.saveTimer = null; }
      ctx.watchers = {};
      if (typeof ctx.sbCleanup === 'function') {
        try { ctx.sbCleanup(); } catch (e) { console.error('Lỗi dọn Storyboard:', e); }
        ctx.sbCleanup = null;
      }
    };
  }

  var mountSeq = 0;

  function openSession(el, id) {
    el.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải phiên…'));
    API.get('/api/t2v/sessions/' + encodeURIComponent(id)).then(function (s) {
      el.innerHTML = '';
      if (!s || !s.id) throw new Error('Phản hồi không hợp lệ từ máy chủ');
      mountSeq++;
      build({
        el: el, sess: s, voices: null, voicesErr: '',
        open: initOpen(Number(s.step) || 0), inline: {}, renderVoiceGrid: null,
        unsubs: [], timers: [], saveTimer: null,
        busy: '', job: null, jobId: null, jobErr: '', errStep: '',
        mountId: s.id + '#' + mountSeq, watchers: {}, sbCleanup: null
      });
    }).catch(function (e) {
      el.innerHTML = '';
      App.setHeader('Text → Video', 'Không mở được phiên');
      el.appendChild(UI.card({
        title: 'Không mở được phiên', icon: '❌',
        body: [redAlert(e.message),
          h('div', { class: 'row mt-16' },
            UI.btn('← Danh sách phiên', { variant: 'ghost', onclick: function () { App.navigate('text2video'); } }),
            UI.btn('Thử lại', { variant: 'ghost', onclick: function () { el.innerHTML = ''; openSession(el, id); } }))]
      }));
    });
  }

  // ---------- Đăng ký trang ----------

  App.pages.text2video = {
    title: 'Text → Video',
    subtitle: 'Văn bản hoặc link bài viết → kịch bản đọc → giọng đọc → video. Phiên lưu lại, sửa tiếp bất cứ lúc nào.',
    render: function (el, param) {
      ensureCss();
      el.innerHTML = '';
      if (param) { openSession(el, param); return; }
      renderList(el);
    }
  };
})();
