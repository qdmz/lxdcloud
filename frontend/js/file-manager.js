/* ============================================================
   LXD Panel - User 在线文件管理（类 WebSFTP）
   依赖：user.js 的 USER.shell / USER.request
   ============================================================ */
(function () {
  'use strict';

  const params = new URLSearchParams(window.location.search);
  const cname = params.get('name') || '';
  const api = '/api/user/containers/' + encodeURIComponent(cname) + '/files';

  let curPath = '/';
  let entries = [];

  if (!cname) {
    USER.shell('containers', '在线文件管理', '缺少容器名称参数');
    document.getElementById('userContent').innerHTML = '<div class="card" style="padding:14px"><div class="empty">缺少容器名称参数，请从「我的容器」详情进入。</div></div>';
    return;
  }

  USER.shell('containers', '在线文件管理', '容器：' + cname + ' · 目录浏览 / 上传 / 下载 / 新建 / 重命名 / 删除');

  const content = document.getElementById('userContent');
  content.innerHTML =
    '<div class="card" style="padding:12px 14px;margin-bottom:16px">' +
    '<div class="toolbar" style="gap:8px;flex-wrap:wrap">' +
    '<button class="btn btn-ghost btn-sm" id="fmBack" title="返回容器列表">← 我的容器</button>' +
    '<button class="btn btn-ghost btn-sm" id="fmRefresh">↻ 刷新</button>' +
    '<button class="btn btn-ghost btn-sm" id="fmMkdir">＋ 新建目录</button>' +
    '<label class="btn btn-primary btn-sm" style="cursor:pointer" id="fmUploadLabel">⇧ 上传文件<input type="file" id="fmUpload" multiple style="display:none"></label>' +
    '<span class="grow"></span>' +
    '<span id="fmPathLabel" class="mono" style="font-size:13px;color:var(--text-2);word-break:break-all">/</span>' +
    '</div></div>' +
    '<div class="card" style="padding:14px" id="fmArea"></div>';

  document.getElementById('fmBack').addEventListener('click', () => { window.location.href = 'containers.html'; });
  document.getElementById('fmRefresh').addEventListener('click', load);

  // 面包屑导航（点击父路径可跳转）
  function renderPathBar() {
    const bar = document.getElementById('fmPathLabel');
    if (!bar) return;
    const parts = curPath.split('/').filter(Boolean);
    let html = '<a href="javascript:;" data-path="/" style="color:var(--accent);text-decoration:none">/</a>';
    let acc = '';
    parts.forEach((p, i) => {
      acc += '/' + p;
      html += ' <span style="color:var(--text-3)">/</span> <a href="javascript:;" data-path="' + USER.esc(acc) + '" style="color:var(--accent);text-decoration:none">' + USER.esc(p) + '</a>';
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

  function esc(s) { return LXD.esc(s); }

  function render() {
    const area = document.getElementById('fmArea');
    if (!entries.length) {
      area.innerHTML = USER.empty('目录为空');
      return;
    }
    let h = '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
      '<th>名称</th><th>大小</th><th>修改时间</th><th>类型</th><th>操作</th>' +
      '</tr></thead><tbody>';
    entries.forEach((e) => {
      const icon = e.is_dir ? '📁' : (e.is_symlink ? '🔗' : '📄');
      const nameCell = e.is_dir
        ? '<a href="javascript:;" class="fm-open" data-path="' + esc(e.path) + '" style="color:var(--accent);text-decoration:none">' + icon + ' ' + esc(e.name) + '</a>'
        : '<span>' + icon + ' ' + esc(e.name) + '</span>';
      const ops = [];
      if (!e.is_dir) ops.push('<button class="btn btn-ghost btn-xs" data-dl="' + esc(e.path) + '">下载</button>');
      ops.push('<button class="btn btn-ghost btn-xs" data-rename="' + esc(e.path) + '" data-name="' + esc(e.name) + '">重命名</button>');
      ops.push('<button class="btn btn-danger-ghost btn-xs" data-del="' + esc(e.path) + '" data-name="' + esc(e.name) + '">删除</button>');
      h += '<tr>' +
        '<td>' + nameCell + '</td>' +
        '<td class="mono">' + (e.is_dir ? '-' : fmtSize(e.size)) + '</td>' +
        '<td class="mono" style="white-space:nowrap">' + esc(e.mtime || '') + '</td>' +
        '<td>' + (e.is_dir ? '目录' : (e.is_symlink ? '链接 → ' + esc(e.link || '') : '文件')) + '</td>' +
        '<td><div style="display:flex;gap:6px;flex-wrap:wrap">' + ops.join('') + '</div></td>' +
        '</tr>';
    });
    h += '</tbody></table></div>';
    area.innerHTML = h;

    // 目录跳转
    area.querySelectorAll('.fm-open').forEach((a) => {
      a.addEventListener('click', () => { curPath = a.dataset.path; load(); });
    });
    // 下载
    area.querySelectorAll('[data-dl]').forEach((b) => {
      b.addEventListener('click', () => {
        const a = document.createElement('a');
        a.href = api + '/download?path=' + encodeURIComponent(b.dataset.dl);
        a.download = '';
        document.body.appendChild(a); a.click(); a.remove();
      });
    });
    // 重命名
    area.querySelectorAll('[data-rename]').forEach((b) => {
      b.addEventListener('click', () => {
        const oldName = b.dataset.name;
        const newName = window.prompt('重命名：' + oldName, oldName);
        if (!newName || newName === oldName) return;
        const idx = b.dataset.rename.lastIndexOf('/');
        const dst = (idx >= 0 ? b.dataset.rename.slice(0, idx + 1) : '') + newName;
        USER.request(api + '/rename', { method: 'POST', body: { src: b.dataset.rename, dst } })
          .then(() => { LXD.toast('success', '重命名成功'); load(); })
          .catch((e) => LXD.toast('error', '重命名失败：' + (e.message || '')));
      });
    });
    // 删除
    area.querySelectorAll('[data-del]').forEach((b) => {
      b.addEventListener('click', () => {
        if (!window.confirm('确定删除「' + b.dataset.name + '」吗？目录将递归删除，操作不可恢复。')) return;
        USER.request(api + '/delete', { method: 'POST', body: { path: b.dataset.del } })
          .then(() => { LXD.toast('success', '删除成功'); load(); })
          .catch((e) => LXD.toast('error', '删除失败：' + (e.message || '')));
      });
    });
  }

  function load() {
    const area = document.getElementById('fmArea');
    area.innerHTML = '<div class="empty">加载中...</div>';
    renderPathBar();
    USER.request(api + '?path=' + encodeURIComponent(curPath)).then((res) => {
      const d = res.data || {};
      entries = Array.isArray(d.entries) ? d.entries : [];
      render();
    }).catch((e) => {
      area.innerHTML = USER.empty('加载失败：' + (e.message || ''));
      entries = [];
    });
  }

  // 新建目录
  document.getElementById('fmMkdir').addEventListener('click', () => {
    const name = window.prompt('新建目录名称：', 'newdir');
    if (!name) return;
    const p = (curPath === '/' ? '' : curPath) + '/' + name.replace(/^\/+/, '');
    USER.request(api + '/mkdir', { method: 'POST', body: { path: p } })
      .then(() => { LXD.toast('success', '目录已创建'); load(); })
      .catch((e) => LXD.toast('error', '创建失败：' + (e.message || '')));
  });

  // 上传（multipart 不走 USER.request，避免被强制 JSON 序列化）
  document.getElementById('fmUpload').addEventListener('change', (ev) => {
    const files = Array.from(ev.target.files || []);
    if (!files.length) return;
    const fd = new FormData();
    files.forEach((f) => fd.append('file', f));
    const base = curPath === '/' ? '' : curPath;
    const proms = files.map((f) => {
      const fd1 = new FormData();
      fd1.append('file', f);
      fd1.append('path', base + '/' + f.name);
      return fetch(api + '/upload', { method: 'POST', body: fd1 })
        .then((r) => r.json().catch(() => ({})))
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
      LXD.toast('error', '上传失败：' + (e.message || ''));
      load();
    });
    ev.target.value = '';
  });

  window.__fmGoto = (p) => { curPath = p; load(); };

  load();
})();
