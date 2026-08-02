/* ============================================================
   Biz Studio — Trang Quản lý prompt (prompts)
   Mở từ project detail qua #/prompts (không nằm trên nav).
   Load sau app.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  // ---------- CSS nội bộ ----------

  function injectStyles() {
    if (document.getElementById('prompts-page-style')) return;
    var css = [
      '.prompt-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:16px}',
      '.prompt-grid .card+.card{margin-top:0}',
      '.prompt-name{font-size:14px;font-weight:700;margin:0 0 4px;word-break:break-word}',
      '.prompt-body{font-size:12.5px;color:var(--muted);line-height:1.55;white-space:pre-wrap;word-break:break-word;',
      '  display:-webkit-box;-webkit-line-clamp:3;-webkit-box-orient:vertical;overflow:hidden;margin:0 0 12px}'
    ].join('\n');
    document.head.appendChild(h('style', { id: 'prompts-page-style' }, css));
  }

  // ---------- Modal thêm / sửa ----------

  function openEditor(p, onSaved) {
    var nameInput = UI.input({ value: p ? p.name : '', placeholder: 'VD: Cắt highlight 60 giây' });
    var bodyTa = UI.textarea({
      value: p ? p.body : '', rows: 12,
      placeholder: 'Nội dung prompt — mô tả yêu cầu edit chi tiết, có thể dùng cho nhiều dự án…'
    });
    bodyTa.style.minHeight = '220px';

    var m = UI.modal({
      title: p ? 'Sửa prompt mẫu' : 'Thêm prompt mẫu',
      body: [UI.field('Tên', nameInput), UI.field('Nội dung', bodyTa)],
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn(p ? '💾 Lưu thay đổi' : '＋ Thêm prompt', {
          onclick: function () {
            var name = nameInput.value.trim();
            var body = bodyTa.value.trim();
            if (!name) { UI.toast('Vui lòng nhập tên prompt', 'error'); return; }
            if (!body) { UI.toast('Vui lòng nhập nội dung prompt', 'error'); return; }
            var req = p
              ? API.put('/api/prompts/' + encodeURIComponent(p.id), { id: p.id, name: name, body: body })
              : API.post('/api/prompts', { name: name, body: body });
            req.then(function () {
              m.close();
              UI.toast(p ? 'Đã cập nhật prompt mẫu' : 'Đã thêm prompt mẫu');
              onSaved();
            }).catch(function (err) {
              UI.toast('Lưu prompt thất bại: ' + err.message, 'error');
            });
          }
        })
      ]
    });
  }

  function confirmDelete(p, onDeleted) {
    var m = UI.modal({
      title: 'Xóa prompt mẫu',
      body: h('p', { class: 'muted', style: { margin: '0' } },
        'Xóa prompt "' + p.name + '"? Thao tác này không thể hoàn tác.'),
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('🗑 Xóa', {
          variant: 'danger',
          onclick: function () {
            API.del('/api/prompts/' + encodeURIComponent(p.id)).then(function () {
              m.close();
              UI.toast('Đã xóa prompt mẫu');
              onDeleted();
            }).catch(function (err) {
              UI.toast('Xóa prompt thất bại: ' + err.message, 'error');
            });
          }
        })
      ]
    });
  }

  // ---------- Danh sách ----------

  function promptCard(p, reload) {
    return h('div', { class: 'card' },
      h('div', { class: 'prompt-name' }, p.name || '(không tên)'),
      h('div', { class: 'prompt-body', title: p.body || '' }, p.body || ''),
      h('div', { class: 'row' },
        UI.btn('✏️ Sửa', { variant: 'ghost', small: true, onclick: function () { openEditor(p, reload); } }),
        UI.btn('🗑 Xóa', { variant: 'danger', small: true, onclick: function () { confirmDelete(p, reload); } })));
  }

  function load(el) {
    var listHost = el.listHost;
    listHost.innerHTML = '';
    listHost.appendChild(h('div', { class: 'empty' }, UI.spinner(), h('span', null, 'Đang tải danh sách prompt…')));
    API.get('/api/prompts').then(function (list) {
      list = Array.isArray(list) ? list : [];
      listHost.innerHTML = '';
      if (!list.length) {
        listHost.appendChild(UI.empty('Chưa có prompt mẫu nào. Prompt mẫu giúp tái sử dụng yêu cầu edit cho nhiều dự án.', '📝'));
        return;
      }
      var grid = h('div', { class: 'prompt-grid' });
      list.forEach(function (p) {
        grid.appendChild(promptCard(p, function () { load(el); }));
      });
      listHost.appendChild(grid);
    }).catch(function (err) {
      listHost.innerHTML = '';
      listHost.appendChild(UI.card({
        title: 'Không tải được danh sách prompt', icon: '❌',
        body: h('div', { class: 'text-red' }, err.message),
        foot: UI.btn('Thử lại', { variant: 'ghost', small: true, onclick: function () { load(el); } })
      }));
    });
  }

  App.pages['prompts'] = {
    title: 'Quản lý prompt',
    subtitle: 'Prompt mẫu dùng lại cho yêu cầu edit của nhiều dự án',
    render: function (el) {
      injectStyles();
      var listHost = h('div');
      el.listHost = listHost;
      el.appendChild(h('div', { class: 'row-between', style: { marginBottom: '16px' } },
        UI.btn('← Quay lại', { variant: 'ghost', onclick: function () { history.back(); } }),
        UI.btn('＋ Thêm prompt mẫu', { onclick: function () { openEditor(null, function () { load(el); }); } })));
      el.appendChild(listHost);
      load(el);
    }
  };
})();
