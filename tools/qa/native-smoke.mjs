import assert from 'node:assert/strict';
import { once } from 'node:events';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '@playwright/test';
import jsQR from 'jsqr';
import { PNG } from 'pngjs';
import { createHarness, command } from './desktop-harness.mjs';

assert.ok(process.argv[2], 'Usage: node tools/qa/native-smoke.mjs <native executable>');
const app = await createHarness(process.argv[2]);
let browser;
try {
  await app.start();
  const state = await app.request('/api/state');
  assert.equal(state.app.name, 'Biz Studio');
  assert.notEqual(state.port, app.port, 'Must survive an occupied preferred port');
  assert.ok(state.mobilePort > 0);
  const second = app.launch();
  const [code] = await Promise.race([once(second, 'exit'), delay(15000).then(() => { second.kill(); throw new Error('Second launch hung'); })]);
  assert.equal(code, 0, 'Second launch must reuse the existing instance');
  await app.request('/api/instance');

  if (process.platform === 'win32') {
    const windows = await app.request('/api/setup/windows/status');
    assert.equal(windows.supported, true);
    assert.ok(!windows.detail, windows.detail);
  }
  const project = await app.request('/api/projects', 'POST', { name: 'Kiểm tra Windows và Mac', width: 320, height: 180, fps: 24 });
  // Decode the actual QR returned by the app, then exercise its upload URL.
  const qrRes = await fetch(app.base + '/api/qr.png?project=' + project.id);
  assert.equal(qrRes.status, 200, await qrRes.clone().text());
  const png = PNG.sync.read(Buffer.from(await qrRes.arrayBuffer()));
  const qr = jsQR(new Uint8ClampedArray(png.data), png.width, png.height);
  assert.ok(qr, 'App QR must be scannable');
  const phone = new URL(qr.data);
  assert.equal(Number(phone.port), state.mobilePort);
  const denied = await fetch(phone.origin + '/api/setup/tools');
  assert.equal(denied.status, 404, 'Phone listener must not expose admin APIs');
  assert.equal((await fetch(phone.origin + phone.pathname)).status, 403, 'Phone needs QR token');

  const video = path.join(app.dir, 'video nguồn.mp4');
  const audio = path.join(app.dir, 'lời đọc.wav');
  command('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'color=c=blue:s=320x180:r=24:d=2', '-f', 'lavfi', '-i', 'sine=frequency=440:duration=2', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-shortest', '-y', video]);
  command('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'sine=frequency=880:duration=2', '-y', audio]);
  browser = await chromium.launch({ headless: true, ...(process.env.BIZSTUDIO_CHROME ? { executablePath: process.env.BIZSTUDIO_CHROME } : {}) });
  const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } });
  const errors = [];
  mobile.on('pageerror', err => errors.push(err.message));
  await mobile.goto(phone.href);
  // The real mobile page selects and sends both video and audio together.
  await mobile.locator('input[type=file]').first().setInputFiles([video, audio]);
  const uploaded = mobile.waitForResponse(res => res.request().method() === 'POST' && res.url().includes('/upload'));
  await mobile.locator('#uploadBtn').click();
  assert.equal((await uploaded).status(), 200);
  const result = await app.request('/api/projects/' + project.id);
  assert.equal(result.assets.length, 2);
  assert.deepEqual(new Set(result.assets.map(a => a.kind)), new Set(['video', 'audio']));
  for (const asset of result.assets) assert.ok(asset.duration > 1.5, `Cannot probe ${asset.name}`);

  const timeline = await app.request('/api/projects/' + project.id + '/timeline');
  assert.ok(timeline.video);
  await app.request('/api/projects/' + project.id + '/timeline', 'PUT', timeline);
  const job = await app.waitJob(await app.request('/api/projects/' + project.id + '/timeline/render', 'POST', {}));
  assert.equal((await app.request('/api/projects/' + project.id)).project.outputFile, job.output,
    'Timeline render must be available as the project output for preview/QC/export');
  const output = path.join(app.data, job.output);
  const probe = JSON.parse(command('ffprobe', ['-v', 'error', '-show_format', '-show_streams', '-of', 'json', output]));
  assert.ok(Number(probe.format.duration) > 1.5);
  assert.ok(probe.streams.some(s => s.codec_type === 'video'));
  assert.ok(probe.streams.some(s => s.codec_type === 'audio'));
  command('ffmpeg', ['-v', 'error', '-i', output, '-f', 'null', '-']);

  const page = await browser.newPage({ viewport: { width: 1440, height: 960 } });
  page.on('pageerror', err => errors.push(err.message));
  await page.addInitScript(() => localStorage.setItem('bizstudio-setup-ready-v2', '1'));
  await page.goto(app.base + '/#/projects/' + project.id);
  await page.waitForFunction(() => document.body.innerText.includes('Kiểm tra Windows và Mac'));
  await page.screenshot({ path: path.join(app.dir, 'desktop.png'), fullPage: true });
  const player = await browser.newPage();
  await player.goto(app.base);
  const playable = await player.evaluate(async src => {
    const video = document.createElement('video');
    video.muted = true;
    video.src = src;
    document.body.appendChild(video);
    await video.play();
    await new Promise(resolve => setTimeout(resolve, 400));
    return video.videoWidth > 0 && video.currentTime > 0;
  }, app.base + '/data/' + job.output.replaceAll('\\', '/'));
  assert.equal(playable, true, 'Rendered output must actually play in Chromium');
  assert.deepEqual(errors, []);
  await browser.close(); browser = null;

  await app.stop();
  await app.start();
  const restored = await app.request('/api/projects/' + project.id);
  assert.equal(restored.assets.length, 2, 'Restart must preserve uploaded resources');
  assert.equal((await app.request('/api/projects/' + project.id + '/timeline')).video, timeline.video);
  console.log(`PASS ${process.platform}/${process.arch}: port collision, second launch, QR audio/video, timeline render/playback, restart persistence. Evidence: ${app.dir}`);
} finally {
  if (browser) await browser.close();
  await app.dispose();
}
