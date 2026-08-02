/* ============================================================
   Biz Studio — Trang TTS / Giọng đọc
   Load sau app.js. Tự đăng ký App.pages['tts'].
   ============================================================ */
(function () {
  'use strict';

  // ---------- Helpers nội bộ ----------

  function copyText(text, okMsg) {
    function fallback() {
      var ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand('copy'); UI.toast(okMsg || 'Đã sao chép vào clipboard'); }
      catch (e) { UI.toast('Không sao chép được — hãy copy thủ công', 'error'); }
      ta.remove();
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () {
        UI.toast(okMsg || 'Đã sao chép vào clipboard');
      }, fallback);
    } else fallback();
  }

  function statusMeta(s) {
    if (s === 'done') return { label: 'Hoàn thành', cls: 'badge badge-green' };
    if (s === 'error') return { label: 'Lỗi', cls: 'badge badge-red' };
    if (s === 'running') return { label: 'Đang tạo', cls: 'badge badge-blue' };
    return { label: 'Chờ xử lý', cls: 'badge badge-gray' };
  }

  function isRel(p) { return !!p && p.charAt(0) !== '/'; }
  function dataUrl(p) { return '/data/' + p.split('/').map(encodeURIComponent).join('/'); }
  function fileName(p) { return (p || '').split('/').pop(); }

  function redAlert(msg) {
    return h('div', {
      class: 'text-red',
      style: { background: 'rgba(239,68,68,.08)', border: '1px solid var(--red)', borderRadius: '10px', padding: '10px 12px', fontSize: '13px', fontWeight: '600' }
    }, '⚠ ' + msg);
  }

  function amberWarn(msg) {
    return h('div', {
      style: { background: 'rgba(245,158,11,.09)', border: '1px solid var(--amber)', borderRadius: '10px', padding: '10px 12px', marginBottom: '10px' }
    },
      h('div', { class: 'row' },
        h('span', null, '⚠️'),
        h('span', { class: 'text-amber', style: { fontWeight: '600', flex: '1', minWidth: '160px' } }, msg),
        h('a', { href: '#/settings', class: 'btn btn-ghost btn-sm' }, 'Mở Cấu hình & API →')));
  }

  function voiceColor(s) {
    var hsh = 0;
    s = String(s || '?');
    for (var i = 0; i < s.length; i++) hsh = (hsh * 31 + s.charCodeAt(i)) % 360;
    return 'hsl(' + hsh + ', 62%, 45%)';
  }

  function genderIcon(g) {
    if (g === 'female') return '♀';
    if (g === 'male') return '♂';
    return '';
  }

  function audioResult(j) {
    var host = h('div');
    if (!j.output) {
      host.appendChild(h('span', { class: 'muted' }, 'Hoàn thành (không có file đầu ra)'));
      return host;
    }
    if (!isRel(j.output)) {
      host.appendChild(h('div', { class: 'row' },
        h('code', { style: { fontSize: '12px', wordBreak: 'break-all' } }, j.output),
        UI.btn('📋 Copy đường dẫn', { variant: 'ghost', small: true, onclick: function () { copyText(j.output); } })));
      return host;
    }
    var url = dataUrl(j.output);
    host.appendChild(h('audio', { controls: 'controls', src: url, style: { width: '100%', marginBottom: '8px' } }));
    host.appendChild(h('div', { class: 'row' },
      h('a', { class: 'btn btn-ghost btn-sm', href: url, download: fileName(j.output) }, '⬇ Tải ' + fileName(j.output)),
      UI.btn('📋 Copy link', { variant: 'ghost', small: true, onclick: function () { copyText(location.origin + url); } })));
    return host;
  }

  function jobCard(job) {
    var badge = h('span', { class: 'badge badge-gray' }, '');
    var pctEl = h('span', { class: 'muted', style: { fontWeight: '600', flex: 'none' } }, '0%');
    var detailEl = h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '4px', wordBreak: 'break-all' } }, '');
    var bar = UI.progress(0);
    var outHost = h('div', { class: 'mt-8' });
    var card = h('div', { class: 'card' },
      h('div', { class: 'row-between' },
        h('div', { class: 'row' }, h('strong', null, '🎙 Giọng đọc'), badge),
        pctEl),
      detailEl,
      h('div', { class: 'mt-8' }, bar),
      outHost);
    card.update = function (j) {
      var m = statusMeta(j.status);
      badge.className = m.cls;
      badge.textContent = m.label;
      var p = Math.max(0, Math.min(100, Math.round(Number(j.progress) || 0)));
      pctEl.textContent = p + '%';
      bar.set(p);
      detailEl.textContent = j.detail || '';
      if (j.status === 'done') {
        if (card._done) return;
        card._done = true;
        outHost.innerHTML = '';
        outHost.appendChild(audioResult(j));
      } else {
        card._done = false;
        outHost.innerHTML = '';
        if (j.status === 'error') outHost.appendChild(redAlert(j.error || 'Lỗi không xác định'));
      }
    };
    card.update(job);
    return card;
  }

  // ---------- Trang ----------

  App.pages.tts = {
    title: 'TTS / Giọng đọc',
    subtitle: 'Chuyển văn bản thành giọng đọc — giọng macOS (say) hoặc Gemini TTS',
    render: function (el) {
      var voices = [];
      var selected = null;
      var rate = 175;
      var query = '';
      var engineFilter = '';
      var jobEls = {};

      // --- 1. Nguồn nội dung
      var counter = h('div', { class: 'muted', style: { fontSize: '12px', textAlign: 'right', marginTop: '4px' } }, '0 ký tự');
      var textTa = UI.textarea({
        placeholder: 'Nhập hoặc dán văn bản cần chuyển thành giọng đọc…',
        rows: 7,
        oninput: function () { counter.textContent = textTa.value.length + ' ký tự'; }
      });
      el.appendChild(UI.card({
        title: '1. Nguồn nội dung', icon: '📄',
        body: h('div', null, textTa, counter)
      }));

      // --- 4. Chọn giọng đọc
      var tabRow = h('div', { class: 'row', style: { marginBottom: '12px' } },
        h('button', { class: 'btn btn-primary', type: 'button' }, '⚡ Dubbing nhanh'),
        h('button', { class: 'btn btn-ghost', type: 'button', disabled: true },
          '💎 Dubbing chất lượng ', h('span', { class: 'badge badge-amber' }, 'sắp có')),
        h('button', { class: 'btn btn-ghost', type: 'button', disabled: true },
          '🧬 Clone voice ', h('span', { class: 'badge badge-amber' }, 'sắp có')));

      var searchInput = UI.input({
        placeholder: '🔍 Tìm giọng theo tên…',
        oninput: function () { query = searchInput.value.trim().toLowerCase(); renderGrid(); }
      });
      var engineSel = UI.select(null, [
        { value: '', label: 'Tất cả engine' },
        { value: 'say', label: 'macOS' },
        { value: 'gemini', label: 'Gemini' }
      ], '', function (v) { engineFilter = v; renderGrid(); });

      var gridHost = h('div', null, h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải danh sách giọng…'));
      var selectedLine = h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '10px' } }, 'Chưa chọn giọng nào');

      function voiceCard(v) {
        var isSel = selected && selected.id === v.id;
        var name = v.name || v.id;
        return h('div', {
          class: 'card',
          style: {
            padding: '12px', cursor: 'pointer', textAlign: 'center', marginTop: '0',
            borderColor: isSel ? 'var(--blue)' : '',
            boxShadow: isSel ? 'var(--focus-ring)' : ''
          },
          onclick: function () { selected = v; renderGrid(); updateSelectedLine(); renderWarn(); }
        },
          h('div', {
            style: {
              width: '42px', height: '42px', borderRadius: '50%',
              background: voiceColor(name), color: '#fff',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontWeight: '700', fontSize: '17px', margin: '0 auto 8px'
            }
          }, name.charAt(0).toUpperCase()),
          h('div', { style: { fontWeight: '600', fontSize: '13px', wordBreak: 'break-word' } },
            name, genderIcon(v.gender) ? ' ' + genderIcon(v.gender) : ''),
          h('div', { class: 'muted', style: { fontSize: '11.5px', margin: '3px 0 6px' } }, v.lang || '—'),
          h('span', { class: v.engine === 'gemini' ? 'badge badge-blue' : 'badge badge-gray' },
            v.engine === 'gemini' ? 'Gemini' : 'macOS'));
      }

      function renderGrid() {
        gridHost.innerHTML = '';
        var filtered = voices.filter(function (v) {
          if (engineFilter && v.engine !== engineFilter) return false;
          if (query && String(v.name || v.id).toLowerCase().indexOf(query) < 0) return false;
          return true;
        });
        if (!filtered.length) {
          gridHost.appendChild(UI.empty('Không có giọng nào khớp bộ lọc', '🎙️'));
          return;
        }
        var grid = h('div', {
          style: {
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))',
            gap: '10px'
          }
        });
        filtered.forEach(function (v) { grid.appendChild(voiceCard(v)); });
        gridHost.appendChild(grid);
      }

      function updateSelectedLine() {
        if (!selected) { selectedLine.textContent = 'Chưa chọn giọng nào'; return; }
        selectedLine.innerHTML = '';
        selectedLine.appendChild(h('span', null, '✅ Giọng đã chọn: '));
        selectedLine.appendChild(h('strong', null, selected.name || selected.id));
        selectedLine.appendChild(h('span', null,
          ' (' + (selected.lang || '—') + ' · ' + (selected.engine === 'gemini' ? 'Gemini' : 'macOS') + ')'));
      }

      API.get('/api/tools/voices').then(function (list) {
        voices = list || [];
        if (!voices.length) {
          gridHost.innerHTML = '';
          gridHost.appendChild(UI.empty('Không tìm thấy giọng đọc nào trên hệ thống', '🎙️'));
          return;
        }
        selected = null;
        for (var i = 0; i < voices.length; i++) {
          if (String(voices[i].lang || '').toLowerCase().indexOf('vi') === 0) { selected = voices[i]; break; }
        }
        if (!selected) selected = voices[0];
        renderGrid();
        updateSelectedLine();
        renderWarn();
      }).catch(function (err) {
        gridHost.innerHTML = '';
        gridHost.appendChild(redAlert('Không tải được danh sách giọng: ' + err.message));
      });

      el.appendChild(UI.card({
        title: '4. Chọn giọng đọc', icon: '🎙️',
        body: h('div', null,
          tabRow,
          h('div', { class: 'grid-2', style: { marginBottom: '12px' } }, searchInput, engineSel),
          gridHost,
          selectedLine)
      }));

      // --- 5. Cấu hình
      var rateSlider = UI.slider(null, {
        min: 100, max: 260, step: 5, value: rate,
        oninput: function (v) { rate = v; }
      });
      el.appendChild(UI.card({
        title: '5. Cấu hình', icon: '⚙️',
        body: h('div', null,
          UI.field('Tốc độ đọc (từ/phút)', rateSlider),
          h('div', { class: 'muted', style: { fontSize: '12px' } },
            '💡 Tốc độ chỉ áp dụng cho giọng macOS (engine say). Giọng Gemini dùng tốc độ mặc định.'))
      }));

      // --- Cảnh báo Gemini key
      var warnHost = h('div');
      function renderWarn() {
        warnHost.innerHTML = '';
        var st = App.state;
        if (selected && selected.engine === 'gemini' && st && st.tools && !st.tools.geminiKey) {
          warnHost.appendChild(amberWarn('Giọng đang chọn dùng Gemini nhưng chưa cấu hình Gemini API key.'));
        }
      }
      var onState = function () { renderWarn(); };
      Bus.on('state', onState);

      // --- CTA + kết quả
      var errBox = h('div', { class: 'mt-8', style: { display: 'none' } });
      function showErr(msg) {
        errBox.style.display = '';
        errBox.innerHTML = '';
        errBox.appendChild(redAlert(msg));
      }

      var resultHost = h('div');
      var resultEmpty = UI.empty('File âm thanh sẽ hiện tại đây sau khi tạo xong', '🔊');
      resultHost.appendChild(resultEmpty);

      function addJobCard(j) {
        if (jobEls[j.id]) { jobEls[j.id].update(j); return; }
        if (resultEmpty.parentNode) resultEmpty.remove();
        var c = jobCard(j);
        jobEls[j.id] = c;
        if (resultHost.firstChild) resultHost.insertBefore(c, resultHost.firstChild);
        else resultHost.appendChild(c);
      }

      var startBtn = UI.btn('🎙 BẮT ĐẦU TẠO GIỌNG ĐỌC', {
        variant: 'primary', large: true,
        onclick: function () {
          errBox.style.display = 'none';
          var text = textTa.value.trim();
          if (!text) { showErr('Chưa có văn bản — nhập nội dung ở mục "1. Nguồn nội dung".'); return; }
          if (!selected) { showErr('Chưa chọn giọng đọc.'); return; }
          startBtn.disabled = true;
          API.post('/api/tools/tts', { text: text, voice: selected.id, rate: rate, engine: selected.engine })
            .then(function (job) {
              addJobCard(job);
              UI.toast('Đã bắt đầu tạo giọng đọc');
            })
            .catch(function (err) { showErr(err.message); })
            .finally(function () { startBtn.disabled = false; });
        }
      });

      el.appendChild(UI.card({ body: h('div', null, warnHost, startBtn, errBox) }));
      el.appendChild(UI.card({ title: 'Kết quả', icon: '🔊', body: resultHost }));

      var onJob = function (j) {
        if (!j || !jobEls[j.id]) return;
        jobEls[j.id].update(j);
      };
      Bus.on('job', onJob);
      App._cleanup = function () {
        Bus.off('job', onJob);
        Bus.off('state', onState);
      };
    }
  };
})();
