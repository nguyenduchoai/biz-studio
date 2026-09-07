import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { after, before, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { chromium } from '@playwright/test';

let browser;
const staticRoot = fileURLToPath(new URL('../../web/static/', import.meta.url));
before(async () => {
  browser = await chromium.launch({ headless: true, ...(process.env.BIZSTUDIO_CHROME ? { executablePath: process.env.BIZSTUDIO_CHROME } : {}) });
});
after(async () => { await browser?.close(); });

// Use the real index, app router, API client and wizard. Only HTTP responses
// are fixtures; a first-run test must not install tools or invoke real UAC.
async function macApp(alreadyConfigured = false) {
  const page = await browser.newPage();
  const mutations = [], errors = [];
  page.on('pageerror', error => errors.push(error.message));
  if (alreadyConfigured) {
    await page.addInitScript(() => localStorage.setItem('bizstudio-setup-ready-v2', '1'));
  }
  await page.route('**/*', async route => {
    const request = route.request();
    const url = new URL(request.url());
    if (request.method() !== 'GET') {
      mutations.push(url.pathname);
      return route.fulfill({ status: 405, contentType: 'application/json', body: '{"error":"No installer mutations allowed in this test"}' });
    }
    if (url.pathname === '/api/events/stream') {
      return route.fulfill({ contentType: 'text/event-stream', body: ': connected\n\n' });
    }
    const fixtures = {
      '/api/instance': { app: 'bizstudio', version: 'test', dataID: 'fixture', platform: 'darwin', wizardEnabled: true },
      '/api/state': { app: { name: 'Biz Studio', version: 'test' }, host: {}, counts: {} },
      '/api/update': { available: false },
      '/api/logs': [],
      '/api/setup/full/plan': {
        goos: 'darwin', planID: 'mac-consent-plan', needsSetup: true, needsLogin: true, running: false,
        windows: { supported: false },
        tools: [{ id: 'ffmpeg', label: 'FFmpeg', desc: 'Bộ xử lý video và âm thanh' }]
      }
    };
    if (Object.hasOwn(fixtures, url.pathname)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify(fixtures[url.pathname]) });
    }
    if (url.pathname.startsWith('/api/')) {
      return route.fulfill({ status: 404, contentType: 'application/json', body: '{"error":"Unexpected API request"}' });
    }
    const relative = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
    const contentType = relative.endsWith('.js') ? 'text/javascript' : relative.endsWith('.css') ? 'text/css' : 'text/html';
    try {
      return await route.fulfill({ contentType, body: await readFile(path.join(staticRoot, relative)) });
    } catch {
      return route.fulfill({ status: 404, body: 'Not found' });
    }
  });
  return { page, mutations, errors };
}

test('fresh macOS opens the prerequisite wizard without installing before consent', async () => {
  const { page, mutations, errors } = await macApp();
  try {
    await page.goto('http://bizstudio.test/');
    await page.waitForURL('**/#/setup');
    const install = page.getByRole('button', { name: 'Cài đầy đủ thành phần còn thiếu', exact: true });
    await install.waitFor({ state: 'visible' });
    assert.match(await page.locator('.setup-login').innerText(), /Mở Terminal/);
    assert.equal(await page.locator('.setup-windows').count(), 0);
    assert.doesNotMatch(await page.locator('.setup-wrap').innerText(), /PowerShell|UAC|WinGet|Git for Windows/);
    assert.deepEqual(mutations, [], 'Opening the wizard must never start an installer');
    page.once('dialog', dialog => dialog.dismiss());
    await install.click();
    assert.deepEqual(mutations, [], 'Declining consent must not start an installer');
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test('configured macOS preserves the requested page instead of reopening setup', async () => {
  const { page, mutations, errors } = await macApp(true);
  try {
    await page.goto('http://bizstudio.test/#/logs');
    await page.getByRole('heading', { name: 'Nhật ký', exact: true }).waitFor();
    assert.equal(new URL(page.url()).hash, '#/logs');
    assert.equal(await page.locator('.setup-wrap').count(), 0);
    assert.deepEqual(mutations, []);
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});
