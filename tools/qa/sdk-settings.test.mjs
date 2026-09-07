import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { chromium } from '@playwright/test';

const staticRoot = fileURLToPath(new URL('../../web/static/', import.meta.url));

test('SDK is opt-in, key stays masked, settings save does not call AI or install', async () => {
  const browser = await chromium.launch({ headless: true,
    ...(process.env.BIZSTUDIO_CHROME ? { executablePath: process.env.BIZSTUDIO_CHROME } : {}) });
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const errors = [], mutations = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.addInitScript(() => localStorage.setItem('bizstudio-setup-ready-v2', '1'));
  await page.route('**/*', async route => {
    const req = route.request(), url = new URL(req.url());
    if (req.method() !== 'GET') {
      mutations.push({ path: url.pathname, body: req.postDataJSON() });
      return route.fulfill({ contentType: 'application/json', body: '{}' });
    }
    if (url.pathname === '/api/events/stream') return route.fulfill({ contentType: 'text/event-stream', body: ': connected\n\n' });
    const fixtures = {
      '/api/instance': { app: 'bizstudio', version: 'test', dataID: 'fixture', platform: 'darwin', wizardEnabled: true },
      '/api/state': { app: { name: 'Biz Studio', version: 'test' }, host: {}, counts: {} },
      '/api/update': { available: false }, '/api/setup/tools': [],
      '/api/settings': { anthropicApiKey: '••••••••', claudeBackend: '', theme: 'light' },
      '/api/setup/full/plan': { needsSetup: false, needsLogin: false, statuses: [], tools: [], windows: { supported: false } }
    };
    if (Object.hasOwn(fixtures, url.pathname)) return route.fulfill({ contentType: 'application/json', body: JSON.stringify(fixtures[url.pathname]) });
    if (url.pathname.startsWith('/api/')) return route.fulfill({ contentType: 'application/json', body: '{}' });
    const relative = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
    const contentType = relative.endsWith('.js') ? 'text/javascript' : relative.endsWith('.css') ? 'text/css' : 'text/html';
    try { return await route.fulfill({ contentType, body: await readFile(path.join(staticRoot, relative)) }); }
    catch { return route.fulfill({ status: 404, body: 'not found' }); }
  });
  try {
    await page.goto('http://bizstudio.test/#/settings');
    const backend = page.locator('select').filter({ has: page.locator('option[value="sdk"]') });
    await backend.waitFor();
    assert.equal(await backend.inputValue(), 'cli');
    const key = page.getByPlaceholder('Không phải tài khoản hay mật khẩu Claude');
    assert.equal(await key.getAttribute('type'), 'password');
    assert.equal(await key.inputValue(), '••••••••');
    assert.equal(mutations.length, 0);
    await backend.selectOption('sdk');
    await page.getByRole('button', { name: '💾 Lưu cấu hình', exact: true }).click();
    await page.waitForFunction(() => document.body.innerText.includes('Đã lưu'));
    assert.equal(mutations.length, 1);
    assert.equal(mutations[0].path, '/api/settings');
    assert.equal(mutations[0].body.claudeBackend, 'sdk');
    assert.equal(mutations[0].body.anthropicApiKey, '••••••••');
    assert.deepEqual(errors, []);
    if (process.env.BIZSTUDIO_QA_SCREENSHOT) await page.screenshot({ path: process.env.BIZSTUDIO_QA_SCREENSHOT, fullPage: true });
  } finally { await browser.close(); }
});
