/* ============================================================
   Biz Studio — Trang "Avatar nói": ảnh + giọng → video người nói.
   Nối thẳng vào engine giọng sẵn có, nên gõ chữ là ra video.
   Load sau app.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  function dataURL(rel) {
    return '/data/' + String(rel).split('/').map(encodeURIComponent).join('/');
  }

  App.pages['avatar'] = {
    title: 'Avatar nói',
    subtitle: 'Một tấm ảnh + một file giọng → video nhân vật nói, chạy trên máy của bạn',
    render: render
  };

  function render(root) {
    var st = {
      info: null, voices: [], projects: [],
      imagePath: '', audioPath: '', prompt: '',
      text: '', voice: '', projectId: '', handlers: []
    };
    App._cleanup = function () {
      st.handlers.forEach(function (fn) { Bus.off('job', fn); });
      st.handlers = [];
    };

    root.appendChild(h('div', { class: 'card' }, UI.empty('Đang kiểm tra engine avatar…', '🗣️')));
    Promise.all([
      API.get('/api/tools/avatar'),
      API.get('/api/tools/voices').catch(function () { return []; }),
      API.get('/api/projects').catch(function () { return []; })
    ]).then(function (res) {
      st.info = res[0];
      st.voices = res[1] || [];
      st.projects = res[2] || [];
      if (st.voices.length) st.voice = st.voices[0].id || st.voices[0].name || '';
      root.innerHTML = '';
      build(root, st);
    }).catch(function (err) {
      root.innerHTML = '';
      root.appendChild(h('div', { class: 'card' },
        UI.empty('Không kiểm tra được engine: ' + err.message, '⚠️')));
    });
  }

  function build(root, st) {
    root.appendChild(engineCard(st));
    root.appendChild(imageCard(st));
    root.appendChild(voiceCard(st));
    root.appendChild(runCard(st));
  }

  // ---------- trạng thái engine ----------

  function engineCard(st) {
    var i = st.info;
    var body = [h('div', {
      class: i.ready ? 'muted' : '',
      style: { fontSize: '13px', lineHeight: '1.6', color: i.ready ? '' : 'var(--red)' }
    }, i.detail)];

    if (!i.ready) {
      body.push(h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '10px', lineHeight: '1.7' } },
        h('strong', null, 'Vì sao cần máy riêng: '),
        'LongCat-Video-Avatar là model 13,6 tỉ tham số, bắt buộc GPU NVIDIA — không có bản cho ' +
        'Apple Silicon hay CPU. Máy Mac/máy thường vẫn dùng được bằng cách đẩy việc sang một máy GPU:',
        h('div', { style: { marginTop: '8px', fontFamily: 'ui-monospace, Menlo, monospace', fontSize: '12px' } },
          '1. Trên máy GPU:  ./scripts/setup-longcat.sh', h('br'),
          '2. Rồi bật xưởng: python3 scripts/longcat-worker.py --repo … --checkpoint … --port 7070', h('br'),
          '3. Ở đây chọn chế độ "remote" và điền http://<ip-máy-GPU>:7070')));
      body.push(h('div', { style: { marginTop: '10px' } },
        UI.btn('⚙️ Mở Cấu hình & API', {
          variant: 'primary', onclick: function () { location.hash = '#/settings'; }
        })));
    }
    var label = i.mode === 'remote' ? 'máy GPU từ xa' : (i.mode === 'local' ? 'máy này' : 'chưa bật');
    return h('div', { class: 'card' },
      h('div', { class: 'card-title' },
        (i.ready ? '✅ ' : '⚠️ ') + 'Engine avatar — ' + label),
      body);
  }

  // ---------- ảnh nhân vật ----------

  function imageCard(st) {
    var preview = h('div', { style: { marginTop: '10px' } });
    var input = UI.input({
      value: '', placeholder: 'Đường dẫn ảnh trong data, vd: avatar/face.png',
      oninput: function (e) { st.imagePath = e.target.value.trim(); showImg(); }
    });
    function showImg() {
      preview.innerHTML = '';
      if (!st.imagePath) return;
      preview.appendChild(h('img', {
        src: dataURL(st.imagePath) + '?t=' + Date.now(),
        style: { maxWidth: '220px', borderRadius: '12px', border: '1px solid var(--border)', display: 'block' }
      }));
    }
    var pick = h('input', { type: 'file', accept: 'image/*', style: { display: 'none' } });
    pick.onchange = function () {
      if (!pick.files || !pick.files.length) return;
      UI.toast('Đang tải ảnh lên…');
      API.upload('/api/tools/upload', pick.files).then(function (res) {
        if (res && res.length) {
          st.imagePath = res[0].path; input.value = res[0].path; showImg();
          UI.toast('Đã tải lên: ' + res[0].name);
        }
      }).catch(function (err) { UI.toast('Tải lên thất bại: ' + err.message, 'error'); });
    };

    return h('div', { class: 'card mt-16' },
      h('div', { class: 'card-title' }, '🖼 Ảnh nhân vật'),
      h('div', { class: 'muted', style: { fontSize: '12.5px', marginBottom: '10px' } },
        'Ảnh chân dung rõ mặt, nhìn thẳng ống kính cho kết quả tốt nhất.'),
      h('div', { class: 'row' }, input,
        UI.btn('📂 Chọn ảnh…', { variant: 'ghost', onclick: function () { pick.click(); } }), pick),
      preview);
  }

  // ---------- giọng đọc ----------

  function voiceCard(st) {
    var audioPathInput = UI.input({
      value: '', placeholder: 'Đường dẫn file giọng, vd: avatar/loi.wav',
      oninput: function (e) { st.audioPath = e.target.value.trim(); }
    });
    var player = h('div', { style: { marginTop: '10px' } });
    function showAudio() {
      player.innerHTML = '';
      if (!st.audioPath) return;
      player.appendChild(h('audio', {
        controls: 'controls', src: dataURL(st.audioPath) + '?t=' + Date.now(),
        style: { width: '100%', maxWidth: '420px' }
      }));
    }

    var pick = h('input', { type: 'file', accept: 'audio/*', style: { display: 'none' } });
    pick.onchange = function () {
      if (!pick.files || !pick.files.length) return;
      UI.toast('Đang tải file giọng lên…');
      API.upload('/api/tools/upload', pick.files).then(function (res) {
        if (res && res.length) {
          st.audioPath = res[0].path; audioPathInput.value = res[0].path; showAudio();
        }
      }).catch(function (err) { UI.toast('Tải lên thất bại: ' + err.message, 'error'); });
    };

    // Nhánh 2: gõ chữ, để engine giọng của máy đọc — đây là chỗ nối hai module
    // thành một dây chuyền, không phải đi tìm file giọng ở đâu khác.
    var textBox = h('textarea', {
      class: 'input', rows: '3', placeholder: 'Hoặc gõ lời cần nói, hệ thống tự đọc bằng giọng Việt trên máy…'
    });
    textBox.oninput = function () { st.text = textBox.value; };
    var voiceSel = UI.select(null,
      (st.voices.length ? st.voices : [{ id: '', name: 'Chưa tải được danh sách giọng' }]).map(function (v) {
        return { value: v.id || v.name, label: (v.name || v.id) + (v.engine ? ' · ' + v.engine : '') };
      }), st.voice, function (v) { st.voice = v; });

    var readBtn = UI.btn('🎙 Đọc thành file giọng', {
      variant: 'ghost',
      onclick: function () {
        if (!st.text.trim()) { UI.toast('Chưa nhập lời cần đọc.', 'error'); return; }
        readBtn.disabled = true;
        API.post('/api/tools/avatar/voice', { text: st.text, voice: st.voice })
          .then(function (job) {
            UI.toast('Đang đọc…');
            waitJob(job.id, function (out) {
              st.audioPath = out; audioPathInput.value = out; showAudio();
              UI.toast('Đã có file giọng — bấm Dựng video ở dưới.');
            }, function (err) { UI.toast('Đọc thất bại: ' + err, 'error'); });
          })
          .catch(function (err) { UI.toast('Không chạy được: ' + err.message, 'error'); })
          .finally(function () { readBtn.disabled = false; });
      }
    });

    return h('div', { class: 'card mt-16' },
      h('div', { class: 'card-title' }, '🎙 Giọng nói'),
      h('div', { class: 'muted', style: { fontSize: '12.5px', marginBottom: '10px' } },
        'Dùng file giọng có sẵn, hoặc gõ chữ để máy tự đọc bằng giọng Việt (kể cả giọng bạn đã nhân bản).'),
      h('div', { class: 'row' }, audioPathInput,
        UI.btn('📂 Chọn file…', { variant: 'ghost', onclick: function () { pick.click(); } }), pick),
      player,
      h('div', { style: { borderTop: '1px solid var(--border)', margin: '14px 0' } }),
      textBox,
      h('div', { class: 'row mt-8' }, voiceSel, readBtn));
  }

  // ---------- dựng video ----------

  function runCard(st) {
    var promptBox = UI.input({
      value: '', placeholder: 'Mô tả bối cảnh cho model (tuỳ chọn) — để trống dùng mô tả mặc định',
      oninput: function (e) { st.prompt = e.target.value; }
    });
    var projSel = UI.select(null,
      [{ value: '', label: 'Không — lưu vào data/avatar' }].concat(
        st.projects.map(function (p) { return { value: p.id, label: p.name }; })),
      '', function (v) { st.projectId = v; });

    var out = h('div', { style: { marginTop: '12px' } });
    var runBtn = UI.btn('🗣️ Dựng video người nói', {
      variant: 'primary',
      onclick: function () {
        if (!st.info.ready) { UI.toast('Engine avatar chưa sẵn sàng.', 'error'); return; }
        if (!st.imagePath) { UI.toast('Chưa chọn ảnh nhân vật.', 'error'); return; }
        if (!st.audioPath) { UI.toast('Chưa có file giọng.', 'error'); return; }
        runBtn.disabled = true;
        API.post('/api/tools/avatar', {
          imagePath: st.imagePath, audioPath: st.audioPath,
          prompt: st.prompt, projectId: st.projectId
        }).then(function (job) {
          UI.toast('Đã bắt đầu dựng — model chạy vài phút.');
          waitJob(job.id, function (path) {
            out.innerHTML = '';
            out.appendChild(h('video', {
              controls: 'controls', src: dataURL(path),
              style: { width: '100%', maxWidth: '420px', borderRadius: '12px' }
            }));
            UI.toast('Xong!');
          }, function (err) { UI.toast('Dựng thất bại: ' + err, 'error'); });
        }).catch(function (err) {
          UI.toast('Không chạy được: ' + err.message, 'error');
        }).finally(function () { runBtn.disabled = false; });
      }
    });

    return h('div', { class: 'card mt-16' },
      h('div', { class: 'card-title' }, '🎬 Dựng video'),
      UI.field('Mô tả bối cảnh', promptBox),
      UI.field('Lưu vào dự án', projSel),
      h('div', { class: 'muted', style: { fontSize: '12.5px', margin: '4px 0 12px' } },
        'Model dựng theo thời lượng file giọng — giọng càng dài càng lâu. Bạn có thể rời trang, job vẫn chạy.'),
      runBtn, out);
  }

  // waitJob theo dõi một job tới khi xong.
  function waitJob(id, onDone, onErr) {
    var timer = setInterval(function () {
      API.get('/api/jobs/' + id).then(function (j) {
        if (j.status === 'done') { clearInterval(timer); onDone(j.output); }
        else if (j.status === 'error') { clearInterval(timer); onErr(j.error || 'không rõ nguyên nhân'); }
      }).catch(function () { clearInterval(timer); onErr('mất kết nối'); });
    }, 3000);
  }
})();
