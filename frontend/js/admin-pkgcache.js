/* ============================================================
   LXD Panel - Admin 包缓存（v1.1.22）
   节点本地常用包共享仓库：容器无法联网时，开通/重装/改密后用本地缓存离线安装 openssh(sftp)/wget/curl
   - 查看各发行版缓存状态 / 删除
   - 选择预装包列表、在线安装超时
   - 触发同步：节点用所选镜像起临时容器，下载完整依赖闭包到 <缓存目录>/<发行版>/<版本>/<架构>/
   请求经 ADMIN.request 自动附带当前节点 node_id
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('pkgcache', '包缓存', '节点离线软件包缓存（SSH/SFTP 等）');
  const c = document.getElementById('adminContent');
  const esc = LXD.esc;
  let st = null;
  let pollTimer = null;

  function render() {
    const s = st.settings || {};
    const pk = s.packages || ['openssh', 'wget', 'curl'];
    const sugg = Array.from(new Set((st.suggestions || []).concat(pk)));
    const auto = s.auto_install === undefined || s.auto_install === null ? true : !!s.auto_install;
    const running = st.running;
    const entries = st.entries || [];
    const imgs = st.images || [];
    c.innerHTML =
      (!st.lxd_available ? '<div class="card" style="padding:12px 14px;margin-bottom:16px;color:var(--warning,#f5a524)">本机未连接 LXD：包缓存只在有 LXD 的节点上生效，请在右上角切换到节点。</div>' : '') +
      (st.disabled ? '<div class="card" style="padding:12px 14px;margin-bottom:16px;color:var(--warning,#f5a524)">配置文件 pkgcache.disabled=true：开通后不会自动安装/配置 sshd。</div>' : '') +
      '<div class="card" style="padding:14px;margin-bottom:16px">' +
      '<div class="card-title">预装设置</div>' +
      '<div style="font-size:12.5px;color:var(--text-3);margin-bottom:10px">开通、重装、修改 root 密码后，后台先在线安装下列软件包（超时或失败则用本地缓存离线安装），再配置 sshd：允许 root 密码登录、生成主机密钥、开机自启。openssh（含 sftp-server）始终安装。失败只记日志，不影响开通。</div>' +
      '<div style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:10px" id="pkChips">' +
      sugg.map(p => '<button class="chip' + (pk.indexOf(p) >= 0 ? ' active' : '') + '" data-pk="' + esc(p) + '"' + (p === 'openssh' ? ' disabled title="必选"' : '') + '>' + esc(p) + '</button>').join('') +
      '</div>' +
      '<div class="form-grid">' +
      '<div class="field"><label>其它包（空格分隔，按包管理器的包名）</label><input class="input" id="pkExtra" placeholder="例如 sudo nano"></div>' +
      '<div class="field"><label>在线安装超时（秒，0=默认 ' + (st.default_online_timeout || 90) + '）</label><input class="input" id="pkTimeout" type="number" min="0" max="1800" value="' + (s.online_timeout || 0) + '"></div>' +
      '</div>' +
      '<label style="display:flex;gap:8px;align-items:center;font-size:13px;margin:10px 0"><input type="checkbox" id="pkAuto"' + (auto ? ' checked' : '') + '> 开通/重装/改密后自动安装并配置 sshd</label>' +
      '<button class="btn btn-primary" id="btnSaveSet">保存设置</button>' +
      '</div>' +

      '<div class="card" style="padding:14px;margin-bottom:16px">' +
      '<div class="card-title">缓存状态</div>' +
      '<div style="font-size:12.5px;color:var(--text-3);margin-bottom:8px">缓存目录：<span class="mono">' + esc(st.abs_dir || st.dir) + '</span>（配置项 pkgcache.dir）</div>' +
      (entries.length ? '<div class="tbl-wrap"><table class="tbl"><thead><tr><th>发行版/版本/架构</th><th>包管理器</th><th>包</th><th>文件数</th><th>大小</th><th>同步时间</th><th>来源镜像</th><th></th></tr></thead><tbody>' +
        entries.map(e => '<tr><td class="mono">' + esc(e.key) + '</td><td>' + esc(e.pm || '-') + '</td><td>' + esc((e.packages || []).join(' ')) + '</td>' +
          '<td class="mono">' + (e.files || 0) + '</td><td class="mono">' + esc(e.size ? ADMIN.fmt.bytes(e.size) : '-') + '</td>' +
          '<td class="mono">' + esc(e.synced_at || '-') + '</td><td class="mono" title="' + esc(e.image || '') + '">' + esc((e.image || '-').slice(0, 12)) + '</td>' +
          '<td><button class="btn btn-sm btn-danger-ghost" data-delkey="' + esc(e.key) + '">删除</button></td></tr>').join('') +
        '</tbody></table></div>' : ADMIN.empty('暂无缓存，在下方选择镜像同步')) +
      '</div>' +

      '<div class="card" style="padding:14px">' +
      '<div class="card-title">同步（下载）</div>' +
      '<div style="font-size:12.5px;color:var(--text-3);margin-bottom:8px">节点用所选镜像起一个临时容器（需节点能联网），在容器内用该发行版的包管理器下载上述包及完整依赖（apk fetch --recursive / apt-get download / dnf download --resolve），再拉回节点缓存目录，完成后自动删除临时容器。完整支持 Alpine、Debian/Ubuntu，RHEL 系/openSUSE/Arch 尽力而为。同一系统版本架构只需同步一次。</div>' +
      (running ? '<div style="margin-bottom:10px"><div style="font-size:13px">同步中：' + esc(running.message || running.label || '') + '</div><div class="progress" style="margin-top:6px"><i style="width:' + Math.max(3, running.progress || 0) + '%"></i></div></div>' : '') +
      (imgs.length ? '<div class="tbl-wrap" style="max-height:300px;overflow:auto"><table class="tbl"><thead><tr><th style="width:32px"><input type="checkbox" id="imgAll"></th><th>别名</th><th>系统</th><th>架构</th><th>指纹</th></tr></thead><tbody>' +
        imgs.map(im => '<tr><td><input type="checkbox" class="img-ck" value="' + esc(im.fingerprint) + '"></td><td class="mono">' + esc((im.aliases || []).join(', ') || '-') + '</td>' +
          '<td>' + esc([im.os, im.release, im.variant].filter(Boolean).join(' ') || '-') + '</td><td class="mono">' + esc(im.architecture || '-') + '</td>' +
          '<td class="mono">' + esc((im.fingerprint || '').slice(0, 12)) + '</td></tr>').join('') +
        '</tbody></table></div>' : ADMIN.empty(st.lxd_available ? '节点没有本地镜像，请先在「镜像管理」下载' : '无法读取镜像')) +
      '<div class="toolbar" style="margin-top:10px">' +
      '<button class="btn btn-primary" id="btnSyncSel"' + (running || !imgs.length ? ' disabled' : '') + '>同步所选镜像</button>' +
      '<button class="btn" id="btnSyncAll"' + (running || !imgs.length ? ' disabled' : '') + '>同步全部（按系统去重）</button>' +
      '<span class="grow"></span><button class="btn btn-ghost" id="btnReload">↻ 刷新</button></div>' +
      ((st.jobs || []).length ? '<div style="margin-top:12px">' + st.jobs.slice(0, 5).map(j =>
        '<div style="font-size:12.5px;padding:6px 0;border-top:1px solid var(--border)">' +
        (j.status === 'success' ? '<span class="badge badge-running">完成</span>' : j.status === 'failed' ? '<span class="badge badge-stopped">失败</span>' : '<span class="badge badge-other">进行中</span>') +
        ' <span class="mono">' + esc(LXD.fmtTime ? LXD.fmtTime(j.created_at) : j.created_at) + '</span>' + (j.task_id ? ' · 任务 #' + j.task_id : '') +
        '<div style="color:' + (j.error ? 'var(--danger,#e5484d)' : 'var(--text-3)') + ';margin-top:3px">' + esc(j.error || j.message || '') + '</div></div>').join('') + '</div>' : '') +
      '</div>';
    bind();
    clearTimeout(pollTimer);
    if (running) pollTimer = setTimeout(load, 3000);
  }

  function bind() {
    c.querySelectorAll('[data-pk]').forEach(b => b.addEventListener('click', () => { if (!b.disabled) b.classList.toggle('active'); }));
    document.getElementById('btnSaveSet').addEventListener('click', () => {
      const pk = Array.from(c.querySelectorAll('[data-pk].active')).map(b => b.dataset.pk)
        .concat(document.getElementById('pkExtra').value.split(/[\s,]+/).map(s => s.trim().toLowerCase()).filter(Boolean));
      ADMIN.request('/api/admin/pkgcache/settings', { method: 'PUT', body: {
        packages: pk, online_timeout: parseInt(document.getElementById('pkTimeout').value || '0', 10) || 0,
        auto_install: document.getElementById('pkAuto').checked } })
        .then(() => { LXD.toast('success', '设置已保存（新包需重新同步缓存）'); load(); })
        .catch(e => LXD.toast('error', e.message));
    });
    c.querySelectorAll('[data-delkey]').forEach(b => b.addEventListener('click', () => {
      LXD.confirmDialog('删除缓存', '删除 ' + b.dataset.delkey + ' 的本地包缓存？').then(ok => {
        if (!ok) return;
        ADMIN.request('/api/admin/pkgcache/entry?key=' + encodeURIComponent(b.dataset.delkey), { method: 'DELETE' })
          .then(() => { LXD.toast('success', '已删除'); load(); }).catch(e => LXD.toast('error', e.message));
      });
    }));
    const all = document.getElementById('imgAll');
    if (all) all.addEventListener('change', () => c.querySelectorAll('.img-ck').forEach(x => { x.checked = all.checked; }));
    const sync = (body) => ADMIN.request('/api/admin/pkgcache/sync', { method: 'POST', body: body })
      .then(() => { LXD.toast('success', '同步任务已开始'); load(); }).catch(e => LXD.toast('error', e.message));
    const bs = document.getElementById('btnSyncSel');
    if (bs) bs.addEventListener('click', () => {
      const sel = Array.from(c.querySelectorAll('.img-ck:checked')).map(x => x.value);
      if (!sel.length) { LXD.toast('error', '请先勾选镜像'); return; }
      sync({ images: sel });
    });
    const ba = document.getElementById('btnSyncAll');
    if (ba) ba.addEventListener('click', () => sync({ all: true }));
    document.getElementById('btnReload').addEventListener('click', load);
  }

  function load() {
    ADMIN.request('/api/admin/pkgcache').then(res => { st = res.data || {}; render(); })
      .catch(e => { c.innerHTML = '<div class="empty">加载失败：' + esc(e.message) + '</div>'; });
  }
  c.innerHTML = '<div class="loading">加载中...</div>';
  load();
})();
