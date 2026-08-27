/* Thao tác dùng chung của màn Biên tập video. */
(function () {
  'use strict';

  function hint(text) {
    return h('div', { class: 'muted', style: { fontSize: '12px', marginTop: '-8px', marginBottom: '14px' } }, text);
  }

  function showQueued(host, job) {
    host.innerHTML = '';
    host.appendChild(h('div', { class: 'update-inline mt-8' },
      h('span', null, 'Đã xếp hàng cắt khoảng lặng.'),
      h('span', { class: 'muted' }, job && job.id ? 'Mã tác vụ: ' + job.id : ''),
      h('a', { class: 'btn btn-ghost btn-sm', href: '#/logs' }, 'Xem tiến độ')));
  }

  function openAutocut(assets, resultHost) {
    var videos = (assets || []).filter(function (a) { return a.kind === 'video'; });
    if (!videos.length) {
      UI.toast('Dự án chưa có video để cắt.', 'error');
      return;
    }

    var chosen = videos[0].id;
    var guard = true;
    var autoThreshold = true;
    var silenceDb = -35;
    var minSilence = 0.8;
    var transcript = UI.input({ placeholder: 'Đường dẫn .words.json — có thể để trống' });
    var guardHost = h('div');
    var thresholdHost = h('div');

    function video() {
      return videos.filter(function (item) { return item.id === chosen; })[0];
    }

    function repaint() {
      guardHost.innerHTML = '';
      thresholdHost.innerHTML = '';
      if (guard) {
        guardHost.appendChild(UI.field('Transcript bảo vệ lời nói', transcript));
        guardHost.appendChild(hint('Có transcript, máy tránh cắt vào giữa chữ. Không có vẫn chạy được.'));
        thresholdHost.appendChild(UI.toggle('Tự đo ngưỡng theo video',
          'Khuyên dùng vì mỗi file có một mức ồn khác nhau.', autoThreshold,
          function (value) { autoThreshold = value; repaint(); }));
      }
      if (!guard || !autoThreshold) {
        thresholdHost.appendChild(UI.slider('Ngưỡng im lặng (dB)', {
          min: -50, max: -20, step: 1, value: silenceDb,
          oninput: function (value) { silenceDb = value; }
        }));
      }
    }
    repaint();

    var modal = UI.modal({
      title: 'Cắt khoảng lặng',
      body: h('div', null,
        UI.select('Video cần cắt', videos.map(function (item) {
          return { value: item.id, label: item.name };
        }), chosen, function (value) { chosen = value; }),
        UI.toggle('Bảo vệ bằng transcript', 'Giảm nguy cơ cắt mất âm cuối của lời nói.', guard,
          function (value) { guard = value; repaint(); }),
        guardHost,
        thresholdHost,
        UI.slider('Im lặng tối thiểu (giây)', {
          min: 0.3, max: 2, step: 0.1, value: minSilence,
          oninput: function (value) { minSilence = value; }
        })),
      actions: [
        UI.btn('Hủy', { variant: 'ghost', onclick: function () { modal.close(); } }),
        UI.btn('Bắt đầu cắt', {
          onclick: function () {
            var item = video();
            var body = {
              path: item.path,
              silenceDb: guard && autoThreshold ? 0 : silenceDb,
              minSilence: minSilence,
              guard: guard
            };
            if (guard && transcript.value.trim()) body.transcriptPath = transcript.value.trim();
            API.post('/api/tools/autocut', body).then(function (job) {
              modal.close();
              showQueued(resultHost, job);
              UI.toast('Đã bắt đầu cắt khoảng lặng.');
            }).catch(function (error) {
              UI.toast('Không chạy được: ' + error.message, 'error');
            });
          }
        })
      ]
    });
  }

  window.EditorTools = { openAutocut: openAutocut };
})();
