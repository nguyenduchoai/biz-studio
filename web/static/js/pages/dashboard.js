/* ============================================================
   Biz Studio — Trang Tổng quan (dashboard)
   ============================================================ */
(function () {
  'use strict';

  var PROJECT_STATUS = {
    done:    { label: 'Hoàn thành', cls: 'badge-green' },
    running: { label: 'Đang chạy',  cls: 'badge-amber' },
    draft:   { label: 'Nháp',       cls: 'badge-gray'  },
    error:   { label: 'Lỗi',        cls: 'badge-red'   }
  };
  var JOB_STATUS = {
    queued:  { label: 'Chờ',        cls: 'badge-gray'  },
    running: { label: 'Đang chạy',  cls: 'badge-amber' },
    done:    { label: 'Hoàn thành', cls: 'badge-green' },
    error:   { label: 'Lỗi',        cls: 'badge-red'   }
  };
  var JOB_KIND = {
    download: 'Tải video', render: 'Render', qc: 'QC tự động', tts: 'TTS',
    translate: 'Dịch thuật', ocr: 'OCR', asr: 'ASR', vox: 'Vox',
    autocut: 'Cắt tự động', publish: 'Gói xuất bản', thumbnail: 'Thumbnail'
  };
  var THUMB_COLORS = ['#2563EB', '#7C3AED', '#DB2777', '#059669', '#D97706', '#0891B2'];

  function badge(map, status) {
    var m = map[status] || { label: status || '—', cls: 'badge-gray' };
    return h('span', { class: 'badge ' + m.cls }, m.label);
  }

  function thumbColor(id) {
    var s = 0;
    for (var i = 0; i < (id || '').length; i++) s += id.charCodeAt(i);
    return THUMB_COLORS[s % THUMB_COLORS.length];
  }

  function ensureCss() {
    if (document.getElementById('dash-style')) return;
    var st = document.createElement('style');
    st.id = 'dash-style';
    st.textContent =
      '.dash-main{display:grid;grid-template-columns:minmax(0,2fr) minmax(0,1fr);gap:16px;margin-top:16px;align-items:start}' +
      '@media(max-width:1100px){.dash-main{grid-template-columns:1fr}}' +
      '.dash-pcard{border:1px solid var(--border);border-radius:12px;overflow:hidden;cursor:pointer;background:var(--card);transition:border-color .12s ease,transform .12s ease}' +
      '.dash-pcard:hover{border-color:var(--blue);transform:translateY(-1px)}' +
      '.dash-pthumb{height:96px;background-size:cover;background-position:center}' +
      '.dash-pbody{padding:10px 12px}' +
      '.dash-pname{font-weight:600;font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}' +
      '.dash-pgrid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}' +
      '@media(max-width:900px){.dash-pgrid{grid-template-columns:repeat(2,minmax(0,1fr))}}' +
      '.dash-jrow{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:9px 0;border-bottom:1px solid var(--border)}' +
      '.dash-jrow:last-child{border-bottom:none}';
    document.head.appendChild(st);
  }

  // ---------- Stat cards ----------

  function statCard(label, value, sub) {
    return UI.card({
      body: h('div', { class: 'stat-card' },
        h('div', { class: 'stat-label' }, label),
        h('div', { class: 'stat-value' }, String(value)),
        sub ? h('div', { class: 'stat-sub' }, sub) : null)
    });
  }

  function toolRow(name, ok) {
    return h('div', { class: 'row-between', style: { padding: '1px 0' } },
      h('span', { style: { fontSize: '12px' } }, name),
      h('span', { style: { fontSize: '10px', color: ok ? 'var(--green)' : 'var(--red)' }, title: ok ? 'Sẵn sàng' : 'Chưa sẵn sàng' }, '●'));
  }

  function renderStats(host, st) {
    host.innerHTML = '';
    var counts = (st && st.counts) || {};
    var tools = (st && st.tools) || {};
    host.appendChild(statCard('Dự án', counts.projects !== undefined ? counts.projects : '—', 'Tổng số dự án trong studio'));
    host.appendChild(statCard('Tác vụ đang chạy', counts.jobsRunning !== undefined ? counts.jobsRunning : '—', 'Job nền đang xử lý'));
    host.appendChild(UI.card({
      body: h('div', { class: 'stat-card' },
        h('div', { class: 'stat-label' }, 'Công cụ'),
        h('div', null,
          toolRow('ffmpeg', !!tools.ffmpeg),
          toolRow('claude', !!tools.claude),
          toolRow('yt-dlp', !!tools.ytdlp),
          toolRow('Gemini', !!tools.geminiKey)))
    }));
  }

  // ---------- Dự án gần đây ----------

  function projectCard(p) {
    var thumbStyle = p.thumbFile
      ? { backgroundImage: 'url("/data/' + p.thumbFile + '")' }
      : { background: thumbColor(p.id) };
    return h('div', { class: 'dash-pcard', onclick: function () { App.navigate('projects/' + p.id); } },
      h('div', { class: 'dash-pthumb', style: thumbStyle }),
      h('div', { class: 'dash-pbody' },
        h('div', { class: 'dash-pname', title: p.name }, p.name),
        h('div', { class: 'row-between', style: { marginTop: '6px' } },
          badge(PROJECT_STATUS, p.status),
          h('span', { class: 'muted', style: { fontSize: '11.5px' } }, p.width + '×' + p.height)),
        h('div', { class: 'muted', style: { fontSize: '11px', marginTop: '4px' } }, UI.timeAgo(p.updatedAt))));
  }

  function loadProjects(host) {
    host.innerHTML = '';
    host.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải dự án…'));
    API.get('/api/projects').then(function (list) {
      list = (list || []).slice().sort(function (a, b) {
        return new Date(b.updatedAt) - new Date(a.updatedAt);
      }).slice(0, 6);
      host.innerHTML = '';
      if (!list.length) {
        host.appendChild(UI.empty('Chưa có dự án nào — bấm "＋ Tạo dự án" để bắt đầu.', '📁'));
        return;
      }
      var grid = h('div', { class: 'dash-pgrid' });
      list.forEach(function (p) { grid.appendChild(projectCard(p)); });
      host.appendChild(grid);
    }).catch(function (e) {
      host.innerHTML = '';
      host.appendChild(UI.empty('Không tải được danh sách dự án: ' + e.message, '❌'));
    });
  }

  // ---------- Tác vụ gần đây ----------

  function jobRow(j) {
    var right;
    if (j.status === 'running') {
      var bar = UI.progress(j.progress);
      bar.style.width = '70px';
      right = h('div', { class: 'row', style: { flex: 'none' } },
        h('span', { class: 'muted', style: { fontSize: '11px' } }, Math.round(j.progress) + '%'), bar);
    } else {
      right = badge(JOB_STATUS, j.status);
    }
    return h('div', { class: 'dash-jrow' },
      h('div', { style: { minWidth: 0 } },
        h('div', { style: { fontSize: '13px', fontWeight: 600 } }, JOB_KIND[j.kind] || j.kind),
        h('div', {
          class: 'muted',
          title: j.status === 'error' ? j.error : j.detail,
          style: { fontSize: '11.5px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: '230px' }
        }, j.status === 'error' ? (j.error || 'Lỗi không rõ') : (j.detail || UI.timeAgo(j.createdAt)))),
      right);
  }

  // ---------- Trang ----------

  App.pages['dashboard'] = {
    title: 'Tổng quan',
    subtitle: 'Toàn cảnh studio: dự án, tác vụ và công cụ',
    render: function (el) {
      ensureCss();
      var jobs = [];

      var statsHost = h('div', { class: 'grid-3' });
      var projectsHost = h('div');
      var jobsHost = h('div');

      el.appendChild(statsHost);
      el.appendChild(h('div', { class: 'dash-main' },
        UI.card({
          body: [h('div', { class: 'row-between', style: { marginBottom: '12px' } },
            h('div', { class: 'card-title', style: { margin: 0 } }, '📁 Dự án gần đây'),
            UI.btn('＋ Tạo dự án', { small: true, onclick: function () { App.navigate('projects'); } })),
          projectsHost]
        }),
        UI.card({ title: 'Tác vụ gần đây', icon: '⚙️', body: jobsHost })));

      renderStats(statsHost, App.state);
      loadProjects(projectsHost);

      function renderJobs() {
        jobsHost.innerHTML = '';
        if (!jobs.length) {
          jobsHost.appendChild(UI.empty('Chưa có tác vụ nào', '💤'));
          return;
        }
        jobs.forEach(function (j) { jobsHost.appendChild(jobRow(j)); });
      }

      jobsHost.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải tác vụ…'));
      API.get('/api/jobs').then(function (list) {
        jobs = (list || []).slice().sort(function (a, b) {
          return new Date(b.createdAt) - new Date(a.createdAt);
        }).slice(0, 8);
        renderJobs();
      }).catch(function (e) {
        jobsHost.innerHTML = '';
        jobsHost.appendChild(UI.empty('Không tải được tác vụ: ' + e.message, '❌'));
      });

      var onState = Bus.on('state', function (st) { renderStats(statsHost, st); });
      var onJob = Bus.on('job', function (j) {
        var found = false;
        for (var i = 0; i < jobs.length; i++) {
          if (jobs[i].id === j.id) { jobs[i] = j; found = true; break; }
        }
        if (!found) jobs.unshift(j);
        if (jobs.length > 8) jobs.length = 8;
        renderJobs();
      });

      App._cleanup = function () {
        Bus.off('state', onState);
        Bus.off('job', onJob);
      };
    }
  };
})();
