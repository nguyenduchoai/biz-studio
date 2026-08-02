/* ============================================================
   Biz Studio — Trang Dự án (danh sách + tạo mới)
   Khi hash có param (#/projects/<id>) → ủy quyền cho trang
   'project-detail' (đăng ký trong project-detail.js).
   ============================================================ */
(function () {
  'use strict';

  var STATUS = {
    done:    { label: 'Hoàn thành', cls: 'badge-green' },
    running: { label: 'Đang chạy',  cls: 'badge-amber' },
    draft:   { label: 'Nháp',       cls: 'badge-gray'  },
    error:   { label: 'Lỗi',        cls: 'badge-red'   }
  };
  var PRESETS = [
    { value: '1080x1920', label: '1080×1920 dọc (TikTok)', w: 1080, h: 1920 },
    { value: '1920x1080', label: '1920×1080 ngang',        w: 1920, h: 1080 },
    { value: '1080x1080', label: '1080×1080 vuông',        w: 1080, h: 1080 }
  ];
  var THUMB_COLORS = ['#2563EB', '#7C3AED', '#DB2777', '#059669', '#D97706', '#0891B2'];

  function statusBadge(s) {
    var m = STATUS[s] || { label: s || '—', cls: 'badge-gray' };
    return h('span', { class: 'badge ' + m.cls }, m.label);
  }

  function thumbEl(p) {
    var style = { width: '54px', height: '34px', borderRadius: '6px', flex: 'none', backgroundSize: 'cover', backgroundPosition: 'center' };
    if (p.thumbFile) style.backgroundImage = 'url("/data/' + p.thumbFile + '")';
    else {
      var s = 0;
      for (var i = 0; i < p.id.length; i++) s += p.id.charCodeAt(i);
      style.background = THUMB_COLORS[s % THUMB_COLORS.length];
    }
    return h('div', { style: style });
  }

  // ---------- Modal tạo dự án ----------

  function openCreateModal() {
    var nameIn = UI.input({ placeholder: 'VD: Review quán ăn Đà Lạt…' });
    var kindSel = UI.select(null, [
      { value: 'video', label: 'Video edit (AI)' },
      { value: 'vox', label: 'Vox / Bài viết → Video' }
    ], 'video');
    var presetSel = UI.select(null, PRESETS, '1080x1920');
    var fpsSel = UI.select(null, [{ value: '30', label: '30 fps' }, { value: '60', label: '60 fps' }], '30');

    var modal = UI.modal({
      title: '＋ Tạo dự án mới',
      body: h('div', null,
        UI.field('Tên dự án', nameIn),
        UI.field('Loại dự án', kindSel),
        UI.field('Kích thước khung hình', presetSel),
        UI.field('Tốc độ khung hình (fps)', fpsSel)),
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { modal.close(); } }),
        UI.btn('Tạo dự án', { onclick: submit })
      ]
    });
    setTimeout(function () { nameIn.focus(); }, 60);
    nameIn.onkeydown = function (e) { if (e.key === 'Enter') submit(); };

    function submit() {
      var name = nameIn.value.trim();
      if (!name) { UI.toast('Vui lòng nhập tên dự án', 'error'); nameIn.focus(); return; }
      var pr = null;
      PRESETS.forEach(function (x) { if (x.value === presetSel.value) pr = x; });
      pr = pr || PRESETS[0];
      API.post('/api/projects', {
        name: name, kind: kindSel.value, width: pr.w, height: pr.h, fps: Number(fpsSel.value) || 30
      }).then(function (p) {
        modal.close();
        UI.toast('Đã tạo dự án "' + p.name + '"');
        App.navigate('projects/' + p.id);
      }).catch(function (e) {
        UI.toast('Không tạo được dự án: ' + e.message, 'error');
      });
    }
  }

  // ---------- Hành động trên dự án ----------

  function duplicateProject(p, reload) {
    API.post('/api/projects/' + encodeURIComponent(p.id) + '/duplicate').then(function (np) {
      UI.toast('Đã nhân bản thành "' + (np && np.name ? np.name : p.name) + '"');
      reload();
    }).catch(function (e) {
      UI.toast('Không nhân bản được: ' + e.message, 'error');
    });
  }

  function deleteProject(p, reload) {
    var modal = UI.modal({
      title: 'Xóa dự án',
      body: h('p', null, 'Xóa vĩnh viễn dự án "', h('b', null, p.name),
        '" cùng toàn bộ asset, output và phiên AI? Hành động này không thể hoàn tác.'),
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { modal.close(); } }),
        UI.btn('Xóa vĩnh viễn', {
          variant: 'danger',
          onclick: function () {
            API.del('/api/projects/' + encodeURIComponent(p.id)).then(function () {
              modal.close();
              UI.toast('Đã xóa dự án "' + p.name + '"');
              reload();
            }).catch(function (e) {
              UI.toast('Không xóa được: ' + e.message, 'error');
            });
          }
        })
      ]
    });
  }

  // ---------- Danh sách ----------

  function renderCell(p, c, reload) {
    switch (c.key) {
      case 'name':
        return h('div', { class: 'row', style: { flexWrap: 'nowrap', minWidth: 0 } },
          thumbEl(p),
          h('div', { style: { minWidth: 0 } },
            h('div', { style: { fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }, title: p.name }, p.name),
            h('div', { class: 'muted', style: { fontSize: '11px' } }, p.id)));
      case 'size':
        return h('span', { class: 'muted' }, p.width + '×' + p.height + ' · ' + p.fps + 'fps');
      case 'status':
        return statusBadge(p.status);
      case 'tags':
        if (!p.tags || !p.tags.length) return h('span', { class: 'muted' }, '—');
        return h('div', null, p.tags.map(function (t) { return UI.chip(t); }));
      case 'updated':
        return h('span', { class: 'muted' }, UI.timeAgo(p.updatedAt));
      case 'actions':
        return h('div', { class: 'row', style: { flexWrap: 'nowrap', justifyContent: 'flex-end' } },
          UI.btn('Mở', { variant: 'ghost', small: true, onclick: function () { App.navigate('projects/' + p.id); } }),
          UI.btn('Nhân bản', { variant: 'ghost', small: true, onclick: function () { duplicateProject(p, reload); } }),
          UI.btn('Xóa', { variant: 'danger', small: true, onclick: function () { deleteProject(p, reload); } }));
      default:
        return '';
    }
  }

  function loadList(host) {
    host.innerHTML = '';
    host.appendChild(h('div', { class: 'row muted' }, UI.spinner(), 'Đang tải danh sách dự án…'));
    function reload() { loadList(host); }

    API.get('/api/projects').then(function (list) {
      list = (list || []).slice().sort(function (a, b) {
        return new Date(b.updatedAt) - new Date(a.updatedAt);
      });
      host.innerHTML = '';
      if (!list.length) {
        host.appendChild(UI.card({
          body: h('div', { class: 'empty' },
            h('span', { class: 'empty-icon' }, '🎬'),
            h('div', { style: { fontWeight: 700, fontSize: '15px', color: 'var(--text)' } }, 'Chưa có dự án nào'),
            h('div', null, 'Tạo dự án đầu tiên để bắt đầu edit video bằng AI.'),
            h('div', { class: 'mt-8' }, UI.btn('＋ Tạo dự án mới', { onclick: openCreateModal })))
        }));
        return;
      }
      var cols = [
        { key: 'name', label: 'Dự án' },
        { key: 'size', label: 'Kích thước', w: '140px' },
        { key: 'status', label: 'Trạng thái', w: '110px' },
        { key: 'tags', label: 'Tags' },
        { key: 'updated', label: 'Cập nhật', w: '120px' },
        { key: 'actions', label: '', w: '220px' }
      ];
      host.appendChild(UI.card({
        body: UI.table(cols, list, function (p, c) { return renderCell(p, c, reload); })
      }));
    }).catch(function (e) {
      host.innerHTML = '';
      host.appendChild(UI.card({
        title: 'Không tải được danh sách dự án', icon: '❌',
        body: h('p', { class: 'text-red' }, e.message)
      }));
    });
  }

  // ---------- Đăng ký trang ----------

  App.pages['projects'] = {
    title: 'Dự án',
    subtitle: 'Quản lý toàn bộ dự án video trong studio',
    render: function (el, param) {
      if (param) {
        var detail = App.pages['project-detail'];
        if (detail && typeof detail.render === 'function') { detail.render(el, param); return; }
        el.appendChild(UI.empty('Không tải được trang chi tiết dự án (thiếu project-detail.js)', '❌'));
        return;
      }
      var listHost = h('div', { class: 'mt-16' });
      el.appendChild(h('div', { class: 'row-between' },
        h('span', { class: 'muted' }, 'Toàn bộ dự án, sắp xếp theo lần cập nhật gần nhất.'),
        UI.btn('＋ Tạo dự án mới', { onclick: openCreateModal })));
      el.appendChild(listHost);
      loadList(listHost);
    }
  };
})();
