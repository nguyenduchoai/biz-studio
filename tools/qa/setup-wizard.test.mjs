import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from '@playwright/test';
import { fileURLToPath } from 'node:url';

let browser;
before(async () => {
  browser = await chromium.launch({ headless: true, ...(process.env.BIZSTUDIO_CHROME ? { executablePath: process.env.BIZSTUDIO_CHROME } : {}) });
});
after(async () => { await browser?.close(); });

async function wizard(options = {}) {
  const page = await browser.newPage();
  const errors = [];
  page.on('pageerror', err => errors.push(err.message));
  await page.setContent('<div id="wizard"></div>');
  await page.evaluate(options => {
    window.App = { pages: {}, navigate(route) { window.navigatedTo = route; } };
    window.Bus = { on() {}, off() {} };
    window.confirm = () => true;
    window.calls = [];
    window.getCount = 0;
    const plan = {
      planID: 'confirmed-plan', tools: options.noTools ? [] : [{ id: 'ffmpeg', label: 'FFmpeg', desc: 'Media tools' }],
      needsSetup: true, needsLogin: options.login || false, goos: options.goos || 'windows',
      windows: { supported: !options.login, firewallReady: false, winGetReady: true, networkReady: true }
    };
    window.API = {
      async get() { window.getCount++; return { ...plan, running: window.calls.some(c => c.path === '/api/setup/full') }; },
      async post(path, body) {
        calls.push({ path, body });
        if (path.endsWith('/firewall')) {
          if (options.slow) await new Promise(resolve => { window.finishUAC = resolve; });
          throw new Error('Windows chưa cho phép Firewall');
        }
        return { ok: true };
      }
    };
  }, options);
  await page.addScriptTag({ path: fileURLToPath(new URL('../../web/static/js/ui.js', import.meta.url)) });
  await page.addScriptTag({ path: fileURLToPath(new URL('../../web/static/js/pages/setup-wizard.js', import.meta.url)) });
  await page.evaluate(() => App.pages.setup.render(document.getElementById('wizard')));
  return { page, errors };
}

test('UAC/Firewall failure does not block confirmed dependency installation', async () => {
  const { page, errors } = await wizard();
  try {
    await page.getByRole('button', { name: 'Chuẩn bị Windows và cài đầy đủ', exact: true }).click();
    await page.waitForFunction(() => calls.some(c => c.path === '/api/setup/full'));
    const full = await page.evaluate(() => calls.find(c => c.path === '/api/setup/full'));
    assert.deepEqual(full.body, { confirmed: true, planID: 'confirmed-plan' });
    assert.match(await page.locator('.setup-log').innerText(), /Chưa bật nhận file QR/);
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test('slow UAC response must keep current log and disabled install button', async () => {
  const { page, errors } = await wizard({ slow: true });
  try {
    await page.getByRole('button', { name: 'Chuẩn bị Windows và cài đầy đủ', exact: true }).click();
    await page.waitForFunction(() => getCount >= 2);
    assert.equal(await page.getByRole('button', { name: '⏳ Chờ xác nhận UAC…', exact: true }).isDisabled(), true);
    await page.evaluate(() => finishUAC());
    await page.waitForFunction(() => calls.some(c => c.path === '/api/setup/full'));
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test('macOS Claude sign-in directs users to Terminal', async () => {
  const { page } = await wizard({ login: true, noTools: true, goos: 'darwin' });
  try {
    assert.match(await page.locator('.setup-login').innerText(), /Mở Terminal/);
    assert.doesNotMatch(await page.locator('.setup-login').innerText(), /PowerShell/);
  } finally { await page.close(); }
});

test('Windows Claude sign-in directs users to PowerShell', async () => {
  const { page } = await wizard({ login: true, noTools: true, goos: 'windows' });
  try { assert.match(await page.locator('.setup-login').innerText(), /Mở PowerShell/); }
  finally { await page.close(); }
});

test('installed tools allow entering app while optional QR is not ready', async () => {
  const { page } = await wizard({ noTools: true });
  try {
    await page.getByRole('button', { name: 'Vào Biz Studio', exact: true }).click();
    assert.equal(await page.evaluate(() => window.navigatedTo), 'dashboard');
    assert.equal(await page.evaluate(() => calls.length), 0);
  } finally { await page.close(); }
});
