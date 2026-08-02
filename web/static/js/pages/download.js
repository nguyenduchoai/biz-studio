/* ============================================================
   Biz Studio — Trang Tải Video (yt-dlp)
   Load sau app.js. Tự đăng ký App.pages['download'].
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
    if (s === 'running') return { label: 'Đang tải', cls: 'badge badge-blue' };
    return { label: 'Chờ xử lý', cls: 'badge badge-gray' };
  }

  function isRel(p) { return !!p && p.charAt(0) !== '/'; }
  function dataUrl(p) { return '/data/' + p.split('/').map(encodeURIComponent).join('/'); }
  function fileName(p) { return (p || '').split('/').pop(); }

  var VIDEO_EXT = ['mp4', 'mov', 'webm', 'mkv', 'm4v'];
  var AUDIO_EXT = ['mp3', 'm4a', 'wav', 'aac', 'flac', 'opus', 'ogg'];

  function previewEl(url, name) {
    var ext = (name.split('.').pop() || '').toLowerCase();
    if (VIDEO_EXT.indexOf(ext) >= 0) {
      return h('video', { controls: 'controls', src: url, style: { width: '100%', maxHeight: '260px', borderRadius: '10px', marginTop: '8px', background: '#000' } });
    }
    if (AUDIO_EXT.indexOf(ext) >= 0) {
      return h('audio', { controls: 'controls', src: url, style: { width: '100%', marginTop: '8px' } });
    }
    return null;
  }

  function outputView(j) {
    if (!j.output) return h('span', { class: 'muted' }, 'Hoàn thành (không có file đầu ra)');
    if (!isRel(j.output)) {
      return h('div', { class: 'row' },
        h('code', { style: { fontSize: '12px', wordBreak: 'break-all' } }, j.output),
        UI.btn('📋 Copy đường dẫn', { variant: 'ghost', small: true, onclick: function () { copyText(j.output); } }));
    }
    var url = dataUrl(j.output);
    var name = fileName(j.output);
    var host = h('div', null,
      h('div', { class: 'row' },
        h('a', { class: 'btn btn-ghost btn-sm', href: url, download: name }, '⬇ Tải ' + name),
        UI.btn('📋 Copy link', { variant: 'ghost', small: true, onclick: function () { copyText(location.origin + url); } })));
    var pv = previewEl(url, name);
    if (pv) host.appendChild(pv);
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
        h('div', { class: 'row' }, h('strong', null, '⬇️ Tải video'), badge),
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
        outHost.appendChild(outputView(j));
      } else {
        card._done = false;
        outHost.innerHTML = '';
        if (j.status === 'error') {
          outHost.appendChild(h('div', {
            class: 'text-red',
            style: { background: 'rgba(239,68,68,.08)', border: '1px solid var(--red)', borderRadius: '10px', padding: '10px 12px', fontSize: '13px', fontWeight: '600' }
          }, '⚠ ' + (j.error || 'Lỗi không xác định')));
        }
      }
    };
    card.update(job);
    return card;
  }

  // ---------- Trang ----------

  App.pages.download = {
    title: 'Tải Video',
    subtitle: 'Tải video từ YouTube, TikTok, Facebook… bằng yt-dlp — mỗi link một tác vụ',
    render: function (el) {
      var quality = 'best';
      var threads = 4;
      var downloadDir = 'data/downloads';
      var jobEls = {};

      // --- Nguồn liên kết
      var linksTa = UI.textarea({
        placeholder: 'https://www.youtube.com/watch?v=…\nhttps://www.tiktok.com/@…\n(mỗi dòng 1 link)',
        rows: 5
      });

      var dz = UI.dropzone({
        hint: 'Kéo thả file TXT hoặc dán Link — mỗi dòng 1 link',
        sub: 'Nhận file .txt chứa danh sách liên kết, hoặc dán trực tiếp vào ô bên dưới',
        accept: '.txt,text/plain',
        multiple: true,
        onfiles: function (files) {
          files.forEach(function (f) {
            if (f.name && !/\.txt$/i.test(f.name) && f.type !== 'text/plain') {
              UI.toast('Bỏ qua "' + f.name + '" — chỉ nhận file .txt', 'error');
              return;
            }
            var reader = new FileReader();
            reader.onload = function () {
              var txt = String(reader.result || '').trim();
              if (!txt) { UI.toast('File "' + f.name + '" rỗng', 'error'); return; }
              linksTa.value = (linksTa.value.trim() ? linksTa.value.trim() + '\n' : '') + txt;
              UI.toast('Đã thêm link từ ' + f.name);
            };
            reader.onerror = function () { UI.toast('Không đọc được file ' + f.name, 'error'); };
            reader.readAsText(f);
          });
        }
      });

      el.appendChild(UI.card({
        title: 'Nguồn liên kết', icon: '🔗',
        body: h('div', null, dz, h('div', { class: 'mt-8' }, linksTa))
      }));

      // --- Cấu hình tải
      var cfgBody = h('div', null, h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải cấu hình…'));
      el.appendChild(UI.card({ title: 'Cấu hình Tải Video', icon: '⚙️', body: cfgBody }));

      function buildConfig(st) {
        if (st) {
          if (st.quality) quality = st.quality;
          if (st.threads >= 1 && st.threads <= 8) threads = st.threads;
          if (st.downloadDir) downloadDir = st.downloadDir;
        }
        cfgBody.innerHTML = '';
        var qualitySel = UI.select(null, [
          { value: 'best', label: 'Tốt nhất' },
          { value: '1080', label: '1080p' },
          { value: '720', label: '720p' },
          { value: 'audio', label: 'Chỉ âm thanh' }
        ], quality, function (v) { quality = v; });
        var threadsSlider = UI.slider(null, {
          min: 1, max: 8, step: 1, value: threads,
          oninput: function (v) { threads = v; }
        });
        cfgBody.appendChild(h('div', { class: 'grid-2' },
          UI.field('Chất lượng', qualitySel),
          UI.field('Luồng tải (1–8)', threadsSlider)));
        cfgBody.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px' } },
          '📁 Thư mục lưu: ', h('code', null, downloadDir)));
        cfgBody.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '4px' } },
          '🍪 File Cookies: ', st && st.cookiesFile
            ? h('code', null, st.cookiesFile)
            : h('span', null, 'Chưa cấu hình — thiết lập trong ', h('a', { href: '#/settings' }, 'Cấu hình & API'))));
      }

      API.get('/api/settings').then(buildConfig).catch(function (err) {
        UI.toast('Không tải được cấu hình: ' + err.message, 'error');
        buildConfig(null);
      });

      // --- CTA + lỗi inline
      var errBox = h('div', { class: 'mt-8', style: { display: 'none' } });
      function showErr(msg) {
        errBox.style.display = '';
        errBox.innerHTML = '';
        errBox.appendChild(h('div', {
          class: 'text-red',
          style: { background: 'rgba(239,68,68,.08)', border: '1px solid var(--red)', borderRadius: '10px', padding: '10px 12px', fontWeight: '600' }
        }, '⚠ ' + msg));
      }

      var startBtn = UI.btn('⬇ BẮT ĐẦU TẢI VIDEO', {
        variant: 'primary', large: true,
        onclick: function () {
          errBox.style.display = 'none';
          var links = linksTa.value.split('\n').map(function (s) { return s.trim(); }).filter(Boolean);
          if (!links.length) { showErr('Chưa có link nào — dán link hoặc thả file TXT vào ô phía trên.'); return; }
          startBtn.disabled = true;
          API.post('/api/tools/download', { links: links, quality: quality, threads: threads })
            .then(function (jobs) {
              jobs = jobs || [];
              jobs.forEach(function (j) { addJobCard(j, true); });
              UI.toast('Đã bắt đầu tải ' + jobs.length + ' video');
              linksTa.value = '';
            })
            .catch(function (err) { showErr(err.message); })
            .finally(function () { startBtn.disabled = false; });
        }
      });

      var openBtn = UI.btn('📂 MỞ THƯ MỤC LƯU', {
        variant: 'ghost',
        onclick: function () {
          UI.modal({
            title: 'Thư mục lưu video',
            body: h('div', null,
              h('code', { style: { display: 'block', wordBreak: 'break-all', padding: '10px', background: 'var(--bg)', borderRadius: '8px' } }, downloadDir),
              h('p', { class: 'muted', style: { fontSize: '12.5px' } }, 'Mở Finder → nhấn Cmd+Shift+G → dán đường dẫn này để tới thư mục.')),
            actions: [UI.btn('📋 Copy đường dẫn', { variant: 'primary', onclick: function () { copyText(downloadDir); } })]
          });
        }
      });

      el.appendChild(UI.card({
        body: h('div', null, startBtn, h('div', { class: 'mt-8', style: { textAlign: 'center' } }, openBtn), errBox)
      }));

      // --- Danh sách tác vụ tải
      var jobsHost = h('div');
      var jobsEmpty = UI.empty('Chưa có tác vụ tải nào', '⬇️');
      jobsHost.appendChild(jobsEmpty);
      el.appendChild(UI.card({ title: 'Tác vụ tải video', icon: '📥', body: jobsHost }));

      function addJobCard(j, prepend) {
        if (jobEls[j.id]) { jobEls[j.id].update(j); return; }
        if (jobsEmpty.parentNode) jobsEmpty.remove();
        var c = jobCard(j);
        jobEls[j.id] = c;
        if (prepend && jobsHost.firstChild) jobsHost.insertBefore(c, jobsHost.firstChild);
        else jobsHost.appendChild(c);
      }

      API.get('/api/jobs').then(function (list) {
        (list || [])
          .filter(function (j) { return j.kind === 'download'; })
          .sort(function (a, b) { return new Date(b.createdAt) - new Date(a.createdAt); })
          .slice(0, 10)
          .forEach(function (j) { addJobCard(j, false); });
      }).catch(function (err) {
        jobsHost.appendChild(h('div', { class: 'muted', style: { fontSize: '12.5px' } },
          'Không tải được danh sách tác vụ cũ: ' + err.message));
      });

      var onJob = function (j) {
        if (!j || j.kind !== 'download') return;
        addJobCard(j, true);
      };
      Bus.on('job', onJob);
      App._cleanup = function () { Bus.off('job', onJob); };
    }
  };
})();
