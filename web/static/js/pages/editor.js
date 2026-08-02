/* ============================================================
   Biz Studio — Trang "Studio Editor" (lite)
   3 vùng: thư viện media / xem trước / thuộc tính + timeline.
   Đăng ký App.pages['editor'].
   ============================================================ */
(function () {
  'use strict';

  var KIND_LABELS = { video: 'Video', image: 'Ảnh', audio: 'Âm thanh', other: 'Khác' };

  function dataURL(rel) {
    return '/data/' + String(rel).split('/').map(encodeURIComponent).join('/');
  }

  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(
        function () { UI.toast('Đã sao chép đường dẫn.'); },
        function () { UI.toast('Không sao chép được đường dẫn.', 'error'); });
    } else {
      UI.toast('Trình duyệt không hỗ trợ sao chép tự động.', 'error');
    }
  }

  App.pages['editor'] = {
    title: 'Studio Editor',
    subtitle: 'Xem media của dự án, timeline trực quan và xử lý nhanh',
    render: render
  };

  function render(root, param) {
    var st = { projectId: param || '', detail: null, selected: null, zoom: 6, handlers: [] };
    App._cleanup = function () {
      st.handlers.forEach(function (fn) { Bus.off('job', fn); });
      st.handlers = [];
    };

    var previewBox = null, previewBar = null, propsHost = null, trackV = null, trackA = null;

    // ---------- Hàng đầu: chọn dự án ----------
    var projSel = h('select', { class: 'select', style: { maxWidth: '360px' } });
    projSel.onchange = function () { loadProject(projSel.value); };
    var headCard = h('div', { class: 'card' },
      h('div', { class: 'row' },
        h('span', { class: 'field-label' }, 'Dự án:'), projSel,
        UI.btn('↻ Tải lại', {
          variant: 'ghost', small: true,
          onclick: function () { st.projectId ? loadProject(st.projectId) : loadProjects(); }
        })));
    var bodyHost = h('div', { class: 'mt-16' });
    root.appendChild(headCard);
    root.appendChild(bodyHost);
    showEmpty();
    loadProjects();

    function showEmpty() {
      bodyHost.innerHTML = '';
      bodyHost.appendChild(h('div', { class: 'card' },
        UI.empty('Chọn một dự án ở trên để mở Studio Editor.', '🎛️')));
    }

    async function loadProjects() {
      var projects = [];
      try {
        projects = (await API.get('/api/projects')) || [];
      } catch (err) {
        UI.toast('Không tải được danh sách dự án: ' + err.message, 'error');
      }
      projSel.innerHTML = '';
      projSel.appendChild(h('option', { value: '' },
        projects.length ? '— Chọn dự án —' : 'Chưa có dự án nào'));
      projects.forEach(function (p) {
        projSel.appendChild(h('option', { value: p.id }, p.name + (p.kind ? ' · ' + p.kind : '')));
      });
      if (st.projectId) {
        projSel.value = st.projectId;
        if (projSel.value === st.projectId) loadProject(st.projectId);
        else { st.projectId = ''; }
      }
    }

    async function loadProject(id) {
      st.projectId = id;
      st.selected = null;
      if (!id) { st.detail = null; showEmpty(); return; }
      try {
        st.detail = await API.get('/api/projects/' + encodeURIComponent(id));
      } catch (err) {
        st.detail = null;
        UI.toast('Không tải được dự án: ' + err.message, 'error');
        showEmpty();
        return;
      }
      renderStudio();
    }

    // ---------- Studio 3 vùng + timeline ----------
    function renderStudio() {
      bodyHost.innerHTML = '';
      var assets = (st.detail && st.detail.assets) || [];
      bodyHost.appendChild(h('div', {
        style: { display: 'grid', gridTemplateColumns: '250px minmax(0,1fr) 280px', gap: '16px', alignItems: 'start' }
      }, libraryCard(assets), previewCard(), propsCard()));
      bodyHost.appendChild(timelineCard(assets));
      updatePreview();
      updateProps();
    }

    function selectAsset(a) {
      st.selected = a;
      updatePreview();
      updateProps();
    }

    // --- Trái: thư viện media (tabs Media / SFX)
    function libraryCard(assets) {
      var gridEl = h('div', { style: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px' } });
      var tabMedia = h('button', { class: 'btn btn-sm btn-primary' }, 'Media');
      var tabSfx = h('button', { class: 'btn btn-sm btn-ghost' }, 'SFX');
      function setTab(t) {
        tabMedia.className = 'btn btn-sm ' + (t === 'media' ? 'btn-primary' : 'btn-ghost');
        tabSfx.className = 'btn btn-sm ' + (t === 'sfx' ? 'btn-primary' : 'btn-ghost');
        gridEl.innerHTML = '';
        if (t === 'sfx') {
          gridEl.appendChild(h('div', { style: { gridColumn: '1 / -1' } },
            UI.empty('Thư viện SFX sẽ sớm có mặt.', '🔔')));
          return;
        }
        fillAssets();
      }
      tabMedia.onclick = function () { setTab('media'); };
      tabSfx.onclick = function () { setTab('sfx'); };

      function thumb(a) {
        var inner;
        if (a.kind === 'image') {
          inner = h('img', {
            src: dataURL(a.path), alt: a.name,
            style: { width: '100%', height: '64px', objectFit: 'cover', borderRadius: '8px', display: 'block' }
          });
        } else {
          inner = h('div', {
            style: { height: '64px', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '26px', background: 'var(--bg)', borderRadius: '8px' }
          }, a.kind === 'video' ? '🎬' : a.kind === 'audio' ? '🎵' : '📄');
        }
        return h('div', {
          title: a.name,
          style: { cursor: 'pointer', border: '1px solid var(--border)', borderRadius: '10px', padding: '6px' },
          onclick: function () { selectAsset(a); }
        }, inner,
        h('div', { style: { fontSize: '11px', marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }, a.name));
      }

      function fillAssets() {
        gridEl.innerHTML = '';
        if (!assets.length) {
          gridEl.appendChild(h('div', { style: { gridColumn: '1 / -1' } },
            UI.empty('Dự án chưa có media nào.', '🗂️')));
          return;
        }
        assets.forEach(function (a) { gridEl.appendChild(thumb(a)); });
      }
      fillAssets();

      return h('div', { class: 'card' },
        h('div', { class: 'card-title' }, '🗂️ Thư viện media'),
        h('div', { class: 'row', style: { marginBottom: '10px' } }, tabMedia, tabSfx),
        gridEl);
    }

    // --- Giữa: xem trước
    function previewCard() {
      previewBox = h('div', {
        style: { background: '#000', borderRadius: '12px', minHeight: '300px', display: 'flex', alignItems: 'center', justifyContent: 'center', overflow: 'hidden' }
      });
      previewBar = h('div', { class: 'row-between mt-8 muted', style: { fontSize: '12px' } });
      return h('div', { class: 'card' },
        h('div', { class: 'card-title' }, '🖥️ Xem trước'), previewBox, previewBar);
    }

    function updatePreview() {
      if (!previewBox) return;
      previewBox.innerHTML = '';
      previewBar.innerHTML = '';
      var item = st.selected;
      var proj = st.detail && st.detail.project;
      if (!item && proj && proj.outputFile) {
        previewBox.appendChild(h('video', {
          controls: 'controls', src: dataURL(proj.outputFile), style: { width: '100%', maxHeight: '420px' }
        }));
        previewBar.appendChild(h('span', null, 'Video đầu ra: ' + proj.outputFile));
        return;
      }
      if (!item) {
        previewBox.appendChild(h('div', { class: 'muted', style: { padding: '40px 16px', textAlign: 'center' } },
          'Chọn media trong thư viện để xem trước'));
        return;
      }
      var url = dataURL(item.path);
      if (item.kind === 'video') {
        previewBox.appendChild(h('video', { controls: 'controls', src: url, style: { width: '100%', maxHeight: '420px' } }));
      } else if (item.kind === 'image') {
        previewBox.appendChild(h('img', { src: url, alt: item.name, style: { maxWidth: '100%', maxHeight: '420px' } }));
      } else if (item.kind === 'audio') {
        previewBox.appendChild(h('div', { style: { padding: '30px 16px', width: '100%', textAlign: 'center' } },
          h('div', { style: { fontSize: '40px', marginBottom: '10px' } }, '🎵'),
          h('audio', { controls: 'controls', src: url, style: { width: '100%' } })));
      } else {
        previewBox.appendChild(h('div', { class: 'muted', style: { padding: '40px' } },
          'Không hỗ trợ xem trước loại file này'));
      }
      previewBar.appendChild(h('span', {
        style: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }
      }, item.name));
      previewBar.appendChild(h('span', null, item.duration ? UI.fmtDur(item.duration) : ''));
    }

    // --- Phải: thuộc tính
    function propsCard() {
      propsHost = h('div');
      return h('div', { class: 'card' }, h('div', { class: 'card-title' }, '🧾 Thuộc tính'), propsHost);
    }

    function propRow(label, valueNode) {
      return h('tr', null,
        h('td', { class: 'muted', style: { width: '38%' } }, label),
        h('td', null, valueNode));
    }

    function updateProps() {
      if (!propsHost) return;
      propsHost.innerHTML = '';
      var a = st.selected;
      if (!a) {
        propsHost.appendChild(h('div', { class: 'muted' }, 'Chưa chọn media nào.'));
        return;
      }
      propsHost.appendChild(h('table', { class: 'table' }, h('tbody', null,
        propRow('Tên', a.name),
        propRow('Loại', KIND_LABELS[a.kind] || a.kind),
        propRow('Kích thước', UI.fmtBytes(a.size)),
        propRow('Thời lượng', a.duration ? UI.fmtDur(a.duration) : '—'),
        propRow('Đường dẫn', h('div', null,
          h('code', { style: { fontSize: '11px', wordBreak: 'break-all' } }, a.path),
          h('div', { class: 'mt-8' }, UI.btn('📋 Sao chép', {
            variant: 'ghost', small: true, onclick: function () { copyText(a.path); }
          })))))));
    }

    // --- Timeline
    function block(a, color) {
      var w = Math.max(46, Math.round((Number(a.duration) || 4) * st.zoom));
      return h('div', {
        title: a.name + (a.duration ? ' · ' + UI.fmtDur(a.duration) : ''),
        onclick: function () { selectAsset(a); },
        style: { flex: 'none', width: w + 'px', height: '34px', lineHeight: '34px', borderRadius: '6px', background: color, color: '#fff', fontSize: '11px', padding: '0 8px', overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis', cursor: 'pointer' }
      }, a.name);
    }

    function fillTracks(assets) {
      if (!trackV || !trackA) return;
      trackV.innerHTML = '';
      trackA.innerHTML = '';
      var vids = assets.filter(function (a) { return a.kind === 'video'; });
      var auds = assets.filter(function (a) { return a.kind === 'audio'; });
      if (!vids.length) trackV.appendChild(h('span', { class: 'muted', style: { fontSize: '11px', lineHeight: '34px' } }, 'Chưa có video'));
      vids.forEach(function (a) { trackV.appendChild(block(a, 'var(--blue-h)')); });
      if (!auds.length) trackA.appendChild(h('span', { class: 'muted', style: { fontSize: '11px', lineHeight: '34px' } }, 'Chưa có audio'));
      auds.forEach(function (a) { trackA.appendChild(block(a, 'var(--green)')); });
    }

    function trackRow(label, track) {
      return h('div', { class: 'row', style: { gap: '10px', flexWrap: 'nowrap', borderTop: '1px solid var(--border)', padding: '5px 0' } },
        h('span', { class: 'badge badge-gray', style: { flex: 'none', width: '34px', justifyContent: 'center' } }, label),
        track);
    }

    function timelineCard(assets) {
      trackV = h('div', { class: 'row', style: { gap: '6px', flexWrap: 'nowrap', overflowX: 'auto', minWidth: 0, flex: '1' } });
      trackA = h('div', { class: 'row', style: { gap: '6px', flexWrap: 'nowrap', overflowX: 'auto', minWidth: 0, flex: '1' } });
      var resultLine = h('div', { class: 'mt-8' });
      var zoomRow = UI.slider(null, {
        min: 2, max: 20, step: 1, value: st.zoom,
        oninput: function (v) { st.zoom = v; fillTracks(assets); }
      });
      var toolbar = h('div', { class: 'row-between', style: { marginBottom: '8px', flexWrap: 'wrap', gap: '8px' } },
        h('div', { class: 'row' },
          UI.btn('✂ Cắt khoảng lặng', { variant: 'ghost', small: true, onclick: function () { openAutocut(assets, resultLine); } }),
          UI.btn('⬆ Render final', { variant: 'ghost', small: true, onclick: function () { renderFinal(resultLine); } }),
          UI.btn('📦 Xuất', {
            variant: 'ghost', small: true,
            onclick: function () { location.hash = '#/projects/' + st.projectId; }
          })),
        h('div', { class: 'row', style: { width: '230px', flexWrap: 'nowrap' } },
          h('span', { class: 'muted', style: { fontSize: '12px', flex: 'none' } }, '🔍 Zoom'), zoomRow));
      fillTracks(assets);
      return h('div', { class: 'card mt-16' },
        h('div', { class: 'card-title' }, '🎚️ Timeline'),
        toolbar,
        trackRow('V1', trackV),
        trackRow('A1', trackA),
        resultLine);
    }

    // --- Cắt khoảng lặng (modal)
    function openAutocut(assets, resultLine) {
      var vids = assets.filter(function (a) { return a.kind === 'video'; });
      if (!vids.length) { UI.toast('Dự án chưa có video nào để cắt.', 'error'); return; }
      var chosen = vids[0].id, silenceDb = -35, minSilence = 0.8;
      var m = UI.modal({
        title: '✂ Cắt khoảng lặng',
        body: h('div', null,
          UI.select('Video cần cắt', vids.map(function (a) { return { value: a.id, label: a.name }; }),
            chosen, function (v) { chosen = v; }),
          UI.slider('Ngưỡng im lặng (dB)', { min: -50, max: -20, step: 1, value: -35, oninput: function (v) { silenceDb = v; } }),
          UI.slider('Im lặng tối thiểu (giây)', { min: 0.3, max: 2, step: 0.1, value: 0.8, oninput: function (v) { minSilence = v; } })),
        actions: [
          UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
          UI.btn('Bắt đầu cắt', {
            variant: 'primary',
            onclick: async function () {
              var asset = null;
              vids.forEach(function (a) { if (a.id === chosen) asset = a; });
              if (!asset) { UI.toast('Không tìm thấy video đã chọn.', 'error'); return; }
              try {
                var job = await API.post('/api/tools/autocut',
                  { path: asset.path, silenceDb: silenceDb, minSilence: minSilence });
                m.close();
                UI.toast('Đã bắt đầu cắt khoảng lặng…');
                watchJob(job, resultLine, 'Cắt khoảng lặng');
              } catch (err) {
                UI.toast('Không chạy được cắt khoảng lặng: ' + err.message, 'error');
              }
            }
          })]
      });
    }

    // --- Render final
    async function renderFinal(resultLine) {
      if (!st.projectId) { UI.toast('Hãy chọn dự án trước.', 'error'); return; }
      try {
        var job = await API.post('/api/projects/' + encodeURIComponent(st.projectId) + '/render-final');
        UI.toast('Đã bắt đầu render final…');
        watchJob(job, resultLine, 'Render final');
      } catch (err) {
        UI.toast('Không chạy được render final: ' + err.message, 'error');
      }
    }

    // --- Theo dõi job qua SSE
    function watchJob(job, host, label) {
      var prog = UI.progress(job.progress || 0);
      var detail = h('span', { class: 'muted', style: { fontSize: '12px' } }, job.detail || 'Đang chạy…');
      host.innerHTML = '';
      host.appendChild(h('div', null,
        h('div', { class: 'row' }, UI.spinner(), h('b', null, label + '…'), detail),
        h('div', { class: 'mt-8' }, prog)));
      var fn = Bus.on('job', function (j) {
        if (!j || j.id !== job.id) return;
        prog.set(j.progress || 0);
        if (j.detail) detail.textContent = j.detail;
        if (j.status === 'done') {
          Bus.off('job', fn);
          UI.toast(label + ' hoàn tất!');
          host.innerHTML = '';
          if (j.output && j.output.charAt(0) !== '/') {
            host.appendChild(h('div', { class: 'row' },
              h('span', null, '✅ ' + label + ' xong:'),
              h('a', { class: 'btn btn-ghost btn-sm', href: dataURL(j.output), target: '_blank' }, '▶ Mở kết quả'),
              h('span', { class: 'muted', style: { fontSize: '12px', wordBreak: 'break-all' } }, j.output)));
          } else {
            host.appendChild(h('div', { class: 'row' },
              h('span', null, '✅ ' + label + ' xong:'),
              h('code', { style: { fontSize: '12px', wordBreak: 'break-all' } }, j.output || '(không rõ)'),
              UI.btn('📋', { variant: 'ghost', small: true, onclick: function () { copyText(j.output || ''); } })));
          }
          if (label === 'Render final' && st.projectId) loadProject(st.projectId);
        } else if (j.status === 'error') {
          Bus.off('job', fn);
          host.innerHTML = '';
          host.appendChild(h('div', { class: 'text-red' },
            '❌ ' + label + ' thất bại: ' + (j.error || 'lỗi không xác định')));
          UI.toast(label + ' thất bại: ' + (j.error || 'lỗi không xác định'), 'error');
        }
      });
      st.handlers.push(fn);
    }
  }
})();
