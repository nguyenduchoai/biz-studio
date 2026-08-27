/* ============================================================
   Biz Studio — Biên tập video nhiều lớp (âm thanh + phụ đề) trên một lớp video nền.

   Chỉ một lớp video là chủ ý, không phải làm dở: nhiều lớp âm thanh và phụ đề
   thì trình duyệt tự trộn và tự hiện được nên xem trước ĐÚNG bằng bản xuất ra.
   Lồng video trên video bắt buộc phải ghép hình, kéo một cái phải chờ — chuyện
   khác, để sau.

   Nạp sau timeline-engine.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var ROW_H = 40;
  var MIN_ITEM = 0.2;          // giây — ngắn hơn thì kéo mép thành khối vô hình
  var ROLES = [
    { value: 'narration', label: 'Lời đọc' },
    { value: 'music', label: 'Nhạc nền' },
    { value: 'sfx', label: 'Tiếng động' }
  ];
  var ROLE_COLOR = {
    source: 'var(--muted)', narration: 'var(--blue-h)',
    music: 'var(--green)', sfx: '#b06ad9'
  };

  function injectStyles() {
    if (document.getElementById('tl-style')) return;
    document.head.appendChild(h('style', { id: 'tl-style' }, [
      '.tl-wrap{overflow-x:auto;overflow-y:hidden;position:relative;border:1px solid var(--border);border-radius:8px}',
      '.tl-inner{position:relative}',
      '.tl-row{position:relative;height:' + ROW_H + 'px;border-top:1px solid var(--border)}',
      '.tl-row:first-child{border-top:none}',
      '.tl-item{position:absolute;top:4px;height:' + (ROW_H - 8) + 'px;border-radius:5px;color:#fff;',
      'font-size:11px;line-height:' + (ROW_H - 8) + 'px;padding:0 6px;overflow:hidden;white-space:nowrap;',
      'cursor:grab;user-select:none;box-sizing:border-box}',
      '.tl-item.sel{outline:2px solid var(--amber,#e0a800);outline-offset:1px}',
      '.tl-item canvas{position:absolute;inset:0;width:100%;height:100%;opacity:.45;pointer-events:none}',
      '.tl-item span{position:relative}',
      '.tl-grip{position:absolute;top:0;bottom:0;width:7px;cursor:ew-resize;background:rgba(0,0,0,.25)}',
      '.tl-grip.l{left:0;border-radius:5px 0 0 5px}.tl-grip.r{right:0;border-radius:0 5px 5px 0}',
      '.tl-head{position:sticky;left:0;z-index:3;background:var(--card,#fff);border-right:1px solid var(--border)}',
      '.tl-play{position:absolute;top:0;bottom:0;width:2px;background:#e03131;z-index:4;pointer-events:none}',
      '.tl-ruler{position:relative;height:20px;font-size:10px;color:var(--muted);user-select:none;cursor:pointer}',
      '.tl-tick{position:absolute;top:0;bottom:0;border-left:1px solid var(--border);padding-left:3px}',
      '.tl-sub{position:absolute;left:0;right:0;bottom:8%;text-align:center;pointer-events:none;',
      'font-weight:600;font-size:clamp(13px,3.2vw,22px);color:#fff;text-shadow:0 2px 6px rgba(0,0,0,.9);padding:0 6%}'
    ].join('\n')));
  }

  function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
  function uid(p) { return p + '-' + Math.random().toString(36).slice(2, 9); }
  function itemDur(it) { return (it.out > it.in) ? (it.out - it.in) : 0; }

  // ---------- trang ----------

  App.pages['editor'] = {
    title: 'Biên tập video',
    subtitle: 'Xem trước, cắt khoảng lặng và dựng nhiều lớp âm thanh, tiếng động, phụ đề trong một nơi',
    render: function (el, param) { injectStyles(); boot(el, param); }
  };

  function boot(el, param) {
    el.innerHTML = '';
    el.appendChild(h('div', { class: 'empty' }, UI.spinner(), h('span', null, 'Đang tải dự án…')));

    API.get('/api/projects').then(function (ps) {
      ps = ps || [];
      if (!ps.length) {
        el.innerHTML = '';
        el.appendChild(UI.card({
          title: 'Chưa có dự án nào', icon: '🎬',
          body: h('div', { class: 'muted' }, 'Tạo một dự án và thêm media trước đã — bản dựng dùng media của dự án.')
        }));
        return;
      }
      var id = param || ps[0].id;
      openProject(el, ps, id);
    }).catch(function (e) {
      el.innerHTML = '';
      el.appendChild(h('div', { class: 'text-red' }, 'Không tải được danh sách dự án: ' + e.message));
    });
  }

  function openProject(el, projects, id) {
    Promise.all([
      API.get('/api/projects/' + encodeURIComponent(id) + '/timeline'),
      API.get('/api/projects/' + encodeURIComponent(id))
    ]).then(function (r) {
      draw(el, projects, id, r[0] || {}, (r[1] && r[1].assets) || []);
    }).catch(function (e) {
      el.innerHTML = '';
      el.appendChild(h('div', { class: 'text-red' }, 'Không mở được bản dựng: ' + e.message));
    });
  }

  function draw(el, projects, projectID, doc, assets) {
    el.innerHTML = '';
    doc.tracks = doc.tracks || [];
    doc.subs = doc.subs || [];

    var st = { zoom: 12, sel: null, dirty: false };
    var engine = null;

    // --- xem trước
    var subLayer = h('div', { class: 'tl-sub' }, '');
    var video = h('video', {
      controls: true, preload: 'metadata',
      style: { width: '100%', maxHeight: '46vh', background: '#000', display: 'block', borderRadius: '8px' }
    });
    if (doc.video) video.src = '/data/' + doc.video;
    var stage = h('div', { style: { position: 'relative' } }, video, subLayer);

    engine = new TimelineEngine(video);
    engine.onCue = function (c) { subLayer.textContent = c ? c.text : ''; };
    engine.setDoc(doc);
    engine.preload(doc);

    // --- khung timeline
    var inner = h('div', { class: 'tl-inner' });
    var wrap = h('div', { class: 'tl-wrap' }, inner);
    var playhead = h('div', { class: 'tl-play', style: { left: '0px' } });

    function total() { return Math.max(10, doc.videoDur || 0, maxEnd(doc)); }
    function pxPerSec() { return st.zoom; }
    function xOf(t) { return t * pxPerSec(); }
    function tOf(x) { return x / pxPerSec(); }

    function markDirty() { st.dirty = true; saveBtn.disabled = false; }

    function repaint() {
      inner.innerHTML = '';
      inner.style.width = Math.max(600, xOf(total()) + 40) + 'px';
      inner.appendChild(ruler());
      inner.appendChild(videoRow());
      doc.tracks.forEach(function (t) {
        if (t.role === 'source') return; // lớp tiếng gốc không có khối riêng
        inner.appendChild(audioRow(t));
      });
      inner.appendChild(subRow());
      inner.appendChild(playhead);
      movePlayhead();
      engine.setDoc(doc);
    }

    function movePlayhead() { playhead.style.left = xOf(video.currentTime || 0) + 'px'; }
    video.addEventListener('timeupdate', movePlayhead);
    video.addEventListener('seeked', movePlayhead);

    function ruler() {
      var r = h('div', { class: 'tl-ruler' });
      var step = pxPerSec() >= 30 ? 1 : (pxPerSec() >= 12 ? 5 : 15);
      for (var t = 0; t <= total(); t += step) {
        r.appendChild(h('div', { class: 'tl-tick', style: { left: xOf(t) + 'px' } }, UI.fmtDur(t)));
      }
      r.onclick = function (e) {
        video.currentTime = clamp(tOf(e.offsetX), 0, total());
      };
      return r;
    }

    function videoRow() {
      var row = h('div', { class: 'tl-row' });
      if (doc.video) {
        row.appendChild(h('div', {
          class: 'tl-item',
          style: { left: '0px', width: xOf(doc.videoDur || 0) + 'px', background: '#4a5568', cursor: 'default' }
        }, h('span', null, '🎬 ' + doc.video.split('/').pop())));
      } else {
        row.appendChild(h('div', { class: 'muted', style: { fontSize: '11px', lineHeight: ROW_H + 'px', paddingLeft: '6px' } },
          'Chưa chọn video nền'));
      }
      return row;
    }

    function audioRow(track) {
      var row = h('div', { class: 'tl-row' });
      track.items.forEach(function (it) {
        row.appendChild(itemBlock(track, it));
      });
      return row;
    }

    function itemBlock(track, it) {
      var w = Math.max(6, xOf(itemDur(it)));
      var box = h('div', {
        class: 'tl-item' + (st.sel && st.sel.id === it.id ? ' sel' : ''),
        title: it.name || it.path,
        style: { left: xOf(it.at) + 'px', width: w + 'px', background: ROLE_COLOR[track.role] || '#777' }
      }, h('span', null, it.name || it.path.split('/').pop()));

      drawWave(box, it, w);
      box.appendChild(h('div', { class: 'tl-grip l' }));
      box.appendChild(h('div', { class: 'tl-grip r' }));
      bindDrag(box, track, it);
      return box;
    }

    // --- sóng âm
    var waveCache = {};
    function drawWave(box, it, w) {
      var cv = h('canvas');
      box.appendChild(cv);
      var key = it.path;
      function paint(peaks) {
        if (!peaks || !peaks.length) return;
        var W = Math.max(1, Math.round(w)), H = ROW_H - 8;
        cv.width = W; cv.height = H;
        var g = cv.getContext('2d');
        g.fillStyle = '#fff';
        // Chỉ vẽ phần nằm trong [in, out]: kéo mép mà sóng vẫn hiện nguyên file
        // thì hình không còn khớp tiếng, nhìn càng rối.
        var full = peaks.length;
        var a = Math.floor(full * (it.in / Math.max(it.in + itemDur(it), 0.001)));
        for (var x = 0; x < W; x++) {
          var i = a + Math.floor((x / W) * (full - a));
          var v = peaks[clamp(i, 0, full - 1)] || 0;
          var hgt = Math.max(1, v * H);
          g.fillRect(x, (H - hgt) / 2, 1, hgt);
        }
      }
      if (waveCache[key]) { paint(waveCache[key]); return; }
      if (waveCache[key] === null) return;
      waveCache[key] = null;
      API.get('/api/timeline/peaks?n=600&path=' + encodeURIComponent(it.path))
        .then(function (r) { waveCache[key] = r.peaks; paint(r.peaks); })
        .catch(function () { /* không có sóng thì khối vẫn dùng được */ });
    }

    // --- kéo / cắt mép
    function bindDrag(box, track, it) {
      box.addEventListener('mousedown', function (e) {
        e.preventDefault();
        select(track, it);
        var mode = e.target.classList.contains('l') ? 'trimL'
          : (e.target.classList.contains('r') ? 'trimR' : 'move');
        var x0 = e.clientX, at0 = it.at, in0 = it.in, out0 = it.out;
        box.style.cursor = mode === 'move' ? 'grabbing' : 'ew-resize';

        function onMove(ev) {
          var dt = tOf(ev.clientX - x0);
          if (mode === 'move') {
            it.at = Math.max(0, at0 + dt);
          } else if (mode === 'trimL') {
            // Kéo mép trái đổi CẢ hai: vị trí trên timeline và điểm cắt trong
            // file nguồn. Chỉ đổi một cái là tiếng trượt đi so với hình.
            var d = clamp(dt, -in0, (out0 - in0) - MIN_ITEM);
            it.in = in0 + d;
            it.at = Math.max(0, at0 + d);
          } else {
            it.out = Math.max(in0 + MIN_ITEM, out0 + dt);
          }
          repaint();
          refreshProps();
        }
        function onUp() {
          document.removeEventListener('mousemove', onMove);
          document.removeEventListener('mouseup', onUp);
          markDirty();
          engine.setDoc(doc);
          if (!video.paused) engine.resync();
        }
        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
      });
    }

    function select(track, it) {
      st.sel = it;
      st.selTrack = track;
      repaint();
      refreshProps();
    }

    // --- lớp phụ đề
    function subRow() {
      var row = h('div', { class: 'tl-row' });
      doc.subs.forEach(function (c) {
        var w = Math.max(6, xOf(c.end - c.start));
        row.appendChild(h('div', {
          class: 'tl-item', title: c.text,
          style: { left: xOf(c.start) + 'px', width: w + 'px', background: '#d97706' },
          onclick: function () { editCue(c); }
        }, h('span', null, c.text)));
      });
      return row;
    }

    function editCue(c) {
      var txt = UI.input({ value: c.text, oninput: function (e) { c.text = e.target.value; } });
      var m = UI.modal({
        title: 'Sửa dòng phụ đề',
        body: h('div', null,
          UI.field('Nội dung', txt),
          h('div', { class: 'grid-2 mt-8' },
            UI.field('Bắt đầu (giây)', UI.input({
              type: 'number', value: String(c.start),
              oninput: function (e) { c.start = Number(e.target.value) || 0; }
            })),
            UI.field('Kết thúc (giây)', UI.input({
              type: 'number', value: String(c.end),
              oninput: function (e) { c.end = Number(e.target.value) || 0; }
            })))),
        actions: [
          UI.btn('Xoá dòng', {
            variant: 'ghost', onclick: function () {
              doc.subs = doc.subs.filter(function (x) { return x !== c; });
              m.close(); markDirty(); repaint();
            }
          }),
          UI.btn('Xong', { onclick: function () { m.close(); markDirty(); repaint(); } })
        ]
      });
    }

    // ---------- thanh công cụ ----------

    var propsHost = h('div');
    function refreshProps() { propsHost.innerHTML = ''; propsHost.appendChild(propsPanel()); }

    function propsPanel() {
      if (!st.sel) {
        return h('div', { class: 'muted', style: { fontSize: '12px' } },
          'Bấm vào một khối để chỉnh âm lượng, fade và vị trí.');
      }
      var it = st.sel;
      return h('div', null,
        h('div', { style: { fontWeight: '700', fontSize: '13px', marginBottom: '8px' } },
          it.name || it.path.split('/').pop()),
        h('div', { class: 'grid-2' },
          UI.field('Đặt ở giây', UI.input({
            type: 'number', step: '0.1', value: it.at.toFixed(2),
            oninput: function (e) { it.at = Math.max(0, Number(e.target.value) || 0); markDirty(); repaint(); }
          })),
          UI.field('Âm lượng (dB)', UI.input({
            type: 'number', step: '1', value: String(it.gain || 0),
            oninput: function (e) { it.gain = clamp(Number(e.target.value) || 0, -60, 12); markDirty(); engine.setDoc(doc); }
          })),
          UI.field('Fade vào (giây)', UI.input({
            type: 'number', step: '0.1', value: String(it.fadeIn || 0),
            oninput: function (e) { it.fadeIn = Math.max(0, Number(e.target.value) || 0); markDirty(); engine.setDoc(doc); }
          })),
          UI.field('Fade ra (giây)', UI.input({
            type: 'number', step: '0.1', value: String(it.fadeOut || 0),
            oninput: function (e) { it.fadeOut = Math.max(0, Number(e.target.value) || 0); markDirty(); engine.setDoc(doc); }
          }))),
        h('div', { class: 'row mt-8' },
          UI.btn('✂ Tách tại playhead', { variant: 'ghost', small: true, onclick: splitSel }),
          UI.btn('🗑 Xoá khối', { variant: 'ghost', small: true, onclick: deleteSel })));
    }

    // splitSel tách khối đang chọn thành hai tại vị trí playhead.
    function splitSel() {
      var it = st.sel, tr = st.selTrack;
      if (!it || !tr) return;
      var t = video.currentTime;
      if (t <= it.at + MIN_ITEM || t >= it.at + itemDur(it) - MIN_ITEM) {
        UI.toast('Playhead phải nằm trong khối, cách mép ít nhất ' + MIN_ITEM + 's.', 'error');
        return;
      }
      var cut = it.in + (t - it.at); // điểm cắt tính theo file nguồn
      var right = JSON.parse(JSON.stringify(it));
      right.id = uid('i');
      right.at = t;
      right.in = cut;
      right.fadeIn = 0;
      it.out = cut;
      it.fadeOut = 0;
      tr.items.push(right);
      markDirty(); repaint(); refreshProps();
    }

    function deleteSel() {
      if (!st.sel || !st.selTrack) return;
      var tr = st.selTrack, it = st.sel;
      tr.items = tr.items.filter(function (x) { return x !== it; });
      st.sel = null;
      markDirty(); repaint(); refreshProps();
    }

    // --- thêm lớp / thêm đoạn
    function addTrack() {
      doc.tracks.push({
        id: uid('t'), name: 'Lớp ' + (doc.tracks.length), role: 'sfx',
        gain: 0, mute: false, duck: false, items: []
      });
      markDirty(); repaint(); refreshTracks();
    }

    function addItemTo(track) {
      var auds = assets.filter(function (a) { return a.kind === 'audio'; });
      if (!auds.length) { UI.toast('Dự án chưa có file âm thanh nào.', 'error'); return; }
      var pick = auds[0].path;
      var sel = UI.select('Chọn file', auds.map(function (a) {
        return { value: a.path, label: a.name + (a.duration ? ' · ' + UI.fmtDur(a.duration) : '') };
      }), pick, function (v) { pick = v; });
      var m = UI.modal({
        title: 'Thêm đoạn vào ' + track.name,
        body: h('div', null, sel, h('div', { class: 'muted mt-8', style: { fontSize: '12px' } },
          'Đoạn được đặt tại playhead. Kéo để dời, kéo mép để cắt.')),
        actions: [UI.btn('Thêm', {
          onclick: function () {
            var a = auds.filter(function (x) { return x.path === pick; })[0];
            track.items.push({
              id: uid('i'), name: a.name, path: a.path,
              at: video.currentTime || 0, in: 0, out: a.duration || 0,
              gain: 0, fadeIn: 0, fadeOut: 0
            });
            m.close(); markDirty(); repaint();
            engine.preload(doc).then(function () { repaint(); });
          }
        })]
      });
    }

    // --- bảng điều khiển lớp
    var tracksHost = h('div');

    // narrowNum — ô số hẹp. UI.input cố tình không nhận style (nó chỉ dựng thẻ
    // input chuẩn), mà class .input là width:100% nên không đặt tay thì ô dB ăn
    // hết cả hàng và bảng lớp vỡ thành từng dòng dài.
    function narrowNum(value, onInput) {
      var el = UI.input({ type: 'number', step: '1', value: String(value || 0), oninput: onInput });
      el.style.width = '68px';
      el.style.flex = 'none';
      return el;
    }

    function refreshTracks() {
      tracksHost.innerHTML = '';
      doc.tracks.forEach(function (t) {
        var isSrc = t.role === 'source';

        var roleCell = h('div', { style: { width: '130px', flex: 'none' } },
          isSrc ? h('span', { class: 'badge badge-gray' }, 'tiếng gốc')
            : UI.select(null, ROLES, t.role, function (v) {
              t.role = v; markDirty(); repaint(); refreshTracks();
            }));

        var row = h('div', {
          class: 'row',
          style: {
            gap: '8px', flexWrap: 'nowrap', alignItems: 'center',
            padding: '7px 0', borderTop: '1px solid var(--border)', overflowX: 'auto'
          }
        },
          h('span', {
            style: {
              fontWeight: '600', fontSize: '12px', width: '130px', flex: 'none',
              overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'
            }, title: t.name || t.id
          }, t.name || t.id),
          roleCell,
          UI.btn(t.mute ? '🔇 Tắt' : '🔊 Bật', {
            variant: 'ghost', small: true,
            onclick: function () {
              t.mute = !t.mute; markDirty(); engine.setDoc(doc); refreshTracks();
              if (!video.paused) engine.resync();
            }
          }),
          narrowNum(t.gain, function (e) {
            t.gain = clamp(Number(e.target.value) || 0, -60, 12); markDirty(); engine.setDoc(doc);
          }),
          h('span', { class: 'muted', style: { fontSize: '12px', flex: 'none' } }, 'dB'),
          t.role === 'music' ? UI.btn(t.duck ? '✓ Né lời đọc' : 'Né lời đọc', {
            variant: t.duck ? 'primary' : 'ghost', small: true,
            onclick: function () {
              t.duck = !t.duck; markDirty(); engine.setDoc(doc); refreshTracks();
              if (!video.paused) engine.resync();
            }
          }) : h('span', { style: { flex: 'none' } }),
          isSrc ? h('span', { style: { flex: 'none' } })
            : UI.btn('+ đoạn', { variant: 'ghost', small: true, onclick: function () { addItemTo(t); } }),
          isSrc ? h('span', { style: { flex: 'none' } })
            : UI.btn('🗑', {
              variant: 'ghost', small: true, title: 'Xoá lớp',
              onclick: function () {
                doc.tracks = doc.tracks.filter(function (x) { return x !== t; });
                markDirty(); repaint(); refreshTracks();
              }
            }));
        tracksHost.appendChild(row);
      });
    }

    // --- lưu & dựng
    var saveBtn = UI.btn('💾 Lưu bản dựng', {
      onclick: function () {
        saveBtn.disabled = true;
        API.put('/api/projects/' + encodeURIComponent(projectID) + '/timeline', doc)
          .then(function (saved) {
            doc = saved; st.dirty = false;
            UI.toast('Đã lưu bản dựng');
            repaint(); refreshTracks(); refreshProps();
          })
          .catch(function (e) { saveBtn.disabled = false; UI.toast('Lưu thất bại: ' + e.message, 'error'); });
      }
    });
    saveBtn.disabled = true;

    var renderBtn = UI.btn('⬆ Dựng video', {
      variant: 'ghost',
      onclick: function () {
        var go = function () {
          API.post('/api/projects/' + encodeURIComponent(projectID) + '/timeline/render')
            .then(function () { UI.toast('Đã xếp hàng — theo dõi ở Nhật ký.'); })
            .catch(function (e) { UI.toast('Không dựng được: ' + e.message, 'error'); });
        };
        // Dựng bản chưa lưu là dựng ra một video khác cái đang nghe — lưu trước.
        if (st.dirty) {
          API.put('/api/projects/' + encodeURIComponent(projectID) + '/timeline', doc)
            .then(function () { st.dirty = false; saveBtn.disabled = true; go(); })
            .catch(function (e) { UI.toast('Lưu trước khi dựng thất bại: ' + e.message, 'error'); });
        } else { go(); }
      }
    });

    var projSel = UI.select(null, projects.map(function (p) { return { value: p.id, label: p.name }; }),
      projectID, function (v) { engine.destroy(); location.hash = '#/editor/' + v; });

    var zoomSlider = UI.slider(null, {
      min: 2, max: 60, step: 1, value: st.zoom,
      oninput: function (v) { st.zoom = v; repaint(); }
    });

    // --- phím tắt
    function onKey(e) {
      if (/^(INPUT|TEXTAREA|SELECT)$/.test((e.target.tagName || ''))) return;
      if (e.key === ' ') { e.preventDefault(); video.paused ? video.play() : video.pause(); }
      else if (e.key === 's' || e.key === 'S') { e.preventDefault(); splitSel(); }
      else if (e.key === 'Delete' || e.key === 'Backspace') { e.preventDefault(); deleteSel(); }
      else if (e.key === 'ArrowLeft') { video.currentTime = Math.max(0, video.currentTime - (e.shiftKey ? 1 : 0.1)); }
      else if (e.key === 'ArrowRight') { video.currentTime = video.currentTime + (e.shiftKey ? 1 : 0.1); }
    }
    document.addEventListener('keydown', onKey);
    // Router gọi App._cleanup trước khi vẽ trang mới. Không gỡ thì phím tắt còn
    // sống: bấm Space ở trang khác lại tua một video đã bị vứt, và AudioContext
    // rò ra mỗi lần vào lại.
    App._cleanup = function () { document.removeEventListener('keydown', onKey); engine.destroy(); };

    var resultLine = h('div');

    // --- lắp ráp
    el.appendChild(h('div', { class: 'card' },
      h('div', { class: 'row-between', style: { flexWrap: 'wrap', gap: '8px' } },
        h('div', { class: 'row', style: { gap: '8px' } },
          h('span', { class: 'muted', style: { fontSize: '12px' } }, 'Dự án'), projSel),
        h('div', { class: 'row', style: { gap: '8px' } },
          UI.btn('✂ Cắt khoảng lặng', {
            variant: 'ghost',
            onclick: function () { EditorTools.openAutocut(assets, resultLine); }
          }),
          h('a', { class: 'btn btn-ghost', href: '#/projects/' + projectID }, '📦 Mở dự án'),
          renderBtn, saveBtn)),
      resultLine));

    el.appendChild(h('div', { class: 'grid-2 mt-16', style: { alignItems: 'start' } },
      h('div', { class: 'card' }, h('div', { class: 'card-title' }, '🖥️ Xem trước'), stage,
        h('div', { class: 'muted', style: { fontSize: '11.5px', marginTop: '8px', lineHeight: '1.6' } },
          'Nghe thử trộn ngay trong trình duyệt nên đúng bằng bản xuất ra. Riêng né giọng ở đây là ' +
          'bản gần đúng — khác biệt nằm ở đường cong lên xuống của nhạc, không nằm ở việc nhạc có lùi hay không.')),
      h('div', { class: 'card' }, h('div', { class: 'card-title' }, '🧾 Khối đang chọn'), propsHost)));

    el.appendChild(h('div', { class: 'card mt-16' },
      h('div', { class: 'row-between', style: { marginBottom: '8px', flexWrap: 'wrap', gap: '8px' } },
        h('div', { class: 'card-title', style: { margin: 0 } }, '🎚️ Dòng thời gian'),
        h('div', { class: 'row', style: { gap: '8px', flexWrap: 'nowrap', width: '240px' } },
          h('span', { class: 'muted', style: { fontSize: '12px', flex: 'none' } }, '🔍'), zoomSlider)),
      wrap,
      h('div', { class: 'muted', style: { fontSize: '11.5px', marginTop: '6px' } },
        'Kéo khối để dời · kéo mép để cắt · Space phát/dừng · S tách tại playhead · Delete xoá · ←/→ tua'),
      h('div', { class: 'row mt-8', style: { gap: '8px' } },
        UI.btn('+ Lớp âm thanh', { variant: 'ghost', small: true, onclick: addTrack }),
        UI.btn('+ Dòng phụ đề', {
          variant: 'ghost', small: true,
          onclick: function () {
            var t = video.currentTime || 0;
            doc.subs.push({ id: uid('c'), start: t, end: t + 2, text: 'Dòng phụ đề mới' });
            markDirty(); repaint();
          }
        })),
      tracksHost));

    repaint();
    refreshTracks();
    refreshProps();
  }

  function maxEnd(doc) {
    var m = 0;
    (doc.tracks || []).forEach(function (t) {
      (t.items || []).forEach(function (it) {
        var e = it.at + itemDur(it);
        if (e > m) m = e;
      });
    });
    (doc.subs || []).forEach(function (c) { if (c.end > m) m = c.end; });
    return m;
  }
})();
