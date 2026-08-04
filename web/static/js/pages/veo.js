/* ============================================================
   Biz Studio — Trang "Veo — Sinh video AI".
   Module DUY NHẤT tiêu tiền thật theo từng lần bấm, nên chi phí
   luôn hiện sẵn và có bước xác nhận trước khi chạy.
   Load sau app.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  App.pages['veo'] = {
    title: 'Veo — Sinh video AI',
    subtitle: 'Mô tả cảnh bằng lời, Google Veo dựng thành video có tiếng',
    render: render
  };

  function render(root) {
    var st = {
      info: null, projects: [],
      prompt: '', negative: '', model: '', aspect: '9:16',
      resolution: '720p', seconds: 8, imagePath: '', projectId: '',
      estimate: null, estimateErr: '', handlers: []
    };
    App._cleanup = function () {
      st.handlers.forEach(function (fn) { Bus.off('job', fn); });
      st.handlers = [];
    };

    root.appendChild(h('div', { class: 'card' }, UI.empty('Đang tải thông tin Veo…', '🎥')));
    Promise.all([
      API.get('/api/tools/veo'),
      API.get('/api/projects').catch(function () { return []; })
    ]).then(function (res) {
      st.info = res[0];
      st.projects = res[1] || [];
      st.model = st.info.model;
      st.resolution = st.info.resolution || '720p';
      st.seconds = st.info.seconds || 8;
      root.innerHTML = '';
      build(root, st);
    }).catch(function (err) {
      root.innerHTML = '';
      root.appendChild(h('div', { class: 'card' },
        UI.empty('Không tải được thông tin Veo: ' + err.message, '⚠️')));
    });
  }

  function build(root, st) {
    root.appendChild(keyCard(st));

    var costLine = h('div', {
      style: {
        fontSize: '15px', fontWeight: '800', padding: '10px 14px', borderRadius: '10px',
        background: 'var(--bg)', border: '1px solid var(--border)', display: 'inline-block'
      }
    }, '…');

    function refreshCost() {
      API.post('/api/tools/veo/estimate', {
        model: st.model, resolution: st.resolution, seconds: st.seconds, count: 1
      }).then(function (r) {
        st.estimate = r.usd; st.estimateErr = '';
        costLine.textContent = '💵 Lần tạo này tốn khoảng $' + r.usd.toFixed(2);
        costLine.style.color = '';
      }).catch(function (err) {
        st.estimate = null; st.estimateErr = err.message;
        costLine.textContent = '⚠️ Không ước tính được chi phí: ' + err.message;
        costLine.style.color = 'var(--red)';
      });
    }

    root.appendChild(formCard(st, refreshCost));
    root.appendChild(runCard(st, costLine, refreshCost));
    refreshCost();
  }

  // ---------- trạng thái khoá ----------

  function keyCard(st) {
    var i = st.info;
    var body = [h('div', { class: 'muted', style: { fontSize: '13px', lineHeight: '1.6' } }, i.note)];
    if (!i.ready) {
      body.push(h('div', { style: { marginTop: '10px' } },
        UI.btn('⚙️ Mở Cấu hình & API để nhập khoá', {
          variant: 'primary', onclick: function () { location.hash = '#/settings'; }
        })));
    } else if (i.usingGemini) {
      body.push(h('div', { class: 'muted', style: { fontSize: '12.5px', marginTop: '8px' } },
        'Đang dùng chung khoá Gemini. Nếu dự án Google của khoá đó chưa bật thanh toán thì Veo sẽ báo lỗi — ' +
        'khi đó hãy nhập riêng khoá Veo trong Cấu hình & API.'));
    }
    return h('div', { class: 'card' },
      h('div', { class: 'card-title' },
        (i.ready ? '✅ ' : '⚠️ ') + 'Khoá Veo — ' + (i.ready ? 'đã có' : 'chưa có')),
      body);
  }

  // ---------- tuỳ chọn ----------

  function formCard(st, onChange) {
    var modelOpts = st.info.models.map(function (m) {
      var price = m.pricePerSec && m.pricePerSec['720p'];
      return {
        value: m.id,
        label: m.name + (price ? ' — $' + price + '/giây' : '') + (m.deprecated ? ' ⚠' : '')
      };
    });
    var modelDesc = h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '-8px' } }, '');
    function syncDesc() {
      var m = null;
      st.info.models.forEach(function (x) { if (x.id === st.model) m = x; });
      modelDesc.textContent = m ? m.desc : '';
      modelDesc.style.color = m && m.deprecated ? 'var(--red)' : '';
    }

    var promptBox = h('textarea', {
      class: 'input', rows: '4',
      placeholder: 'VD: Máy quay lướt chậm qua quầy phở buổi sáng ở Hà Nội, hơi nước bốc lên, ánh nắng xiên qua cửa'
    });
    promptBox.oninput = function () { st.prompt = promptBox.value; };

    var negBox = UI.input({
      value: '', placeholder: 'Thứ cần tránh, vd: chữ trên hình, mặt người méo',
      oninput: function (e) { st.negative = e.target.value; }
    });

    var imgInput = UI.input({
      value: '', placeholder: 'Đường dẫn ảnh trong data (tuỳ chọn) — Veo lấy làm khung hình đầu',
      oninput: function (e) { st.imagePath = e.target.value.trim(); }
    });

    var projOpts = [{ value: '', label: 'Không — lưu vào data/veo' }].concat(
      st.projects.map(function (p) { return { value: p.id, label: p.name }; }));

    return h('div', { class: 'card mt-16' },
      h('div', { class: 'card-title' }, '🎬 Mô tả cảnh cần tạo'),
      UI.field('Mô tả (bằng tiếng Việt cũng được)', promptBox),
      UI.field('Tránh những gì', negBox),
      h('div', { class: 'grid-2 mt-8' },
        h('div', null,
          UI.select('Model', modelOpts, st.model, function (v) { st.model = v; syncDesc(); onChange(); }),
          modelDesc),
        UI.select('Khung hình', [
          { value: '9:16', label: 'Dọc 9:16 — TikTok / Shorts / Reels' },
          { value: '16:9', label: 'Ngang 16:9 — YouTube' }
        ], st.aspect, function (v) { st.aspect = v; })),
      h('div', { class: 'grid-2 mt-8' },
        UI.select('Độ phân giải', [
          { value: '720p', label: '720p' },
          { value: '1080p', label: '1080p' },
          { value: '4k', label: '4K' }
        ], st.resolution, function (v) { st.resolution = v; onChange(); }),
        UI.select('Thời lượng', (st.info.durations || [4, 6, 8]).map(function (d) {
          return { value: String(d), label: d + ' giây' };
        }), String(st.seconds), function (v) { st.seconds = Number(v) || 8; onChange(); })),
      h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '6px' } },
        '1080p và 4K chỉ tạo được clip 8 giây — chọn sai hệ thống sẽ báo trước khi gọi Veo.'),
      h('div', { class: 'grid-2 mt-8' },
        UI.field('Ảnh khung hình đầu', imgInput),
        UI.select('Lưu vào dự án', projOpts, st.projectId, function (v) { st.projectId = v; })),
      (syncDesc(), h('span')));
  }

  // ---------- chạy ----------

  function runCard(st, costLine, refreshCost) {
    var runBtn = UI.btn('🎥 Tạo video', {
      variant: 'primary',
      onclick: function () {
        if (!st.info.ready) { UI.toast('Chưa có khoá Veo.', 'error'); return; }
        if (!st.prompt.trim()) { UI.toast('Chưa nhập mô tả cảnh.', 'error'); return; }
        if (st.estimate === null) {
          UI.toast('Chưa ước tính được chi phí — không chạy để tránh bị trừ tiền ngoài dự kiến.', 'error');
          return;
        }
        confirmCost(st, runBtn);
      }
    });

    return h('div', { class: 'card mt-16' },
      h('div', { class: 'card-title' }, '💵 Chi phí & chạy'),
      costLine,
      h('div', { class: 'muted', style: { fontSize: '12.5px', margin: '10px 0 14px', lineHeight: '1.6' } },
        'Con số trên là ước tính theo bảng giá công bố của Google — hoá đơn thật do Google tính. ' +
        'Veo dựng video mất khoảng 1–4 phút; bạn có thể rời trang, job vẫn chạy tiếp.'),
      h('div', { class: 'row' }, runBtn,
        UI.btn('↻ Tính lại chi phí', { variant: 'ghost', onclick: refreshCost })));
  }

  // Bước xác nhận: người dùng phải đọc con số rồi mới bấm — cờ confirmed chỉ
  // được gửi từ đây, backend từ chối mọi yêu cầu thiếu cờ này.
  function confirmCost(st, runBtn) {
    var m = UI.modal({
      title: 'Xác nhận chi phí',
      body: h('div', null,
        h('p', { style: { margin: '0 0 10px', fontSize: '15px' } },
          'Lần tạo này sẽ bị Google tính khoảng ',
          h('strong', null, '$' + st.estimate.toFixed(2)),
          ' trên khoá của bạn.'),
        h('p', { class: 'muted', style: { margin: '0', fontSize: '13px', lineHeight: '1.6' } },
          st.seconds + ' giây · ' + st.resolution + ' · ' + st.aspect + ' · ' + st.model)),
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { m.close(); } }),
        UI.btn('Đồng ý, tạo video', {
          variant: 'primary',
          onclick: function () {
            m.close();
            runBtn.disabled = true;
            API.post('/api/tools/veo', {
              prompt: st.prompt, negative: st.negative, model: st.model,
              aspect: st.aspect, resolution: st.resolution, seconds: st.seconds,
              imagePath: st.imagePath, projectId: st.projectId, confirmed: true
            }).then(function () {
              UI.toast('Đã bắt đầu tạo video — theo dõi ở thanh tác vụ.');
            }).catch(function (err) {
              UI.toast('Không chạy được: ' + err.message, 'error');
            }).finally(function () { runBtn.disabled = false; });
          }
        })
      ]
    });
  }
})();
