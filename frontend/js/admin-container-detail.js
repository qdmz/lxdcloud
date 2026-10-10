/* ============================================================
   LXD Panel - Admin 容器详情页
   GET    /api/admin/containers/:name
   POST   /api/admin/containers/:name/action?action=start|stop|restart|pause|resume|reinstall|reset-password|reset-traffic
   GET    /api/admin/containers/:name/credential
   POST   /api/admin/cache/refresh?name={name}
   ============================================================ */
(function () {
  'use strict';

  let name = '';
  let detail = null;

  // adminContent 由 ADMIN.shell() 动态创建，须在 shell 之后获取
  let content = null;

  function getParam(key) {
    return new URLSearchParams(window.location.search).get(key) || '';
  }

  function actionBtn(id, label, cls, fn) {
    return '<button class="btn ' + (cls || 'btn-ghost') + ' btn-sm" id="' + id + '" onclick="' + fn + '">' + label + '</button>';
  }

  function renderActions() {
    const running = (detail && (detail.status || '').toLowerCase() === 'running');
    const paused = (detail && (detail.status || '').toLowerCase() === 'paused');
    let h = '<div class="card" style="padding:12px 14px;margin-bottom:16px">' +
      '<div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">' +
      '<a class="btn btn-ghost btn-sm" href="containers.html">‹ 返回容器列表</a>' +
      '<button class="btn btn-ghost btn-sm" onclick="refreshOne()">↻ 刷新</button>' +
      '<span class="grow"></span>' +
      actionBtn('btnStart', '▶ 启动', 'btn-success', 'confirmAction(\'start\', \'启动容器\')') +
      actionBtn('btnStop', '■ 停止', 'btn-danger', 'confirmAction(\'stop\', \'停止容器\')') +
      actionBtn('btnRestart', '↻ 重启', 'btn-warning', 'confirmAction(\'restart\', \'重启容器\')') +
      actionBtn('btnPause', '❚❚ 暂停', '', 'confirmAction(\'pause\', \'暂停容器\')') +
      actionBtn('btnResume', '▶ 恢复', 'btn-success', 'confirmAction(\'resume\', \'恢复容器\')') +
      '</div>' +
      '<div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin-top:10px;padding-top:10px;border-top:1px solid var(--border)">' +
      '<a class="btn btn-primary btn-sm" href="file-manager.html?name=' + encodeURIComponent(name) + '" target="_blank">◈ 在线管理文件</a>' +
      actionBtn('btnReinstall', '⟲ 重装系统', 'btn-primary', 'confirmAction(\'reinstall\', \'重装系统\')') +
      actionBtn('btnResetPwd', '◉ 重置密码', 'btn-warning', 'confirmAction(\'reset-password\', \'重置密码\')') +
      actionBtn('btnResetTraffic', '⇅ 重置流量', '', 'confirmAction(\'reset-traffic\', \'重置流量\')') +
      '</div></div>';

    content.innerHTML = h + '<div id="detailArea"><div class="empty" style="padding:40px">加载中...</div></div>';

    if (running) {
      document.getElementById('btnStart').disabled = true;
    } else {
      document.getElementById('btnStop').disabled = true;
      document.getElementById('btnRestart').disabled = true;
      document.getElementById('btnPause').disabled = true;
      document.getElementById('btnResume').disabled = true;
    }
    if (paused) {
      document.getElementById('btnResume').disabled = false;
      document.getElementById('btnStart').disabled = true;
    }
  }

  function renderDetail(d) {
    detail = d;
    const nm = d.name || '-';
    const status = (d.status || '').toLowerCase();

    const rows = [
      ['容器名称', '<span class="mono">' + ADMIN.esc(nm) + '</span>'],
      ['状态', LXD.statusBadge(d.status)],
      ['镜像', ADMIN.esc(d.image || '-')],
      ['Root 密码', '<code class="mono" style="word-break:break-all">' + ADMIN.esc(d.password || '-') + '</code> <button class="btn btn-ghost btn-xs" onclick="copyPwd()">复制</button>'],
      ['IPv4', (d.ipv4 && d.ipv4.length) ? d.ipv4.map((ip) => '<code class="mono">' + ADMIN.esc(ip) + '</code>').join(' ') : '-'],
      ['IPv6', (d.ipv6 && d.ipv6.length) ? d.ipv6.map((ip) => '<code class="mono">' + ADMIN.esc(ip) + '</code>').join(' ') : '-'],
      ['CPU / 内存 / 磁盘', ADMIN.esc(String(d.cpu || '-')) + ' 核 · ' + ADMIN.esc(d.memory || '-') + ' · ' + ADMIN.esc(d.disk || '-')],
      ['月流量', ADMIN.esc(String(d.traffic_limit || 0)) + ' GB（已用 ' + ADMIN.esc(d.traffic_usage || '0') + '）'],
      ['配额', 'IPv4 池 ' + ADMIN.esc(String(d.ipv4_pool_limit || 0)) + ' · IPv6 池 ' + ADMIN.esc(String(d.ipv6_pool_limit || 0)) +
        ' · IPv4 映射 ' + ADMIN.esc(String(d.ipv4_mapping_limit || 0)) + ' · IPv6 映射 ' + ADMIN.esc(String(d.ipv6_mapping_limit || 0)) +
        ' · 反代 ' + ADMIN.esc(String(d.reverse_proxy_limit || 0))],
      ['创建时间', ADMIN.esc(d.created_at || '-')],
      ['容器访问码', '<span id="admHashCode">加载中...</span> <button class="btn btn-ghost btn-xs" onclick="copyHash()">复制</button>'],
      ['快捷连接', '<a class="btn btn-primary btn-xs" id="admQuickLink" href="javascript:;" target="_blank">打开容器管理面板</a>']
    ];

    const rowsHtml = rows.map((r) =>
      '<tr><td style="width:140px;color:var(--text-3)">' + r[0] + '</td><td>' + r[1] + '</td></tr>'
    ).join('');

    const usage = '<div class="stat-grid" style="grid-template-columns:repeat(4,1fr);margin-bottom:16px">' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px">' + ADMIN.esc(String(d.cpu_usage || 0)) + '%</div><div class="stat-name">CPU 使用率</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#34d399">' + ADMIN.esc(fmtMem(d)) + '</div><div class="stat-name">内存使用</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#fbbf24">' + ADMIN.esc(fmtDisk(d)) + '</div><div class="stat-name">磁盘使用</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#38bdf8">' + ADMIN.esc(d.traffic_usage || '0') + '</div><div class="stat-name">流量使用</div></div>' +
      '</div>';

    document.getElementById('detailArea').innerHTML =
      '<div class="card" style="padding:16px;margin-bottom:16px"><div class="card-title" style="margin-bottom:12px">资源监控</div>' + usage + '</div>' +
      '<div class="card" style="padding:16px"><div class="card-title" style="margin-bottom:12px">基本信息</div>' +
      '<div class="tbl-wrap"><table class="tbl">' + rowsHtml + '</table></div></div>';
  }

  function fmtMem(d) {
    if (d.memory_usage_raw) return (d.memory_usage_raw / 1024 / 1024).toFixed(1) + ' MB';
    return d.memory_usage || '-';
  }
  function fmtDisk(d) {
    if (d.disk_usage_raw) return (d.disk_usage_raw / 1024 / 1024).toFixed(1) + ' MB';
    return d.disk_usage || '-';
  }

  function load() {
    ADMIN.request('/api/admin/containers/' + encodeURIComponent(name)).then((res) => {
      renderActions();
      renderDetail(res.data || {});
      loadCred();
      document.title = '容器详情 ' + name + ' - LXD 管理后台';
    }).catch((err) => {
      content.innerHTML = '<div class="card" style="padding:24px"><div class="empty">加载失败：' + ADMIN.esc(err.message || '未知错误') + '</div>' +
        '<div style="text-align:center;margin-top:12px"><a class="btn btn-ghost btn-sm" href="containers.html">返回容器列表</a></div></div>';
    });
  }

  function refreshOne() {
    ADMIN.request('/api/admin/cache/refresh?name=' + encodeURIComponent(name))
      .then(() => load()).catch(() => load());
  }

  /** 重装系统：选择镜像 + 可选新密码（镜像来自 /api/admin/image-options） */
  function showReinstallModal() {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:520px"><h3>重装系统 - ' + ADMIN.esc(name) + '</h3>' +
      '<div class="field" style="margin:10px 0"><label>系统镜像</label><select class="input select" id="admRiImage" style="width:100%"><option value="">加载镜像列表...</option></select></div>' +
      '<div class="field" style="margin:10px 0"><label>新 root 密码</label><input class="input" id="admRiPwd" type="text" placeholder="留空则保持原密码" style="width:100%"></div>' +
      '<div style="font-size:12px;color:var(--text-2)">重装将清空容器磁盘数据，操作不可逆，请提前备份。</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-danger" data-act="ok" disabled>确认重装</button></div></div>';
    document.body.appendChild(mask);
    const close = () => mask.remove();
    mask.querySelector('[data-act="cancel"]').onclick = close;
    mask.addEventListener('click', (e) => { if (e.target === mask) close(); });
    const sel = mask.querySelector('#admRiImage');
    const okBtn = mask.querySelector('[data-act="ok"]');
    ADMIN.request('/api/admin/image-options').then((res) => {
      const list = (res.data && res.data.images) || [];
      if (!list.length) { sel.innerHTML = '<option value="">暂无可用镜像，请先在模板管理中同步</option>'; return; }
      const opt = (t) => '<option value="' + ADMIN.esc(t.value) + '">' + ADMIN.esc(t.label || t.value) + (t.arch ? ' (' + ADMIN.esc(t.arch) + ')' : '') + '</option>';
      const local = list.filter((t) => t.source === 'local');
      const remote = list.filter((t) => t.source !== 'local');
      sel.innerHTML = (local.length ? '<optgroup label="本地镜像">' + local.map(opt).join('') + '</optgroup>' : '') +
        (remote.length ? '<optgroup label="远程镜像（首次使用需下载）">' + remote.map(opt).join('') + '</optgroup>' : '');
      sel.value = list[0].value;
      okBtn.disabled = false;
    }).catch((err) => { sel.innerHTML = '<option value="">加载镜像失败：' + ADMIN.esc(err.message || '') + '</option>'; });
    okBtn.onclick = () => {
      const image = sel.value;
      if (!image) return LXD.toast('warning', '请选择镜像');
      if (!window.confirm('确认使用「' + (sel.options[sel.selectedIndex] || {}).text + '」重装容器「' + name + '」吗？容器数据将被清空！')) return;
      okBtn.disabled = true; okBtn.textContent = '提交中...';
      ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/action?action=reinstall', {
        method: 'POST', body: { image: image, password: mask.querySelector('#admRiPwd').value.trim() }
      }).then((res) => {
        LXD.toast('success', '重装任务已提交' + (res.data && res.data.task_id ? '（任务 #' + res.data.task_id + '）' : '') + '，可在任务列表查看进度');
        close();
        setTimeout(load, 3000);
      }).catch((err) => {
        LXD.toast('error', err.message || '重装失败');
        okBtn.disabled = false; okBtn.textContent = '确认重装';
      });
    };
  }

  function confirmAction(action, label) {
    if (action === 'reinstall') return showReinstallModal();
    const tip = { start: '确定要启动该容器吗？', stop: '确定要停止该容器吗？', restart: '确定要重启该容器吗？',
      pause: '确定要暂停该容器吗？', resume: '确定要恢复该容器吗？', reinstall: '确定要重装系统吗？容器数据将被重置！',
      'reset-password': '确定要重置容器密码吗？', 'reset-traffic': '确定要重置容器流量统计吗？' };
    LXD.confirmDialog(label, tip[action] || '确定执行该操作吗？', action === 'reinstall').then((ok) => {
      if (!ok) return;
      ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/action?action=' + action, { method: 'POST' })
        .then(() => { LXD.toast('success', label + '成功'); load(); })
        .catch((err) => LXD.toast('error', err.message || label + '失败'));
    });
  }

  function copyPwd() {
    const pwd = (detail && detail.password) || '';
    if (!pwd) return LXD.toast('warn', '无密码可复制');
    (navigator.clipboard ? navigator.clipboard.writeText(pwd) : Promise.reject()).then(
      () => LXD.toast('success', '密码已复制'),
      () => { LXD.toast('success', '密码：' + pwd); }
    );
  }

  function setHash(h) {
    const el = document.getElementById('admHashCode');
    if (el) el.textContent = h || '-';
    const link = document.getElementById('admQuickLink');
    if (link) link.href = (h ? '../container/dashboard.html?hash=' + encodeURIComponent(h) : 'javascript:;');
  }

  function loadCred() {
    ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/credential').then((res) => {
      setHash((res.data && res.data.hash) || '');
    }).catch(() => setHash(''));
  }

  function copyHash() {
    const el = document.getElementById('admHashCode');
    const h = el ? el.textContent : '';
    if (!h || h === '-') return LXD.toast('warn', '暂无访问码可复制');
    (navigator.clipboard ? navigator.clipboard.writeText(h) : Promise.reject()).then(
      () => LXD.toast('success', '访问码已复制'),
      () => { LXD.toast('success', '访问码：' + h); }
    );
  }

  window.confirmAction = confirmAction;
  window.copyPwd = copyPwd;
  window.refreshOne = refreshOne;
  window.loadCred = loadCred;
  window.copyHash = copyHash;

  // ---------- 初始化 ----------
  ADMIN.shell('containers', '容器详情', '查看与管理容器运行状态');
  content = document.getElementById('adminContent');

  name = getParam('name');
  if (!name) {
    content.innerHTML = '<div class="card" style="padding:24px"><div class="empty">缺少容器名称参数</div>' +
      '<div style="text-align:center;margin-top:12px"><a class="btn btn-ghost btn-sm" href="containers.html">返回容器列表</a></div></div>';
  } else {
    load();
  }
})();
