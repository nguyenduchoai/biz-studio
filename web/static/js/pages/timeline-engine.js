/* ============================================================
   Biz Studio — bộ máy nghe thử timeline nhiều lớp.

   Vì sao tự trộn trong trình duyệt: kéo một đoạn nhạc rồi phải chờ máy chủ
   render mới biết nghe ra sao thì không ai dựng nổi. Trình duyệt trộn âm thanh
   chính xác và tức thì, nên xem trước ĐÚNG bằng bản xuất ra — trừ một chỗ nói
   rõ bên dưới.

   Video nền do thẻ <video> phát (kèm tiếng gốc). Các lớp còn lại phát bằng
   WebAudio, hẹn giờ theo currentTime của video.

   Né giọng: WebAudio không có sidechain như ffmpeg. Nhưng ta BIẾT TRƯỚC lời đọc
   nằm ở đâu, nên hạ nhạc đúng những khoảng đó bằng automation. Gần đúng, không
   phải y hệt — khác biệt nằm ở đường cong lên xuống, không nằm ở việc nhạc có
   lùi hay không.

   Nạp trước timeline.js. Không framework / ES modules.
   ============================================================ */
(function () {
  'use strict';

  var DUCK_DB = -9;      // nhạc lùi bao nhiêu khi có lời đọc
  var DUCK_ATTACK = 0.02; // giây — khớp attack 20ms của ffmpeg
  var DUCK_RELEASE = 0.4; // giây — khớp release 400ms

  function dbToGain(db) { return Math.pow(10, (Number(db) || 0) / 20); }

  function Engine(video) {
    this.video = video;
    this.ctx = null;
    this.buffers = {};   // path → AudioBuffer
    this.playing = [];   // các nguồn đang phát, để dừng khi tua
    this.doc = null;
    this.onCue = null;   // gọi lại khi dòng phụ đề đang hiện đổi
    this.lastCue = undefined;
    this._bind();
  }

  Engine.prototype._bind = function () {
    var self = this;
    var v = this.video;
    v.addEventListener('play', function () { self.resync(); });
    v.addEventListener('pause', function () { self.stopAll(); });
    v.addEventListener('seeking', function () { self.stopAll(); });
    v.addEventListener('seeked', function () { if (!v.paused) self.resync(); self.tickCue(); });
    v.addEventListener('timeupdate', function () { self.tickCue(); });
    v.addEventListener('ratechange', function () { self.resync(); });
  };

  Engine.prototype.setDoc = function (doc) {
    this.doc = doc;
    if (!this.video.paused) this.resync();
    this.tickCue();
  };

  // ---------- nạp & giải mã ----------

  Engine.prototype.audio = function () {
    if (!this.ctx) {
      var AC = window.AudioContext || window.webkitAudioContext;
      this.ctx = new AC();
    }
    // Trình duyệt khoá AudioContext cho tới khi người dùng bấm gì đó; không mở
    // lại thì mọi lớp im re mà không có lỗi nào.
    if (this.ctx.state === 'suspended') this.ctx.resume();
    return this.ctx;
  };

  // preload nạp mọi file trong tài liệu. Trả Promise xong khi tải hết.
  Engine.prototype.preload = function (doc) {
    var self = this, ctx = this.audio(), jobs = [];
    (doc.tracks || []).forEach(function (t) {
      (t.items || []).forEach(function (it) {
        if (!it.path || self.buffers[it.path]) return;
        self.buffers[it.path] = null; // đánh dấu đang tải, tránh tải hai lần
        jobs.push(fetch('/data/' + it.path)
          .then(function (r) {
            if (!r.ok) throw new Error('HTTP ' + r.status);
            return r.arrayBuffer();
          })
          .then(function (buf) { return ctx.decodeAudioData(buf); })
          .then(function (audio) { self.buffers[it.path] = audio; })
          .catch(function (e) {
            delete self.buffers[it.path];
            console.error('Không nghe thử được ' + it.path + ':', e);
          }));
      });
    });
    return Promise.all(jobs);
  };

  // ---------- phát ----------

  Engine.prototype.stopAll = function () {
    this.playing.forEach(function (s) { try { s.stop(); } catch (e) { /* đã dừng */ } });
    this.playing = [];
  };

  // resync dừng hết rồi hẹn lại mọi đoạn theo vị trí hiện tại của video.
  //
  // Hẹn lại từ đầu thay vì chỉnh từng nguồn: tua tới lui vài lần là các nguồn
  // lệch nhau vài chục mili giây, mà tai nghe ra ngay ở lời đọc.
  Engine.prototype.resync = function () {
    this.stopAll();
    if (!this.doc || this.video.paused) return;

    var ctx = this.audio();
    var now = ctx.currentTime;
    var t = this.video.currentTime;
    var rate = this.video.playbackRate || 1;
    var self = this;

    var narration = this.narrationSpans();

    (this.doc.tracks || []).forEach(function (tr) {
      if (tr.mute || tr.role === 'source') return;
      var trackGain = dbToGain(tr.gain);

      (tr.items || []).forEach(function (it) {
        var buf = self.buffers[it.path];
        if (!buf) return;
        var dur = (it.out > it.in) ? (it.out - it.in) : 0;
        if (dur <= 0) return;

        var start = it.at, end = it.at + dur;
        if (end <= t) return;                       // đã qua

        var offsetIn = Math.max(0, t - start);      // đang phát dở đoạn này
        var when = now + Math.max(0, (start - t) / rate);
        var play = dur - offsetIn;
        if (play <= 0) return;

        var g = ctx.createGain();
        g.gain.value = trackGain * dbToGain(it.gain);
        self.applyFades(g, it, when, offsetIn, play, rate);
        if (tr.role === 'music' && tr.duck) {
          self.applyDuck(g, narration, t, now, rate, trackGain * dbToGain(it.gain));
        }
        g.connect(ctx.destination);

        var src = ctx.createBufferSource();
        src.buffer = buf;
        src.playbackRate.value = rate;
        src.connect(g);
        src.start(when, it.in + offsetIn, play);
        self.playing.push(src);
      });
    });
  };

  Engine.prototype.applyFades = function (g, it, when, offsetIn, play, rate) {
    var base = g.gain.value;
    if (it.fadeIn > 0 && offsetIn < it.fadeIn) {
      var left = (it.fadeIn - offsetIn) / rate;
      g.gain.setValueAtTime(base * (offsetIn / it.fadeIn), when);
      g.gain.linearRampToValueAtTime(base, when + left);
    }
    if (it.fadeOut > 0) {
      var outAt = when + (play - it.fadeOut) / rate;
      if (outAt > when) {
        g.gain.setValueAtTime(base, outAt);
        g.gain.linearRampToValueAtTime(0.0001, outAt + it.fadeOut / rate);
      }
    }
  };

  // narrationSpans trả các khoảng [đầu, cuối] có lời đọc trên timeline.
  Engine.prototype.narrationSpans = function () {
    var out = [];
    (this.doc.tracks || []).forEach(function (t) {
      if (t.mute || t.role !== 'narration') return;
      (t.items || []).forEach(function (it) {
        var dur = (it.out > it.in) ? (it.out - it.in) : 0;
        if (dur > 0) out.push([it.at, it.at + dur]);
      });
    });
    return out.sort(function (a, b) { return a[0] - b[0]; });
  };

  Engine.prototype.applyDuck = function (g, spans, tNow, ctxNow, rate, base) {
    var low = base * dbToGain(DUCK_DB);
    var inside = spans.some(function (s) { return tNow >= s[0] && tNow < s[1]; });
    g.gain.setValueAtTime(inside ? low : base, ctxNow);
    spans.forEach(function (s) {
      if (s[1] <= tNow) return;
      var down = ctxNow + Math.max(0, (s[0] - tNow) / rate);
      var up = ctxNow + Math.max(0, (s[1] - tNow) / rate);
      if (s[0] > tNow) {
        g.gain.setTargetAtTime(low, down, DUCK_ATTACK);
      }
      g.gain.setTargetAtTime(base, up, DUCK_RELEASE);
    });
  };

  // ---------- phụ đề ----------

  Engine.prototype.cueAt = function (t) {
    var subs = (this.doc && this.doc.subs) || [];
    for (var i = 0; i < subs.length; i++) {
      if (t >= subs[i].start && t < subs[i].end) return subs[i];
    }
    return null;
  };

  Engine.prototype.tickCue = function () {
    if (!this.onCue) return;
    var c = this.cueAt(this.video.currentTime);
    var id = c ? c.id + '|' + c.text : null;
    if (id === this.lastCue) return; // chỉ báo khi ĐỔI, tránh vẽ lại 4 lần/giây
    this.lastCue = id;
    this.onCue(c);
  };

  Engine.prototype.destroy = function () {
    this.stopAll();
    if (this.ctx) { try { this.ctx.close(); } catch (e) { /* đã đóng */ } this.ctx = null; }
  };

  window.TimelineEngine = Engine;
})();
