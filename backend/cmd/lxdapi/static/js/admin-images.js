/* ============================================================
   LXD Panel - Admin 镜像管理（v1.1.22）
   - 节点本地镜像：列表 / 删除 / 编辑别名
   - 远程下载：内置/自定义 simplestreams 或 LXD 远程，浏览目录 → 选择下载到当前节点（异步作业，显示进度）
   - 从 URL 导入 tar 镜像
   所有请求经 ADMIN.request 自动附带当前节点 node_id（镜像源列表除外，存于主控）
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('images', '镜像管理', '节点本地镜像 · 远程镜像下载');
  const c = document.getElementById('adminContent');
  const esc = LXD.esc;
  const bytes = (n) => (n ? ADMIN.fmt.bytes(n) : '-');
  let images = [];
  let hostArch = '';
  let pollTimer = null;
  let lastRunning = 0;

  c.innerHTML =
    '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
    '<button class="btn btn-primary" id="btnRemote">⇣ 从远程下载</button>' +
    '<button class="btn" id="btnURL">从 URL 导入</button>' +
    '<span class="grow"></span>' +
    '<button class="btn btn-ghost" id="btnReload">↻ 刷新</button>' +
    '</div>' +
    '<div style="font-size:12.5px;color:var(--text-3);margin-top:8px">镜像是节点本地资源，操作作用于右上角所选节点。商品请绑定<b>别名</b>而不是指纹：LXD 自动更新或重新下载后指纹会变化，用同名别名覆盖即可让商品使用新镜像。</div>' +
    '</div>' +
    '<div class="card" id="jobsCard" style="padding:14px;margin-bottom:16px;display:none"><div class="card-title">下载 / 导入任务</div><div id="jobsBox"></div></div>' +
    '<div class="card" style="padding:14px"><div id="imgBox"><div class="loading">加载中...</div></div></div>';

  document.getElementById('btnReload').addEventListener('click', () => { load(); loadJobs(); });
  document.getElementById('btnRemote').addEventListener('click', openRemote);
  document.getElementById('btnURL').addEventListener('click', openURLImport);

  function aliasTags(list) {
    if (!list || !list.length) return '<span class="tag" style="background:rgba(229,72,77,.15);color:#e5484d" title="无别名：只能按指纹引用，指纹变化后商品会失效">无别名</span>';
    return list.map(a => '<span class="tag mono" style="margin:1px 2px">' + esc(a) + '</span>').join('');
  }

  function render() {
    const box = document.getElementById('imgBox');
    if (!images.length) { box.innerHTML = ADMIN.empty('该节点暂无本地镜像，点击「从远程下载」获取'); return; }
    const rows = images.map(im => {
      const fp = im.fingerprint || '';
      const sys = [im.os, im.release, im.variant && im.variant !== 'default' ? im.variant : ''].filter(Boolean).join(' ') || (im.description || '-');
      return '<tr>' +
        '<td>' + aliasTags(im.aliases) + '</td>' +
        '<td class="mono" title="' + esc(fp) + '">' + esc(fp.slice(0, 12)) + '</td>' +
        '<td>' + esc(sys) + (im.type === 'virtual-machine' ? ' <span class="tag tag-vm">VM</span>' : '') + '</td>' +
        '<td class="mono">' + esc(im.architecture || '-') + '</td>' +
        '<td class="mono">' + esc(bytes(im.size)) + '</td>' +
        '<td class="mono">' + esc(LXD.fmtTime ? LXD.fmtTime(im.created_at) : (im.created_at || '-')) + '</td>' +
        '<td class="mono">' + esc(LXD.fmtTime ? LXD.fmtTime(im.uploaded_at) : (im.uploaded_at || '-')) + '</td>' +
        '<td>' + (im.auto_update ? '<span class="badge badge-running">自动更新</span>' : '<span class="badge badge-other">手动</span>') + '</td>' +
        '<td class="row-actions">' +
        '<button class="btn btn-sm" data-alias="' + esc(fp) + '">别名</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + esc(fp) + '">删除</button>' +
        '</td></tr>';
    }).join('');
    box.innerHTML = '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
      '<th>别名</th><th>指纹</th><th>系统</th><th>架构</th><th>大小</th><th>创建时间</th><th>导入时间</th><th>更新</th><th style="width:120px">操作</th>' +
      '</tr></thead><tbody>' + rows + '</tbody></table></div>' +
      '<div style="display:flex;justify-content:flex-end;margin-top:12px;font-size:13px;color:var(--text-3)">共 ' + images.length + ' 个镜像 · 节点架构 ' + esc(hostArch || '-') + '</div>';
    box.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => delImage(b.dataset.del)));
    box.querySelectorAll('[data-alias]').forEach(b => b.addEventListener('click', () => editAliases(b.dataset.alias)));
  }

  function load() {
    ADMIN.request('/api/admin/images').then(res => {
      const d = res.data || {};
      images = d.images || []; hostArch = d.host_arch || '';
      render();
    }).catch(e => { document.getElementById('imgBox').innerHTML = '<div class="empty">加载失败：' + esc(e.message) + '</div>'; });
  }

  function delImage(fp) {
    const im = images.find(x => x.fingerprint === fp) || {};
    const name = (im.aliases && im.aliases[0]) || fp.slice(0, 12);
    LXD.confirmDialog('删除镜像', '确定删除镜像「' + name + '」？已创建的容器不受影响，但使用该镜像（别名）的商品将无法再开通。').then(ok => {
      if (!ok) return;
      ADMIN.request('/api/admin/images/' + encodeURIComponent(fp), { method: 'DELETE' })
        .then(() => { LXD.toast('success', '镜像已删除'); load(); })
        .catch(e => LXD.toast('error', e.message));
    });
  }

  function modal(html, width) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:' + (width || 480) + 'px;width:96%">' + html + '</div>';
    document.body.appendChild(mask);
    mask.addEventListener('click', e => { if (e.target === mask) mask.remove(); });
    mask.querySelectorAll('[data-act="cancel"]').forEach(b => b.onclick = () => mask.remove());
    return mask;
  }

  function editAliases(fp) {
    const im = images.find(x => x.fingerprint === fp) || {};
    const m = modal('<h3>编辑别名</h3>' +
      '<p class="mono" style="color:var(--text-2);font-size:12.5px;margin-bottom:10px">' + esc(fp) + '</p>' +
      '<div class="field"><label>别名（每行一个；商品/创建容器按别名引用）</label>' +
      '<textarea class="input" id="aliasText" rows="4" placeholder="debian-12">' + esc((im.aliases || []).join('\n')) + '</textarea>' +
      '<div style="font-size:12px;color:var(--text-3)">字母数字开头，可含 . _ / -；别名已被其它镜像使用时需先从那个镜像移除</div></div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">保存</button></div>');
    m.querySelector('[data-act="save"]').onclick = () => {
      const list = m.querySelector('#aliasText').value.split(/[\n,]/).map(s => s.trim()).filter(Boolean);
      ADMIN.request('/api/admin/images/' + encodeURIComponent(fp) + '/aliases', { method: 'PUT', body: { aliases: list } })
        .then(() => { LXD.toast('success', '别名已更新'); m.remove(); load(); })
        .catch(e => LXD.toast('error', e.message));
    };
  }

  /* ---------------- 作业进度 ---------------- */
  function renderJobs(jobs) {
    const card = document.getElementById('jobsCard');
    const box = document.getElementById('jobsBox');
    if (!jobs.length) { card.style.display = 'none'; return; }
    card.style.display = '';
    box.innerHTML = jobs.slice(0, 8).map(j => {
      const st = j.status === 'success' ? '<span class="badge badge-running">完成</span>' :
        j.status === 'failed' ? '<span class="badge badge-stopped">失败</span>' :
        j.status === 'running' ? '<span class="badge badge-other">进行中</span>' : '<span class="badge badge-other">排队</span>';
      const p = j.progress >= 0 ? j.progress : (j.status === 'success' ? 100 : 0);
      return '<div style="padding:8px 0;border-bottom:1px solid var(--border)">' +
        '<div style="display:flex;gap:8px;align-items:center;font-size:13px">' + st +
        '<b>' + esc(j.kind === 'image_import' ? 'URL 导入' : '远程下载') + '</b><span class="mono">' + esc(j.label || '') + '</span>' +
        (j.task_id ? '<span style="color:var(--text-3)">任务 #' + j.task_id + '</span>' : '') +
        '<span class="grow" style="flex:1"></span><span style="color:var(--text-3)">' + (j.progress >= 0 ? p + '%' : '') + '</span></div>' +
        '<div class="progress" style="margin:6px 0"><i style="width:' + p + '%"></i></div>' +
        '<div style="font-size:12px;color:' + (j.error ? 'var(--danger,#e5484d)' : 'var(--text-3)') + '">' + esc(j.error || j.message || '') + '</div></div>';
    }).join('');
  }

  function loadJobs() {
    ADMIN.request('/api/admin/images/jobs').then(res => {
      const jobs = (res.data && res.data.jobs) || [];
      renderJobs(jobs);
      const running = jobs.filter(j => j.status === 'running' || j.status === 'queued').length;
      if (lastRunning && running < lastRunning) load(); // 有作业结束，刷新镜像列表
      lastRunning = running;
      clearTimeout(pollTimer);
      if (running) pollTimer = setTimeout(loadJobs, 2000);
    }).catch(() => {});
  }

  /* ---------------- 从远程下载 ---------------- */
  function openRemote() {
    const m = modal('<h3>从远程服务器下载镜像</h3>' +
      '<div class="form-grid">' +
      '<div class="field"><label>镜像源</label><select class="input" id="rSrc"><option value="">加载中...</option></select></div>' +
      '<div class="field"><label>自定义地址（选择「自定义」时）</label><input class="input" id="rURL" placeholder="https://mirror.example.com/lxc-images"></div>' +
      '<div class="field"><label>协议</label><select class="input" id="rProto"><option value="simplestreams">simplestreams</option><option value="lxd">LXD 公共远程</option></select></div>' +
      '<div class="field"><label>&nbsp;</label><div style="display:flex;gap:6px"><button class="btn btn-primary" id="rLoad">加载目录</button><button class="btn btn-ghost" id="rSave" title="保存为自定义镜像源">保存源</button><button class="btn btn-ghost" id="rDelSrc" title="删除所选自定义镜像源">删除源</button></div></div>' +
      '</div>' +
      '<div id="rNote" style="font-size:12px;color:var(--text-3);margin:6px 0"></div>' +
      '<div class="toolbar" style="margin:8px 0">' +
      '<input class="input" id="rQ" placeholder="搜索 系统/版本/别名" style="max-width:220px">' +
      '<select class="input" id="rOS" style="width:auto"><option value="">全部系统</option></select>' +
      '<select class="input" id="rArch" style="width:auto"><option value="">全部架构</option></select>' +
      '</div>' +
      '<div id="rList" style="max-height:46vh;overflow:auto"><div class="empty">选择镜像源后点击「加载目录」</div></div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">关闭</button></div>', 980);
    let sources = [], catalog = [];
    const $ = (s) => m.querySelector(s);
    function curSource() {
      const v = $('#rSrc').value;
      if (v === '__custom') return { url: $('#rURL').value.trim(), protocol: $('#rProto').value };
      return sources.find(s => s.id === v) || null;
    }
    function fillSources(selId) {
      $('#rSrc').innerHTML = sources.map(s => '<option value="' + esc(s.id) + '">' + esc(s.name) + (s.builtin ? '' : '（自定义）') + '</option>').join('') +
        '<option value="__custom">自定义地址…</option>';
      if (selId) $('#rSrc').value = selId;
      onSrc();
    }
    function onSrc() {
      const s = curSource();
      const custom = $('#rSrc').value === '__custom';
      $('#rURL').disabled = !custom; $('#rProto').disabled = !custom;
      if (s && !custom) { $('#rURL').value = s.url; $('#rProto').value = s.protocol; }
      $('#rDelSrc').style.display = (s && !custom && !s.builtin) ? '' : 'none';
      $('#rSave').style.display = custom ? '' : 'none';
      $('#rNote').textContent = (s && s.note) ? s.note : '';
    }
    $('#rSrc').addEventListener('change', onSrc);
    ADMIN.request('/api/admin/image-sources').then(res => { sources = (res.data && res.data.sources) || []; fillSources(); })
      .catch(e => LXD.toast('error', e.message));
    $('#rSave').onclick = () => {
      const url = $('#rURL').value.trim();
      const name = window.prompt('镜像源名称', url.replace(/^https?:\/\//, '').slice(0, 40));
      if (!name) return;
      ADMIN.request('/api/admin/image-sources', { method: 'POST', body: { name: name, url: url, protocol: $('#rProto').value } })
        .then(res => ADMIN.request('/api/admin/image-sources').then(r2 => { sources = (r2.data && r2.data.sources) || []; fillSources(String((res.data && (res.data.ID || res.data.id)) || '')); LXD.toast('success', '已保存'); }))
        .catch(e => LXD.toast('error', e.message));
    };
    $('#rDelSrc').onclick = () => {
      const s = curSource(); if (!s || s.builtin) return;
      if (!window.confirm('删除镜像源「' + s.name + '」？')) return;
      ADMIN.request('/api/admin/image-sources/' + encodeURIComponent(s.id), { method: 'DELETE' })
        .then(() => ADMIN.request('/api/admin/image-sources').then(r2 => { sources = (r2.data && r2.data.sources) || []; fillSources(); }))
        .catch(e => LXD.toast('error', e.message));
    };
    function renderCatalog() {
      const q = $('#rQ').value.trim().toLowerCase(), os = $('#rOS').value, arch = $('#rArch').value;
      const list = catalog.filter(it => (!os || it.os === os) && (!arch || it.arch === arch) &&
        (!q || [it.os, it.release, it.release_title, it.variant, (it.aliases || []).join(' ')].join(' ').toLowerCase().indexOf(q) >= 0));
      if (!list.length) { $('#rList').innerHTML = '<div class="empty">没有符合条件的镜像</div>'; return; }
      $('#rList').innerHTML = '<table class="tbl"><thead><tr><th>系统</th><th>版本</th><th>变体</th><th>架构</th><th>远程别名</th><th>构建</th><th>大小</th><th>方式</th><th></th></tr></thead><tbody>' +
        list.slice(0, 400).map((it, i) => '<tr>' +
          '<td>' + esc(it.os) + '</td><td>' + esc(it.release_title || it.release) + '</td><td>' + esc(it.variant || '-') + '</td>' +
          '<td class="mono">' + esc(it.arch) + '</td><td class="mono" style="max-width:200px;overflow:hidden;text-overflow:ellipsis">' + esc((it.aliases || []).join(', ')) + '</td>' +
          '<td class="mono">' + esc(it.version || '') + '</td><td class="mono">' + esc(bytes(it.size)) + '</td>' +
          '<td>' + (it.method === 'files' ? '<span class="tag" title="Incus 格式：节点下载文件后导入">文件导入</span>' : '<span class="tag">LXD 拉取</span>') + '</td>' +
          '<td><button class="btn btn-sm btn-primary" data-dl="' + catalog.indexOf(it) + '">下载</button></td></tr>').join('') +
        '</tbody></table>' + (list.length > 400 ? '<div style="font-size:12px;color:var(--text-3);padding:6px">仅显示前 400 条，请用搜索缩小范围</div>' : '');
      $('#rList').querySelectorAll('[data-dl]').forEach(b => b.addEventListener('click', () => confirmDownload(catalog[+b.dataset.dl], curSource())));
    }
    ['#rQ', '#rOS', '#rArch'].forEach(s => $(s).addEventListener(s === '#rQ' ? 'input' : 'change', renderCatalog));
    $('#rLoad').onclick = () => {
      const s = curSource();
      if (!s || !s.url) { LXD.toast('error', '请选择或填写镜像源地址'); return; }
      $('#rList').innerHTML = '<div class="loading">正在由节点读取远程目录（大型源可能需要十几秒）...</div>';
      ADMIN.request('/api/admin/images/remote?server=' + encodeURIComponent(s.url) + '&protocol=' + encodeURIComponent(s.protocol || 'simplestreams'))
        .then(res => {
          const d = res.data || {};
          catalog = d.images || [];
          const oses = Array.from(new Set(catalog.map(x => x.os))).sort();
          const arches = Array.from(new Set(catalog.map(x => x.arch))).sort();
          $('#rOS').innerHTML = '<option value="">全部系统</option>' + oses.map(o => '<option>' + esc(o) + '</option>').join('');
          $('#rArch').innerHTML = '<option value="">全部架构</option>' + arches.map(o => '<option>' + esc(o) + '</option>').join('');
          if (d.host_arch && arches.indexOf(d.host_arch) >= 0) $('#rArch').value = d.host_arch;
          renderCatalog();
        }).catch(e => { $('#rList').innerHTML = '<div class="empty">加载失败：' + esc(e.message) + '</div>'; });
    };
  }

  function suggestAlias(it) {
    const base = (it.os || 'image').toLowerCase().replace(/[^a-z0-9]+/g, '-') + '-' + String(it.release || '').toLowerCase().replace(/[^a-z0-9.]+/g, '-');
    const v = it.variant && it.variant !== 'default' ? '-' + it.variant.toLowerCase().replace(/[^a-z0-9]+/g, '-') : '';
    const a = it.arch && hostArch && it.arch !== hostArch ? '-' + it.arch : '';
    return (base + v + a).replace(/-+$/, '');
  }

  function confirmDownload(it, src) {
    if (!it || !src) return;
    const exists = images.some(im => im.fingerprint === it.fingerprint);
    const remoteAlias = (it.aliases || [])[0] || '';
    const m = modal('<h3>下载到当前节点</h3>' +
      '<p style="font-size:13px;color:var(--text-2)">' + esc(it.os + ' ' + (it.release_title || it.release) + ' ' + (it.variant || '') + ' · ' + it.arch) + ' · ' + esc(bytes(it.size)) + '</p>' +
      (exists ? '<p style="font-size:12.5px;color:var(--warning,#f5a524)">节点上已有相同指纹的镜像，下载只会补充别名。</p>' : '') +
      (it.arch && hostArch && it.arch !== hostArch ? '<p style="font-size:12.5px;color:var(--danger,#e5484d)">注意：镜像架构 ' + esc(it.arch) + ' 与节点 ' + esc(hostArch) + ' 不同，容器将无法启动。</p>' : '') +
      '<div class="field"><label>本地别名（商品按别名引用）</label><input class="input" id="dAlias" value="' + esc(suggestAlias(it)) + '"></div>' +
      '<label style="display:flex;gap:8px;align-items:center;font-size:13px;margin:8px 0"><input type="checkbox" id="dOver"> 覆盖同名别名（把别名改指向新镜像，旧镜像保留）</label>' +
      (it.method === 'lxd' && remoteAlias ? '<label style="display:flex;gap:8px;align-items:center;font-size:13px;margin:8px 0"><input type="checkbox" id="dAuto"> LXD 自动更新（按远程别名 ' + esc(remoteAlias) + ' 定期更新，指纹会变化，别名保持）</label>' : '') +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="go">开始下载</button></div>');
    m.querySelector('[data-act="go"]').onclick = () => {
      const auto = m.querySelector('#dAuto');
      const body = {
        server: src.url, protocol: src.protocol || 'simplestreams', fingerprint: it.fingerprint, remote_alias: remoteAlias,
        method: it.method, meta_path: it.meta_path || '', rootfs_path: it.rootfs_path || '', meta_sha256: it.meta_sha256 || '', rootfs_sha256: it.rootfs_sha256 || '',
        alias: m.querySelector('#dAlias').value.trim(), overwrite_alias: m.querySelector('#dOver').checked, auto_update: !!(auto && auto.checked),
        label: (m.querySelector('#dAlias').value.trim() || (it.os + ' ' + it.release))
      };
      ADMIN.request('/api/admin/images/download', { method: 'POST', body: body })
        .then(() => { LXD.toast('success', '已加入任务队列，可在本页查看进度'); m.remove(); loadJobs(); })
        .catch(e => LXD.toast('error', e.message));
    };
  }

  /* ---------------- 从 URL 导入 ---------------- */
  function openURLImport() {
    const m = modal('<h3>从 URL 导入镜像</h3>' +
      '<div class="field"><label>镜像 URL（统一格式 tar.gz/tar.xz，或拆分格式的 metadata）</label><input class="input" id="uURL" placeholder="https://example.com/debian-12-amd64-lxc.tar.gz"></div>' +
      '<div class="field" style="margin-top:8px"><label>rootfs URL（拆分格式时填写，可选）</label><input class="input" id="uRoot" placeholder="https://example.com/rootfs.squashfs"></div>' +
      '<div class="field" style="margin-top:8px"><label>本地别名</label><input class="input" id="uAlias" placeholder="debian-12"></div>' +
      '<label style="display:flex;gap:8px;align-items:center;font-size:13px;margin:8px 0"><input type="checkbox" id="uOver"> 覆盖同名别名</label>' +
      '<div style="font-size:12px;color:var(--text-3)">由节点下载到临时目录后执行 lxc image import；需节点能访问该 URL 且磁盘空间足够。</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="go">开始导入</button></div>', 560);
    m.querySelector('[data-act="go"]').onclick = () => {
      ADMIN.request('/api/admin/images/import-url', { method: 'POST', body: {
        url: m.querySelector('#uURL').value.trim(), rootfs_url: m.querySelector('#uRoot').value.trim(),
        alias: m.querySelector('#uAlias').value.trim(), overwrite_alias: m.querySelector('#uOver').checked } })
        .then(() => { LXD.toast('success', '已加入任务队列'); m.remove(); loadJobs(); })
        .catch(e => LXD.toast('error', e.message));
    };
  }

  load();
  loadJobs();
})();
