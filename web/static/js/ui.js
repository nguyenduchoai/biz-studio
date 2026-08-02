/* ============================================================
   Biz Studio — UI helpers (h + UI.*)
   Load sau api.js, trước app.js. Không framework.
   ============================================================ */
(function () {
  'use strict';

  // ---------- h(tag, attrs, ...children) ----------

  function appendChild(el, child) {
    if (child === null || child === undefined || child === false || child === true) return;
    if (Array.isArray(child)) { child.forEach(function (c) { appendChild(el, c); }); return; }
    if (child instanceof Node) { el.appendChild(child); return; }
    el.appendChild(document.createTextNode(String(child)));
  }

  function h(tag, attrs) {
    var el = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        var v = attrs[k];
        if (v === null || v === undefined) return;
        if (k === 'class') el.className = v;
        else if (k === 'style' && typeof v === 'object') Object.assign(el.style, v);
        else if (k === 'dataset' && typeof v === 'object') Object.assign(el.dataset, v);
        else if (k.slice(0, 2) === 'on' && typeof v === 'function') el[k.toLowerCase()] = v;
        else if (k === 'value') el.value = v;
        else if (k === 'checked') el.checked = !!v;
        else if (k === 'disabled') el.disabled = !!v;
        else if (k === 'selected') el.selected = !!v;
        else el.setAttribute(k, v);
      });
    }
    for (var i = 2; i < arguments.length; i++) appendChild(el, arguments[i]);
    return el;
  }
  window.h = h;

  var UI = {};
  window.UI = UI;

  // ---------- Format ----------

  UI.fmtBytes = function (n) {
    n = Number(n) || 0;
    if (n < 1024) return n + ' B';
    var units = ['KB', 'MB', 'GB', 'TB'];
    var v = n, u = -1;
    while (v >= 1024 && u < units.length - 1) { v /= 1024; u++; }
    return (v >= 100 ? Math.round(v) : v.toFixed(1)) + ' ' + units[u];
  };

  UI.fmtDur = function (s) {
    s = Math.max(0, Math.round(Number(s) || 0));
    var hh = Math.floor(s / 3600), mm = Math.floor((s % 3600) / 60), ss = s % 60;
    var pad = function (x) { return (x < 10 ? '0' : '') + x; };
    if (hh > 0) return hh + ':' + pad(mm) + ':' + pad(ss);
    return pad(mm) + ':' + pad(ss);
  };

  UI.timeAgo = function (iso) {
    if (!iso) return '';
    var t = new Date(iso).getTime();
    if (isNaN(t)) return String(iso);
    var diff = Math.floor((Date.now() - t) / 1000);
    if (diff < 5) return 'vừa xong';
    if (diff < 60) return diff + ' giây trước';
    if (diff < 3600) return Math.floor(diff / 60) + ' phút trước';
    if (diff < 86400) return Math.floor(diff / 3600) + ' giờ trước';
    if (diff < 30 * 86400) return Math.floor(diff / 86400) + ' ngày trước';
    return new Date(t).toLocaleDateString('vi-VN');
  };

  UI.spinner = function () { return h('span', { class: 'spinner' }); };

  // ---------- Card ----------

  UI.card = function (opts) {
    opts = opts || {};
    var el = h('div', { class: 'card' });
    if (opts.title) {
      el.appendChild(h('div', { class: 'card-title' }, opts.icon ? h('span', null, opts.icon) : null, opts.title));
    }
    if (opts.desc) el.appendChild(h('p', { class: 'card-desc' }, opts.desc));
    if (opts.body) appendChild(el, opts.body);
    if (opts.foot) el.appendChild(h('div', { class: 'card-foot' }, opts.foot));
    return el;
  };

  // ---------- Button ----------

  UI.btn = function (label, opts) {
    opts = opts || {};
    var cls = 'btn btn-' + (opts.variant || 'primary');
    if (opts.small) cls += ' btn-sm';
    if (opts.large) cls += ' btn-lg';
    return h('button', { class: cls, type: 'button', onclick: opts.onclick, disabled: opts.disabled },
      opts.icon ? h('span', null, opts.icon) : null, label);
  };

  // ---------- Form controls ----------

  UI.field = function (label, inputEl) {
    return h('div', { class: 'field' },
      label ? h('label', { class: 'field-label' }, label) : null, inputEl);
  };

  UI.input = function (opts) {
    opts = opts || {};
    return h('input', {
      class: 'input', type: opts.type || 'text',
      value: opts.value !== undefined ? opts.value : '',
      placeholder: opts.placeholder, min: opts.min, max: opts.max, step: opts.step,
      oninput: opts.oninput, onchange: opts.onchange, onkeydown: opts.onkeydown,
      disabled: opts.disabled
    });
  };

  UI.textarea = function (opts) {
    opts = opts || {};
    var el = h('textarea', {
      class: 'textarea', placeholder: opts.placeholder, rows: opts.rows,
      oninput: opts.oninput, onchange: opts.onchange, disabled: opts.disabled
    });
    if (opts.value !== undefined) el.value = opts.value;
    return el;
  };

  UI.select = function (label, options, value, onchange) {
    var sel = h('select', { class: 'select', onchange: onchange ? function () { onchange(sel.value); } : null });
    (options || []).forEach(function (o) {
      var opt = (typeof o === 'object') ? o : { value: o, label: o };
      sel.appendChild(h('option', { value: opt.value, selected: opt.value === value }, opt.label));
    });
    if (value !== undefined) sel.value = value;
    return label ? UI.field(label, sel) : sel;
  };

  UI.toggle = function (label, desc, checked, onchange) {
    var input = h('input', { type: 'checkbox', checked: !!checked });
    var el = h('label', { class: 'toggle' },
      h('div', { class: 'toggle-texts' },
        h('div', { class: 'toggle-label' }, label),
        desc ? h('div', { class: 'toggle-desc' }, desc) : null),
      input, h('span', { class: 'toggle-track' }));
    input.onchange = function () { if (onchange) onchange(input.checked); };
    el.input = input;
    return el;
  };

  UI.slider = function (label, opts) {
    opts = opts || {};
    var range = h('input', { type: 'range', min: opts.min, max: opts.max, step: opts.step, value: opts.value });
    var num = h('input', { class: 'slider-num', type: 'number', min: opts.min, max: opts.max, step: opts.step, value: opts.value });
    function fire(v) { if (opts.oninput) opts.oninput(Number(v)); }
    range.oninput = function () { num.value = range.value; fire(range.value); };
    num.oninput = function () { range.value = num.value; fire(num.value); };
    var row = h('div', { class: 'slider-row' }, range, num);
    row.input = range;
    return label ? UI.field(label, row) : row;
  };

  // ---------- Dropzone ----------

  UI.dropzone = function (opts) {
    opts = opts || {};
    var input = h('input', {
      type: 'file', style: { display: 'none' },
      accept: opts.accept, multiple: opts.multiple ? 'multiple' : null
    });
    var zone = h('div', { class: 'dropzone' },
      h('span', { class: 'dropzone-icon' }, '☁️'),
      h('div', { class: 'dropzone-hint' }, opts.hint || 'Kéo thả file vào đây hoặc bấm để chọn'),
      h('div', { class: 'dropzone-sub' }, opts.sub || 'Hỗ trợ kéo thả nhiều file'),
      input);
    zone.onclick = function () { input.click(); };
    input.onchange = function () {
      if (input.files && input.files.length && opts.onfiles) opts.onfiles(Array.prototype.slice.call(input.files));
      input.value = '';
    };
    zone.ondragover = function (e) { e.preventDefault(); zone.classList.add('drag'); };
    zone.ondragleave = function () { zone.classList.remove('drag'); };
    zone.ondrop = function (e) {
      e.preventDefault();
      zone.classList.remove('drag');
      var files = e.dataTransfer && e.dataTransfer.files ? Array.prototype.slice.call(e.dataTransfer.files) : [];
      if (files.length && opts.onfiles) opts.onfiles(files);
    };
    return zone;
  };

  // ---------- Table ----------

  UI.table = function (cols, rows, renderCell) {
    var thead = h('thead', null, h('tr', null, (cols || []).map(function (c) {
      return h('th', { style: c.w ? { width: c.w } : null }, c.label);
    })));
    var tbody = h('tbody');
    (rows || []).forEach(function (row) {
      tbody.appendChild(h('tr', null, cols.map(function (c) {
        var v = renderCell ? renderCell(row, c) : row[c.key];
        return h('td', null, v === undefined || v === null ? '' : v);
      })));
    });
    return h('table', { class: 'table' }, thead, tbody);
  };

  // ---------- Chip / progress / empty ----------

  UI.chip = function (text, opts) {
    opts = opts || {};
    return h('span', { class: 'chip' }, text,
      opts.onremove ? h('i', { class: 'chip-x', onclick: function (e) { e.stopPropagation(); opts.onremove(); } }, '×') : null);
  };

  UI.progress = function (pct) {
    var fill = h('div', { class: 'progress-fill', style: { width: (Math.max(0, Math.min(100, Number(pct) || 0))) + '%' } });
    var el = h('div', { class: 'progress' }, fill);
    el.set = function (p) { fill.style.width = Math.max(0, Math.min(100, Number(p) || 0)) + '%'; };
    return el;
  };

  UI.empty = function (msg, icon) {
    return h('div', { class: 'empty' },
      h('span', { class: 'empty-icon' }, icon || '📭'),
      h('span', null, msg || 'Chưa có dữ liệu'));
  };

  // ---------- Toast ----------

  UI.toast = function (msg, type) {
    var host = document.getElementById('toasts');
    if (!host) { console.log('[toast]', type || 'success', msg); return; }
    var el = h('div', { class: 'toast' + (type === 'error' ? ' error' : '') },
      h('span', null, type === 'error' ? '⚠️' : '✅'), h('span', null, msg));
    host.appendChild(el);
    setTimeout(function () {
      el.classList.add('out');
      setTimeout(function () { el.remove(); }, 300);
    }, type === 'error' ? 5000 : 3500);
  };

  // ---------- Modal ----------

  UI.modal = function (opts) {
    opts = opts || {};
    var overlay;
    function close() {
      document.removeEventListener('keydown', onKey);
      if (overlay) overlay.remove();
    }
    function onKey(e) { if (e.key === 'Escape') close(); }

    var box = h('div', { class: 'modal' },
      h('div', { class: 'modal-title' },
        h('span', null, opts.title || ''),
        h('button', { class: 'modal-close', onclick: close, title: 'Đóng' }, '×')));
    if (opts.body) appendChild(box, opts.body);
    if (opts.actions && opts.actions.length) {
      box.appendChild(h('div', { class: 'modal-actions' }, opts.actions));
    }
    overlay = h('div', { class: 'modal-overlay' }, box);
    overlay.onclick = function (e) { if (e.target === overlay) close(); };
    document.addEventListener('keydown', onKey);
    document.body.appendChild(overlay);
    return { close: close, el: box };
  };
})();
