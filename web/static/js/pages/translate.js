/* ============================================================
   Biz Studio — Trang Dịch thuật (SRT/TXT/văn bản)
   Load sau app.js. Tự đăng ký App.pages['translate'].
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
    if (s === 'running') return { label: 'Đang dịch', cls: 'badge badge-blue' };
    return { label: 'Chờ xử lý', cls: 'badge badge-gray' };
  }

  function isRel(p) { return !!p && p.charAt(0) !== '/'; }
  function dataUrl(p) { return '/data/' + p.split('/').map(encodeURIComponent).join('/'); }
  function fileName(p) { return (p || '').split('/').pop(); }
  function dirOf(p) {
    var i = (p || '').lastIndexOf('/');
    return i > 0 ? p.slice(0, i) : '';
  }

  function redAlert(msg) {
    return h('div', {
      class: 'text-red',
      style: { background: 'rgba(239,68,68,.08)', border: '1px solid var(--red)', borderRadius: '10px', padding: '10px 12px', fontSize: '13px', fontWeight: '600' }
    }, '⚠ ' + msg);
  }

  var MODES = [
    { id: 'phim', label: '🎬 Phim / Vlog' },
    { id: 'sub', label: '📝 Dịch SUB' },
    { id: 'truyen', label: '📖 Dịch truyện' },
    { id: 'khoahoc', label: '🔬 Khoa học' }
  ];

  // Kết quả job dịch file: link tải + mở thư mục đầu ra
  function fileResult(j) {
    var host = h('div');
    if (!j.output) {
      host.appendChild(h('span', { class: 'muted' }, 'Hoàn thành (không có file đầu ra)'));
      return host;
    }
    var dir, row = h('div', { class: 'row' });
    if (isRel(j.output)) {
      dir = 'data/' + dirOf(j.output);
      row.appendChild(h('a', { class: 'btn btn-ghost btn-sm', href: dataUrl(j.output), download: fileName(j.output) },
        '⬇ Tải ' + fileName(j.output)));
    } else {
      dir = dirOf(j.output);
      row.appendChild(h('code', { style: { fontSize: '12px', wordBreak: 'break-all' } }, j.output));
      row.appendChild(UI.btn('📋 Copy đường dẫn', { variant: 'ghost', small: true, onclick: function () { copyText(j.output); } }));
    }
    row.appendChild(UI.btn('📂 Mở thư mục đầu ra', {
      variant: 'ghost', small: true,
      onclick: function () { copyText(dir, 'Đã copy đường dẫn thư mục: ' + dir + ' — mở Finder, nhấn Cmd+Shift+G và dán'); }
    }));
    host.appendChild(row);
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
        h('div', { class: 'row' }, h('strong', null, '🌐 Dịch thuật'), badge),
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
        outHost.appendChild(fileResult(j));
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

  App.pages.translate = {
    title: 'Dịch thuật',
    subtitle: 'Dịch phụ đề SRT, file TXT hoặc văn bản dán trực tiếp — Claude CLI / Gemini',
    render: function (el) {
      var mode = 'phim';
      var engine = 'claude';
      var targetLang = 'Tiếng Việt';
      var jobEls = {};

      // --- Nguồn nội dung: upload trực tiếp, đường dẫn, hoặc dán văn bản
      var pathInput = UI.input({
        placeholder: 'vd: uploads/phim.srt (tương đối trong data/) hoặc /Users/ban/Documents/truyen.txt'
      });
      var upInfo = h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } });
      var dz = UI.dropzone({
        hint: 'Kéo thả hoặc nhấp để thêm File (SRT/TXT)',
        sub: 'Hỗ trợ: TXT, SRT, SUB, ASS, VTT — file được tải vào data/uploads/',
        accept: '.txt,.srt,.sub,.ass,.vtt',
        onfiles: function (files) {
          upInfo.textContent = '⏳ Đang tải lên ' + files[0].name + '…';
          API.upload('/api/tools/upload', [files[0]]).then(function (list) {
            var f = list && list[0];
            if (!f) throw new Error('server không trả về thông tin file');
            pathInput.value = f.path;
            upInfo.textContent = '✓ Đã tải lên: ' + f.name + ' (' + UI.fmtBytes(f.size) + ')';
            UI.toast('Đã tải file lên — sẵn sàng dịch');
          }).catch(function (e) {
            upInfo.textContent = '';
            UI.toast('Tải lên thất bại: ' + e.message, 'error');
          });
        }
      });

      var textTa = UI.textarea({
        placeholder: 'Dán văn bản hoặc nội dung SRT cần dịch vào đây…\n(Nếu ô này có nội dung sẽ ưu tiên dịch văn bản thay vì file)',
        rows: 6
      });

      el.appendChild(UI.card({
        title: 'Nguồn nội dung', icon: '📄',
        desc: 'Tải file SRT/TXT lên trực tiếp, nhập đường dẫn file có sẵn, HOẶC dán văn bản. Văn bản dán được ưu tiên.',
        body: h('div', null, dz, upInfo,
          h('div', { style: { marginTop: '12px' } }, UI.field('Hoặc đường dẫn file SRT / TXT', pathInput)),
          UI.field('Hoặc dán văn bản', textTa))
      }));

      // --- Chế độ dịch (4 tab)
      var modeRow = h('div', { class: 'row', style: { marginBottom: '14px' } });
      function renderModes() {
        modeRow.innerHTML = '';
        MODES.forEach(function (m) {
          modeRow.appendChild(h('button', {
            class: 'btn ' + (mode === m.id ? 'btn-primary' : 'btn-ghost'),
            type: 'button',
            onclick: function () { if (mode !== m.id) { mode = m.id; renderModes(); } }
          }, m.label));
        });
      }
      renderModes();

      // --- Cấu hình engine / ngôn ngữ / slider trang trí
      var engineSel = UI.select(null, [
        { value: 'claude', label: 'Claude CLI (subscription — khuyên dùng)' },
        { value: 'gemini', label: 'Gemini API' },
        { value: 'openai', label: 'API Trực Tiếp (OpenAI-compatible)' }
      ], engine, function (v) { engine = v; });

      var langSel = UI.select(null, [
        'Tiếng Việt', 'English', '日本語', '中文', '한국어'
      ], targetLang, function (v) { targetLang = v; });

      var lineSlider = UI.slider(null, { min: 20, max: 80, step: 1, value: 42 });
      var creativeSlider = UI.slider(null, { min: 0, max: 100, step: 5, value: 50 });

      el.appendChild(UI.card({
        title: 'Chế độ & cấu hình dịch', icon: '⚙️',
        body: h('div', null,
          modeRow,
          h('div', { class: 'grid-2' },
            UI.field('Engine dịch', engineSel),
            UI.field('Ngôn ngữ đích', langSel)),
          h('div', { class: 'grid-2' },
            UI.field('Độ dài dòng (ký tự)', lineSlider),
            UI.field('Sáng tạo', creativeSlider)),
          h('div', { class: 'muted', style: { fontSize: '12px' } },
            '💡 Hai thanh trượt "Độ dài dòng" và "Sáng tạo" là tuỳ chọn trang trí — phiên bản hiện tại chưa áp dụng.'))
      }));

      // --- CTA + kết quả
      var errBox = h('div', { class: 'mt-8', style: { display: 'none' } });
      function showErr(msg) {
        errBox.style.display = '';
        errBox.innerHTML = '';
        errBox.appendChild(redAlert(msg));
      }

      var resultHost = h('div');
      var resultEmpty = UI.empty('Kết quả dịch sẽ hiện tại đây', '🌐');
      resultHost.appendChild(resultEmpty);

      function clearEmpty() { if (resultEmpty.parentNode) resultEmpty.remove(); }

      function addJobCard(j) {
        if (jobEls[j.id]) { jobEls[j.id].update(j); return; }
        clearEmpty();
        var c = jobCard(j);
        jobEls[j.id] = c;
        if (resultHost.firstChild) resultHost.insertBefore(c, resultHost.firstChild);
        else resultHost.appendChild(c);
      }

      function showTextResult(text) {
        clearEmpty();
        var ta = UI.textarea({ rows: 10 });
        ta.value = text;
        var card = h('div', { class: 'card' },
          h('div', { class: 'row-between' },
            h('div', { class: 'row' }, h('strong', null, '🌐 Bản dịch'), h('span', { class: 'badge badge-green' }, 'Hoàn thành')),
            UI.btn('📋 Copy bản dịch', { variant: 'ghost', small: true, onclick: function () { copyText(ta.value, 'Đã copy bản dịch'); } })),
          h('div', { class: 'mt-8' }, ta));
        if (resultHost.firstChild) resultHost.insertBefore(card, resultHost.firstChild);
        else resultHost.appendChild(card);
      }

      var startBtn = UI.btn('▶ Bắt đầu dịch', {
        variant: 'primary', large: true,
        onclick: function () {
          errBox.style.display = 'none';
          var text = textTa.value.trim();
          var path = pathInput.value.trim();
          if (!text && !path) {
            showErr('Chưa có nội dung — nhập đường dẫn file hoặc dán văn bản cần dịch.');
            return;
          }
          var body = text
            ? { text: text, mode: mode, engine: engine, targetLang: targetLang }
            : { path: path, mode: mode, engine: engine, targetLang: targetLang };
          startBtn.disabled = true;
          API.post('/api/tools/translate', body)
            .then(function (resp) {
              if (resp && typeof resp.text === 'string') {
                showTextResult(resp.text);
                UI.toast('Dịch xong');
              } else if (resp && resp.id) {
                addJobCard(resp);
                UI.toast('Đã bắt đầu dịch — theo dõi tiến độ bên dưới');
              } else {
                showErr('Phản hồi máy chủ không hợp lệ.');
              }
            })
            .catch(function (err) { showErr(err.message); })
            .finally(function () { startBtn.disabled = false; });
        }
      });

      el.appendChild(UI.card({ body: h('div', null, startBtn, errBox) }));
      el.appendChild(UI.card({ title: 'Kết quả', icon: '📄', body: resultHost }));

      var onJob = function (j) {
        if (!j || !jobEls[j.id]) return;
        jobEls[j.id].update(j);
      };
      Bus.on('job', onJob);
      App._cleanup = function () { Bus.off('job', onJob); };
    }
  };
})();
