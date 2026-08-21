/* ============================================================
   Biz Studio — thẻ "Công cụ trên máy" trong trang Cấu hình.

   Vì sao có: người dùng gặp "HTTP Error 403: Forbidden" lúc tải video và không
   có cách nào đoán ra nguyên nhân là yt-dlp cũ. Bắt mở terminal gõ brew/winget
   là mất luôn phần lớn người dùng. Nút ngay cạnh dòng trạng thái xử lý đúng chỗ
   đó — và cùng chỗ đó cài luôn giọng đọc, bóc băng, Chrome.

   Nạp sau settings.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var LOG_MAX = 400;   // giữ đủ để xem lỗi, không để phình DOM khi pip nói nhiều

  function injectStyles() {
    if (document.getElementById('setup-tools-style')) return;
    document.head.appendChild(h('style', { id: 'setup-tools-style' }, [
      '.tool-row{display:flex;align-items:flex-start;gap:10px;padding:10px 0;border-top:1px solid var(--border)}',
      '.tool-row:first-child{border-top:none}',
      '.tool-main{flex:1;min-width:0}',
      '.tool-name{font-weight:600;font-size:13px}',
      '.tool-detail{font-size:12px;color:var(--muted);word-break:break-word;margin-top:2px}',
      '.tool-log{margin-top:12px;padding:10px;border-radius:8px;background:var(--code-bg,rgba(127,127,127,.10));',
      'font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:11.5px;line-height:1.55;',
      'max-height:260px;overflow:auto;white-space:pre-wrap;word-break:break-word}'
    ].join('\n')));
  }

  // ---------- Ô nhật ký cài đặt ----------

  function makeLog() {
    var box = h('div', { class: 'tool-log', style: { display: 'none' } });
    return {
      el: box,
      show: function () { box.style.display = ''; box.textContent = ''; },
      add: function (line) {
        box.appendChild(document.createTextNode(line + '\n'));
        while (box.childNodes.length > LOG_MAX) box.removeChild(box.firstChild);
        box.scrollTop = box.scrollHeight;   // bám đáy để luôn thấy dòng mới nhất
      }
    };
  }

  // ---------- Một dòng công cụ ----------

  // toolRow trả cả các nút/ô chữ cần cập nhật về sau, thay vì để nơi gọi đi dò
  // lại bằng lastChild/querySelector — đổi chút bố cục là hỏng ngay.
  function toolRow(t, ctx) {
    var status = h('span', { class: t.installed ? 'text-green' : 'text-red' }, t.installed ? '✓' : '✗');
    var detail = h('div', { class: 'tool-detail' }, t.detail || '');

    // Đã có thì nút là "Cập nhật" — đây mới là thứ chữa lỗi 403, chứ cài đè lên
    // bản đang chạy chẳng giải quyết gì.
    var action = t.installed ? 'update' : 'install';
    var btn = UI.btn(t.installed ? '⬆ Cập nhật' : '⬇ Cài', {
      variant: t.installed ? 'ghost' : 'primary', small: true,
      onclick: function () { run(t, action, row); }
    });

    var row = {
      tool: t, btn: btn, detail: detail, ctx: ctx,
      el: h('div', { class: 'tool-row' },
        status,
        h('div', { class: 'tool-main' },
          h('div', { class: 'tool-name' }, t.label),
          h('div', { class: 'tool-detail' }, t.desc),
          detail),
        btn)
    };
    return row;
  }

  // ---------- Chạy cài đặt ----------

  function run(t, action, row) {
    var ctx = row.ctx;
    ctx.log.show();
    ctx.log.add((action === 'update' ? 'Đang cập nhật ' : 'Đang cài ') + t.label + '…');
    // Khoá hết nút: hai lượt pip cùng ghi vào một venv là hỏng venv, mà lỗi sinh
    // ra lại chẳng nói gì về nguyên nhân. Server cũng chặn, đây là lớp thứ hai.
    setBusy(ctx, true);
    row.btn.textContent = '⏳ Đang chạy…';
    row.detail.className = 'tool-detail';
    row.detail.textContent = '';

    // Server chạy nền và phát tiến trình qua SSE; POST chỉ khởi động rồi trả
    // ngay, nên đóng tab giữa chừng cũng không bỏ dở một venv đang cài.
    API.post('/api/setup/' + t.id + '?action=' + action).catch(function (err) {
      finish(row, { state: 'error', error: err.message, manual: t.manual });
    });
  }

  function setBusy(ctx, busy) {
    ctx.rows.forEach(function (r) { r.btn.disabled = busy; });
  }

  function finish(row, ev) {
    var ctx = row.ctx;
    setBusy(ctx, false);
    if (ev.state === 'done') {
      ctx.log.add('✅ Xong — đang kiểm tra lại…');
      ctx.reload();          // đọc lại phiên bản thật thay vì tin là đã xong
      UI.toast(row.tool.label + ': đã xong');
      return;
    }
    row.btn.textContent = '↻ Thử lại';
    row.detail.className = 'tool-detail text-red';
    row.detail.textContent = '✗ ' + (ev.error || 'thất bại');
    ctx.log.add('❌ ' + (ev.error || 'thất bại'));
    if (ev.manual) ctx.log.add('→ Tải thủ công: ' + ev.manual);
    UI.toast(row.tool.label + ': ' + (ev.error || 'thất bại'), 'error');
  }

  // ---------- Thẻ ----------

  // Bus.on không trả hàm hủy, nên phải giữ lại handler để Bus.off. Vào lại trang
  // Cấu hình là dựng thẻ mới: không gỡ handler cũ thì nó vẫn sống và trỏ vào DOM
  // đã bị vứt, mỗi lần vào lại rò thêm một cái.
  var listener = null;

  function card() {
    injectStyles();
    var body = h('div', null, h('div', { class: 'empty' }, UI.spinner(), h('span', null, 'Đang dò công cụ…')));
    var log = makeLog();
    var ctx = { log: log, reload: reload, rows: [], byID: {} };

    function reload() {
      API.get('/api/setup/tools').then(function (tools) {
        body.innerHTML = '';
        ctx.rows = [];
        ctx.byID = {};
        (tools || []).forEach(function (t) {
          var row = toolRow(t, ctx);
          ctx.rows.push(row);
          ctx.byID[t.id] = row;
          body.appendChild(row.el);
        });
      }).catch(function (err) {
        body.innerHTML = '';
        body.appendChild(h('div', { class: 'text-red' }, 'Không dò được công cụ: ' + err.message));
      });
    }

    if (listener) Bus.off('setup', listener);
    listener = function (ev) {
      var row = ctx.byID[ev.tool];
      if (!row) return;
      if (ev.line) { log.add(ev.line); return; }
      if (ev.state === 'done' || ev.state === 'error') finish(row, ev);
    };
    Bus.on('setup', listener);

    reload();
    return UI.card({
      title: 'Công cụ trên máy', icon: '🧰',
      desc: 'Cài hoặc cập nhật các công cụ ngoài mà studio cần — không phải mở terminal. ' +
            'Lỗi 403 khi tải video hầu hết là do yt-dlp đã cũ: bấm Cập nhật.',
      body: h('div', null, body, log.el)
    });
  }

  window.SettingsTools = { card: card };
})();
