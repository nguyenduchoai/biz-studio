/* ============================================================
   Biz Studio — API client + Bus (SSE)
   Load đầu tiên. Không dùng framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  // ---------- HTTP ----------

  async function parseResponse(res) {
    var text = '';
    try { text = await res.text(); } catch (e) { text = ''; }
    var data = null;
    if (text) {
      try { data = JSON.parse(text); } catch (e) { data = null; }
    }
    if (!res.ok) {
      var msg = (data && data.error) ? data.error : ('Lỗi HTTP ' + res.status);
      throw new Error(msg);
    }
    return data;
  }

  async function request(method, path, body) {
    var opts = { method: method };
    if (body !== undefined) {
      opts.headers = { 'Content-Type': 'application/json' };
      opts.body = JSON.stringify(body);
    }
    var res;
    try {
      res = await fetch(path, opts);
    } catch (e) {
      throw new Error('Không kết nối được máy chủ (' + method + ' ' + path + ')');
    }
    return parseResponse(res);
  }

  window.API = {
    get: function (p) { return request('GET', p); },
    post: function (p, body) { return request('POST', p, body === undefined ? {} : body); },
    put: function (p, body) { return request('PUT', p, body === undefined ? {} : body); },
    del: function (p) { return request('DELETE', p); },

    // upload: multipart field "files" (nhiều file) + extra object → append thêm field
    upload: async function (p, files, extra) {
      var fd = new FormData();
      var list = files ? Array.prototype.slice.call(files) : [];
      for (var i = 0; i < list.length; i++) fd.append('files', list[i]);
      if (extra) {
        for (var k in extra) {
          if (Object.prototype.hasOwnProperty.call(extra, k) && extra[k] !== undefined && extra[k] !== null) {
            fd.append(k, extra[k]);
          }
        }
      }
      var res;
      try {
        res = await fetch(p, { method: 'POST', body: fd });
      } catch (e) {
        throw new Error('Không tải lên được (' + p + ')');
      }
      return parseResponse(res);
    }
  };

  // ---------- Bus (pub/sub) ----------

  var handlers = {};

  window.Bus = {
    on: function (event, fn) {
      if (!handlers[event]) handlers[event] = [];
      handlers[event].push(fn);
      return fn;
    },
    off: function (event, fn) {
      var arr = handlers[event];
      if (!arr) return;
      var i = arr.indexOf(fn);
      if (i >= 0) arr.splice(i, 1);
    },
    emit: function (event, data) {
      var arr = handlers[event];
      if (!arr) return;
      // copy để handler có thể tự off trong lúc emit
      arr.slice().forEach(function (fn) {
        try { fn(data); } catch (e) {
          console.error('Bus handler lỗi [' + event + ']:', e);
        }
      });
    }
  };

  // ---------- SSE /api/events/stream ----------

  var SSE_EVENTS = ['job', 'session_event', 'session', 'log', 'project'];
  var es = null;
  var reconnectTimer = null;
  var everConnected = false;

  // Sau khi mất kết nối rồi nối lại, các bản tin job phát trong lúc rớt đã mất.
  // Kéo lại danh sách job và phát cho các trang tự cập nhật (job dài như lồng
  // tiếng/render nếu không có bước này sẽ đứng mãi ở trạng thái cũ).
  function resyncJobs() {
    API.get('/api/jobs').then(function (jobs) {
      (jobs || []).slice(0, 30).forEach(function (j) { Bus.emit('job', j); });
    }).catch(function () { /* im lặng — lần reconnect sau sẽ thử lại */ });
  }

  function scheduleReconnect() {
    if (reconnectTimer) return;
    reconnectTimer = setTimeout(function () {
      reconnectTimer = null;
      connectSSE();
    }, 3000);
  }

  function connectSSE() {
    if (es) { try { es.close(); } catch (e) { /* bỏ qua */ } es = null; }
    try {
      es = new EventSource('/api/events/stream');
    } catch (e) {
      console.error('Không mở được SSE:', e);
      scheduleReconnect();
      return;
    }
    SSE_EVENTS.forEach(function (ev) {
      es.addEventListener(ev, function (msg) {
        var data = null;
        try { data = JSON.parse(msg.data); } catch (e) {
          console.error('SSE data không hợp lệ [' + ev + ']:', msg.data);
          return;
        }
        Bus.emit(ev, data);
      });
    });
    es.onopen = function () {
      Bus.emit('sse_status', { connected: true });
      if (everConnected) resyncJobs();
      everConnected = true;
    };
    es.onerror = function () {
      Bus.emit('sse_status', { connected: false });
      if (es) { try { es.close(); } catch (e) { /* bỏ qua */ } es = null; }
      scheduleReconnect();
    };
  }

  connectSSE();
})();
