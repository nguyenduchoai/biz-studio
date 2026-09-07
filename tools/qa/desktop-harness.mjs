import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, mkdir, copyFile, readFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';

export function command(bin, args) {
  const result = spawnSync(bin, args, { encoding: 'utf8', timeout: 120000, windowsHide: true });
  assert.equal(result.status, 0, `${bin}: ${result.error || result.stderr || result.stdout}`);
  return result.stdout;
}

export async function createHarness(binary) {
  const dir = await mkdtemp(path.join(tmpdir(), 'bizstudio-qa-'));
  const data = path.join(dir, 'Dữ liệu có khoảng trắng');
  const appDir = path.join(dir, 'Ứng dụng');
  await mkdir(appDir);
  const executable = path.join(appDir, path.basename(binary));
  await copyFile(path.resolve(binary), executable);
  // Hold the preferred port throughout: the app must choose a free port and
  // subsequent launches must find the actual instance, not this listener.
  const occupied = createServer(socket => socket.end('HTTP/1.1 503 Busy\r\nContent-Length: 0\r\n\r\n'));
  occupied.listen(0, '127.0.0.1');
  await once(occupied, 'listening');
  const port = occupied.address().port;
  const args = ['-window=false', '-port', String(port), '-data', data];
  let proc, output = '', base = '';
  const launch = () => {
    const child = spawn(executable, args, { cwd: dir, windowsHide: true });
    child.stdout.on('data', buf => { output += buf; });
    child.stderr.on('data', buf => { output += buf; });
    return child;
  };
  async function stop() {
    if (!proc || proc.exitCode !== null || proc.signalCode !== null) return;
    const ended = once(proc, 'exit');
    proc.kill();
    await Promise.race([ended, delay(10000).then(() => { throw new Error('App did not stop'); })]);
  }
  async function start() {
    proc = launch();
    for (let i = 0; i < 100; i++) {
      assert.equal(proc.exitCode, null, `App exited: ${output}`);
      try {
        const marker = JSON.parse(await readFile(path.join(data, 'instance.json'), 'utf8'));
        const res = await fetch(marker.url + '/api/instance', { signal: AbortSignal.timeout(1000) });
        const info = await res.json();
        if (info.app === 'bizstudio') { base = marker.url; return; }
      } catch {}
      await delay(200);
    }
    throw new Error(`App never ready: ${output}`);
  }
  async function request(route, method = 'GET', body) {
    const res = await fetch(base + route, {
      method, signal: AbortSignal.timeout(60000),
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
    assert.ok(res.ok, `${method} ${route}: ${res.status} ${await res.clone().text()}`);
    return res.json();
  }
  return {
    dir, data, port, args, executable, start, stop, launch, request,
    get base() { return base; },
    async dispose() { await stop(); occupied.close(); },
    async waitJob(job) {
      for (let i = 0; i < 180; i++) {
        const current = await request('/api/jobs/' + job.id);
        if (current.status === 'done') return current;
        assert.notEqual(current.status, 'error', current.error);
        await delay(500);
      }
      throw new Error('Media job did not finish');
    }
  };
}
