/* ============================================================
   Biz Studio — Trang Nhật ký (logs)
   Load sau app.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var MAX_ROWS = 500;

  var LEVEL_OPTIONS = [
    { value: 'all', label: 'Tất cả mức độ' },
    { value: 'info', label: 'Info' },
    { value: 'warn', label: 'Warn' },
    { value: 'error', label: 'Error' }
  ];

  function levelBadge(level) {
    var lv = level || 'info';
    var cls = lv === 'error' ? 'badge-red' : lv === 'warn' ? 'badge-amber' : 'badge-blue';
    return h('span', { class: 'badge ' + cls }, lv.toUpperCase());
  }

  function fmtTime(iso) {
    var d = new Date(iso);
    if (isNaN(d.getTime())) return '--:--:--';
    var pad = function (x) { return (x < 10 ? '0' : '') + x; };
    return pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
  }

  App.pages['logs'] = {
    title: 'Nhật ký',
    subtitle: 'Theo dõi hoạt động của hệ thống theo thời gian thực',
    render: function (el) {
      var entries = [];   // mới nhất trước (server trả newest-first)
      var level = 'all';
      var tableHost = h('div');

      function visibleEntries() {
        if (level === 'all') return entries;
        return entries.filter(function (e) { return e && e.level === level; });
      }

      function renderTable() {
        tableHost.innerHTML = '';
        var rows = visibleEntries();
        if (!rows.length) {
          tableHost.appendChild(UI.empty(
            level === 'all' ? 'Chưa có nhật ký nào' : 'Không có nhật ký ở mức "' + level + '"', '🧾'));
          return;
        }
        tableHost.appendChild(UI.table([
          { key: 'time', label: 'Thời gian', w: '90px' },
          { key: 'level', label: 'Mức độ', w: '90px' },
          { key: 'module', label: 'Module', w: '150px' },
          { key: 'message', label: 'Nội dung' }
        ], rows, function (row, col) {
          if (col.key === 'time') return fmtTime(row.createdAt);
          if (col.key === 'level') return levelBadge(row.level);
          if (col.key === 'module') return h('span', { class: 'muted' }, row.module || '—');
          return h('span', { style: { whiteSpace: 'pre-wrap', wordBreak: 'break-word' } }, row.message || '');
        }));
      }

      var refreshBtn = UI.btn('🔄 Làm mới', {
        variant: 'ghost', small: true,
        onclick: function () { loadLogs(); }
      });

      var clearBtn = UI.btn('Xóa hiển thị', {
        variant: 'ghost', small: true,
        onclick: function () {
          entries = [];
          renderTable();
          UI.toast('Đã xóa nhật ký đang hiển thị (không xóa dữ liệu trên máy chủ)');
        }
      });

      var levelSel = UI.select(null, LEVEL_OPTIONS, level, function (v) {
        level = v;
        renderTable();
      });
      levelSel.style.width = '190px';

      function loadLogs() {
        refreshBtn.disabled = true;
        API.get('/api/logs?limit=300').then(function (list) {
          entries = Array.isArray(list) ? list : [];
          renderTable();
        }).catch(function (err) {
          UI.toast('Không tải được nhật ký: ' + err.message, 'error');
        }).finally(function () { refreshBtn.disabled = false; });
      }

      // Realtime: prepend log mới từ SSE
      var onLog = function (e) {
        if (!e || !e.id) return;
        entries.unshift(e);
        if (entries.length > MAX_ROWS) entries.length = MAX_ROWS;
        renderTable();
      };
      Bus.on('log', onLog);
      App._cleanup = function () { Bus.off('log', onLog); };

      el.appendChild(UI.card({
        title: 'Nhật ký hệ thống', icon: '🧾',
        body: [
          h('div', { class: 'row', style: { marginBottom: '14px' } }, levelSel, refreshBtn, clearBtn),
          tableHost
        ]
      }));
      loadLogs();
    }
  };
})();
