// AI生成
/* ============================================================
   LXD Panel - User 在线文件管理（类 WebSFTP，增强版）
   依赖：user.js 的 USER.shell / USER.request
   功能：浏览 / 上传 / 下载(原文件) / 打包下载 / 解压 / 权限(含属主) / 重命名 / 删除
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

  USER.shell('containers', '在线文件管理', '容器：' + cname + ' · 浏览 / 上传 / 下载 / 打包 / 解压 / 权限 / 重命名 / 删除');

  const content = document.getElementById('userContent');
  content.innerHTML =
    '<div class="card" style="padding:12px 14px;margin-bottom:16px">' +
    '<div class="toolbar" style="gap:8px;flex-wrap:wrap">' +
    '<button class="btn btn-ghost btn-sm" id="fmBack" title="返回容器列表">← 我的容器</button>' +
    '<button class="btn btn-ghost btn-sm" id="fmRefresh">↻ 刷新</button>' +
    '<button class="btn btn-ghost btn-sm" id="fmMkdir">＋ 新建目录</button>' +
    '<button class="btn btn-ghost btn-sm" id="fmNewFile">＋ 新建文件</button>' +
    '<label class="btn btn-primary btn-sm" style="cursor:pointer" id="fmUploadLabel">⇧ 上传文件<input type="file" id="fmUpload" multiple style="display:none"></label>' +
    '<button class="btn btn-ghost btn-sm" id="fmPack" title="将当前目录打包为 tar.gz 下载">⬇ 打包当前目录</button>' +
    '<span class="grow"></span>' +
    '<span id="fmPathLabel" class="mono" style="font-size:13px;color:var(--text-2);word-break:break-all">/</span>' +
    '</div></div>' +
    '<div class="card" style="padding:14px" id="fmArea"></div>';

  document.getElementById('fmBack').addEventListener('click', function () { window.location.href = 'containers.html'; });
  document.getElementById('fmRefresh').addEventListener('click', load);
  document.getElementById('fmPack').addEventListener('click', function () { packCurrent(); });

  function esc(s) { return LXD.esc(s == null ? '' : String(s)); }
  function toastErr(m) { LXD.toast('error', m); }
  function post(url, body) { return USER.request(url, { method: 'POST', body: body }); }

  function renderPathBar() {
    const bar = document.getElementById('fmPathLabel');
    if (!bar) return;
    const parts = curPath.split('/').filter(Boolean);
    let html = '<a href="javascript:;" data-path="/" style="color:var(--accent);text-decoration:none">/</a>';
    let acc = '';
    parts.forEach(function (p) {
      acc += '/' + p;
      html += ' <span style="color:var(--text-3)">/</span> <a href="javascript:;" data-path="' + esc(acc) + '" style="color:var(--accent);text-decoration:none">' + esc(p) + '</a>';
    });
    bar.innerHTML = html;
    bar.querySelectorAll('a[data-path]').forEach(function (a) {
      a.addEventListener('click', function () { curPath = a.dataset.path; load(); });
    });
  }

  function fmtSize(n) {
    if (n == null || isNaN(n) || n < 0) return '-';
    if (n < 1024) return n + ' B';
    const units = ['KB', 'MB', 'GB', 'TB'];
    let v = n;
    for (const u of units) { v /= 1024; if (v < 1024) return v.toFixed(1) + ' ' + u; }
    return v.toFixed(1) + ' PB';
  }

  function modeToOctal(m) {
    if (!m || m.length < 10) return '644';
    const s = m.slice(1);
    const map = { r: 4, w: 2, x: 1, s: 1, t: 1 };
    let out = '';
    for (let i = 0; i < 3; i++) {
      let v = 0;
      for (let j = 0; j < 3; j++) { if (map[s[i * 3 + j]]) v += map[s[i * 3 + j]]; }
      out += v;
    }
    return out;
  }

  function isArchive(name) {
    return /\.(zip|tar|tar\.gz|tgz|tar\.bz2|tbz2|tar\.xz)$/i.test(name || '');
  }

  function saveBlob(blob, name) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = name || 'download';
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(function () { URL.revokeObjectURL(url); }, 5000);
  }

  function downloadPath(p, isDir) {
    const url = isDir ? (api + '/archive?path=' + encodeURIComponent(p))
                      : (api + '/download?path=' + encodeURIComponent(p));
    fetch(url, { credentials: 'same-origin' }).then(function (r) {
      if (!r.ok) {
        return r.text().then(function (t) {
          let m = t;
          try { const j = JSON.parse(t); m = j.msg || j.message || t; } catch (e) {}
          throw new Error(m || ('HTTP ' + r.status));
        });
      }
      let name = p.split('/').pop() + (isDir ? '.tar.gz' : '');
      const cd = r.headers.get('Content-Disposition') || '';
      const m1 = /filename\*=UTF-8''([^;]+)/i.exec(cd);
      const m2 = /filename="?([^";]+)"?/i.exec(cd);
      if (m1) { try { name = decodeURIComponent(m1[1]); } catch (e) {} }
      else if (m2) { name = m2[1]; }
      return r.blob().then(function (b) { saveBlob(b, name); });
    }).catch(function (e) { toastErr('下载失败：' + (e.message || '')); });
  }

  function packCurrent() {
    if (curPath === '/') { toastErr('根目录不支持整体打包，请进入子目录或对单个条目打包'); return; }
    downloadPath(curPath, true);
  }

  function extractArchive(p, nm) {
    const idx = p.lastIndexOf('/');
    const dest = idx > 0 ? p.slice(0, idx) : '/';
    if (!window.confirm('将「' + nm + '」解压到 ' + dest + ' ？')) return;
    post(api + '/extract', { path: p, dest: dest })
      .then(function () { LXD.toast('success', '解压完成'); load(); })
      .catch(function (e) { toastErr('解压失败：' + (e.message || '')); });
  }

  function applyPerms(p, mode, owner, group) {
    const cur = modeToOctal(mode);
    const nm = window.prompt('权限（3-4 位八进制，如 755 / 0644）：', cur);
    if (nm === null) return;
    const no = window.prompt('属主:属组（留空不修改，例如 root:root）：', owner + (group ? ':' + group : ''));
    if (no === null) return;
    const nms = (nm || '').trim();
    const nos = (no || '').trim();
    const curOwner = owner + (group ? ':' + group : '');
    const tasks = [];
    if (nms && nms !== cur) tasks.push(function () { return post(api + '/chmod', { path: p, mode: nms }); });
    if (nos && nos !== curOwner) {
      const parts = nos.split(':');
      tasks.push(function () { return post(api + '/chown', { path: p, owner: parts[0] || '', group: (parts.length > 1 ? parts[1] : '') }); });
    }
    if (!tasks.length) return;
    let chain = Promise.resolve();
    tasks.forEach(function (t) { chain = chain.then(t); });
    chain.then(function () { LXD.toast('success', '属性已更新'); load(); })
      .catch(function (e) { toastErr('属性更新失败：' + (e.message || e)); });
  }


  function isEditable(name, size) {
    if (size != null && size > 2 * 1024 * 1024) return false;
    const n = (name || '').toLowerCase();
    if (/\.(txt|text|md|markdown|json|json5|yaml|yml|toml|ini|conf|cfg|cnf|log|sh|bash|zsh|py|js|mjs|cjs|ts|tsx|jsx|css|scss|less|html|htm|xml|svg|csv|tsv|env|go|java|c|h|cpp|hpp|cs|php|rb|rs|sql|vue|svelte|properties|service|list|rules|lst|desktop|repo|lock|flag|tmpl|tpl|nfo|srt|vtt|pem|pub|key|pubxml|editorconfig|gradle|mod|sum|babelrc|eslintrc|prettierrc|htaccess)$/i.test(n)) return true;
    if (/^(dockerfile|makefile|readme|license|changelog|\.gitignore|\.gitattributes|\.env|\.bashrc|\.bash_profile|\.profile|\.vimrc|hostname|hosts|fstab|crontab|inittab|resolv\.conf|sysctl\.conf|nsswitch\.conf|passwd|group|shadow|ssh_config|sshd_config|authorized_keys)$/i.test(n)) return true;
    return false;
  }

  function newFile() {
    const name = window.prompt('\u65b0\u5efa\u6587\u4ef6\u540d\u79f0\uff08\u542b\u6269\u5c55\u540d\uff0c\u5982 note.txt / config.yaml\uff09\uff1a', 'untitled.txt');
    if (!name) return;
    const clean = name.replace(/^\/+/, '');
    if (!clean || clean.indexOf('/') >= 0) { toastErr('\u6587\u4ef6\u540d\u4e0d\u80fd\u5305\u542b /'); return; }
    const p = (curPath === '/' ? '' : curPath) + '/' + clean;
    post(api + '/write', { path: p, content: '' })
      .then(function () {
        LXD.toast('success', '\u6587\u4ef6\u5df2\u521b\u5efa');
        load();
        openEditor(p, clean);
      })
      .catch(function (e) { toastErr('\u521b\u5efa\u5931\u8d25\uff1a' + (e.message || '')); });
  }

  function openEditor(p, name) {
    const url = api + '/download?path=' + encodeURIComponent(p);
    fetch(url, { credentials: 'same-origin' }).then(function (r) {
      if (!r.ok) {
        return r.text().then(function (t) {
          let m = t;
          try { const j = JSON.parse(t); m = j.msg || j.message || t; } catch (e) {}
          throw new Error(m || ('HTTP ' + r.status));
        });
      }
      return r.text();
    }).then(function (text) {
      showEditorModal(p, name, text);
    }).catch(function (e) { toastErr('\u8bfb\u53d6\u5931\u8d25\uff1a' + (e.message || '')); });
  }

  function showEditorModal(p, name, text) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:960px;width:92vw">' +
      '<h3 style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">\u7f16\u8f91\u6587\u4ef6 <span class="mono" style="font-size:12px;color:var(--text-3);font-weight:400;word-break:break-all">' + esc(p) + '</span></h3>' +
      '<textarea id="fmEditor" spellcheck="false" style="width:100%;height:55vh;box-sizing:border-box;font-family:Consolas,Monaco,monospace;font-size:13px;line-height:1.5;background:var(--bg-card);color:var(--text-1);border:1px solid var(--border);border-radius:8px;padding:10px 12px;resize:vertical;white-space:pre;overflow:auto"></textarea>' +
      '<div style="display:flex;justify-content:space-between;align-items:center;margin-top:8px;font-size:12px;color:var(--text-3)"><span id="fmEditorStat"></span><span>Ctrl+S \u4fdd\u5b58</span></div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">\u53d6\u6d88</button><button class="btn btn-primary" data-act="save">\u4fdd\u5b58</button></div>' +
      '</div>';
    document.body.appendChild(mask);
    const ta = mask.querySelector('#fmEditor');
    const stat = mask.querySelector('#fmEditorStat');
    ta.value = text || '';
    const upd = function () { stat.textContent = ta.value.length + ' \u5b57\u7b26'; };
    ta.addEventListener('input', upd); upd();
    const close = function () { mask.remove(); };
    mask.querySelector('[data-act="cancel"]').onclick = close;
    mask.addEventListener('click', function (e) { if (e.target === mask) close(); });
    const save = function () {
      post(api + '/write', { path: p, content: ta.value })
        .then(function () { LXD.toast('success', '\u5df2\u4fdd\u5b58'); close(); load(); })
        .catch(function (e) { toastErr('\u4fdd\u5b58\u5931\u8d25\uff1a' + (e.message || '')); });
    };
    mask.querySelector('[data-act="save"]').onclick = save;
    ta.addEventListener('keydown', function (e) {
      if ((e.ctrlKey || e.metaKey) && (e.key === 's' || e.key === 'S')) { e.preventDefault(); save(); }
    });
    ta.focus();
  }

  function render() {
    const area = document.getElementById('fmArea');
    if (!entries.length) { area.innerHTML = USER.empty('目录为空'); return; }
    let h = '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
      '<th>名称</th><th>大小</th><th>权限</th><th>修改时间</th><th>类型</th><th>操作</th>' +
      '</tr></thead><tbody>';
    entries.forEach(function (e) {
      const icon = e.is_dir ? '📁' : (e.is_symlink ? '🔗' : '📄');
      const nameCell = e.is_dir
        ? '<a href="javascript:;" class="fm-open" data-path="' + esc(e.path) + '" style="color:var(--accent);text-decoration:none">' + icon + ' ' + esc(e.name) + '</a>'
        : '<span>' + icon + ' ' + esc(e.name) + '</span>';
      const ops = [];
      if (e.is_dir) {
        ops.push('<button class="btn btn-ghost btn-xs" data-pack="' + esc(e.path) + '">打包下载</button>');
      } else {
        ops.push('<button class="btn btn-ghost btn-xs" data-dl="' + esc(e.path) + '">下载</button>');
        if (isEditable(e.name, e.size)) ops.push('<button class="btn btn-ghost btn-xs" data-edit="' + esc(e.path) + '" data-name="' + esc(e.name) + '">编辑</button>');
        if (isArchive(e.name)) ops.push('<button class="btn btn-ghost btn-xs" data-extract="' + esc(e.path) + '" data-name="' + esc(e.name) + '">解压</button>');
      }
      ops.push('<button class="btn btn-ghost btn-xs" data-perm="' + esc(e.path) + '" data-mode="' + esc(e.mode) + '" data-owner="' + esc(e.owner) + '" data-group="' + esc(e.group) + '">权限</button>');
      ops.push('<button class="btn btn-ghost btn-xs" data-rename="' + esc(e.path) + '" data-name="' + esc(e.name) + '">重命名</button>');
      ops.push('<button class="btn btn-danger-ghost btn-xs" data-del="' + esc(e.path) + '" data-name="' + esc(e.name) + '">删除</button>');
      const permCell = '<div class="mono" style="font-size:12px">' + esc(e.mode || '') + '</div>' +
        '<div style="font-size:11px;color:var(--text-3)">' + esc((e.owner || '') + ':' + (e.group || '')) + '</div>';
      h += '<tr>' +
        '<td>' + nameCell + '</td>' +
        '<td class="mono">' + (e.is_dir ? '-' : fmtSize(e.size)) + '</td>' +
        '<td>' + permCell + '</td>' +
        '<td class="mono" style="white-space:nowrap">' + esc(e.mtime || '') + '</td>' +
        '<td>' + (e.is_dir ? '目录' : (e.is_symlink ? '链接 → ' + esc(e.link || '') : '文件')) + '</td>' +
        '<td><div style="display:flex;gap:6px;flex-wrap:wrap">' + ops.join('') + '</div></td>' +
        '</tr>';
    });
    h += '</tbody></table></div>';
    area.innerHTML = h;

    area.querySelectorAll('.fm-open').forEach(function (a) {
      a.addEventListener('click', function () { curPath = a.dataset.path; load(); });
    });
    area.querySelectorAll('[data-dl]').forEach(function (b) {
      b.addEventListener('click', function () { downloadPath(b.dataset.dl, false); });
    });
    area.querySelectorAll('[data-pack]').forEach(function (b) {
      b.addEventListener('click', function () { downloadPath(b.dataset.pack, true); });
    });
    area.querySelectorAll('[data-extract]').forEach(function (b) {
      b.addEventListener('click', function () { extractArchive(b.dataset.extract, b.dataset.name); });
    });
    area.querySelectorAll('[data-edit]').forEach(function (b) {
      b.addEventListener('click', function () { openEditor(b.dataset.edit, b.dataset.name); });
    });
    area.querySelectorAll('[data-perm]').forEach(function (b) {
      b.addEventListener('click', function () { applyPerms(b.dataset.perm, b.dataset.mode, b.dataset.owner, b.dataset.group); });
    });
    area.querySelectorAll('[data-rename]').forEach(function (b) {
      b.addEventListener('click', function () {
        const oldName = b.dataset.name;
        const newName = window.prompt('重命名：' + oldName, oldName);
        if (!newName || newName === oldName) return;
        const idx = b.dataset.rename.lastIndexOf('/');
        const dst = (idx >= 0 ? b.dataset.rename.slice(0, idx + 1) : '') + newName;
        post(api + '/rename', { src: b.dataset.rename, dst: dst })
          .then(function () { LXD.toast('success', '重命名成功'); load(); })
          .catch(function (e) { toastErr('重命名失败：' + (e.message || '')); });
      });
    });
    area.querySelectorAll('[data-del]').forEach(function (b) {
      b.addEventListener('click', function () {
        if (!window.confirm('确定删除「' + b.dataset.name + '」吗？目录将递归删除，操作不可恢复。')) return;
        post(api + '/delete', { path: b.dataset.del })
          .then(function () { LXD.toast('success', '删除成功'); load(); })
          .catch(function (e) { toastErr('删除失败：' + (e.message || '')); });
      });
    });
  }

  function load() {
    const area = document.getElementById('fmArea');
    area.innerHTML = '<div class="empty">加载中...</div>';
    renderPathBar();
    USER.request(api + '?path=' + encodeURIComponent(curPath)).then(function (res) {
      const d = res.data || {};
      entries = Array.isArray(d.entries) ? d.entries : [];
      render();
    }).catch(function (e) {
      area.innerHTML = USER.empty('加载失败：' + (e.message || ''));
      entries = [];
    });
  }

  document.getElementById('fmMkdir').addEventListener('click', function () {
    const name = window.prompt('新建目录名称：', 'newdir');
    if (!name) return;
    const p = (curPath === '/' ? '' : curPath) + '/' + name.replace(/^\/+/, '');
    post(api + '/mkdir', { path: p })
      .then(function () { LXD.toast('success', '目录已创建'); load(); })
      .catch(function (e) { toastErr('创建失败：' + (e.message || '')); });
  });

  document.getElementById('fmUpload').addEventListener('change', function (ev) {
    const files = Array.from(ev.target.files || []);
    if (!files.length) return;
    const base = curPath === '/' ? '' : curPath;
    const proms = files.map(function (f) {
      const fd = new FormData();
      fd.append('file', f);
      fd.append('path', base + '/' + f.name);
      return fetch(api + '/upload', { method: 'POST', body: fd, credentials: 'same-origin' })
        .then(function (r) { return r.json().catch(function () { return {}; }); })
        .then(function (j) { if (j.code !== 200) throw new Error(j.msg || '上传失败'); return f.name; });
    });
    LXD.toast('info', '正在上传 ' + files.length + ' 个文件...');
    Promise.all(proms).then(function (names) {
      LXD.toast('success', '上传成功：' + names.join(', '));
      load();
    }).catch(function (e) {
      toastErr('上传失败：' + (e.message || ''));
      load();
    });
    ev.target.value = '';
  });

  document.getElementById('fmNewFile').addEventListener('click', newFile);

  load();
})();