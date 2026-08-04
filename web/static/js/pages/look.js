/* ============================================================
   Biz Studio — Trang "Diện mạo": chỉnh màu, tiếng động, font Việt.
   Load sau app.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  function dataURL(rel) {
    return '/data/' + String(rel).split('/').map(encodeURIComponent).join('/');
  }

  App.pages['look'] = {
    title: 'Diện mạo',
    subtitle: 'Chỉnh màu, tiếng động và font chữ dùng chung cho mọi video',
    render: render
  };

  function render(root) {
    var st = { file: '', grades: [], sfx: [], cues: [], strength: 1, atSec: 1, handlers: [] };
    App._cleanup = function () {
      st.handlers.forEach(function (fn) { Bus.off('job', fn); });
      st.handlers = [];
    };

    root.appendChild(fileCard(st));
    var gradeHost = h('div', { class: 'mt-16' });
    var sfxHost = h('div', { class: 'mt-16' });
    var fontHost = h('div', { class: 'mt-16' });
    root.appendChild(gradeHost);
    root.appendChild(sfxHost);
    root.appendChild(fontHost);

    loadGrades(st, gradeHost);
    loadSfx(st, sfxHost);
    loadFont(fontHost);
  }

  // ---------- chọn file nguồn ----------

  function fileCard(st) {
    var input = UI.input({
      value: '', placeholder: 'vd: tmp/htmlvideo_abc/htmlvideo-final.mp4',
      oninput: function (e) { st.file = e.target.value.trim(); }
    });
    var pick = h('input', { type: 'file', accept: 'video/*,audio/*', style: { display: 'none' } });
    pick.onchange = function () {
      if (!pick.files || !pick.files.length) return;
      UI.toast('Đang tải file lên…');
      API.upload('/api/tools/upload', pick.files).then(function (res) {
        if (res && res.length) {
          st.file = res[0].path;
          input.value = res[0].path;
          UI.toast('Đã tải lên: ' + res[0].name);
        }
      }).catch(function (err) { UI.toast('Tải lên thất bại: ' + err.message, 'error'); });
    };
    return h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '🎞️ Video / âm thanh cần xử lý'),
      h('div', { class: 'row' }, input,
        UI.btn('📂 Chọn file…', { variant: 'ghost', onclick: function () { pick.click(); } }), pick),
      h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } },
        'Đường dẫn tính từ thư mục data. File kết quả nằm cùng thư mục với file gốc.'));
  }

  // ---------- chỉnh màu ----------

  function loadGrades(st, host) {
    host.appendChild(h('div', { class: 'card' }, UI.empty('Đang tải danh sách kiểu màu…', '🎨')));
    API.get('/api/tools/grades').then(function (list) {
      st.grades = list || [];
      host.innerHTML = '';
      host.appendChild(gradeCard(st));
    }).catch(function (err) {
      host.innerHTML = '';
      host.appendChild(h('div', { class: 'card' },
        UI.empty('Không tải được kiểu màu: ' + err.message, '⚠️')));
    });
  }

  function gradeCard(st) {
    var shotHost = h('div', {
      style: { marginTop: '12px', display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(180px,1fr))', gap: '10px' }
    });
    var strengthLabel = h('span', { class: 'muted', style: { fontSize: '12px' } }, 'Độ mạnh: 100%');
    var slider = h('input', {
      type: 'range', min: '10', max: '100', step: '5', value: '100', style: { width: '180px' }
    });
    slider.oninput = function () {
      st.strength = Number(slider.value) / 100;
      strengthLabel.textContent = 'Độ mạnh: ' + slider.value + '%';
    };

    var grid = h('div', {
      style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(200px,1fr))', gap: '8px' }
    });
    st.grades.forEach(function (g) {
      grid.appendChild(h('div', {
        style: {
          border: '1px solid var(--border)', borderRadius: '10px', padding: '10px',
          minWidth: '0', display: 'flex', flexDirection: 'column', gap: '6px'
        }
      },
        h('div', { style: { fontWeight: '700', fontSize: '13px' } }, g.name),
        h('div', { class: 'muted', style: { fontSize: '11.5px', lineHeight: '1.35' } }, g.desc),
        h('div', { class: 'row', style: { gap: '6px', marginTop: 'auto' } },
          UI.btn('Xem thử', { variant: 'ghost', small: true, onclick: function () { preview(st, g, shotHost); } }),
          UI.btn('Áp dụng', { variant: 'primary', small: true, onclick: function () { apply(st, g); } }))));
    });

    return h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '🎨 Chỉnh màu — ' + st.grades.length + ' kiểu'),
      h('div', { class: 'muted', style: { fontSize: '12px', marginBottom: '10px' } },
        'Xem thử chỉ dựng một khung hình nên rất nhanh; ưng rồi mới chạy cả video.'),
      h('div', { class: 'row', style: { marginBottom: '12px', gap: '10px' } }, slider, strengthLabel),
      grid, shotHost);
  }

  function preview(st, g, shotHost) {
    if (!st.file) { UI.toast('Chọn file video trước đã.', 'error'); return; }
    API.post('/api/tools/grade/preview', {
      path: st.file, preset: g.id, atSec: st.atSec, strength: st.strength
    }).then(function (res) {
      var card = h('div', {
        style: { border: '1px solid var(--border)', borderRadius: '10px', overflow: 'hidden' }
      },
        h('img', { src: dataURL(res.path) + '?t=' + Date.now(), style: { width: '100%', display: 'block' } }),
        h('div', { style: { fontSize: '12px', padding: '6px 8px', fontWeight: '600' } }, res.name));
      shotHost.insertBefore(card, shotHost.firstChild);
      while (shotHost.children.length > 6) shotHost.removeChild(shotHost.lastChild);
    }).catch(function (err) { UI.toast('Xem thử thất bại: ' + err.message, 'error'); });
  }

  function apply(st, g) {
    if (!st.file) { UI.toast('Chọn file video trước đã.', 'error'); return; }
    API.post('/api/tools/grade', { path: st.file, preset: g.id, strength: st.strength })
      .then(function () { UI.toast('Đã xếp hàng: chỉnh màu ' + g.name); })
      .catch(function (err) { UI.toast('Không chạy được: ' + err.message, 'error'); });
  }

  // ---------- tiếng động ----------

  function loadSfx(st, host) {
    host.appendChild(h('div', { class: 'card' }, UI.empty('Đang chuẩn bị thư viện tiếng động…', '🔔')));
    API.get('/api/tools/sfx').then(function (list) {
      st.sfx = list || [];
      host.innerHTML = '';
      host.appendChild(sfxCard(st));
    }).catch(function (err) {
      host.innerHTML = '';
      host.appendChild(h('div', { class: 'card' },
        UI.empty('Không tải được tiếng động: ' + err.message, '⚠️')));
    });
  }

  function sfxCard(st) {
    var cueHost = h('div', { style: { marginTop: '12px' } });
    var atInput = UI.input({ value: '1.0', placeholder: 'giây' });
    atInput.style.maxWidth = '110px';

    function redrawCues() {
      cueHost.innerHTML = '';
      if (!st.cues.length) {
        cueHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12px' } },
          'Chưa chọn mốc nào. Nhập giây rồi bấm “+ Chèn” ở hiệu ứng muốn dùng.'));
        return;
      }
      st.cues.slice().sort(function (a, b) { return a.atSec - b.atSec; }).forEach(function (c) {
        cueHost.appendChild(h('div', { class: 'row', style: { gap: '8px', marginBottom: '6px' } },
          h('span', { class: 'badge badge-blue' }, c.atSec.toFixed(2) + 's'),
          h('span', { style: { fontSize: '13px' } }, c.name),
          UI.btn('✕', {
            variant: 'ghost', small: true,
            onclick: function () { st.cues.splice(st.cues.indexOf(c), 1); redrawCues(); }
          })));
      });
      cueHost.appendChild(UI.btn('🔊 Chèn ' + st.cues.length + ' hiệu ứng vào video', {
        variant: 'primary',
        onclick: function () {
          if (!st.file) { UI.toast('Chọn file video trước đã.', 'error'); return; }
          API.post('/api/tools/sfx/mix', {
            path: st.file,
            cues: st.cues.map(function (c) { return { sfx: c.id, atSec: c.atSec }; })
          }).then(function () { UI.toast('Đã xếp hàng: chèn tiếng động'); })
            .catch(function (err) { UI.toast('Không chạy được: ' + err.message, 'error'); });
        }
      }));
    }

    var grid = h('div', {
      style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(210px,1fr))', gap: '8px' }
    });
    st.sfx.forEach(function (s) {
      var audio = h('audio', { src: dataURL(s.path), preload: 'none' });
      grid.appendChild(h('div', {
        style: { border: '1px solid var(--border)', borderRadius: '10px', padding: '10px', minWidth: '0' }
      },
        h('div', { style: { fontWeight: '700', fontSize: '13px' } }, s.name + ' · ' + s.secs.toFixed(2) + 's'),
        h('div', { class: 'muted', style: { fontSize: '11.5px', lineHeight: '1.35', marginBottom: '6px' } }, s.desc),
        h('div', { class: 'row', style: { gap: '6px' } },
          UI.btn('▶ Nghe', { variant: 'ghost', small: true, onclick: function () { audio.currentTime = 0; audio.play(); } }),
          UI.btn('+ Chèn', {
            variant: 'ghost', small: true,
            onclick: function () {
              var at = Number(atInput.value);
              if (!(at >= 0)) { UI.toast('Mốc thời gian không hợp lệ.', 'error'); return; }
              st.cues.push({ id: s.id, name: s.name, atSec: at });
              redrawCues();
            }
          })),
        audio));
    });

    redrawCues();
    return h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '🔔 Tiếng động — ' + st.sfx.length + ' hiệu ứng'),
      h('div', { class: 'muted', style: { fontSize: '12px', marginBottom: '10px' } },
        'Tất cả đều được tổng hợp tại chỗ và cân về cùng một độ to, chèn vào là nghe đều tay.'),
      h('div', { class: 'row', style: { marginBottom: '12px', gap: '8px' } },
        h('span', { class: 'field-label' }, 'Chèn tại giây:'), atInput),
      grid, cueHost);
  }

  // ---------- font ----------

  function loadFont(host) {
    API.get('/api/tools/font').then(function (f) {
      host.innerHTML = '';
      host.appendChild(fontCard(f, host));
    }).catch(function () { /* không có font cũng không sao */ });
  }

  function fontCard(f, host) {
    var body = [h('div', { class: 'muted', style: { fontSize: '13px', lineHeight: '1.5' } }, f.note)];
    if (!f.ready) {
      body.push(h('div', { style: { marginTop: '10px' } },
        UI.btn('⬇️ Tải font ' + f.family + ' (khoảng 400 KB)', {
          variant: 'primary',
          onclick: function () {
            API.post('/api/tools/font').then(function () {
              UI.toast('Đang tải font…');
              setTimeout(function () { loadFont(host); }, 4000);
            }).catch(function (err) { UI.toast('Tải font thất bại: ' + err.message, 'error'); });
          }
        })));
    }
    return h('div', { class: 'card' },
      h('div', { class: 'card-title' },
        (f.ready ? '✅ ' : '⚠️ ') + 'Font tiếng Việt — ' + f.family),
      body);
  }
})();
