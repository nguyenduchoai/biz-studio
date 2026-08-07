/* ============================================================
   Biz Studio — Trang "Phim → Kể chuyện": chia phim theo chuyển
   cảnh, AI xem khung hình viết lời, đọc giọng Việt, dựng video.
   Load sau app.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  function dataURL(rel) {
    return '/data/' + String(rel).split('/').map(encodeURIComponent).join('/');
  }

  App.pages['recap'] = {
    title: 'Phim → Kể chuyện',
    subtitle: 'Chia phim theo chuyển cảnh, AI viết lời dẫn từng cảnh, đọc giọng Việt rồi dựng lại',
    render: render
  };

  function render(root) {
    var st = {
      file: '', style: 'ke-chuyen', narration: 'ai', maxScenes: 30,
      manifestPath: '', manifest: null, voices: [], voice: '',
      keepOriginal: true, burnSub: false, handlers: []
    };
    App._cleanup = function () {
      st.handlers.forEach(function (fn) { Bus.off('job', fn); });
      st.handlers = [];
    };

    API.get('/api/tools/voices').then(function (vs) {
      st.voices = vs || [];
    }).catch(function () { /* thiếu giọng thì render vẫn tự chọn engine */ });

    var sceneHost = h('div', { class: 'mt-16' });
    var renderHost = h('div', { class: 'mt-16' });
    root.appendChild(sourceCard(st, sceneHost, renderHost));
    root.appendChild(sceneHost);
    root.appendChild(renderHost);
  }

  // ---------- bước 1: nguồn + phân tích ----------

  function sourceCard(st, sceneHost, renderHost) {
    var input = UI.input({
      value: '', placeholder: 'Đường dẫn video trong data, vd: downloads/phim.mp4',
      oninput: function (e) { st.file = e.target.value.trim(); }
    });
    var pick = h('input', { type: 'file', accept: 'video/*', style: { display: 'none' } });
    pick.onchange = function () {
      if (!pick.files || !pick.files.length) return;
      UI.toast('Đang tải video lên…');
      API.upload('/api/tools/upload', pick.files).then(function (res) {
        if (res && res.length) { st.file = res[0].path; input.value = res[0].path; }
      }).catch(function (err) { UI.toast('Tải lên thất bại: ' + err.message, 'error'); });
    };

    var styleSel = UI.select('Phong cách lời dẫn', [
      { value: 'ke-chuyen', label: 'Kể chuyện — thuật lại cho người chưa xem' },
      { value: 'review', label: 'Review — kèm nhận xét, khen chê' },
      { value: 'tom-tat', label: 'Tóm tắt nhanh — câu ngắn, nhịp dồn' }
    ], st.style, function (v) { st.style = v; });

    var narrSel = UI.select('Lời dẫn', [
      { value: 'ai', label: 'AI xem khung hình từng cảnh và viết (cần khoá Gemini)' },
      { value: 'none', label: 'Tôi tự viết — chỉ chia cảnh giúp tôi' }
    ], st.narration, function (v) { st.narration = v; });

    var maxInput = UI.input({
      value: '30', placeholder: '0 = không giới hạn',
      oninput: function (e) { st.maxScenes = Number(e.target.value) || 0; }
    });

    var goBtn = UI.btn('🎬 Phân tích phim', {
      variant: 'primary',
      onclick: function () {
        if (!st.file) { UI.toast('Chưa chọn video.', 'error'); return; }
        goBtn.disabled = true;
        API.post('/api/tools/recap/analyze', {
          path: st.file, style: st.style, narration: st.narration, maxScenes: st.maxScenes
        }).then(function (job) {
          UI.toast('Đang phân tích — phim dài sẽ lâu, theo dõi ở thanh tác vụ.');
          waitJob(job.id, function (out) {
            st.manifestPath = out;
            loadManifest(st, sceneHost, renderHost);
          }, function (err) { UI.toast('Phân tích thất bại: ' + err, 'error'); });
        }).catch(function (err) {
          UI.toast('Không chạy được: ' + err.message, 'error');
        }).finally(function () { goBtn.disabled = false; });
      }
    });

    return h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '🎞️ Phim nguồn'),
      h('div', { class: 'row' }, input,
        UI.btn('📂 Chọn video…', { variant: 'ghost', onclick: function () { pick.click(); } }), pick),
      h('div', { class: 'grid-3 mt-8' }, styleSel, narrSel,
        UI.field('Tối đa số cảnh (cảnh thừa sẽ được GỘP, có báo)', maxInput)),
      h('div', { class: 'mt-8' }, goBtn));
  }

  // ---------- bước 2: bảng cảnh + lời ----------

  function loadManifest(st, sceneHost, renderHost) {
    API.get('/api/tools/recap?path=' + encodeURIComponent(st.manifestPath)).then(function (m) {
      st.manifest = m;
      drawScenes(st, sceneHost, renderHost);
    }).catch(function (err) { UI.toast('Không đọc được kết quả: ' + err.message, 'error'); });
  }

  function drawScenes(st, sceneHost, renderHost) {
    var m = st.manifest;
    sceneHost.innerHTML = '';
    renderHost.innerHTML = '';

    var note = [];
    if (m.merged > 0) {
      note.push(h('div', { class: 'badge badge-blue', style: { marginRight: '8px' } },
        'Đã gộp ' + m.merged + ' lần cho vừa trần số cảnh'));
    }
    if (m.narrationNote) {
      note.push(h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '6px' } }, '⚠ ' + m.narrationNote));
    }

    var rows = m.scenes.map(function (sc) {
      var ta = h('textarea', { class: 'input', rows: '2', placeholder: 'Lời dẫn cho cảnh này…' });
      ta.value = sc.text || '';
      ta.oninput = function () { sc.text = ta.value; };
      return h('div', {
        style: {
          display: 'grid', gridTemplateColumns: '150px 90px minmax(0,1fr)', gap: '10px',
          alignItems: 'start', padding: '8px 0', borderBottom: '1px solid var(--border)'
        }
      },
        h('img', {
          src: dataURL(sc.frame) + '?t=' + Date.now(),
          style: { width: '150px', borderRadius: '8px', display: 'block' }
        }),
        h('div', { style: { fontSize: '12px' } },
          h('div', { style: { fontWeight: '700' } }, 'Cảnh ' + (sc.index + 1)),
          h('div', { class: 'muted' }, fmtT(sc.start) + ' → ' + fmtT(sc.end)),
          h('div', { class: 'muted' }, (sc.end - sc.start).toFixed(1) + 's')),
        ta);
    });

    var saveBtn = UI.btn('💾 Lưu lời dẫn', {
      variant: 'ghost',
      onclick: function () { saveTexts(st, function () { UI.toast('Đã lưu.'); }); }
    });

    sceneHost.appendChild(h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '📋 ' + m.scenes.length + ' cảnh — sửa lời từng cảnh rồi dựng'),
      h('div', null, note),
      h('div', { class: 'mt-8' }, rows),
      h('div', { class: 'mt-8' }, saveBtn)));

    renderHost.appendChild(renderCard(st));
  }

  function saveTexts(st, done) {
    API.post('/api/tools/recap/save', {
      path: st.manifestPath,
      scenes: st.manifest.scenes.map(function (sc) { return { index: sc.index, text: sc.text || '' }; })
    }).then(function () { done && done(); })
      .catch(function (err) { UI.toast('Lưu thất bại: ' + err.message, 'error'); });
  }

  // ---------- bước 3: dựng ----------

  function renderCard(st) {
    var voiceSel = UI.select('Giọng đọc', (st.voices.length ? st.voices : [{ id: '', name: 'Giọng mặc định của máy' }])
      .map(function (v) { return { value: v.id || '', label: v.name || v.id || 'Mặc định' }; }),
      st.voice, function (v) { st.voice = v; });

    var out = h('div', { style: { marginTop: '12px' } });
    var runBtn = UI.btn('🎬 Dựng video kể chuyện', {
      variant: 'primary',
      onclick: function () {
        runBtn.disabled = true;
        saveTexts(st, function () {
          API.post('/api/tools/recap/render', {
            path: st.manifestPath, voice: st.voice,
            keepOriginal: st.keepOriginal, burnSub: st.burnSub
          }).then(function (job) {
            UI.toast('Đang dựng — lời chưa đổi thì dùng lại giọng đã đọc, nhanh hơn nhiều.');
            waitJob(job.id, function (path) {
              out.innerHTML = '';
              out.appendChild(h('video', {
                controls: 'controls', src: dataURL(path) + '?t=' + Date.now(),
                style: { width: '100%', maxWidth: '520px', borderRadius: '12px', display: 'block' }
              }));
              out.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '6px' } },
                path + ' · phụ đề .srt nằm cùng thư mục'));
              UI.toast('Xong!');
            }, function (err) { UI.toast('Dựng thất bại: ' + err, 'error'); });
          }).catch(function (err) {
            UI.toast('Không chạy được: ' + err.message, 'error');
          }).finally(function () { runBtn.disabled = false; });
        });
      }
    });

    return h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '🎛️ Dựng'),
      h('div', { class: 'grid-2' }, voiceSel,
        h('div', null,
          UI.toggle('Giữ tiếng gốc của phim', 'Tiếng phim tự LÙI khi lời dẫn đang nói (đo được ~14dB), hết câu nâng lại.', st.keepOriginal,
            function (v) { st.keepOriginal = v; }),
          UI.toggle('Gắn phụ đề vào hình', 'Burn lời dẫn thẳng lên video.', st.burnSub,
            function (v) { st.burnSub = v; }))),
      h('div', { class: 'muted', style: { fontSize: '12.5px', margin: '8px 0' } },
        'Lời dài hơn cảnh: giọng tăng tốc tối đa 1.6x, vẫn thiếu thì cảnh tự đóng băng khung cuối kéo dài cho tròn lời — không bao giờ cắt cụt câu.'),
      h('div', { class: 'row' }, runBtn, capcutBtn(st, out)), out);
  }

  // capcutBtn — xuất dự án CapCut (.draft) để dựng tiếp trong CapCut.
  function capcutBtn(st, out) {
    var btn = UI.btn('📦 Xuất dự án CapCut', {
      variant: 'ghost',
      onclick: function () {
        btn.disabled = true;
        saveTexts(st, function () {
          API.post('/api/tools/recap/capcut', { path: st.manifestPath }).then(function (job) {
            UI.toast('Đang đóng gói dự án CapCut…');
            waitJob(job.id, function (path) {
              out.appendChild(h('div', { class: 'card mt-8', style: { padding: '12px' } },
                h('div', { style: { fontWeight: '700', marginBottom: '6px' } }, '📦 Dự án CapCut đã sẵn sàng'),
                h('a', { href: dataURL(path), download: '' }, '⬇ Tải ' + path.split('/').pop()),
                h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '6px', lineHeight: '1.6' } },
                  'Giải nén rồi đặt cả thư mục vào kho draft của CapCut. Lưu ý: draft trỏ tới file phim và ' +
                  'file giọng trên MÁY NÀY — mở trên máy khác thì CapCut sẽ hỏi tìm lại media. ' +
                  'Định dạng draft do cộng đồng mổ ngược, phiên bản CapCut quá mới có thể đổi cấu trúc.')));
              UI.toast('Xong!');
            }, function (err) { UI.toast('Xuất thất bại: ' + err, 'error'); });
          }).catch(function (err) { UI.toast('Không chạy được: ' + err.message, 'error'); })
            .finally(function () { btn.disabled = false; });
        });
      }
    });
    return btn;
  }

  // ---------- tiện ích ----------

  function fmtT(sec) {
    var s = Math.floor(sec % 60), mi = Math.floor(sec / 60);
    return mi + ':' + (s < 10 ? '0' : '') + s;
  }

  function waitJob(id, onDone, onErr) {
    var timer = setInterval(function () {
      API.get('/api/jobs/' + id).then(function (j) {
        if (j.status === 'done') { clearInterval(timer); onDone(j.output); }
        else if (j.status === 'error') { clearInterval(timer); onErr(j.error || 'không rõ nguyên nhân'); }
      }).catch(function () { clearInterval(timer); onErr('mất kết nối'); });
    }, 3000);
  }
})();
