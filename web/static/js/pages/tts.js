/* ============================================================
   Biz Studio — Trang TTS / Giọng đọc
   3 chế độ: ⚡ Dubbing nhanh · 💎 Dubbing chất lượng · 🧬 Clone voice
   Load sau app.js. Tự đăng ký App.pages['tts'].
   ============================================================ */
(function () {
  'use strict';

  var ENGINE_LABELS = { vieneu: 'VieNeu', say: 'macOS', gemini: 'Gemini', clone: 'Nhân bản' };
  var VIDEO_EXT = ['.mp4', '.mov', '.mkv', '.webm', '.avi', '.m4v'];
  var STYLE_OPTS = [
    { value: 'tu_nhien', label: 'Tự nhiên (hội thoại)' },
    { value: 'tin_tuc', label: 'Tin tức' },
    { value: 'doc_truyen', label: 'Đọc truyện' }
  ];

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

  function isVideoPath(p) {
    var low = String(p || '').toLowerCase();
    for (var i = 0; i < VIDEO_EXT.length; i++) {
      if (low.slice(-VIDEO_EXT[i].length) === VIDEO_EXT[i]) return true;
    }
    return false;
  }

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

  function genderLabel(g) {
    if (g === 'female') return 'Nữ';
    if (g === 'male') return 'Nam';
    return 'Chưa rõ giới tính';
  }

  function engineBadgeCls(e) {
    if (e === 'vieneu') return 'badge badge-green';
    if (e === 'gemini') return 'badge badge-blue';
    if (e === 'clone') return 'badge badge-amber';
    return 'badge badge-gray';
  }

  // Bật/tắt mọi control bên trong một khối (slider, select đi kèm nhãn…)
  function setEnabled(el, on) {
    if (!el) return;
    var list = el.querySelectorAll('input, select, textarea, button');
    for (var i = 0; i < list.length; i++) list[i].disabled = !on;
    el.style.opacity = on ? '' : '.5';
  }

  function avatar(name, size) {
    var s = (size || 42) + 'px';
    return h('div', {
      style: {
        width: s, height: s, borderRadius: '50%', flex: 'none',
        background: voiceColor(name), color: '#fff',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontWeight: '700', fontSize: (size && size < 40 ? '15px' : '17px')
      }
    }, String(name || '?').charAt(0).toUpperCase());
  }

  // Kết quả job: audio hoặc video + tải về + copy đường dẫn
  function mediaResult(j) {
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
    if (isVideoPath(j.output)) {
      host.appendChild(h('video', {
        controls: 'controls', src: url,
        style: { width: '100%', maxHeight: '440px', borderRadius: '12px', background: '#000', marginBottom: '8px' }
      }));
    } else {
      host.appendChild(h('audio', { controls: 'controls', src: url, style: { width: '100%', marginBottom: '8px' } }));
    }
    host.appendChild(h('div', { class: 'row' },
      h('a', { class: 'btn btn-ghost btn-sm', href: url, download: fileName(j.output) }, '⬇ Tải ' + fileName(j.output)),
      UI.btn('📋 Copy link', { variant: 'ghost', small: true, onclick: function () { copyText(location.origin + url); } }),
      UI.btn('📋 Copy đường dẫn', { variant: 'ghost', small: true, onclick: function () { copyText(j.output, 'Đã copy đường dẫn: ' + j.output); } })));
    return host;
  }

  function jobCard(job, title) {
    var badge = h('span', { class: 'badge badge-gray' }, '');
    var pctEl = h('span', { class: 'muted', style: { fontWeight: '600', flex: 'none' } }, '0%');
    var detailEl = h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '4px', wordBreak: 'break-all' } }, '');
    var bar = UI.progress(0);
    var outHost = h('div', { class: 'mt-8' });
    var card = h('div', { class: 'card' },
      h('div', { class: 'row-between' },
        h('div', { class: 'row' }, h('strong', null, title || '🎙 Giọng đọc'), badge),
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
        if (!card._done) {
          card._done = true;
          outHost.innerHTML = '';
          outHost.appendChild(mediaResult(j));
        }
      } else {
        card._done = false;
        outHost.innerHTML = '';
        if (j.status === 'error') outHost.appendChild(redAlert(j.error || 'Lỗi không xác định'));
      }
      if ((j.status === 'done' || j.status === 'error') && typeof card.onFinish === 'function') {
        var fn = card.onFinish;
        card.onFinish = null;
        fn(j);
      }
    };
    card.update(job);
    return card;
  }

  // ---------- Trang ----------

  App.pages.tts = {
    title: 'TTS / Giọng đọc',
    subtitle: 'Ba chế độ: Dubbing nhanh (văn bản → giọng đọc), Dubbing chất lượng (lồng tiếng video theo phụ đề) và Clone voice (nhân bản giọng từ clip mẫu)',
    render: function (el) {
      var voices = [];
      var selected = null;
      var rate = 175;
      var query = '';
      var engineFilter = '';
      var vieneuStyle = 'tu_nhien';
      var jobEls = {};
      var tab = 'quick';

      // =========================================================
      // KHỐI CHỌN GIỌNG — dùng chung cho tab 1 & tab 2
      // =========================================================
      var searchInput = UI.input({
        placeholder: '🔍 Tìm giọng theo tên…',
        oninput: function () { query = searchInput.value.trim().toLowerCase(); renderGrid(); }
      });
      var engineSel = UI.select(null, [
        { value: '', label: 'Tất cả engine' },
        { value: 'vieneu', label: 'VieNeu (khuyên dùng)' },
        { value: 'clone', label: 'Nhân bản (clone)' },
        { value: 'say', label: 'macOS' },
        { value: 'gemini', label: 'Gemini' }
      ], '', function (v) { engineFilter = v; renderGrid(); });

      var gridHost = h('div');
      var selectedLine = h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '10px' } }, 'Chưa chọn giọng nào');
      var voiceBlock = h('div', null,
        h('div', { class: 'grid-2', style: { marginBottom: '12px' } }, searchInput, engineSel),
        gridHost, selectedLine);

      // Nhiều select "phong cách" (tab 1 + tab 2) cùng trỏ về vieneuStyle
      var styleSels = [];
      function makeStyleSelect() {
        var sel = UI.select(null, STYLE_OPTS, vieneuStyle, function (v) {
          vieneuStyle = v;
          styleSels.forEach(function (s) { if (s.value !== v) s.value = v; });
        });
        styleSels.push(sel);
        return sel;
      }

      function needStyle(v) { return !!v && (v.engine === 'vieneu' || v.engine === 'clone'); }
      function voiceParam() {
        if (!selected) return '';
        return needStyle(selected) ? selected.id + '@' + vieneuStyle : selected.id;
      }

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
          h('div', { style: { display: 'flex', justifyContent: 'center', marginBottom: '8px' } }, avatar(name, 42)),
          h('div', { style: { fontWeight: '600', fontSize: '13px', wordBreak: 'break-word' } },
            name, genderIcon(v.gender) ? ' ' + genderIcon(v.gender) : ''),
          h('div', { class: 'muted', style: { fontSize: '11.5px', margin: '3px 0 6px' } }, v.lang || '—'),
          h('span', { class: engineBadgeCls(v.engine) }, ENGINE_LABELS[v.engine] || v.engine));
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
          style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))', gap: '10px' }
        });
        filtered.forEach(function (v) { grid.appendChild(voiceCard(v)); });
        gridHost.appendChild(grid);
      }

      function updateSelectedLine() {
        var show = needStyle(selected) ? '' : 'none';
        if (styleRow) styleRow.style.display = show;
        if (styleRow2) styleRow2.style.display = show;
        if (!selected) { selectedLine.textContent = 'Chưa chọn giọng nào'; return; }
        selectedLine.innerHTML = '';
        selectedLine.appendChild(h('span', null, '✅ Giọng đã chọn: '));
        selectedLine.appendChild(h('strong', null, selected.name || selected.id));
        selectedLine.appendChild(h('span', null,
          ' (' + (selected.lang || '—') + ' · ' + (ENGINE_LABELS[selected.engine] || selected.engine) + ')'));
      }

      function loadVoices(silent) {
        if (!silent) {
          gridHost.innerHTML = '';
          gridHost.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải danh sách giọng…'));
        }
        return API.get('/api/tools/voices').then(function (list) {
          voices = list || [];
          if (!voices.length) {
            selected = null;
            gridHost.innerHTML = '';
            gridHost.appendChild(UI.empty('Không tìm thấy giọng đọc nào trên hệ thống', '🎙️'));
            updateSelectedLine();
            return;
          }
          var keepId = selected ? selected.id : '';
          var i;
          selected = null;
          for (i = 0; i < voices.length; i++) if (voices[i].id === keepId) { selected = voices[i]; break; }
          if (!selected) {
            for (i = 0; i < voices.length; i++) {
              if (String(voices[i].lang || '').toLowerCase().indexOf('vi') === 0) { selected = voices[i]; break; }
            }
          }
          if (!selected) selected = voices[0];
          renderGrid();
          updateSelectedLine();
          renderWarn();
        }).catch(function (err) {
          gridHost.innerHTML = '';
          gridHost.appendChild(redAlert('Không tải được danh sách giọng: ' + err.message));
        });
      }

      // Kết quả job dùng chung
      function addJobCard(host, emptyEl, job, title) {
        if (!job || !job.id) { UI.toast('Máy chủ không trả về thông tin tác vụ', 'error'); return; }
        if (jobEls[job.id]) { jobEls[job.id].update(job); return; }
        if (emptyEl && emptyEl.parentNode) emptyEl.remove();
        var c = jobCard(job, title);
        jobEls[job.id] = c;
        if (host.firstChild) host.insertBefore(c, host.firstChild);
        else host.appendChild(c);
        return c;
      }

      // Upload file lên data/uploads/ rồi điền vào ô đường dẫn
      function uploadHandler(target, info, okMsg, after) {
        return function (files) {
          var f0 = files[0];
          info.textContent = '⏳ Đang tải lên ' + f0.name + '…';
          API.upload('/api/tools/upload', [f0]).then(function (list) {
            var f = list && list[0];
            if (!f) throw new Error('server không trả về thông tin file');
            target.value = f.path;
            info.textContent = '✓ Đã tải lên: ' + f.name + ' (' + UI.fmtBytes(f.size) + ')';
            UI.toast(okMsg || 'Đã tải file lên');
            if (after) after();
          }).catch(function (e) {
            info.textContent = '';
            UI.toast('Tải lên thất bại: ' + e.message, 'error');
          });
        };
      }

      // =========================================================
      // TAB 1 — ⚡ DUBBING NHANH (giữ nguyên hành vi cũ)
      // =========================================================
      var paneQuick = h('div');

      var counter = h('div', { class: 'muted', style: { fontSize: '12px', textAlign: 'right', marginTop: '4px' } }, '0 ký tự');
      var textTa = UI.textarea({
        placeholder: 'Nhập hoặc dán văn bản cần chuyển thành giọng đọc…',
        rows: 7,
        oninput: function () { counter.textContent = textTa.value.length + ' ký tự'; }
      });
      paneQuick.appendChild(UI.card({
        title: '1. Nguồn nội dung', icon: '📄',
        body: h('div', null, textTa, counter)
      }));

      var voiceSlot1 = h('div');
      paneQuick.appendChild(UI.card({ title: '2. Chọn giọng đọc', icon: '🎙️', body: voiceSlot1 }));

      var rateSlider = UI.slider(null, {
        min: 100, max: 260, step: 5, value: rate,
        oninput: function (v) { rate = v; }
      });
      var styleRow = UI.field('Phong cách đọc (VieNeu / nhân bản)', makeStyleSelect());
      paneQuick.appendChild(UI.card({
        title: '3. Cấu hình', icon: '⚙️',
        body: h('div', null,
          styleRow,
          UI.field('Tốc độ đọc (từ/phút)', rateSlider),
          h('div', { class: 'muted', style: { fontSize: '12px' } },
            '💡 Phong cách đọc áp dụng cho giọng VieNeu và giọng nhân bản; tốc độ áp dụng cho giọng macOS (say). Giọng Gemini dùng mặc định.'))
      }));

      var warnHost = h('div');
      function renderWarn() {
        warnHost.innerHTML = '';
        var st = App.state;
        if (selected && selected.engine === 'gemini' && st && st.tools && !st.tools.geminiKey) {
          warnHost.appendChild(amberWarn('Giọng đang chọn dùng Gemini nhưng chưa cấu hình Gemini API key.'));
        }
      }

      var errBox = h('div', { class: 'mt-8', style: { display: 'none' } });
      function showErr(msg) {
        errBox.style.display = '';
        errBox.innerHTML = '';
        errBox.appendChild(redAlert(msg));
      }

      var resultHost = h('div');
      var resultEmpty = UI.empty('File âm thanh sẽ hiện tại đây sau khi tạo xong', '🔊');
      resultHost.appendChild(resultEmpty);

      var startBtn = UI.btn('🎙 BẮT ĐẦU TẠO GIỌNG ĐỌC', {
        variant: 'primary', large: true,
        onclick: function () {
          errBox.style.display = 'none';
          var text = textTa.value.trim();
          if (!text) { showErr('Chưa có văn bản — nhập nội dung ở mục "1. Nguồn nội dung".'); return; }
          if (!selected) { showErr('Chưa chọn giọng đọc.'); return; }
          startBtn.disabled = true;
          API.post('/api/tools/tts', { text: text, voice: voiceParam(), rate: rate, engine: selected.engine })
            .then(function (job) {
              addJobCard(resultHost, resultEmpty, job, '🎙 Giọng đọc');
              UI.toast('Đã bắt đầu tạo giọng đọc');
            })
            .catch(function (err) { showErr(err.message); })
            .finally(function () { startBtn.disabled = false; });
        }
      });

      paneQuick.appendChild(UI.card({ body: h('div', null, warnHost, startBtn, errBox) }));
      paneQuick.appendChild(UI.card({ title: 'Kết quả', icon: '🔊', body: resultHost }));

      // =========================================================
      // TAB 2 — 💎 DUBBING CHẤT LƯỢNG (lồng tiếng video theo phụ đề)
      // =========================================================
      var paneDub = h('div', { style: { display: 'none' } });

      var dubVideoPath = UI.input({ placeholder: 'vd: uploads/video.mp4 (tương đối trong data/) hoặc /Users/ban/Movies/clip.mov' });
      var dubVideoInfo = h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } });
      var dubVideoDz = UI.dropzone({
        hint: 'Kéo thả hoặc nhấp để thêm Video cần lồng tiếng',
        sub: 'Hỗ trợ: MP4, MOV, MKV, WEBM — file được tải vào data/uploads/',
        accept: 'video/*,.mp4,.mov,.mkv,.webm,.m4v',
        onfiles: uploadHandler(dubVideoPath, dubVideoInfo, 'Đã tải video lên — sẵn sàng lồng tiếng')
      });

      var dubSrtPath = UI.input({
        placeholder: 'vd: uploads/phim.srt — bỏ trống để tự bóc băng bằng Gemini',
        oninput: function () { renderDubWarn(); }
      });
      var dubSrtInfo = h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } });
      var dubSrtDz = UI.dropzone({
        hint: 'Kéo thả hoặc nhấp để thêm phụ đề (.srt)',
        sub: 'Tuỳ chọn — bỏ trống thì hệ thống tự bóc băng bằng Gemini',
        accept: '.srt',
        onfiles: uploadHandler(dubSrtPath, dubSrtInfo, 'Đã tải phụ đề lên', function () { renderDubWarn(); })
      });

      var dubWarnHost = h('div');
      function renderDubWarn() {
        dubWarnHost.innerHTML = '';
        var st = App.state;
        if (!dubSrtPath.value.trim() && st && st.tools && !st.tools.geminiKey) {
          dubWarnHost.appendChild(amberWarn('Chưa có phụ đề .srt và cũng chưa cấu hình Gemini API key — không thể tự bóc băng.'));
        }
      }

      paneDub.appendChild(UI.card({
        title: 'Nguồn video & phụ đề', icon: '🎞️',
        desc: 'Tải video cần lồng tiếng lên (hoặc nhập đường dẫn có sẵn). Phụ đề .srt là tuỳ chọn.',
        body: h('div', null,
          dubVideoDz, dubVideoInfo,
          h('div', { style: { marginTop: '12px' } }, UI.field('Hoặc đường dẫn video', dubVideoPath)),
          h('div', { style: { marginTop: '16px' } }, dubSrtDz), dubSrtInfo,
          h('div', { style: { marginTop: '12px' } }, UI.field('Hoặc đường dẫn .srt', dubSrtPath)),
          h('div', { class: 'muted', style: { fontSize: '12px', marginBottom: '10px' } },
            '💡 Bỏ trống phụ đề → tự bóc băng bằng Gemini (cần API key).'),
          dubWarnHost)
      }));

      var voiceSlot2 = h('div');
      var dubTargetLang = '';
      var dubTransEngine = 'claude';
      var transEngineSel = UI.select(null, [
        { value: 'claude', label: 'Claude CLI (subscription — khuyên dùng)' },
        { value: 'gemini', label: 'Gemini API' },
        { value: 'openai', label: 'API Trực Tiếp (OpenAI-compatible)' }
      ], dubTransEngine, function (v) { dubTransEngine = v; });
      var transEngineField = UI.field('Engine dịch', transEngineSel);
      var langSel = UI.select(null, [
        { value: '', label: 'Không dịch — giữ nguyên phụ đề' },
        { value: 'Tiếng Việt', label: 'Tiếng Việt' },
        { value: 'English', label: 'English' },
        { value: '日本語', label: '日本語' },
        { value: '中文', label: '中文' },
        { value: '한국어', label: '한국어' }
      ], '', function (v) { dubTargetLang = v; syncTransEngine(); });

      function syncTransEngine() { setEnabled(transEngineField, !!dubTargetLang); }

      var styleRow2 = UI.field('Phong cách đọc (VieNeu / nhân bản)', makeStyleSelect());
      paneDub.appendChild(UI.card({
        title: 'Giọng & ngôn ngữ', icon: '🎙️',
        desc: 'Giọng đọc dùng chung với tab "Dubbing nhanh" — chọn ở lưới bên dưới.',
        body: h('div', null,
          voiceSlot2,
          h('div', { class: 'grid-2', style: { marginTop: '14px' } },
            UI.field('Dịch phụ đề sang', langSel),
            transEngineField),
          styleRow2)
      }));

      var dubKeepOriginal = false;
      var dubOriginalVolume = 12;
      var dubFitTiming = true;
      var dubMaxSpeed = 1.6;

      var volSlider = UI.slider('Âm lượng tiếng gốc (%)', {
        min: 0, max: 50, step: 1, value: dubOriginalVolume,
        oninput: function (v) { dubOriginalVolume = v; }
      });
      var keepToggle = UI.toggle('Giữ tiếng gốc làm nền', 'Trộn tiếng gốc ở âm lượng nhỏ phía sau giọng lồng',
        dubKeepOriginal, function (v) { dubKeepOriginal = v; setEnabled(volSlider, v); });

      var speedSlider = UI.slider('Tốc độ tối đa', {
        min: 1, max: 2, step: 0.1, value: dubMaxSpeed,
        oninput: function (v) { dubMaxSpeed = v; }
      });
      var fitToggle = UI.toggle('Khớp timing từng câu', 'Tăng tốc câu đọc dài để vừa khung thời gian của phụ đề',
        dubFitTiming, function (v) { dubFitTiming = v; setEnabled(speedSlider, v); });

      setEnabled(volSlider, dubKeepOriginal);
      setEnabled(speedSlider, dubFitTiming);

      paneDub.appendChild(UI.card({
        title: 'Tùy chọn lồng tiếng', icon: '🎚️',
        body: h('div', null,
          keepToggle, volSlider,
          h('div', { class: 'mt-16' }, fitToggle), speedSlider,
          h('div', { class: 'muted', style: { fontSize: '12px' } },
            '💡 Khớp timing sẽ tăng tốc câu đọc (tối đa theo thanh trượt) để không lệch phụ đề. Câu ngắn hơn khung được giữ nguyên.'))
      }));

      var dubErrBox = h('div', { class: 'mt-8', style: { display: 'none' } });
      function showDubErr(msg) {
        dubErrBox.style.display = '';
        dubErrBox.innerHTML = '';
        dubErrBox.appendChild(redAlert(msg));
      }

      var dubResultHost = h('div');
      var dubResultEmpty = UI.empty('Video đã lồng tiếng sẽ hiện tại đây', '🎬');
      dubResultHost.appendChild(dubResultEmpty);

      var dubBtn = UI.btn('🎬 BẮT ĐẦU LỒNG TIẾNG', {
        variant: 'primary', large: true,
        onclick: function () {
          dubErrBox.style.display = 'none';
          var vp = dubVideoPath.value.trim();
          var sp = dubSrtPath.value.trim();
          if (!vp && !sp) { showDubErr('Chưa có nguồn — cần ít nhất file video hoặc file phụ đề .srt.'); return; }
          if (!selected) { showDubErr('Chưa chọn giọng đọc ở mục "Giọng & ngôn ngữ".'); return; }
          dubBtn.disabled = true;
          API.post('/api/tools/dub', {
            videoPath: vp,
            srtPath: sp,
            voice: voiceParam(),
            engine: selected.engine,
            style: vieneuStyle,
            targetLang: dubTargetLang,
            translateEngine: dubTargetLang ? dubTransEngine : '',
            keepOriginal: dubKeepOriginal,
            originalVolume: dubOriginalVolume / 100,
            fitTiming: dubFitTiming,
            maxSpeed: dubMaxSpeed
          }).then(function (job) {
            addJobCard(dubResultHost, dubResultEmpty, job, '🎬 Lồng tiếng');
            UI.toast('Đã bắt đầu lồng tiếng — theo dõi tiến độ bên dưới');
          }).catch(function (err) { showDubErr(err.message); })
            .finally(function () { dubBtn.disabled = false; });
        }
      });

      paneDub.appendChild(UI.card({ body: h('div', null, dubBtn, dubErrBox) }));
      paneDub.appendChild(UI.card({ title: 'Kết quả lồng tiếng', icon: '🎬', body: dubResultHost }));

      // =========================================================
      // TAB 3 — 🧬 CLONE VOICE (nhân bản giọng)
      // =========================================================
      var paneClone = h('div', { style: { display: 'none' } });

      var cloneFile = null;
      var cloneBlobUrl = '';
      var rec = { mr: null, stream: null, timer: null, chunks: [], sec: 0 };
      var clonesLoaded = false;

      var cloneFileInfo = h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } });
      var clonePreview = h('div', { class: 'mt-8' });

      function clearCloneBlobUrl() {
        if (!cloneBlobUrl) return;
        try { URL.revokeObjectURL(cloneBlobUrl); } catch (e) { /* bỏ qua */ }
        cloneBlobUrl = '';
      }

      function setCloneFile(f, label) {
        cloneFile = f;
        cloneFileInfo.textContent = '✓ Clip mẫu: ' + label + ' (' + UI.fmtBytes(f.size) + ')';
        clonePreview.innerHTML = '';
        clearCloneBlobUrl();
        try {
          cloneBlobUrl = URL.createObjectURL(f);
          clonePreview.appendChild(h('audio', { controls: 'controls', src: cloneBlobUrl, style: { width: '100%' } }));
        } catch (e) { /* trình duyệt không tạo được URL xem trước */ }
      }

      function resetCloneFile() {
        cloneFile = null;
        cloneFileInfo.textContent = '';
        clonePreview.innerHTML = '';
        clearCloneBlobUrl();
      }

      var cloneDz = UI.dropzone({
        hint: 'Kéo thả hoặc nhấp để chọn clip mẫu',
        sub: 'Clip 3–8 giây, giọng rõ, không nhạc nền — WAV, MP3, M4A, MP4, MOV…',
        accept: 'audio/*,video/*',
        onfiles: function (files) { setCloneFile(files[0], files[0].name); }
      });

      // --- Ghi âm trực tiếp bằng MediaRecorder
      var canRecord = !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia && window.MediaRecorder);
      var recInfo = h('span', { class: 'muted', style: { fontSize: '12px' } },
        canRecord ? 'Bấm để ghi 3–8 giây, nói một câu tự nhiên' : '');
      var recBtn = UI.btn('🎤 Ghi âm trực tiếp', { variant: 'ghost', onclick: function () { toggleRecord(); } });
      if (!canRecord) recBtn.style.display = 'none';

      function stopStream() {
        if (rec.timer) { clearInterval(rec.timer); rec.timer = null; }
        if (rec.stream) {
          try { rec.stream.getTracks().forEach(function (t) { t.stop(); }); } catch (e) { /* bỏ qua */ }
          rec.stream = null;
        }
      }

      function stopRecording() {
        if (rec.mr && rec.mr.state !== 'inactive') {
          try { rec.mr.stop(); } catch (e) { /* bỏ qua */ }
        }
      }

      function onRecStop() {
        stopStream();
        recBtn.textContent = '🎤 Ghi âm trực tiếp';
        var type = (rec.chunks[0] && rec.chunks[0].type) || 'audio/webm';
        var blob = new Blob(rec.chunks, { type: type });
        var secs = rec.sec;
        rec.mr = null;
        rec.chunks = [];
        if (!blob.size) { recInfo.textContent = ''; UI.toast('Không ghi được dữ liệu âm thanh', 'error'); return; }
        var ext = type.indexOf('mp4') >= 0 ? '.mp4' : (type.indexOf('ogg') >= 0 ? '.ogg' : '.webm');
        var name = 'ghi-am-' + Date.now() + ext;
        var f = blob;
        try { f = new File([blob], name, { type: type }); } catch (e) { /* trình duyệt cũ: dùng Blob */ }
        setCloneFile(f, name);
        recInfo.textContent = 'Đã ghi ' + secs + ' giây — nghe thử bên dưới trước khi tạo giọng';
      }

      function toggleRecord() {
        if (rec.mr && rec.mr.state === 'recording') { stopRecording(); return; }
        recBtn.disabled = true;
        navigator.mediaDevices.getUserMedia({ audio: true }).then(function (stream) {
          recBtn.disabled = false;
          rec.stream = stream;
          rec.chunks = [];
          rec.sec = 0;
          var mr;
          try { mr = new MediaRecorder(stream); }
          catch (e) { stopStream(); UI.toast('Trình duyệt không ghi âm được: ' + e.message, 'error'); return; }
          rec.mr = mr;
          mr.ondataavailable = function (e) { if (e.data && e.data.size) rec.chunks.push(e.data); };
          mr.onstop = onRecStop;
          try { mr.start(); }
          catch (e) { stopStream(); rec.mr = null; UI.toast('Không bắt đầu ghi được: ' + e.message, 'error'); return; }
          recBtn.textContent = '⏹ Dừng ghi (0s)';
          recInfo.textContent = 'Đang ghi… hãy nói một câu tự nhiên';
          rec.timer = setInterval(function () {
            rec.sec++;
            recBtn.textContent = '⏹ Dừng ghi (' + rec.sec + 's)';
            if (rec.sec >= 20) stopRecording(); // giới hạn mẫu tối đa 20 giây
          }, 1000);
        }).catch(function (err) {
          recBtn.disabled = false;
          UI.toast('Không truy cập được micro: ' + err.message, 'error');
        });
      }

      // --- Form tạo giọng
      var cloneName = UI.input({ placeholder: 'vd: Giọng của tôi' });
      var cloneGender = '';
      var cloneGenderSel = UI.select(null, [
        { value: '', label: '—' },
        { value: 'male', label: 'Nam' },
        { value: 'female', label: 'Nữ' }
      ], '', function (v) { cloneGender = v; });
      var cloneNote = UI.input({ placeholder: 'Ghi chú ngắn (tuỳ chọn)' });

      var cloneErrBox = h('div', { class: 'mt-8', style: { display: 'none' } });
      function showCloneErr(msg) {
        cloneErrBox.style.display = '';
        cloneErrBox.innerHTML = '';
        cloneErrBox.appendChild(redAlert(msg));
      }

      var cloneBtn = UI.btn('✨ Tạo giọng nhân bản', {
        variant: 'primary', large: true,
        onclick: function () {
          cloneErrBox.style.display = 'none';
          var name = cloneName.value.trim();
          if (!cloneFile) { showCloneErr('Chưa có clip mẫu — hãy tải lên hoặc ghi âm trực tiếp.'); return; }
          if (!name) { showCloneErr('Chưa nhập tên giọng.'); return; }
          cloneBtn.disabled = true;
          API.upload('/api/tools/clone-voices', [cloneFile], {
            name: name, gender: cloneGender, note: cloneNote.value.trim()
          }).then(function (cv) {
            UI.toast('Đã tạo giọng nhân bản: ' + ((cv && cv.name) ? cv.name : name));
            cloneName.value = '';
            cloneNote.value = '';
            resetCloneFile();
            recInfo.textContent = canRecord ? 'Bấm để ghi 3–8 giây, nói một câu tự nhiên' : '';
            loadClones();
            loadVoices(true);
          }).catch(function (err) { showCloneErr(err.message); })
            .finally(function () { cloneBtn.disabled = false; });
        }
      });

      paneClone.appendChild(UI.card({
        title: 'Tạo giọng mới', icon: '🧬',
        desc: 'Tải lên hoặc ghi âm một clip mẫu ngắn — hệ thống sẽ nhân bản giọng bằng VieNeu-TTS.',
        body: h('div', null,
          cloneDz, cloneFileInfo,
          h('div', { class: 'row mt-8' }, recBtn, recInfo),
          clonePreview,
          h('div', { class: 'grid-3', style: { marginTop: '14px' } },
            UI.field('Tên giọng *', cloneName),
            UI.field('Giới tính', cloneGenderSel),
            UI.field('Ghi chú', cloneNote)),
          h('div', { class: 'muted', style: { fontSize: '12px', marginBottom: '12px' } },
            '💡 Clip mẫu nên dài 3–8 giây, giọng rõ, không nhạc nền. Mẫu ngoài khoảng 2–20 giây sẽ bị từ chối.'),
          cloneBtn, cloneErrBox)
      }));

      var cloneListHost = h('div');
      paneClone.appendChild(UI.card({ title: 'Giọng đã nhân bản', icon: '📚', body: cloneListHost }));

      function cloneMeta(cv) {
        var bits = [genderLabel(cv.gender)];
        if (Number(cv.duration) > 0) bits.push('mẫu ' + UI.fmtDur(cv.duration));
        if (cv.createdAt) bits.push(UI.timeAgo(cv.createdAt));
        if (cv.note) bits.push(cv.note);
        return bits.join(' · ');
      }

      function previewClone(cv, btn, host) {
        btn.disabled = true;
        host.innerHTML = '';
        host.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tạo bản nghe thử…'));
        API.post('/api/tools/clone-voices/' + encodeURIComponent(cv.id) + '/preview', {})
          .then(function (job) {
            host.innerHTML = '';
            var card = addJobCard(host, null, job, '▶ Nghe thử — ' + (cv.name || cv.id));
            if (card) card.onFinish = function () { btn.disabled = false; };
            else btn.disabled = false;
          })
          .catch(function (err) {
            host.innerHTML = '';
            host.appendChild(redAlert('Không nghe thử được: ' + err.message));
            btn.disabled = false;
          });
      }

      function deleteClone(cv) {
        var m = UI.modal({
          title: 'Xoá giọng nhân bản?',
          body: h('div', null,
            h('p', null, 'Giọng "' + (cv.name || cv.id) + '" và file mẫu sẽ bị xoá vĩnh viễn. Không thể hoàn tác.')),
          actions: [
            UI.btn('Huỷ', { variant: 'ghost', onclick: function () { m.close(); } }),
            UI.btn('🗑 Xoá giọng', {
              variant: 'danger',
              onclick: function () {
                m.close();
                API.del('/api/tools/clone-voices/' + encodeURIComponent(cv.id)).then(function () {
                  UI.toast('Đã xoá giọng nhân bản');
                  loadClones();
                  loadVoices(true);
                }).catch(function (err) { UI.toast('Xoá thất bại: ' + err.message, 'error'); });
              }
            })
          ]
        });
      }

      function cloneRow(cv) {
        var name = cv.name || cv.id;
        var previewHost = h('div');
        var playBtn = UI.btn('▶ Nghe thử', {
          variant: 'ghost', small: true,
          onclick: function () { previewClone(cv, playBtn, previewHost); }
        });
        var delBtn = UI.btn('🗑 Xoá', {
          variant: 'danger', small: true,
          onclick: function () { deleteClone(cv); }
        });
        return h('div', { class: 'card', style: { padding: '12px', marginTop: '10px' } },
          h('div', { class: 'row-between' },
            h('div', { class: 'row' },
              avatar(name, 38),
              h('div', null,
                h('div', { class: 'row', style: { gap: '6px' } },
                  h('strong', { style: { fontSize: '13.5px' } }, name),
                  h('span', { class: 'badge badge-amber' }, 'Nhân bản')),
                h('div', { class: 'muted', style: { fontSize: '12px' } }, cloneMeta(cv)))),
            h('div', { class: 'row' }, playBtn, delBtn)),
          previewHost);
      }

      function loadClones() {
        cloneListHost.innerHTML = '';
        cloneListHost.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải danh sách giọng nhân bản…'));
        API.get('/api/tools/clone-voices').then(function (list) {
          cloneListHost.innerHTML = '';
          list = list || [];
          if (!list.length) {
            cloneListHost.appendChild(UI.empty('Chưa có giọng nhân bản nào — tạo giọng đầu tiên ở khung phía trên', '🧬'));
            return;
          }
          list.forEach(function (cv) { cloneListHost.appendChild(cloneRow(cv)); });
        }).catch(function (err) {
          cloneListHost.innerHTML = '';
          cloneListHost.appendChild(redAlert('Không tải được danh sách giọng nhân bản: ' + err.message));
        });
      }

      // =========================================================
      // TAB BAR + lắp trang
      // =========================================================
      var TABS = [
        { id: 'quick', label: '⚡ Dubbing nhanh' },
        { id: 'dub', label: '💎 Dubbing chất lượng' },
        { id: 'clone', label: '🧬 Clone voice' }
      ];
      var tabBtns = {};
      var tabRow = h('div', { class: 'row', style: { marginBottom: '14px' } });
      TABS.forEach(function (t) {
        var b = h('button', {
          class: 'btn btn-ghost', type: 'button',
          onclick: function () { switchTab(t.id); }
        }, t.label);
        tabBtns[t.id] = b;
        tabRow.appendChild(b);
      });

      function switchTab(id) {
        tab = id;
        TABS.forEach(function (t) {
          tabBtns[t.id].className = 'btn ' + (tab === t.id ? 'btn-primary' : 'btn-ghost');
        });
        paneQuick.style.display = tab === 'quick' ? '' : 'none';
        paneDub.style.display = tab === 'dub' ? '' : 'none';
        paneClone.style.display = tab === 'clone' ? '' : 'none';
        if (tab === 'quick') voiceSlot1.appendChild(voiceBlock);
        else if (tab === 'dub') { voiceSlot2.appendChild(voiceBlock); renderDubWarn(); }
        else if (!clonesLoaded) { clonesLoaded = true; loadClones(); }
      }

      el.appendChild(tabRow);
      el.appendChild(paneQuick);
      el.appendChild(paneDub);
      el.appendChild(paneClone);

      switchTab('quick');
      syncTransEngine();
      updateSelectedLine();
      loadVoices();

      // =========================================================
      // SSE + dọn dẹp
      // =========================================================
      var onJob = function (j) {
        if (!j || !jobEls[j.id]) return;
        jobEls[j.id].update(j);
      };
      var onState = function () { renderWarn(); renderDubWarn(); };
      Bus.on('job', onJob);
      Bus.on('state', onState);

      App._cleanup = function () {
        Bus.off('job', onJob);
        Bus.off('state', onState);
        if (rec.mr) {
          try {
            rec.mr.onstop = null;
            rec.mr.ondataavailable = null;
            if (rec.mr.state !== 'inactive') rec.mr.stop();
          } catch (e) { /* bỏ qua */ }
          rec.mr = null;
        }
        stopStream();
        clearCloneBlobUrl();
      };
    }
  };
})();
