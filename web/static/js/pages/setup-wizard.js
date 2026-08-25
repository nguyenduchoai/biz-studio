/* Biz Studio — wizard cài đầy đủ cho lần chạy đầu. */
(function () {
  'use strict';

  var listener = null;
	var READY_KEY = 'bizstudio-setup-ready-v1';
	var pollTimer = null;

	function firstRunNeeded() {
	  try { return localStorage.getItem(READY_KEY) !== '1'; } catch (e) { return true; }
  }

  function injectStyles() {
    if (document.getElementById('setup-wizard-style')) return;
    document.head.appendChild(h('style', { id: 'setup-wizard-style' }, [
      '.setup-wrap{max-width:820px;margin:0 auto}',
      '.setup-tool{display:flex;gap:12px;padding:12px 0;border-top:1px solid var(--border)}',
      '.setup-tool:first-child{border-top:none}',
      '.setup-tool-name{font-weight:700}',
      '.setup-tool-desc{font-size:12px;color:var(--muted);margin-top:3px}',
      '.setup-pkg{font:11px ui-monospace,SFMono-Regular,Menlo,monospace;color:var(--muted);margin-top:4px}',
      '.setup-actions{display:flex;gap:10px;flex-wrap:wrap;margin-top:16px}',
      '.setup-login{padding:14px;border:1px solid #F59E0B;border-radius:10px;background:rgba(245,158,11,.08);margin-top:14px}',
      '.setup-log{margin-top:14px;max-height:300px;overflow:auto;white-space:pre-wrap;word-break:break-word;',
      'font:12px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace;padding:12px;border-radius:10px;background:rgba(127,127,127,.10)}'
    ].join('\n')));
  }

  function render(host) {
    injectStyles();
    var wrap = h('div', { class: 'setup-wrap' });
    host.appendChild(wrap);
    load(wrap);
  }

  function load(wrap) {
    wrap.innerHTML = '';
    wrap.appendChild(UI.card({ title: 'Đang kiểm tra máy…', icon: '🔎', body: UI.spinner() }));
    API.get('/api/setup/full/plan').then(function (plan) {
      draw(wrap, plan);
    }).catch(function (err) {
      wrap.innerHTML = '';
      wrap.appendChild(UI.card({ title: 'Không kiểm tra được bộ cài', icon: '❌',
        body: h('div', { class: 'text-red' }, err.message) }));
    });
  }

  function draw(wrap, plan) {
	if (pollTimer) { clearTimeout(pollTimer); pollTimer = null; }
    wrap.innerHTML = '';
    var log = h('div', { class: 'setup-log', style: { display: 'none' } });
    var tools = h('div');
    (plan.tools || []).forEach(function (tool) {
      tools.appendChild(h('div', { class: 'setup-tool' },
        h('span', null, '⬇️'),
        h('div', null,
          h('div', { class: 'setup-tool-name' }, tool.label),
          h('div', { class: 'setup-tool-desc' }, tool.desc),
          tool.windowsPackage ? h('div', { class: 'setup-pkg' }, 'WinGet · ' + tool.windowsPackage) : null)));
    });

    var actions = h('div', { class: 'setup-actions' });
    if ((plan.tools || []).length) {
      var install = UI.btn(plan.running ? '⏳ Đang cài…' : 'Cài đầy đủ thành phần còn thiếu', {
        variant: 'primary', disabled: !!plan.running,
        onclick: function () {
          var names = plan.tools.map(function (t) { return '• ' + t.label; }).join('\n');
          if (!window.confirm('Biz Studio sẽ cài tuần tự các thành phần sau:\n\n' + names +
              '\n\nVieNeu/Whisper có thể tải model lớn. Tiếp tục?')) return;
          install.disabled = true;
          install.textContent = '⏳ Đang cài…';
          cancel.style.display = '';
          log.style.display = '';
          appendLog(log, 'Bắt đầu bộ cài Full…');
		  API.post('/api/setup/full', { planID: plan.planID, confirmed: true }).catch(function (err) {
            appendLog(log, '❌ ' + err.message);
			wrap._setupFailed = true;
			install.disabled = false;
			install.textContent = 'Kiểm tra lại để thử lại';
			install.onclick = function () { wrap._setupFailed = false; load(wrap); };
			cancel.style.display = 'none';
		  });
		  pollUntilIdle(wrap);
        }
      });
      var cancel = UI.btn('Hủy cài đặt', { variant: 'ghost',
        onclick: function () { API.post('/api/setup/full/cancel').catch(function (e) { UI.toast(e.message, 'error'); }); }
      });
      cancel.style.display = plan.running ? '' : 'none';
      actions.appendChild(install);
      actions.appendChild(cancel);
    }

    var body = h('div', null,
      h('p', null, plan.note || ''),
      tools,
      plan.needsLogin ? loginBox(load.bind(null, wrap)) : null,
      actions,
      log);
	if (!plan.needsSetup) {
	  try { localStorage.setItem(READY_KEY, '1'); } catch (e) { /* private mode */ }
      body = h('div', null,
        h('div', { class: 'text-green', style: { fontWeight: '700' } }, '✓ Máy đã sẵn sàng vận hành'),
        h('div', { class: 'setup-actions' }, UI.btn('Vào Biz Studio', {
          variant: 'primary', onclick: function () { App.navigate('dashboard'); }
        })));
    }
    wrap.appendChild(UI.card({ title: 'Thiết lập lần đầu', icon: '🧰',
      desc: 'Cài các thư viện cần thiết ngay trong ứng dụng; không cần tự gõ lệnh cài.', body: body }));

    if (listener) Bus.off('setup', listener);
    listener = function (event) {
      if (event.batch !== 'full') return;
      if (event.line) { log.style.display = ''; appendLog(log, event.line); }
      if (event.state === 'error') {
		wrap._setupFailed = true;
        appendLog(log, '❌ ' + (event.error || 'Cài đặt thất bại'));
        if (event.manual) appendLog(log, 'Tải thủ công: ' + event.manual);
		if (install) {
		  install.disabled = false;
		  install.textContent = 'Kiểm tra lại để thử lại';
		  install.onclick = function () { wrap._setupFailed = false; load(wrap); };
		}
		if (cancel) cancel.style.display = 'none';
      }
      if (event.tool === '' && event.state === 'done') load(wrap);
    };
	Bus.on('setup', listener);
	if (plan.running) pollUntilIdle(wrap);
	App._cleanup = function () {
	  if (listener) Bus.off('setup', listener);
	  if (pollTimer) clearTimeout(pollTimer);
	  listener = null; pollTimer = null;
	};
  }

	function pollUntilIdle(wrap) {
	  if (pollTimer) clearTimeout(pollTimer);
	  pollTimer = setTimeout(function () {
		API.get('/api/setup/full/plan').then(function (plan) {
		  if (plan.running) pollUntilIdle(wrap);
		  else if (!wrap._setupFailed) load(wrap);
		}).catch(function () { pollUntilIdle(wrap); });
	  }, 3000);
	}

  function loginBox(reload) {
    var command = 'claude auth login';
    return h('div', { class: 'setup-login' },
      h('div', { style: { fontWeight: '700' } }, 'Bước riêng: đăng nhập Claude'),
      h('div', { class: 'tool-detail' }, 'Claude CLI đã được cài. Mở PowerShell, chạy lệnh dưới đây và đăng nhập trực tiếp với Claude. Biz Studio không đọc hay lưu thông tin đăng nhập.'),
      h('div', { class: 'setup-actions' },
        UI.btn('Sao chép: ' + command, { variant: 'ghost', small: true, onclick: function () {
		  if (!navigator.clipboard || !navigator.clipboard.writeText) {
			UI.toast('Hãy sao chép thủ công: ' + command, 'error'); return;
		  }
		  navigator.clipboard.writeText(command).then(function () { UI.toast('Đã sao chép lệnh'); })
			.catch(function () { UI.toast('Hãy sao chép thủ công: ' + command, 'error'); });
        }}),
        UI.btn('Kiểm tra lại', { variant: 'primary', small: true, onclick: reload })));
  }

  function appendLog(box, line) {
    box.appendChild(document.createTextNode(String(line) + '\n'));
    while (box.childNodes.length > 500) box.removeChild(box.firstChild);
    box.scrollTop = box.scrollHeight;
  }

  App.pages.setup = { title: 'Thiết lập Biz Studio', subtitle: 'Cài đầy đủ thư viện để đưa vào vận hành', render: render };
  window.SetupWizard = { firstRunNeeded: firstRunNeeded };
})();
