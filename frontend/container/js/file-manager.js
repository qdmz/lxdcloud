/* ============================================================
   LXD Panel - 容器端在线文件管理（类 WebSFTP）
   依赖：app.js 的 LXD.authRequest（自动携带 X-Container-Hash）
   ============================================================ */
(function () {
  'use strict';

  const hash = LXD.getHash();
  if (!hash) { window.location.href = 'login.html'; return; }

  const api = '/api/container/files';

  let curPath = '/';
  let entries = [];

  // 品牌
  LXD.loadBrand().then((brand) => {
    LXD.applyBrand(brand);
  }).catch(() => {});

  // 顶部按钮
  document.getElementById('backBtn').addEventListener('click', () => { window.location.href = 'dashboard.html'; });
  document.getElementById('refreshBtn').addEventListener('click', load);
  document.getElementById('logoutBtn').addEventListener('click', () => {
    LXD.clearHash();
    window.location.href = 'login.html';
  });

  function empty(text) {
    return '<div class="empty"><div class="empty-icon">▫</div>' + LXD.esc(text || '暂无数据') + '</div>';
  }

  function renderPathBar() {
    const bar = document.getElementById('fmPathLabel');
    const parts = curPath.split('/').filter(Boolean);
    let html = '<a href="javascript:;" data-path="/" style="color:var(--accent);text-decoration:none">/</a>';
    let acc = '';
    parts.forEach((p, i) => {
      acc += '/' + p;
      html += ' <span style="color:var(--text-3)">/</span> <a href="javascript:;" data-path="' + LXD.esc(acc) + '" style="color:var(--accent);text-decoration:none">' + LXD.esc(p) + '</a>';
    });
    bar.innerHTML = html;
    bar.querySelectorAll('a[data-path]').forEach((a) => {
      a.addEventListener('click', () => { curPath = a.dataset.path; load(); });
    });
  }

  function fmtSize(n) {
    if (n == null || isNaN(n) || n < 0) return '-';
    if (n < 1024) return n + ' B';
    const units = ['KB', 'MB', 'GB', 'TB'];
    let v = n;
    for (const u of units) {
      v /= 1024;
      if (v < 1024) return v.toFixed(1) + ' ' + u;
    }
    return v.toFixed(1) + ' PB';
  }

  function render() {
    const area = document.getElementById('fmArea');
    if (!entries.length) {
      area.innerHTML = empty('目录为空');
      return;
    }
    let h = '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
      '<th>名称</th><th>大小</th><th>修改时间</th><th>类型</th><th>操作</th>' +
      '</tr></thead><tbody>';
    entries.forEach((e) => {
      const icon = e.is_dir ? '📁' : (e.is_symlink ? '🔗' : '📄');
      const nameCell = e.is_dir
        ? '<a href="javascript:;" class="fm-open" data-path="' + LXD.esc(e.path) + '" style="color:var(--accent);text-decoration:none">' + icon + ' ' + LXD.esc(e.name) + '</a>'
        : '<span>' + icon + ' ' + LXD.esc(e.name) + '</span>';
      const ops = [];
      if (!e.is_dir) ops.push('<button class="btn btn-ghost btn-xs" data-dl="' + LXD.esc(e.path) + '">下载</button>');
      ops.push('<button class="btn btn-ghost btn-xs" data-rename="' + LXD.esc(e.path) + '" data-name="' + LXD.esc(e.name) + '">重命名</button>');
      ops.push('<button class="btn btn-danger-ghost btn-xs" data-del="' + LXD.esc(e.path) + '" data-name="' + LXD.esc(e.name) + '">删除</button>');
      h += '<tr>' +
        '<td>' + nameCell + '</td>' +
        '<td class="mono">' + (e.is_dir ? '-' : fmtSize(e.size)) + '</td>' +
        '<td class="mono" style="white-space:nowrap">' + LXD.esc(e.mtime || '') + '</td>' +
        '<td>' + (e.is_dir ? '目录' : (e.is_symlink ? '链接 → ' + LXD.esc(e.link || '') : '文件')) + '</td>' +
        '<td><div style="display:flex;gap:6px;flex-wrap:wrap">' + ops.join('') + '</div></td>' +
        '</tr>';
    });
    h += '</tbody></table></div>';
    area.innerHTML = h;

    area.querySelectorAll('.fm-open').forEach((a) => {
      a.addEventListener('click', () => { curPath = a.dataset.path; load(); });
    });
    area.querySelectorAll('[data-dl]').forEach((b) => {
      b.addEventListener('click', () => {
        const a = document.createElement('a');
        a.href = api + '/download?path=' + encodeURIComponent(b.dataset.dl);
        a.download = '';
        document.body.appendChild(a); a.click(); a.remove();
      });
    });
    area.querySelectorAll('[data-rename]').forEach((b) => {
      b.addEventListener('click', () => {
        const oldName = b.dataset.name;
        const newName = window.prompt('重命名：' + oldName, oldName);
        if (!newName || newName === oldName) return;
        const idx = b.dataset.rename.lastIndexOf('/');
        const dst = (idx >= 0 ? b.dataset.rename.slice(0, idx + 1) : '') + newName;
        LXD.authRequest(api + '/rename', { method: 'POST', body: { src: b.dataset.rename, dst } })
          .then(() => { LXD.toast('success', '重命名成功'); load(); })
          .catch((e) => { if (!LXD.handleAuthError(e)) LXD.toast('error', '重命名失败：' + (e.message || '')); });
      });
    });
    area.querySelectorAll('[data-del]').forEach((b) => {
      b.addEventListener('click', () => {
        if (!window.confirm('确定删除「' + b.dataset.name + '」吗？目录将递归删除，操作不可恢复。')) return;
        LXD.authRequest(api + '/delete', { method: 'POST', body: { path: b.dataset.del } })
          .then(() => { LXD.toast('success', '删除成功'); load(); })
          .catch((e) => { if (!LXD.handleAuthError(e)) LXD.toast('error', '删除失败：' + (e.message || '')); });
      });
    });
  }

  function load() {
    const area = document.getElementById('fmArea');
    area.innerHTML = '<div class="empty">加载中...</div>';
    renderPathBar();
    LXD.authRequest(api + '?path=' + encodeURIComponent(curPath)).then((res) => {
      const d = res.data || {};
      entries = Array.isArray(d.entries) ? d.entries : [];
      render();
    }).catch((e) => {
      if (!LXD.handleAuthError(e)) area.innerHTML = empty('加载失败：' + (e.message || ''));
      entries = [];
    });
  }

  // 新建目录
  document.getElementById('fmMkdir').addEventListener('click', () => {
    const name = window.prompt('新建目录名称：', 'newdir');
    if (!name) return;
    const p = (curPath === '/' ? '' : curPath) + '/' + name.replace(/^\/+/, '');
    LXD.authRequest(api + '/mkdir', { method: 'POST', body: { path: p } })
      .then(() => { LXD.toast('success', '目录已创建'); load(); })
      .catch((e) => { if (!LXD.handleAuthError(e)) LXD.toast('error', '创建失败：' + (e.message || '')); });
  });

  // 上传（multipart 手动带 X-Container-Hash）
  document.getElementById('fmUpload').addEventListener('change', (ev) => {
    const files = Array.from(ev.target.files || []);
    if (!files.length) return;
    const base = curPath === '/' ? '' : curPath;
    const proms = files.map((f) => {
      const fd1 = new FormData();
      fd1.append('file', f);
      fd1.append('path', base + '/' + f.name);
      return fetch(api + '/upload', {
        method: 'POST',
        headers: { 'X-Container-Hash': LXD.getHash() },
        body: fd1
      }).then((r) => r.json().catch(() => ({})))
        .then((j) => {
          if (j.code !== 200) throw new Error(j.msg || '上传失败');
          return f.name;
        });
    });
    LXD.toast('info', '正在上传 ' + files.length + ' 个文件...');
    Promise.all(proms).then((names) => {
      LXD.toast('success', '上传成功：' + names.join(', '));
      load();
    }).catch((e) => {
      if (!LXD.handleAuthError(e)) LXD.toast('error', '上传失败：' + (e.message || ''));
      load();
    });
    ev.target.value = '';
  });

  load();
})();
