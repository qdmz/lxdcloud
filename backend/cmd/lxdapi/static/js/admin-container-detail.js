/* ============================================================
   LXD Panel - Admin 容器详情页（多子功能 Tab）
   子功能：① 资源监控/基本信息  ② IPv4 地址池(申请/释放)
           ③ IPv4 端口映射  ④ IPv6 地址池(申请/释放)
           ⑤ IPv6 端口映射  ⑥ 反向代理  ⑦ DNS 设置
   API：
     GET  /api/admin/containers/:name
     POST /api/admin/containers/:name/action?action=...
     GET  /api/admin/containers/:name/credential
     GET  /api/admin/containers/:name/ip?version=v4|v6
     POST /api/admin/containers/:name/ip/allocate?version=v4|v6
     POST /api/admin/ip/release?version=v4|v6  {name, ip}
     GET  /api/admin/port-mapping?version=v4|v6
     POST /api/admin/port-mapping/allocate?version=v4|v6
     POST /api/admin/port-mapping/release?version=v4|v6  {ids:[...]}
     GET/POST /api/admin/nginx/proxies
     DELETE /api/admin/nginx/proxies/:id
     GET/PUT /api/admin/containers/:name/dns
   ============================================================ */
(function () {
  'use strict';

  let name = '';
  let detail = null;
  let currentTab = 'info';

  // adminContent 由 ADMIN.shell() 动态创建，须在 shell 之后获取
  let content = null;

  function getParam(key) {
    return new URLSearchParams(window.location.search).get(key) || '';
  }

  function esc(s) {
    return LXD.esc(s == null ? '' : String(s));
  }

  function fmtTime(t) {
    if (!t) return '-';
    try { return new Date(t).toLocaleString('zh-CN'); } catch (e) { return t; }
  }

  /* ================= 子功能 Tab ================= */
  function tabBar() {
    return '<div class="tabs" style="margin-bottom:14px">' +
      '<button class="tab' + (currentTab === 'info' ? ' active' : '') + '" data-tab="info">资源监控</button>' +
      '<button class="tab' + (currentTab === 'ipv4Pool' ? ' active' : '') + '" data-tab="ipv4Pool">IPv4 地址池</button>' +
      '<button class="tab' + (currentTab === 'ipv4Mapping' ? ' active' : '') + '" data-tab="ipv4Mapping">IPv4 端口映射</button>' +
      '<button class="tab' + (currentTab === 'ipv6Pool' ? ' active' : '') + '" data-tab="ipv6Pool">IPv6 地址池</button>' +
      '<button class="tab' + (currentTab === 'ipv6Mapping' ? ' active' : '') + '" data-tab="ipv6Mapping">IPv6 端口映射</button>' +
      '<button class="tab' + (currentTab === 'reverseProxy' ? ' active' : '') + '" data-tab="reverseProxy">反向代理</button>' +
      '<button class="tab' + (currentTab === 'dns' ? ' active' : '') + '" data-tab="dns">DNS 设置</button>' +
      '</div>';
  }

  function bindTabs() {
    content.querySelectorAll('.tab[data-tab]').forEach(function (b) {
      b.addEventListener('click', function () { switchTab(b.dataset.tab); });
    });
  }

  function switchTab(tab) {
    currentTab = tab;
    if (tab === 'ipv4Pool') { renderPane('<div class="loading">加载中...</div>'); loadIpv4Pool(); }
    else if (tab === 'ipv4Mapping') { renderPane('<div class="loading">加载中...</div>'); loadIpv4Mapping(); }
    else if (tab === 'ipv6Pool') { renderPane('<div class="loading">加载中...</div>'); loadIpv6Pool(); }
    else if (tab === 'ipv6Mapping') { renderPane('<div class="loading">加载中...</div>'); loadIpv6Mapping(); }
    else if (tab === 'reverseProxy') { renderPane('<div class="loading">加载中...</div>'); loadReverseProxy(); }
    else if (tab === 'dns') { renderDns(); }
    else { currentTab = 'info'; renderInfo(); }
  }

  // 只重绘 pane 区域（头部操作栏保持）
  function renderPane(html) {
    const pane = document.getElementById('detailPane');
    if (pane) { pane.innerHTML = html; bindTabs(); }
  }

  /* ================= 顶部操作栏（全局常驻） ================= */
  function renderHeader() {
    content.innerHTML =
      '<div class="card" style="padding:12px 14px;margin-bottom:16px">' +
      '<div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">' +
      '<a class="btn btn-ghost btn-sm" href="/admin/containers">‹ 返回容器列表</a>' +
      '<button class="btn btn-ghost btn-sm" id="btnRefresh">↻ 刷新</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost btn-sm" id="btnStart">▶ 启动</button>' +
      '<button class="btn btn-ghost btn-sm" id="btnStop">■ 停止</button>' +
      '<button class="btn btn-ghost btn-sm" id="btnRestart">↻ 重启</button>' +
      '<button class="btn btn-ghost btn-sm" id="btnPause">❚❚ 暂停</button>' +
      '<button class="btn btn-ghost btn-sm" id="btnResume">▶ 恢复</button>' +
      '<a class="btn btn-primary btn-sm" id="btnFiles" target="_blank">◈ 在线管理文件</a>' +
      '<button class="btn btn-ghost btn-sm" id="btnReinstall">⟲ 重装系统</button>' +
      '<button class="btn btn-ghost btn-sm" id="btnResetPwd">◉ 重置密码</button>' +
      '<button class="btn btn-ghost btn-sm" id="btnResetTraffic">⇅ 重置流量</button>' +
      '</div></div>' +
      tabBar() +
      '<div id="detailPane"><div class="empty" style="padding:40px">加载中...</div></div>';

    bindTabs();
    document.getElementById('btnRefresh').addEventListener('click', function () { load(); });
    document.getElementById('btnStart').addEventListener('click', function () { confirmAction('start', '启动容器'); });
    document.getElementById('btnStop').addEventListener('click', function () { confirmAction('stop', '停止容器'); });
    document.getElementById('btnRestart').addEventListener('click', function () { confirmAction('restart', '重启容器'); });
    document.getElementById('btnPause').addEventListener('click', function () { confirmAction('pause', '暂停容器'); });
    document.getElementById('btnResume').addEventListener('click', function () { confirmAction('resume', '恢复容器'); });
    document.getElementById('btnFiles').href = '/admin/file-manager?name=' + encodeURIComponent(name);
    document.getElementById('btnReinstall').addEventListener('click', function () { confirmAction('reinstall', '重装系统'); });
    document.getElementById('btnResetPwd').addEventListener('click', function () { confirmAction('reset-password', '重置密码'); });
    document.getElementById('btnResetTraffic').addEventListener('click', function () { confirmAction('reset-traffic', '重置流量'); });

    if (detail) {
      const running = (detail.status || '').toLowerCase() === 'running';
      const paused = (detail.status || '').toLowerCase() === 'paused';
      document.getElementById('btnStart').disabled = running;
      if (running) { document.getElementById('btnStop').disabled = false; document.getElementById('btnRestart').disabled = false; document.getElementById('btnPause').disabled = false; document.getElementById('btnResume').disabled = true; }
      else { document.getElementById('btnStop').disabled = true; document.getElementById('btnRestart').disabled = true; document.getElementById('btnPause').disabled = true; document.getElementById('btnResume').disabled = true; }
      if (paused) { document.getElementById('btnResume').disabled = false; document.getElementById('btnStart').disabled = true; }
    }
  }

  /* ================= ① 资源监控 / 基本信息 ================= */
  function renderInfo() {
    renderHeader();
    const pane = document.getElementById('detailPane');
    const d = detail || {};
    const nm = d.name || '-';
    const rows = [
      ['容器名称', '<span class="mono">' + esc(nm) + '</span>'],
      ['状态', LXD.statusBadge(d.status)],
      ['镜像', esc(d.image || '-')],
      ['Root 密码', '<code class="mono" style="word-break:break-all">' + esc(d.password || '-') + '</code> <button class="btn btn-ghost btn-xs" id="btnCopyPwd">复制</button>'],
      ['IPv4', (d.ipv4 && d.ipv4.length) ? d.ipv4.map(function (ip) { return '<code class="mono">' + esc(ip) + '</code>'; }).join(' ') : '-'],
      ['IPv6', (d.ipv6 && d.ipv6.length) ? d.ipv6.map(function (ip) { return '<code class="mono">' + esc(ip) + '</code>'; }).join(' ') : '-'],
      ['CPU / 内存 / 磁盘', esc(String(d.cpu || '-')) + ' 核 · ' + esc(d.memory || '-') + ' · ' + esc(d.disk || '-')],
      ['月流量', esc(String(d.traffic_limit || 0)) + ' GB（已用 ' + esc(d.traffic_usage || '0') + '）'],
      ['配额', 'IPv4 池 ' + esc(String(d.ipv4_pool_limit || 0)) + ' · IPv6 池 ' + esc(String(d.ipv6_pool_limit || 0)) +
        ' · IPv4 映射 ' + esc(String(d.ipv4_mapping_limit || 0)) + ' · IPv6 映射 ' + esc(String(d.ipv6_mapping_limit || 0)) +
        ' · 反代 ' + esc(String(d.reverse_proxy_limit || 0))],
      ['创建时间', esc(d.created_at || '-')],
      ['容器访问码', '<span id="admHashCode">加载中...</span> <button class="btn btn-ghost btn-xs" id="btnCopyHash">复制</button>'],
      ['快捷连接', '<a class="btn btn-primary btn-xs" id="admQuickLink" href="javascript:;" target="_blank">打开容器管理面板</a>']
    ];
    const rowsHtml = rows.map(function (r) {
      return '<tr><td style="width:140px;color:var(--text-3)">' + r[0] + '</td><td>' + r[1] + '</td></tr>';
    }).join('');

    const usage = '<div class="stat-grid" style="grid-template-columns:repeat(4,1fr);margin-bottom:16px">' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px">' + esc(String(d.cpu_usage || 0)) + '%</div><div class="stat-name">CPU 使用率</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#34d399">' + esc(fmtMem(d)) + '</div><div class="stat-name">内存使用</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#fbbf24">' + esc(fmtDisk(d)) + '</div><div class="stat-name">磁盘使用</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#38bdf8">' + esc(d.traffic_usage || '0') + '</div><div class="stat-name">流量使用</div></div>' +
      '</div>';

    pane.innerHTML =
      '<div class="card" style="padding:16px;margin-bottom:16px"><div class="card-title" style="margin-bottom:12px">资源监控</div>' + usage + '</div>' +
      '<div class="card" style="padding:16px"><div class="card-title" style="margin-bottom:12px">基本信息</div>' +
      '<div class="tbl-wrap"><table class="tbl">' + rowsHtml + '</table></div></div>';

    document.getElementById('btnCopyPwd').addEventListener('click', copyPwd);
    document.getElementById('btnCopyHash').addEventListener('click', copyHash);
    setHash((window._admHash) || '');
  }

  function fmtMem(d) {
    if (d.memory_usage_raw) return (d.memory_usage_raw / 1024 / 1024).toFixed(1) + ' MB';
    return d.memory_usage || '-';
  }
  function fmtDisk(d) {
    if (d.disk_usage_raw) return (d.disk_usage_raw / 1024 / 1024).toFixed(1) + ' MB';
    return d.disk_usage || '-';
  }

  function setHash(h) {
    window._admHash = h || '';
    const el = document.getElementById('admHashCode');
    if (el) el.textContent = h || '-';
    const link = document.getElementById('admQuickLink');
    if (link) link.href = (h ? '../container/dashboard.html?hash=' + encodeURIComponent(h) : 'javascript:;');
  }

  function copyPwd() {
    const pwd = (detail && detail.password) || '';
    if (!pwd) return LXD.toast('warn', '无密码可复制');
    (navigator.clipboard ? navigator.clipboard.writeText(pwd) : Promise.reject()).then(
      function () { LXD.toast('success', '密码已复制'); },
      function () { LXD.toast('success', '密码：' + pwd); }
    );
  }

  function copyHash() {
    const el = document.getElementById('admHashCode');
    const h = el ? el.textContent : '';
    if (!h || h === '-') return LXD.toast('warn', '暂无访问码可复制');
    (navigator.clipboard ? navigator.clipboard.writeText(h) : Promise.reject()).then(
      function () { LXD.toast('success', '访问码已复制'); },
      function () { LXD.toast('success', '访问码：' + h); }
    );
  }

  function confirmAction(action, label) {
    const tip = { start: '确定要启动该容器吗？', stop: '确定要停止该容器吗？', restart: '确定要重启该容器吗？',
      pause: '确定要暂停该容器吗？', resume: '确定要恢复该容器吗？', reinstall: '确定要重装系统吗？容器数据将被重置！',
      'reset-password': '确定要重置容器密码吗？', 'reset-traffic': '确定要重置容器流量统计吗？' };
    LXD.confirmDialog(label, tip[action] || '确定执行该操作吗？', action === 'reinstall').then(function (ok) {
      if (!ok) return;
      ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/action?action=' + action, { method: 'POST' })
        .then(function () { LXD.toast('success', label + '成功'); load(); })
        .catch(function (err) { LXD.toast('error', err.message || label + '失败'); });
    });
  }

  /* ================= 通用 Modal ================= */
  function modal(html, onSave) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:460px">' + html + '</div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="cancel"]').onclick = function () { mask.remove(); };
    mask.addEventListener('click', function (e) { if (e.target === mask) mask.remove(); });
    mask.querySelector('[data-act="save"]').onclick = function () { onSave(mask); };
    return mask;
  }

  /* ================= ② IPv4 地址池 ================= */
  function loadIpv4Pool() {
    ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/ip?version=v4').then(function (res) {
      renderIpv4Pool((res.data && res.data.ipv4) || []);
    }).catch(function (e) { renderPane('<div class="empty">加载失败：' + esc(e.message) + '</div>'); });
  }

  function renderIpv4Pool(list) {
    const rows = list.map(function (b) {
      const ip = b.IPAddress || b.ip_address || '-';
      const st = (b.Status || b.status || 'allocated').toLowerCase();
      const stBadge = st === 'bound' ? '<span class="badge badge-running">已绑定</span>'
        : '<span class="badge badge-other">已分配</span>';
      return '<tr>' +
        '<td class="mono">' + esc(ip) + '</td>' +
        '<td>' + stBadge + '</td>' +
        '<td class="mono">' + esc(fmtTime(b.CreatedAt || b.created_at)) + '</td>' +
        '<td class="row-actions"><button class="btn btn-sm btn-danger-ghost" data-rel-ip="' + esc(ip) + '">释放</button></td>' +
        '</tr>';
    }).join('');

    renderPane(
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnAllocV4">＋ 申请 IPv4</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReloadV4">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      '<div class="card-title" style="margin-bottom:10px">已分配公网 IPv4（' + list.length + '）</div>' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr><th>公网 IPv4</th><th>状态</th><th>分配时间</th><th style="width:90px">操作</th></tr></thead><tbody>' + rows + '</tbody></table></div>'
        : '<div class="empty">暂无 IPv4 地址，点击「申请 IPv4」分配</div>') +
      '</div>'
    );

    document.getElementById('btnAllocV4').addEventListener('click', allocIpv4);
    document.getElementById('btnReloadV4').addEventListener('click', loadIpv4Pool);
    content.querySelectorAll('[data-rel-ip]').forEach(function (b) {
      b.addEventListener('click', function () { releaseIp('v4', b.dataset.relIp); });
    });
  }

  function allocIpv4() {
    modal(
      '<h3>申请公网 IPv4</h3>' +
      '<div class="form-grid">' +
      '<div class="field" style="grid-column:1/-1"><label>容器名称</label><input class="input" id="fCtnV4" value="' + esc(name) + '" disabled></div>' +
      '<div class="field" style="grid-column:1/-1"><label>分配数量</label><input class="input" id="fCountV4" type="number" min="1" value="1"></div>' +
      '</div>' +
      '<p style="font-size:12.5px;color:var(--text-3);margin-top:4px">系统将从 IPv4 地址池中自动分配公网地址并绑定到容器。</p>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">申请</button></div>',
      function (mask) {
        const count = parseInt(mask.querySelector('#fCountV4').value, 10) || 1;
        ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/ip/allocate?version=v4', { method: 'POST', body: { count: count } })
          .then(function () { LXD.toast('success', 'IPv4 分配成功'); mask.remove(); loadIpv4Pool(); })
          .catch(function (e) { LXD.toast('error', e.message); });
      });
  }

  /* ================= ④ IPv6 地址池 ================= */
  function loadIpv6Pool() {
    ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/ip?version=v6').then(function (res) {
      renderIpv6Pool((res.data && res.data.ipv6) || []);
    }).catch(function (e) { renderPane('<div class="empty">加载失败：' + esc(e.message) + '</div>'); });
  }

  function renderIpv6Pool(list) {
    const rows = list.map(function (b) {
      const ip = b.IPAddress || b.ip_address || '-';
      const st = (b.Status || b.status || 'allocated').toLowerCase();
      const stBadge = st === 'bound' ? '<span class="badge badge-running">已绑定</span>'
        : '<span class="badge badge-other">已分配</span>';
      return '<tr>' +
        '<td class="mono">' + esc(ip) + '</td>' +
        '<td>' + stBadge + '</td>' +
        '<td class="mono">' + esc(fmtTime(b.CreatedAt || b.created_at)) + '</td>' +
        '<td class="row-actions"><button class="btn btn-sm btn-danger-ghost" data-rel-ip="' + esc(ip) + '">释放</button></td>' +
        '</tr>';
    }).join('');

    renderPane(
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnAllocV6">＋ 申请 IPv6</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReloadV6">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      '<div class="card-title" style="margin-bottom:10px">已分配公网 IPv6（' + list.length + '）</div>' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr><th>公网 IPv6</th><th>状态</th><th>分配时间</th><th style="width:90px">操作</th></tr></thead><tbody>' + rows + '</tbody></table></div>'
        : '<div class="empty">暂无 IPv6 地址，点击「申请 IPv6」分配</div>') +
      '</div>'
    );

    document.getElementById('btnAllocV6').addEventListener('click', allocIpv6);
    document.getElementById('btnReloadV6').addEventListener('click', loadIpv6Pool);
    content.querySelectorAll('[data-rel-ip]').forEach(function (b) {
      b.addEventListener('click', function () { releaseIp('v6', b.dataset.relIp); });
    });
  }

  function allocIpv6() {
    modal(
      '<h3>申请公网 IPv6</h3>' +
      '<div class="form-grid">' +
      '<div class="field" style="grid-column:1/-1"><label>容器名称</label><input class="input" id="fCtnV6" value="' + esc(name) + '" disabled></div>' +
      '<div class="field" style="grid-column:1/-1"><label>分配数量</label><input class="input" id="fCountV6" type="number" min="1" value="1"></div>' +
      '</div>' +
      '<p style="font-size:12.5px;color:var(--text-3);margin-top:4px">系统将从 IPv6 地址池中自动分配公网地址并绑定到容器。</p>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">申请</button></div>',
      function (mask) {
        const count = parseInt(mask.querySelector('#fCountV6').value, 10) || 1;
        ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/ip/allocate?version=v6', { method: 'POST', body: { count: count } })
          .then(function () { LXD.toast('success', 'IPv6 分配成功'); mask.remove(); loadIpv6Pool(); })
          .catch(function (e) { LXD.toast('error', e.message); });
      });
  }

  function releaseIp(version, ip) {
    LXD.confirmDialog('释放 IP 地址', '确定要释放该地址吗？\n' + ip + '\n\n释放后将删除相关的 NAT 规则。', false).then(function (ok) {
      if (!ok) return;
      ADMIN.request('/api/admin/ip/release?version=' + version, { method: 'POST', body: { name: name, ip: ip } })
        .then(function () { LXD.toast('success', '释放成功'); if (version === 'v4') loadIpv4Pool(); else loadIpv6Pool(); })
        .catch(function (e) { LXD.toast('error', e.message); });
    });
  }

  /* ================= ③ IPv4 端口映射 ================= */
  function loadIpv4Mapping() {
    ADMIN.request('/api/admin/port-mapping?version=v4').then(function (res) {
      const all = (res.data && res.data.ipv4) || [];
      renderIpv4Mapping(all.filter(function (m) { return (m.container_name || '') === name; }));
    }).catch(function (e) { renderPane('<div class="empty">加载失败：' + esc(e.message) + '</div>'); });
  }

  function renderIpv4Mapping(list) {
    const rows = list.map(function (m) {
      const pubPort = m.public_port_end && m.public_port_end !== m.public_port
        ? (m.public_port + '-' + m.public_port_end) : m.public_port;
      const ctnPort = m.container_port_end && m.container_port_end !== m.container_port
        ? (m.container_port + '-' + m.container_port_end) : m.container_port;
      const st = (m.status || 'active').toLowerCase();
      const stBadge = st === 'active' ? '<span class="badge badge-running">生效</span>'
        : '<span class="badge badge-other">' + esc(m.status || '-') + '</span>';
      return '<tr>' +
        '<td class="mono">' + esc(m.public_ip || '-') + ':' + pubPort + '</td>' +
        '<td class="mono">' + ctnPort + '</td>' +
        '<td class="mono">' + esc((m.protocol || 'tcp').toUpperCase()) + '</td>' +
        '<td>' + stBadge + '</td>' +
        '<td>' + esc(m.description || '-') + '</td>' +
        '<td class="mono">' + esc(m.created_at || '-') + '</td>' +
        '<td class="row-actions"><button class="btn btn-sm btn-danger-ghost" data-rel-map="' + m.id + '" data-info="' + esc((m.public_ip || '') + ':' + pubPort + ' → ' + name + ':' + ctnPort) + '">释放</button></td>' +
        '</tr>';
    }).join('');

    renderPane(
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnAddMapV4">＋ 添加端口映射</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReloadMapV4">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      '<div class="card-title" style="margin-bottom:10px">IPv4 端口映射（' + list.length + '）</div>' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr><th>公网地址</th><th>容器端口</th><th>协议</th><th>状态</th><th>描述</th><th>创建时间</th><th style="width:90px">操作</th></tr></thead><tbody>' + rows + '</tbody></table></div>'
        : '<div class="empty">暂无 IPv4 端口映射，点击「添加端口映射」创建</div>') +
      '</div>'
    );

    document.getElementById('btnAddMapV4').addEventListener('click', function () { formAddMapping('v4'); });
    document.getElementById('btnReloadMapV4').addEventListener('click', loadIpv4Mapping);
    content.querySelectorAll('[data-rel-map]').forEach(function (b) {
      b.addEventListener('click', function () { releaseMapping('v4', parseInt(b.dataset.relMap, 10), b.dataset.info); });
    });
  }

  /* ================= ⑤ IPv6 端口映射 ================= */
  function loadIpv6Mapping() {
    ADMIN.request('/api/admin/port-mapping?version=v6').then(function (res) {
      const all = (res.data && res.data.ipv6) || [];
      renderIpv6Mapping(all.filter(function (m) { return (m.container_name || '') === name; }));
    }).catch(function (e) { renderPane('<div class="empty">加载失败：' + esc(e.message) + '</div>'); });
  }

  function renderIpv6Mapping(list) {
    const rows = list.map(function (m) {
      const pubPort = m.public_port_end && m.public_port_end !== m.public_port
        ? (m.public_port + '-' + m.public_port_end) : m.public_port;
      const ctnPort = m.container_port_end && m.container_port_end !== m.container_port
        ? (m.container_port + '-' + m.container_port_end) : m.container_port;
      const st = (m.status || 'active').toLowerCase();
      const stBadge = st === 'active' ? '<span class="badge badge-running">生效</span>'
        : '<span class="badge badge-other">' + esc(m.status || '-') + '</span>';
      return '<tr>' +
        '<td class="mono">' + esc(m.public_ip || '-') + ':' + pubPort + '</td>' +
        '<td class="mono">' + ctnPort + '</td>' +
        '<td class="mono">' + esc((m.protocol || 'tcp').toUpperCase()) + '</td>' +
        '<td>' + stBadge + '</td>' +
        '<td>' + esc(m.description || '-') + '</td>' +
        '<td class="mono">' + esc(m.created_at || '-') + '</td>' +
        '<td class="row-actions"><button class="btn btn-sm btn-danger-ghost" data-rel-map="' + m.id + '" data-info="' + esc((m.public_ip || '') + ':' + pubPort + ' → ' + name + ':' + ctnPort) + '">释放</button></td>' +
        '</tr>';
    }).join('');

    renderPane(
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnAddMapV6">＋ 添加端口映射</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReloadMapV6">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      '<div class="card-title" style="margin-bottom:10px">IPv6 端口映射（' + list.length + '）</div>' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr><th>公网地址</th><th>容器端口</th><th>协议</th><th>状态</th><th>描述</th><th>创建时间</th><th style="width:90px">操作</th></tr></thead><tbody>' + rows + '</tbody></table></div>'
        : '<div class="empty">暂无 IPv6 端口映射，点击「添加端口映射」创建</div>') +
      '</div>'
    );

    document.getElementById('btnAddMapV6').addEventListener('click', function () { formAddMapping('v6'); });
    document.getElementById('btnReloadMapV6').addEventListener('click', loadIpv6Mapping);
    content.querySelectorAll('[data-rel-map]').forEach(function (b) {
      b.addEventListener('click', function () { releaseMapping('v6', parseInt(b.dataset.relMap, 10), b.dataset.info); });
    });
  }

  function formAddMapping(version) {
    modal(
      '<h3>添加端口映射（' + (version === 'v4' ? 'IPv4' : 'IPv6') + '）</h3>' +
      '<div class="form-grid">' +
      '<div class="field" style="grid-column:1/-1"><label>容器名称</label><input class="input" id="fCtnMap" value="' + esc(name) + '" disabled></div>' +
      '<div class="field"><label>容器端口</label><input class="input" id="fCtnPortMap" type="number" min="1" placeholder="如 80"></div>' +
      '<div class="field"><label>端口数量</label><input class="input" id="fCountMap" type="number" min="1" value="1"></div>' +
      '<div class="field" style="grid-column:1/-1"><label>公网端口（留空自动分配）</label><input class="input" id="fPubPortMap" type="number" min="1" placeholder="自动"></div>' +
      '<div class="field" style="grid-column:1/-1"><label>描述</label><input class="input" id="fDescMap" placeholder="可选"></div>' +
      '</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">添加</button></div>',
      function (mask) {
        const body = {
          container_name: name,
          container_port: parseInt(mask.querySelector('#fCtnPortMap').value, 10) || 0,
          port_count: parseInt(mask.querySelector('#fCountMap').value, 10) || 1,
          public_port: parseInt(mask.querySelector('#fPubPortMap').value, 10) || 0,
          description: mask.querySelector('#fDescMap').value.trim()
        };
        if (!body.container_port) { LXD.toast('error', '容器端口必填'); return; }
        ADMIN.request('/api/admin/port-mapping/allocate?version=' + version, { method: 'POST', body: body })
          .then(function (res) {
            const d = res.data || {};
            const n = (d.mappings && d.mappings.length) || d.total || 1;
            LXD.toast('success', '分配成功：' + n + ' 条');
            mask.remove();
            if (version === 'v4') loadIpv4Mapping(); else loadIpv6Mapping();
          }).catch(function (e) { LXD.toast('error', e.message); });
      });
  }

  function releaseMapping(version, id, info) {
    LXD.confirmDialog('释放端口映射', '确定要释放该映射吗？\n' + info, false).then(function (ok) {
      if (!ok) return;
      ADMIN.request('/api/admin/port-mapping/release?version=' + version, { method: 'POST', body: { ids: [id] } })
        .then(function () { LXD.toast('success', '释放成功'); if (version === 'v4') loadIpv4Mapping(); else loadIpv6Mapping(); })
        .catch(function (e) { LXD.toast('error', e.message); });
    });
  }

  /* ================= ⑥ 反向代理 ================= */
  function loadReverseProxy() {
    ADMIN.request('/api/admin/nginx/proxies').then(function (res) {
      const all = Array.isArray(res.data) ? res.data : [];
      renderReverseProxy(all.filter(function (r) { return (r.container_name || '') === name; }));
    }).catch(function (e) { renderPane('<div class="empty">加载失败：' + esc(e.message) + '</div>'); });
  }

  function renderReverseProxy(list) {
    const rows = list.map(function (r) {
      const proto = (r.protocol || 'http').toUpperCase();
      const sslBadge = r.enable_ssl ? '<span class="badge badge-running">已启用</span>' : '<span class="badge badge-other">未启用</span>';
      const st = (r.status || 'active').toLowerCase();
      const stBadge = st === 'active' ? '<span class="badge badge-running">生效</span>' : '<span class="badge badge-other">停用</span>';
      return '<tr>' +
        '<td class="mono"><strong>' + esc(r.domain || '-') + '</strong></td>' +
        '<td class="mono">' + esc(proto) + '</td>' +
        '<td class="mono">' + esc(String(r.target_port || '-')) + '</td>' +
        '<td>' + sslBadge + '</td>' +
        '<td>' + stBadge + '</td>' +
        '<td>' + esc(r.description || '-') + '</td>' +
        '<td class="row-actions"><button class="btn btn-sm btn-danger-ghost" data-del-proxy="' + r.id + '" data-domain="' + esc(r.domain || '') + '">删除</button></td>' +
        '</tr>';
    }).join('');

    renderPane(
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnAddProxy">＋ 新建反向代理</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReloadProxy">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      '<div class="card-title" style="margin-bottom:10px">反向代理规则（' + list.length + '）</div>' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr><th>域名</th><th>协议</th><th>目标端口</th><th>SSL</th><th>状态</th><th>描述</th><th style="width:90px">操作</th></tr></thead><tbody>' + rows + '</tbody></table></div>'
        : '<div class="empty">暂无反向代理规则，点击「新建反向代理」创建</div>') +
      '</div>'
    );

    document.getElementById('btnAddProxy').addEventListener('click', formAddProxy);
    document.getElementById('btnReloadProxy').addEventListener('click', loadReverseProxy);
    content.querySelectorAll('[data-del-proxy]').forEach(function (b) {
      b.addEventListener('click', function () { delProxy(parseInt(b.dataset.delProxy, 10), b.dataset.domain); });
    });
  }

  function formAddProxy() {
    modal(
      '<h3>新建反向代理</h3>' +
      '<div class="form-grid">' +
      '<div class="field" style="grid-column:1/-1"><label>容器名称 *</label><input class="input" id="fCtnProxy" value="' + esc(name) + '" disabled></div>' +
      '<div class="field" style="grid-column:1/-1"><label>域名 *</label><input class="input" id="fDomainProxy" placeholder="例如 www.example.com"></div>' +
      '<div class="field"><label>协议</label><select class="input" id="fProtoProxy"><option value="http">http</option><option value="https">https</option></select></div>' +
      '<div class="field"><label>目标端口</label><input class="input" id="fPortProxy" type="number" min="1" value="80"></div>' +
      '<div class="field" style="grid-column:1/-1"><label>描述</label><input class="input" id="fDescProxy" placeholder="可选"></div>' +
      '<div class="field" style="grid-column:1/-1;display:flex;align-items:center;gap:8px"><input type="checkbox" id="fSSLProxy"> <label style="margin:0">启用 SSL（https 时必填证书）</label></div>' +
      '<div class="field" style="grid-column:1/-1" id="fCertWrap" hidden><label>SSL 证书（PEM）</label><textarea class="input" id="fCertProxy" rows="3" placeholder="-----BEGIN CERTIFICATE-----"></textarea></div>' +
      '<div class="field" style="grid-column:1/-1" id="fKeyWrap" hidden><label>SSL 私钥（PEM）</label><textarea class="input" id="fKeyProxy" rows="3" placeholder="-----BEGIN PRIVATE KEY-----"></textarea></div>' +
      '</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">添加</button></div>',
      function (mask) {
        const sslOn = mask.querySelector('#fSSLProxy').checked;
        const proto = mask.querySelector('#fProtoProxy').value;
        const domain = mask.querySelector('#fDomainProxy').value.trim();
        const port = parseInt(mask.querySelector('#fPortProxy').value, 10) || 0;
        if (!domain) { LXD.toast('error', '域名必填'); return; }
        if (!port) { LXD.toast('error', '目标端口必填'); return; }
        if (sslOn && !mask.querySelector('#fCertProxy').value.trim() && !mask.querySelector('#fKeyProxy').value.trim()) {
          LXD.toast('error', '启用 SSL 需提供证书与私钥'); return;
        }
        const body = {
          container_name: name,
          protocol: sslOn ? 'https' : proto,
          domain: domain,
          target_port: port,
          description: mask.querySelector('#fDescProxy').value.trim(),
          enable_ssl: sslOn,
          status: 'active'
        };
        if (sslOn) {
          body.ssl_cert = mask.querySelector('#fCertProxy').value.trim();
          body.ssl_key = mask.querySelector('#fKeyProxy').value.trim();
        }
        ADMIN.request('/api/admin/nginx/proxies', { method: 'POST', body: body })
          .then(function () { LXD.toast('success', '反向代理规则添加成功'); mask.remove(); loadReverseProxy(); })
          .catch(function (e) { LXD.toast('error', e.message); });
      });
  }

  function delProxy(id, domain) {
    LXD.confirmDialog('删除反向代理', '确定要删除该反向代理规则吗？\n' + domain, false).then(function (ok) {
      if (!ok) return;
      ADMIN.request('/api/admin/nginx/proxies/' + id, { method: 'DELETE' })
        .then(function () { LXD.toast('success', '删除成功'); loadReverseProxy(); })
        .catch(function (e) { LXD.toast('error', e.message); });
    });
  }

  /* ================= ⑦ DNS 设置 ================= */
  function renderDns() {
    renderPane('<div class="loading">加载中...</div>');
    ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/dns').then(function (res) {
      const arr = (res.data && res.data.dns) || [];
      const val = Array.isArray(arr) ? arr.join('\n') : '';
      renderPane(
        '<div class="card" style="padding:18px;max-width:620px">' +
        '<h3 style="margin-bottom:4px">DNS 设置</h3>' +
        '<p style="font-size:13px;color:var(--text-3);margin-bottom:14px">配置容器使用的 DNS 服务器，每行一个地址。</p>' +
        '<div class="field"><label>DNS 服务器</label>' +
        '<textarea class="input" id="dnsText" rows="5" placeholder="例如：&#10;8.8.8.8&#10;8.8.4.4">' + esc(val) + '</textarea></div>' +
        '<div style="display:flex;gap:10px;margin-top:16px">' +
        '<button class="btn btn-primary" id="btnSaveDns">保存设置</button>' +
        '<button class="btn btn-ghost" id="btnReloadDns">↻ 刷新</button>' +
        '</div></div>'
      );
      document.getElementById('btnSaveDns').addEventListener('click', saveDns);
      document.getElementById('btnReloadDns').addEventListener('click', renderDns);
    }).catch(function (e) { renderPane('<div class="empty">加载失败：' + esc(e.message) + '</div>'); });
  }

  function saveDns() {
    const text = document.getElementById('dnsText').value.trim();
    if (!text) { LXD.toast('error', 'DNS 服务器不能为空'); return; }
    const dns = text.split('\n').map(function (s) { return s.trim(); }).filter(function (s) { return s; });
    if (!dns.length) { LXD.toast('error', 'DNS 服务器不能为空'); return; }
    ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/dns', { method: 'PUT', body: { dns: dns } })
      .then(function () { LXD.toast('success', 'DNS 设置成功'); renderDns(); })
      .catch(function (e) { LXD.toast('error', e.message); });
  }

  /* ================= 加载容器详情 ================= */
  function load() {
    ADMIN.request('/api/admin/containers/' + encodeURIComponent(name)).then(function (res) {
      detail = res.data || {};
      document.title = '容器详情 ' + name + ' - LXD 管理后台';
      switchTab(currentTab);
      loadCred();
    }).catch(function (err) {
      content.innerHTML = '<div class="card" style="padding:24px"><div class="empty">加载失败：' + esc(err.message || '未知错误') + '</div>' +
        '<div style="text-align:center;margin-top:12px"><a class="btn btn-ghost btn-sm" href="/admin/containers">返回容器列表</a></div></div>';
    });
  }

  function refreshOne() {
    ADMIN.request('/api/admin/cache/refresh?name=' + encodeURIComponent(name))
      .then(function () { load(); }).catch(function () { load(); });
  }

  function loadCred() {
    ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/credential').then(function (res) {
      setHash((res.data && res.data.hash) || '');
    }).catch(function () { setHash(''); });
  }

  // ---------- 初始化 ----------
  ADMIN.shell('containers', '容器详情', '查看与管理容器运行状态');
  content = document.getElementById('adminContent');

  name = getParam('name');
  if (!name) {
    content.innerHTML = '<div class="card" style="padding:24px"><div class="empty">缺少容器名称参数</div>' +
      '<div style="text-align:center;margin-top:12px"><a class="btn btn-ghost btn-sm" href="/admin/containers">返回容器列表</a></div></div>';
  } else {
    load();
  }
})();
