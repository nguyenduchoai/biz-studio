/* ============================================================
   Biz Studio — Trang "Xưởng làm sẵn": khuôn theo lĩnh vực,
   preset nền tảng, tone nhạc, giọng theo ngôn ngữ.
   Load sau app.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  // Khoá localStorage để trang đích đọc lại khuôn vừa chọn. Hai khoá riêng vì
  // hai đường dựng khác hẳn nhau: HTML Video dựng bằng HTML/CSS, Text → Video
  // dựng bằng ảnh AI theo phiên. Dùng chung một khoá thì bấm sang đường này lại
  // nạp nhầm khuôn cho đường kia.
  var PREFILL_KEY = 'biz-template-prefill';
  var PREFILL_KEY_T2V = 'biz-template-prefill-t2v';

  // chooseTarget hỏi dựng bằng đường nào. Không tự đoán: cùng một khuôn dùng
  // được cả hai, và chọn sai thì người dùng mất công làm lại từ đầu.
  function chooseTarget(t) {
    function go(key, hash, label) {
      try { localStorage.setItem(key, JSON.stringify(t)); } catch (e) { /* chế độ riêng tư */ }
      m.close();
      UI.toast('Đã nạp khuôn "' + t.name + '" — ' + label);
      location.hash = hash;
    }
    var m = UI.modal({
      title: 'Dựng "' + t.name + '" bằng đường nào?',
      body: h('div', null,
        h('p', { class: 'muted', style: { fontSize: '13px', lineHeight: '1.6' } },
          'Cùng khuôn này dùng được cả hai đường — khác nhau ở chỗ hình lấy từ đâu.'),
        h('div', { style: { marginTop: '10px' } },
          row('HTML Video', 'Hình dựng bằng HTML/CSS — chữ sắc nét, số liệu và biểu đồ rõ, ' +
            'không tốn lượt gọi AI sinh ảnh. Hợp nội dung nhiều chữ và số.'),
          row('Text → Video', 'Hình sinh bằng AI theo bộ Style Kit — hợp kể chuyện, cảnh vật, nhân vật. ' +
            'Khuôn này hợp bộ "' + (t.style || '—') + '". Có lưu phiên để quay lại sửa.'))),
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('Text → Video', {
          variant: 'ghost',
          onclick: function () { go(PREFILL_KEY_T2V, '#/text2video', 'bấm "Phiên mới" để tạo phiên theo khuôn.'); }
        }),
        UI.btn('HTML Video', {
          onclick: function () { go(PREFILL_KEY, '#/htmlvideo', 'khung hình và hướng viết đã điền sẵn.'); }
        })
      ]
    });
  }

  function dataURL(rel) {
    return '/data/' + String(rel).split('/').map(encodeURIComponent).join('/');
  }

  App.pages['studio'] = {
    title: 'Xưởng làm sẵn',
    subtitle: 'Chọn khuôn theo lĩnh vực, chuẩn hoá cho từng nền tảng, nhạc nền theo tone',
    render: render
  };

  function render(root) {
    var tabs = ['Khuôn theo lĩnh vực', 'Rút clip ngắn', 'Ghép tư liệu', 'Nền tảng', 'Nhạc nền', 'Giọng theo ngôn ngữ'];
    var active = 0;
    var host = h('div', { class: 'mt-16' });
    var bar = h('div', { class: 'row' }, tabs.map(function (t, i) {
      var b = UI.btn(t, {
        variant: i === 0 ? 'primary' : 'ghost', small: true,
        onclick: function () {
          active = i;
          [].forEach.call(bar.children, function (el, k) {
            el.className = 'btn btn-sm ' + (k === i ? 'btn-primary' : 'btn-ghost');
          });
          draw();
        }
      });
      return b;
    }));
    root.appendChild(h('div', { class: 'card' }, bar));
    root.appendChild(host);

    function draw() {
      host.innerHTML = '';
      if (active === 0) drawTemplates(host);
      else if (active === 1) drawHighlight(host);
      else if (active === 2) drawBroll(host);
      else if (active === 3) drawPlatforms(host);
      else if (active === 4) drawMoods(host);
      else drawLangs(host);
    }
    draw();
  }

  // ---------- khuôn theo lĩnh vực ----------

  function drawTemplates(host) {
    host.appendChild(h('div', { class: 'card' }, UI.empty('Đang tải khuôn…', '🧰')));
    API.get('/api/studio/templates').then(function (d) {
      host.innerHTML = '';
      host.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px', marginBottom: '10px' } },
        d.templates.length + ' khuôn · mỗi khuôn gói sẵn hướng viết kịch bản, phong cách hình, ' +
        'khung hình, nền tảng, kiểu giọng, tone nhạc và nhịp ba đoạn — đổi gì cũng được sau khi chọn.'));
      d.categories.forEach(function (cat) {
        var items = d.templates.filter(function (t) { return t.category === cat; });
        if (!items.length) return;
        host.appendChild(h('div', { class: 'card mt-16' },
          h('div', { class: 'card-title' }, cat),
          h('div', {
            style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(260px,1fr))', gap: '10px' }
          }, items.map(templateCard))));
      });
    }).catch(function (err) {
      host.innerHTML = '';
      host.appendChild(h('div', { class: 'card' }, UI.empty('Không tải được khuôn: ' + err.message, '⚠️')));
    });
  }

  function templateCard(t) {
    var open = false;
    var detail = h('div', { style: { display: 'none', marginTop: '8px' } },
      row('Hướng viết kịch bản', t.script),
      row('Mở đầu', t.hook), row('Thân', t.body), row('Chốt', t.cta));

    var moreBtn = UI.btn('Xem công thức', {
      variant: 'ghost', small: true,
      onclick: function () {
        open = !open;
        detail.style.display = open ? '' : 'none';
        moreBtn.textContent = open ? 'Thu gọn' : 'Xem công thức';
      }
    });
    var useBtn = UI.btn('Dùng khuôn này', {
      variant: 'primary', small: true,
      onclick: function () { chooseTarget(t); }
    });

    var chips = [t.aspect, t.platform, t.seconds + 's', t.voicePace ? 'giọng ' + t.voicePace : '', t.musicMood]
      .filter(Boolean).map(function (s) {
        return h('span', { class: 'badge badge-blue', style: { marginRight: '4px' } }, s);
      });

    return h('div', {
      style: {
        border: '1px solid var(--border)', borderRadius: '10px', padding: '12px',
        minWidth: '0', display: 'flex', flexDirection: 'column', gap: '6px'
      }
    },
      h('div', { style: { fontWeight: '700', fontSize: '14px' } }, t.icon + ' ' + t.name),
      h('div', { class: 'muted', style: { fontSize: '12px', lineHeight: '1.45' } }, t.desc),
      h('div', { style: { marginTop: '2px' } }, chips),
      detail,
      h('div', { class: 'row', style: { gap: '6px', marginTop: 'auto' } }, moreBtn, useBtn));
  }

  function row(label, val) {
    if (!val) return h('span');
    return h('div', { style: { fontSize: '12px', marginBottom: '5px', lineHeight: '1.5' } },
      h('span', { style: { fontWeight: '700' } }, label + ': '),
      h('span', { class: 'muted' }, val));
  }

  // ---------- rút clip ngắn ----------

  function drawHighlight(host) {
    var st = { file: '', secs: 60, platform: 'tiktok', goal: '', minScore: 6 };

    var runBtn = UI.btn('Rút clip', {
      onclick: function () {
        if (!st.file) { UI.toast('Chưa chọn video nguồn.', 'error'); return; }
        runBtn.disabled = true;
        API.post('/api/studio/highlight', {
          path: st.file, seconds: Number(st.secs) || 60, platform: st.platform,
          goal: st.goal, minScore: Number(st.minScore) || 6, lang: 'vi'
        }).then(function () {
          runBtn.disabled = false;
          UI.toast('Đã xếp hàng — theo dõi ở Nhật ký. Bóc băng video dài mất vài phút.');
        }).catch(function (e) {
          runBtn.disabled = false;
          UI.toast('Không chạy được: ' + e.message, 'error');
        });
      }
    });

    var platSel = h('div');
    API.get('/api/studio/platforms').then(function (ps) {
      platSel.appendChild(UI.select('Nền tảng đích', [{ value: '', label: 'Không chuẩn hoá — giữ nguyên khung hình' }]
        .concat(ps.map(function (p) {
          return { value: p.id, label: p.name + (p.maxSec ? ' (tối đa ' + p.maxSec + 's)' : '') };
        })), st.platform, function (v) { st.platform = v; }));
    }).catch(function () { /* mất mạng nội bộ thì bỏ ô này, phần còn lại vẫn chạy */ });

    host.appendChild(h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '✂️ Rút video dài thành clip ngắn'),
      h('div', { class: 'muted', style: { fontSize: '12.5px', marginBottom: '12px', lineHeight: '1.6' } },
        'Hệ thống bóc băng video, để AI chấm điểm từng đoạn theo mức đáng giữ, rồi ghép các đoạn đắt nhất ' +
        'theo ĐÚNG thứ tự thời gian gốc — chọn theo điểm nhưng xếp theo thời gian, vì ghép lộn xộn thì ' +
        'người xem không lần ra mạch. Mép cắt nới ra tới từ trọn vẹn nên không bị nuốt chữ. ' +
        'Video gốc không bị đụng tới.'),
      UI.field('Video nguồn', UI.input({
        value: '', placeholder: 'Đường dẫn video trong data, vd: tmp/abc/phong-van.mp4',
        oninput: function (e) { st.file = e.target.value.trim(); }
      })),
      h('div', { class: 'grid-3 mt-8' },
        UI.field('Thời lượng clip (giây)', UI.input({
          type: 'number', value: '60',
          oninput: function (e) { st.secs = e.target.value; }
        })),
        platSel,
        UI.select('Ngưỡng điểm giữ lại', [
          { value: '7', label: '7 — chỉ đoạn thật đắt (clip ngắn hơn)' },
          { value: '6', label: '6 — cân bằng (mặc định)' },
          { value: '5', label: '5 — giữ rộng tay (clip đầy hơn)' }
        ], String(st.minScore), function (v) { st.minScore = v; })),
      h('div', { class: 'mt-8' },
        UI.field('Clip nhắm vào ý gì (tuỳ chọn)', UI.input({
          placeholder: 'vd: giải thích vì sao hoá đơn điện tăng',
          oninput: function (e) { st.goal = e.target.value; }
        }))),
      h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } },
        'Chọn nền tảng thì hệ thống tự ép thời lượng xuống dưới trần của nền tảng đó. ' +
        'Cần khoá AI trong Cấu hình & API — không có khoá thì báo lỗi rõ chứ không chấm bừa: ' +
        'cắt nhầm đoạn còn tệ hơn không cắt.'),
      h('div', { class: 'row mt-16' }, runBtn)));
  }

  // ---------- ghép tư liệu ----------

  function drawBroll(host) {
    var st = { dir: '', audio: '', aspect: '9:16', maxClip: 5, fps: 30, shuffle: true };

    var runBtn = UI.btn('Ghép tư liệu', {
      onclick: function () {
        if (!st.dir) { UI.toast('Chưa chọn thư mục clip tư liệu.', 'error'); return; }
        if (!st.audio) { UI.toast('Chưa chọn file lời đọc.', 'error'); return; }
        runBtn.disabled = true;
        API.post('/api/studio/broll', {
          clipsDir: st.dir, audio: st.audio, aspect: st.aspect,
          maxClip: Number(st.maxClip) || 5, fps: Number(st.fps) || 30, shuffle: st.shuffle
        }).then(function () {
          runBtn.disabled = false;
          UI.toast('Đã xếp hàng — theo dõi ở Nhật ký.');
        }).catch(function (e) {
          runBtn.disabled = false;
          UI.toast('Không chạy được: ' + e.message, 'error');
        });
      }
    });

    host.appendChild(h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '🎞 Ghép clip tư liệu khớp lời đọc'),
      h('div', { class: 'muted', style: { fontSize: '12.5px', marginBottom: '12px', lineHeight: '1.6' } },
        'Kiểu video "đọc trên nền tư liệu" hay gặp ở kênh tin tức, kiến thức, review. ' +
        'Hệ thống cắt các clip thành mẩu ngắn, xoay vòng qua TỪNG clip để mọi file đều lên hình, ' +
        'ghép cho đủ dài rồi cắt đúng bằng lời đọc. Lời đọc giữ nguyên — hình chạy theo tiếng, ' +
        'không phải ngược lại. Clip lệch tỉ lệ thì thêm viền chứ không bóp méo. File gốc không bị đụng tới.'),
      UI.field('Thư mục clip tư liệu', UI.input({
        placeholder: 'vd: tmp/tuyet-lieu  (chấp nhận .mp4 .mov .mkv .webm .m4v .avi)',
        oninput: function (e) { st.dir = e.target.value.trim(); }
      })),
      h('div', { class: 'mt-8' },
        UI.field('File lời đọc', UI.input({
          placeholder: 'vd: tmp/phien-abc/voice.wav',
          oninput: function (e) { st.audio = e.target.value.trim(); }
        }))),
      h('div', { class: 'grid-3 mt-8' },
        UI.select('Khung hình', [
          { value: '9:16', label: '9:16 — Dọc' },
          { value: '3:4', label: '3:4 — Dọc kiểu trang giấy' },
          { value: '16:9', label: '16:9 — Ngang' },
          { value: '1:1', label: '1:1 — Vuông' },
          { value: '', label: 'Giữ theo clip đầu tiên' }
        ], st.aspect, function (v) { st.aspect = v; }),
        UI.field('Mỗi mẩu tối đa (giây)', UI.input({
          type: 'number', value: '5',
          oninput: function (e) { st.maxClip = e.target.value; }
        })),
        UI.select('FPS', [
          { value: '30', label: '30 fps' },
          { value: '24', label: '24 fps' }
        ], String(st.fps), function (v) { st.fps = v; })),
      h('div', { class: 'mt-8' },
        UI.toggle('Xáo thứ tự mẩu', 'Vẫn tất định — dựng lại ra đúng video cũ, để còn sửa lỗi được',
          st.shuffle, function (v) { st.shuffle = v; })),
      h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '8px' } },
        'Tư liệu ngắn hơn lời đọc thì hệ thống dùng lại và BÁO rõ đã lặp mấy vòng — ' +
        'thêm clip vào thư mục để hình đỡ lặp.'),
      h('div', { class: 'row mt-16' }, runBtn)));
  }

  // ---------- nền tảng ----------

  function drawPlatforms(host) {
    var st = { file: '' };
    var input = UI.input({
      value: '', placeholder: 'Đường dẫn video trong data, vd: tmp/abc/final.mp4',
      oninput: function (e) { st.file = e.target.value.trim(); }
    });
    host.appendChild(h('div', { class: 'card' },
      h('div', { class: 'card-title' }, '🎯 Chuẩn hoá video cho nền tảng'),
      h('div', { class: 'muted', style: { fontSize: '12.5px', marginBottom: '10px' } },
        'Đưa về đúng khung hình và độ to chuẩn phát. Lệch tỉ lệ thì thêm viền chứ KHÔNG bóp méo hình. ' +
        'Video dài quá trần thì hệ thống báo chứ không tự cắt.'),
      input));

    var grid = h('div', {
      style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(240px,1fr))', gap: '10px' }
    });
    host.appendChild(h('div', { class: 'card mt-16' }, h('div', { class: 'card-title' }, 'Chọn nền tảng'), grid));

    API.get('/api/studio/platforms').then(function (ps) {
      ps.forEach(function (p) {
        grid.appendChild(h('div', {
          style: { border: '1px solid var(--border)', borderRadius: '10px', padding: '12px', minWidth: '0' }
        },
          h('div', { style: { fontWeight: '700' } }, p.name),
          h('div', { class: 'muted', style: { fontSize: '12px', margin: '4px 0' } },
            p.width + '×' + p.height + ' · ' + (p.maxSec ? 'tối đa ' + p.maxSec + 's' : 'không giới hạn') +
            ' · ' + p.lufs + ' LUFS'),
          h('div', { class: 'muted', style: { fontSize: '11.5px', lineHeight: '1.45', marginBottom: '8px' } }, p.note),
          UI.btn('Chuẩn hoá', {
            variant: 'ghost', small: true,
            onclick: function () {
              if (!st.file) { UI.toast('Chưa chọn video.', 'error'); return; }
              API.post('/api/studio/normalize', { path: st.file, platform: p.id })
                .then(function () { UI.toast('Đã xếp hàng: chuẩn hoá cho ' + p.name); })
                .catch(function (err) { UI.toast('Không chạy được: ' + err.message, 'error'); });
            }
          })));
      });
    }).catch(function (err) {
      grid.appendChild(h('div', { class: 'muted' }, 'Không tải được: ' + err.message));
    });
  }

  // ---------- nhạc nền ----------

  function drawMoods(host) {
    host.appendChild(h('div', { class: 'card' }, UI.empty('Đang chuẩn bị nhạc nền…', '🎵')));
    API.get('/api/studio/moods').then(function (ms) {
      host.innerHTML = '';
      host.appendChild(h('div', { class: 'card' },
        h('div', { class: 'card-title' }, '🎵 Nhạc nền theo tone — ' + ms.length + ' tone'),
        h('div', { class: 'muted', style: { fontSize: '12.5px', marginBottom: '10px', lineHeight: '1.6' } },
          'Tất cả được TỔNG HỢP tại chỗ, không mang theo nhạc của ai — nhạc có bản quyền lọt vào video ' +
          'là bị nền tảng gỡ tiếng hoặc chặn kiếm tiền. Đây là bè đệm giữ không khí, cố ý không có giai điệu ' +
          'chính để không giành chỗ với lời đọc. Muốn nhạc thật thì bạn vẫn tự đưa file vào như cũ.'),
        h('div', {
          style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(250px,1fr))', gap: '10px' }
        }, ms.map(function (m) {
          var audio = h('audio', { src: dataURL(m.path), preload: 'none', loop: 'loop' });
          return h('div', {
            style: { border: '1px solid var(--border)', borderRadius: '10px', padding: '12px', minWidth: '0' }
          },
            h('div', { style: { fontWeight: '700' } }, m.name),
            h('div', { class: 'muted', style: { fontSize: '12px', margin: '4px 0 8px', lineHeight: '1.45' } }, m.desc),
            h('div', { class: 'row', style: { gap: '6px' } },
              UI.btn('▶ Nghe', { variant: 'ghost', small: true, onclick: function () { audio.currentTime = 0; audio.play(); } }),
              UI.btn('⏸ Dừng', { variant: 'ghost', small: true, onclick: function () { audio.pause(); } }),
              h('a', { href: dataURL(m.path), download: '', class: 'btn btn-sm btn-ghost' }, '⬇')),
            audio);
        }))));
    }).catch(function (err) {
      host.innerHTML = '';
      host.appendChild(h('div', { class: 'card' }, UI.empty('Không tải được nhạc: ' + err.message, '⚠️')));
    });
  }

  // ---------- giọng theo ngôn ngữ ----------

  function drawLangs(host) {
    host.appendChild(h('div', { class: 'card' }, UI.empty('Đang đọc danh sách giọng…', '🗣')));
    API.get('/api/studio/voice-langs').then(function (gs) {
      host.innerHTML = '';
      var total = gs.reduce(function (a, g) { return a + g.count; }, 0);
      host.appendChild(h('div', { class: 'card' },
        h('div', { class: 'card-title' }, '🗣 ' + total + ' giọng · ' + gs.length + ' ngôn ngữ'),
        h('div', { class: 'muted', style: { fontSize: '12.5px', marginBottom: '10px' } },
          'Máy bạn đã có sẵn giọng của tất cả các ngôn ngữ này — làm video tiếng Anh, Nhật, Trung… ' +
          'không cần cài thêm gì. Tiếng Việt dùng VieNeu on-device, chất lượng cao nhất.'),
        h('div', {
          style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(200px,1fr))', gap: '8px' }
        }, gs.map(function (g) {
          return h('div', {
            style: { border: '1px solid var(--border)', borderRadius: '8px', padding: '10px', minWidth: '0' }
          },
            h('div', { style: { fontWeight: '700', fontSize: '13px' } }, g.name),
            h('div', { class: 'muted', style: { fontSize: '12px' } }, g.count + ' giọng'),
            h('div', {
              class: 'muted',
              style: { fontSize: '11px', marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }
            }, g.voices.slice(0, 3).map(function (v) { return v.name || v.id; }).join(', ')));
        }))));
    }).catch(function (err) {
      host.innerHTML = '';
      host.appendChild(h('div', { class: 'card' }, UI.empty('Không tải được: ' + err.message, '⚠️')));
    });
  }
})();
