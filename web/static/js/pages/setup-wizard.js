/* Biz Studio — wizard cài đầy đủ cho lần chạy đầu. */
(function () {
  'use strict';

	function tr(s) { return (window.I18N && window.I18N.t) ? window.I18N.t(s) : s; }

  var listener = null;
	var READY_KEY = 'bizstudio-setup-ready-v2';
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
	  '.setup-windows{padding:14px;border:1px solid var(--border);border-radius:12px;background:rgba(127,127,127,.06);margin:12px 0}',
	  '.setup-check{display:flex;align-items:flex-start;gap:9px;margin-top:8px}',
	  '.setup-check strong{display:block}.setup-check small{display:block;color:var(--muted);margin-top:2px}',
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
	var windowsNeedsFirewall = !!(plan.windows && plan.windows.supported && !plan.windows.firewallReady);
	var windowsMissingWinget = !!(plan.windows && plan.windows.supported && !plan.windows.winGetReady);
	var hasTools = (plan.tools || []).length > 0;
    if (hasTools || windowsNeedsFirewall) {
	  var installLabel = 'Cài đầy đủ thành phần còn thiếu';
	  if (windowsMissingWinget && hasTools) installLabel = 'Cài App Installer / WinGet trước';
	  else if (hasTools && windowsNeedsFirewall) installLabel = 'Chuẩn bị Windows và cài đầy đủ';
	  else if (windowsNeedsFirewall) installLabel = 'Cho phép nhận file từ điện thoại';
      var install = UI.btn(plan.windowsPreparing ? '⏳ Chờ xác nhận UAC…' : (plan.running ? '⏳ Đang cài…' : installLabel), {
        variant: 'primary', disabled: !!plan.running,
        onclick: function () {
		  if (windowsMissingWinget && hasTools) {
			window.open('https://aka.ms/getwinget', '_blank', 'noopener');
			UI.toast(tr('Cài App Installer, mở lại Biz Studio rồi bấm kiểm tra lại.'));
			return;
		  }
          var names = plan.tools.map(function (t) { return '• ' + t.label; }).join('\n');
		  var firewallNote = windowsNeedsFirewall ?
			tr('• Windows sẽ hỏi quyền UAC để cho phép đúng Biz Studio nhận file QR trên mạng Private/Domain.') + '\n' : '';
          if (!window.confirm(tr('Biz Studio sẽ chuẩn bị máy như sau:') + '\n\n' + firewallNote + names +
              '\n\n' + tr('VieNeu/Whisper có thể tải model lớn. Tiếp tục?'))) return;
          install.disabled = true;
		  install.textContent = tr(windowsNeedsFirewall ? '⏳ Chờ xác nhận UAC…' : '⏳ Đang cài…');
          // The UAC dialog is cancelled from Windows itself. The Full installer
          // cancel endpoint only applies after the dependency batch has started.
          cancel.style.display = windowsNeedsFirewall ? 'none' : '';
          log.style.display = '';
		  appendLog(log, tr('Bắt đầu bộ cài Full…'));
		  var prepare = Promise.resolve(plan);
		  if (windowsNeedsFirewall) {
			appendLog(log, '▶ ' + tr('Đang mở cửa sổ quyền quản trị Windows…'));
			prepare = API.post('/api/setup/windows/firewall', { confirmed: true }).then(function () {
			  appendLog(log, '✓ ' + tr('Windows Firewall đã sẵn sàng cho QR'));
			  install.textContent = tr('⏳ Đang cài thư viện…');
			  return API.get('/api/setup/full/plan');
			});
		  }
		  prepare.then(function (freshPlan) {
			if (!(freshPlan.tools || []).length) { load(wrap); return null; }
			if (!freshPlan.planID) throw new Error(tr('Kế hoạch cài đặt chưa sẵn sàng; hãy kiểm tra lại.'));
			cancel.style.display = '';
			return API.post('/api/setup/full', { planID: freshPlan.planID, confirmed: true });
		  }).catch(function (err) {
            appendLog(log, '❌ ' + err.message);
			wrap._setupFailed = true;
			install.disabled = false;
			install.textContent = tr('Kiểm tra lại để thử lại');
			install.onclick = function () { wrap._setupFailed = false; load(wrap); };
			cancel.style.display = 'none';
		  });
		  pollUntilIdle(wrap);
        }
      });
      var cancel = UI.btn('Hủy cài đặt', { variant: 'ghost',
        onclick: function () { API.post('/api/setup/full/cancel').catch(function (e) { UI.toast(e.message, 'error'); }); }
      });
      cancel.style.display = plan.running && !plan.windowsPreparing ? '' : 'none';
      actions.appendChild(install);
      actions.appendChild(cancel);
    }

    var body = h('div', null,
      h('p', null, plan.note || ''),
	  windowsBox(plan.windows, load.bind(null, wrap)),
      tools,
      plan.needsLogin ? loginBox(load.bind(null, wrap)) : null,
      actions,
      log);
	if (!plan.needsSetup) {
	  try { localStorage.setItem(READY_KEY, '1'); } catch (e) { /* private mode */ }
      body = h('div', null,
        h('div', { class: 'text-green', style: { fontWeight: '700' } }, '✓ Máy đã sẵn sàng vận hành'),
		windowsBox(plan.windows),
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
		  install.textContent = tr('Kiểm tra lại để thử lại');
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

  function windowsBox(status, reload) {
	if (!status || !status.supported) return null;
	var box = h('div', { class: 'setup-windows' },
	  h('div', { style: { fontWeight: '800' } }, '🪟 Sẵn sàng trên Windows'),
	  checkLine(status.winGetReady, 'App Installer / WinGet', status.winGetReady ?
		'Dùng để cài các thư viện còn thiếu.' : 'Chưa có — cần cài App Installer trước.'),
	  checkLine(status.firewallReady, 'Nhận file QR qua Firewall', status.firewallReady ?
		'Chỉ cho phép đúng Biz Studio trên mạng Private/Domain.' : 'Cần xác nhận UAC khi thiết lập hoặc đổi vị trí ứng dụng.'),
	  checkLine(status.networkReady, 'Mạng nội bộ', status.networkReady ?
		(tr('Đang dùng') + ' ' + (status.networkCategory || 'Private/Domain') + '.') :
		(tr('Đang là') + ' ' + (status.networkCategory || tr('Public/không xác định')) + tr(' — hãy đổi Wi-Fi sang Private.'))));
	if (!status.winGetReady) {
	  box.appendChild(h('div', { class: 'setup-actions' },
		h('a', { class: 'btn btn-ghost btn-sm', href: 'https://aka.ms/getwinget', target: '_blank', rel: 'noopener' },
		  'Cài App Installer / WinGet')));
	}
	if (!status.networkReady && reload) {
	  box.appendChild(h('div', { class: 'setup-actions' }, UI.btn('Kiểm tra lại', {
		variant: 'ghost', small: true, onclick: reload
	  })));
	}
	if (status.detail) box.appendChild(h('div', { class: 'tool-detail text-red', style: { marginTop: '8px' } }, status.detail));
	return box;
  }

  function checkLine(ok, label, detail) {
	return h('div', { class: 'setup-check' },
	  h('span', null, ok ? '✅' : '⚠️'),
	  h('div', null, h('strong', null, label), h('small', null, detail)));
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
