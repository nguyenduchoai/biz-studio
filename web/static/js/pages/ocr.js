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
    subtitle: 'Bóc băng phụ đề từ hình ảnh (OCR) hoặc âm thanh (ASR) — cần Gemini API',
    render: function (el) {
      var tab = 'ocr'; // ocr | asr
      var fps = 0.5;
      var lang = 'vi';
      var jobEls = {};

      // --- Cảnh báo thiếu Gemini key
      var warnHost = h('div');
      function renderWarn() {
        warnHost.innerHTML = '';
        var st = App.state;
        if (st && st.tools && !st.tools.geminiKey) {
          warnHost.appendChild(amberWarn('Chưa cấu hình Gemini API key — OCR/ASR sẽ không chạy được.'));
        }
      }
      renderWarn();
      var onState = function () { renderWarn(); };
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
        if (tab === 'ocr') {
          paneHost.appendChild(UI.slider('Tần suất quét khung hình (FPS)', {
            min: 0.2, max: 2, step: 0.1, value: fps,
            oninput: function (v) { fps = v; }
          }));
          paneHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12px' } },
            'FPS càng cao quét càng kỹ nhưng tốn thời gian và chi phí API hơn. Mặc định 0.5 (2 giây/khung).'));
        } else {
          paneHost.appendChild(UI.select('Ngôn ngữ âm thanh', [
            { value: 'vi', label: 'Tiếng Việt' },
            { value: 'en', label: 'English' },
            { value: 'zh', label: '中文' },
            { value: 'ja', label: '日本語' }
          ], lang, function (v) { lang = v; }));
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

      var startBtn = UI.btn('▶ Bắt đầu quét', {
        variant: 'primary', large: true,
        onclick: function () {
          errBox.style.display = 'none';
          var path = pathInput.value.trim();
          if (!path) { showErr('Chưa nhập đường dẫn file Video/Audio.'); return; }
          startBtn.disabled = true;
          var req = tab === 'ocr'
            ? API.post('/api/tools/ocr', { path: path, fps: fps })
            : API.post('/api/tools/asr', { path: path, lang: lang });
          req.then(function (job) {
            addJobCard(job);
            UI.toast('Đã bắt đầu ' + (tab === 'ocr' ? 'OCR' : 'ASR'));
          }).catch(function (err) {
            showErr(err.message);
          }).finally(function () { startBtn.disabled = false; });
        }
      });

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
