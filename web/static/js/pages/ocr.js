/* ============================================================
   Biz Studio — Trang OCR / ASR (bóc băng phụ đề)
   Load sau app.js. Tự đăng ký App.pages['ocr'].
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
    if (s === 'running') return { label: 'Đang quét', cls: 'badge badge-blue' };
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
      style: { background: 'rgba(245,158,11,.09)', border: '1px solid var(--amber)', borderRadius: '10px', padding: '10px 12px', marginBottom: '16px' }
    },
      h('div', { class: 'row' },
        h('span', null, '⚠️'),
        h('span', { class: 'text-amber', style: { fontWeight: '600', flex: '1', minWidth: '160px' } }, msg),
        h('a', { href: '#/settings', class: 'btn btn-ghost btn-sm' }, 'Mở Cấu hình & API →')));
  }

  // Kết quả .srt: link tải + preview nội dung + copy
  function srtResult(j) {
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
    var name = fileName(j.output);
    var ta = UI.textarea({ rows: 10, placeholder: 'Đang tải nội dung phụ đề…' });
    ta.readOnly = true;
    host.appendChild(h('div', { class: 'row', style: { marginBottom: '8px' } },
      h('a', { class: 'btn btn-ghost btn-sm', href: url, download: name }, '⬇ Tải ' + name),
      UI.btn('📋 Copy nội dung', {
        variant: 'ghost', small: true,
        onclick: function () {
          if (!ta.value) { UI.toast('Chưa có nội dung để copy', 'error'); return; }
          copyText(ta.value, 'Đã copy nội dung phụ đề');
        }
      })));
    host.appendChild(ta);
    fetch(url).then(function (res) {
      if (!res.ok) throw new Error('HTTP ' + res.status);
      return res.text();
    }).then(function (txt) {
      ta.value = txt;
    }).catch(function (err) {
      ta.placeholder = 'Không đọc được nội dung: ' + err.message;
    });
    return host;
  }

  // Các file kèm theo do faster-whisper sinh ra — đọc từ detail của job:
  // "12 đoạn · 340 từ có mốc · transcript: <path> · karaoke: <path>"
  var EXTRA_LABELS = {
    transcript: '🧩 Transcript mốc từng từ (.words.json)',
    karaoke: '✨ Phụ đề karaoke (.ass)'
  };

  function extraFiles(detail) {
    var out = [];
    String(detail || '').split('·').forEach(function (part) {
      var m = /^\s*(transcript|karaoke)\s*:\s*(\S.*?)\s*$/i.exec(part);
      if (m) out.push({ kind: m[1].toLowerCase(), path: m[2] });
    });
    return out;
  }

  function extraFileRow(item) {
    var row = h('div', { class: 'row', style: { gap: '8px', marginTop: '8px' } },
      h('span', { style: { fontSize: '12.5px', fontWeight: '600', flex: 'none' } },
        EXTRA_LABELS[item.kind] || item.kind));
    if (isRel(item.path)) {
      row.appendChild(h('a', {
        class: 'btn btn-ghost btn-sm', href: dataUrl(item.path), download: fileName(item.path)
      }, '⬇ Tải'));
    }
    row.appendChild(h('code', { style: { fontSize: '11.5px', wordBreak: 'break-all', minWidth: '0' } }, item.path));
    row.appendChild(UI.btn('📋 Copy đường dẫn', {
      variant: 'ghost', small: true,
      onclick: function () { copyText(item.path, 'Đã copy đường dẫn'); }
    }));
    return row;
  }

  // Khối "file kèm theo" hiện dưới nội dung .srt; null nếu job không có file nào.
  function extraResult(j) {
    var items = extraFiles(j && j.detail);
    if (!items.length) return null;
    var box = h('div', {
      style: { marginTop: '12px', borderTop: '1px solid var(--border)', paddingTop: '10px' }
    }, h('div', { class: 'muted', style: { fontSize: '12px' } },
      'File kèm theo — transcript dùng cho “Cắt khoảng lặng an toàn” bên Studio Editor, ' +
      'file .ass dùng để burn phụ đề karaoke vào video.'));
    items.forEach(function (it) { box.appendChild(extraFileRow(it)); });
    return box;
  }

  function jobCard(job, title) {
    var badge = h('span', { class: 'badge badge-gray' }, '');
    var pctEl = h('span', { class: 'muted', style: { fontWeight: '600', flex: 'none' } }, '0%');
    var detailEl = h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '4px', wordBreak: 'break-all' } }, '');
    var bar = UI.progress(0);
    var outHost = h('div', { class: 'mt-8' });
    var card = h('div', { class: 'card' },
      h('div', { class: 'row-between' },
        h('div', { class: 'row' }, h('strong', null, title), badge),
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
        outHost.appendChild(srtResult(j));
        var extra = extraResult(j);
        if (extra) outHost.appendChild(extra);
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

  App.pages.ocr = {
    title: 'OCR / ASR',
    subtitle: 'Bóc băng phụ đề từ hình ảnh (OCR — cần Gemini) hoặc âm thanh (ASR — chạy offline nếu đã cài faster-whisper)',
    render: function (el) {
      var tab = 'ocr'; // ocr | asr
      var fps = 0.5;
      var lang = 'vi';
      var engine = 'auto';   // auto | whisper | gemini
      var karaoke = false;
      var jobEls = {};

      // Phần tử của tab ASR — null khi đang ở tab OCR (syncEngine tự bỏ qua).
      var asrStatusEl = null, karaokeRow = null, karaokeNoteEl = null, asrBlockEl = null;
      var startBtn = null;

      // faster-whisper đã cài chưa (theo /api/state, poll mỗi 5s)
      function whisperReady() {
        var st = App.state;
        return !!(st && st.tools && st.tools.whisper);
      }

      // Engine thực sự sẽ chạy khi người dùng để "Tự động".
      function effectiveEngine() {
        if (engine === 'whisper' || engine === 'gemini') return engine;
        return whisperReady() ? 'whisper' : 'gemini';
      }

      // Có cần Gemini API key cho thao tác đang chọn không?
      function needGeminiKey() {
        if (tab === 'ocr') return true;             // OCR luôn đọc chữ bằng Gemini
        return effectiveEngine() === 'gemini';
      }

      // --- Cảnh báo thiếu Gemini key (chỉ hiện khi thao tác đang chọn thực sự cần)
      var warnHost = h('div');
      function renderWarn() {
        warnHost.innerHTML = '';
        var st = App.state;
        if (!st || !st.tools || st.tools.geminiKey) return;
        if (!needGeminiKey()) return;
        warnHost.appendChild(amberWarn(tab === 'ocr'
          ? 'Chưa cấu hình Gemini API key — OCR (đọc chữ trên khung hình) sẽ không chạy được.'
          : 'Chưa cấu hình Gemini API key — bóc băng bằng Gemini sẽ không chạy được. ' +
            'Cài faster-whisper để bóc băng ngay trên máy, không cần API key.'));
      }
      renderWarn();
      var onState = function () { syncEngine(); };
      Bus.on('state', onState);
      el.appendChild(warnHost);

      // --- Nguồn media: upload trực tiếp hoặc đường dẫn cục bộ
      var pathInput = UI.input({
        placeholder: 'vd: uploads/video.mp4 (tương đối trong data/) hoặc /Users/ban/Movies/clip.mov'
      });
      var upInfo = h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } });
      var dz = UI.dropzone({
        hint: 'Kéo thả hoặc nhấp để thêm Video/Audio',
        sub: 'Hỗ trợ: MP4, MOV, MKV, MP3, WAV, AAC, FLAC — file được tải vào data/uploads/',
        accept: 'video/*,audio/*,.mp4,.mov,.mkv,.mp3,.wav,.aac,.flac,.m4a',
        onfiles: function (files) {
          upInfo.textContent = '⏳ Đang tải lên ' + files[0].name + '…';
          API.upload('/api/tools/upload', [files[0]]).then(function (list) {
            var f = list && list[0];
            if (!f) throw new Error('server không trả về thông tin file');
            pathInput.value = f.path;
            upInfo.textContent = '✓ Đã tải lên: ' + f.name + ' (' + UI.fmtBytes(f.size) + ')';
            UI.toast('Đã tải file lên — sẵn sàng quét');
          }).catch(function (e) {
            upInfo.textContent = '';
            UI.toast('Tải lên thất bại: ' + e.message, 'error');
          });
        }
      });

      el.appendChild(UI.card({
        title: 'Nguồn Video / Audio', icon: '🎞️',
        desc: 'Tải file lên trực tiếp, hoặc nhập đường dẫn file có sẵn trên máy (tương đối trong data/ hoặc tuyệt đối).',
        body: h('div', null, dz, upInfo,
          h('div', { style: { marginTop: '12px' } }, UI.field('Hoặc đường dẫn file trên máy', pathInput)))
      }));

      // --- Tabs OCR / ASR
      var tabRow = h('div', { class: 'row', style: { marginBottom: '14px' } });
      var paneHost = h('div');

      function renderTabs() {
        tabRow.innerHTML = '';
        [
          { id: 'ocr', label: '🖼 Bóc băng Hình ảnh (OCR)' },
          { id: 'asr', label: '🎧 Bóc băng Âm thanh (ASR)' }
        ].forEach(function (t) {
          tabRow.appendChild(h('button', {
            class: 'btn ' + (tab === t.id ? 'btn-primary' : 'btn-ghost'),
            type: 'button',
            onclick: function () { if (tab !== t.id) { tab = t.id; renderTabs(); renderPane(); } }
          }, t.label));
        });
      }

      function renderPane() {
        paneHost.innerHTML = '';
        asrStatusEl = karaokeRow = karaokeNoteEl = asrBlockEl = null;

        if (tab === 'ocr') {
          paneHost.appendChild(UI.slider('Tần suất quét khung hình (FPS)', {
            min: 0.2, max: 2, step: 0.1, value: fps,
            oninput: function (v) { fps = v; }
          }));
          paneHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12px' } },
            'FPS càng cao quét càng kỹ nhưng tốn thời gian và chi phí API hơn. Mặc định 0.5 (2 giây/khung).'));
          syncEngine();
          return;
        }

        // --- Tab ASR: chọn engine bóc băng
        paneHost.appendChild(UI.select('Engine bóc băng', [
          { value: 'auto', label: 'Tự động (khuyên dùng)' },
          { value: 'whisper', label: 'faster-whisper — offline, mốc từng từ' },
          { value: 'gemini', label: 'Gemini API' }
        ], engine, function (v) { engine = v; syncEngine(); }));

        asrStatusEl = h('div', {
          class: 'row', style: { gap: '7px', fontSize: '12.5px', marginTop: '-8px', marginBottom: '14px' }
        });
        paneHost.appendChild(asrStatusEl);

        asrBlockEl = h('div', { style: { display: 'none', marginBottom: '14px' } });
        paneHost.appendChild(asrBlockEl);

        paneHost.appendChild(UI.select('Ngôn ngữ âm thanh', [
          { value: 'vi', label: 'Tiếng Việt' },
          { value: 'en', label: 'English' },
          { value: 'zh', label: '中文' },
          { value: 'ja', label: '日本語' }
        ], lang, function (v) { lang = v; }));

        karaokeRow = UI.toggle('Xuất phụ đề karaoke (.ass)', null, karaoke, function (v) { karaoke = v; });
        paneHost.appendChild(karaokeRow);
        karaokeNoteEl = h('div', { class: 'muted', style: { fontSize: '12px' } });
        paneHost.appendChild(karaokeNoteEl);

        syncEngine();
      }

      // Cập nhật trạng thái phụ thuộc engine + tình trạng cài faster-whisper.
      // Gọi được cả khi đang ở tab OCR (mọi phần tử ASR đều null → chỉ chạy phần chung).
      function syncEngine() {
        renderWarn();

        var ready = whisperReady();
        var blocked = (tab === 'asr' && engine === 'whisper' && !ready);
        if (startBtn) startBtn.disabled = blocked;

        if (asrStatusEl) {
          asrStatusEl.innerHTML = '';
          var cls = ready ? 'text-green' : 'muted';
          asrStatusEl.appendChild(h('span', { class: cls, style: { flex: 'none' } }, '●'));
          asrStatusEl.appendChild(h('span', { class: cls }, ready
            ? 'faster-whisper đã sẵn sàng — bóc băng ngay trên máy, có mốc từng từ, không tốn API'
            : 'faster-whisper chưa cài — chạy ./scripts/setup-whisper.sh trong thư mục app'));
        }

        if (asrBlockEl) {
          asrBlockEl.innerHTML = '';
          asrBlockEl.style.display = blocked ? '' : 'none';
          if (blocked) {
            asrBlockEl.appendChild(redAlert(
              'Bạn đang chọn faster-whisper nhưng máy chưa cài. Chạy ./scripts/setup-whisper.sh ' +
              'rồi thử lại, hoặc chọn “Tự động” / “Gemini API” để bóc băng ngay.'));
          }
        }

        // Karaoke cần mốc từng từ → chỉ faster-whisper làm được.
        var canKaraoke = (effectiveEngine() === 'whisper' && ready);
        if (karaokeRow) {
          karaokeRow.input.disabled = !canKaraoke;
          karaokeRow.style.opacity = canKaraoke ? '' : '.55';
          karaokeRow.style.cursor = canKaraoke ? '' : 'not-allowed';
          if (!canKaraoke && karaoke) {
            karaoke = false;
            karaokeRow.input.checked = false;
          }
        }
        if (karaokeNoteEl) {
          karaokeNoteEl.textContent = canKaraoke
            ? 'Tô sáng từng từ theo giọng nói, dùng để burn vào video.'
            : 'Cần faster-whisper (mốc từng từ) — Gemini chỉ cho phụ đề theo câu nên không làm karaoke được.';
        }
      }

      renderTabs();
      renderPane();

      // --- CTA + kết quả
      var errBox = h('div', { class: 'mt-8', style: { display: 'none' } });
      function showErr(msg) {
        errBox.style.display = '';
        errBox.innerHTML = '';
        errBox.appendChild(redAlert(msg));
      }

      var resultHost = h('div');
      var resultEmpty = UI.empty('Kết quả phụ đề (.srt) sẽ hiện tại đây', '📝');
      resultHost.appendChild(resultEmpty);

      function addJobCard(j) {
        if (jobEls[j.id]) { jobEls[j.id].update(j); return; }
        if (resultEmpty.parentNode) resultEmpty.remove();
        var c = jobCard(j, j.kind === 'asr' ? '🎧 ASR' : '🖼 OCR');
        jobEls[j.id] = c;
        if (resultHost.firstChild) resultHost.insertBefore(c, resultHost.firstChild);
        else resultHost.appendChild(c);
      }

      startBtn = UI.btn('▶ Bắt đầu quét', {
        variant: 'primary', large: true,
        onclick: function () {
          errBox.style.display = 'none';
          var path = pathInput.value.trim();
          if (!path) { showErr('Chưa nhập đường dẫn file Video/Audio.'); return; }
          startBtn.disabled = true;
          var req = tab === 'ocr'
            ? API.post('/api/tools/ocr', { path: path, fps: fps })
            : API.post('/api/tools/asr', {
                path: path, lang: lang, engine: engine, karaoke: !!karaoke
              });
          req.then(function (job) {
            addJobCard(job);
            UI.toast('Đã bắt đầu ' + (tab === 'ocr' ? 'OCR' : 'ASR'));
          }).catch(function (err) {
            showErr(err.message);
          }).finally(function () { syncEngine(); });
        }
      });
      syncEngine();

      el.appendChild(UI.card({
        title: 'Chế độ bóc băng', icon: '⚙️',
        body: h('div', null, tabRow, paneHost, h('div', { class: 'mt-16' }, startBtn), errBox)
      }));

      el.appendChild(UI.card({ title: 'Kết quả', icon: '📄', body: resultHost }));

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
