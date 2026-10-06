/* ============================================================
   LXD Panel - 仪表盘逻辑
   ============================================================ */
(function () {
  'use strict';

  // ---------- 状态 ----------
  let containerData = null;
  let containerName = '';
  let hash = LXD.getHash();
  let currentTab = 'info';
  let pollTimer = null;
  let resHistory = { cpu: [], mem: [], disk: [], traffic: [] };
  let ipPoolSettings = { allow_container_release_ipv4: false, allow_container_release_ipv6: false };
  let portRangeConfig = { v4_port_start: 1, v4_port_end: 65535, v6_port_start: 1, v6_port_end: 65535 };

  // ---------- 初始化 ----------
  function init() {
    if (!hash) { window.location.href = 'login.html'; return; }

    LXD.loadBrand().then((brand) => {
      LXD.applyBrand(brand);
      if (brand.footer_text) document.getElementById('footerText').textContent = brand.footer_text;
      if (brand.container_notice) showNotice(brand.container_notice);
    }).catch(() => {});

    // 事件绑定
    bindEvents();
    // 加载数据
    loadContainerInfo();
    // 标签页
    initTabs();
  }

  function showNotice(text) {
    const el = document.createElement('div');
    el.className = 'card';
    el.style.cssText = 'padding:14px 18px;margin-bottom:16px;border-color:rgba(56,189,248,.35);background:rgba(56,189,248,.07)';
    el.innerHTML = '<div style="display:flex;gap:10px;align-items:flex-start"><span style="flex-shrink:0">📢</span><span style="font-size:13.5px;color:var(--text-1);white-space:pre-wrap">' + LXD.esc(text) + '</span></div>';
    const main = document.querySelector('main.container');
    main.insertBefore(el, main.firstChild);
  }

  function bindEvents() {
    // 顶部
    document.getElementById('copyHashBtn').addEventListener('click', copyHash);
    document.getElementById('openConsoleBtn').addEventListener('click', openConsole);
    document.getElementById('fmBtn').addEventListener('click', () => { window.location.href = 'file-manager.html'; });
    document.getElementById('logoutBtn').addEventListener('click', () => {
      LXD.clearHash();
      window.location.href = 'login.html';
    });

    // 操作按钮
    document.getElementById('startBtn').addEventListener('click', () => confirmAction('start', '启动容器', '确定要启动容器吗？'));
    document.getElementById('stopBtn').addEventListener('click', () => confirmAction('stop', '停止容器', '确定要停止容器吗？'));
    document.getElementById('restartBtn').addEventListener('click', () => confirmAction('restart', '重启容器', '确定要重启容器吗？'));
    document.getElementById('reinstallBtn').addEventListener('click', openReinstallModal);
    document.getElementById('resetPwdBtn').addEventListener('click', () => LXD.openModal('resetPwdModal'));

    // 资源操作
    document.getElementById('allocIpv4Btn').addEventListener('click', () => openAllocIpModal('v4'));
    document.getElementById('allocIpv6Btn').addEventListener('click', () => openAllocIpModal('v6'));
    document.getElementById('addIpv4MappingBtn').addEventListener('click', () => openMappingModal('v4'));
    document.getElementById('addIpv6MappingBtn').addEventListener('click', () => openMappingModal('v6'));
    document.getElementById('addReverseProxyBtn').addEventListener('click', () => LXD.openModal('reverseProxyModal'));
    document.getElementById('saveDnsBtn').addEventListener('click', saveDNS);

    // 模态框关闭
    document.querySelectorAll('[data-close]').forEach((btn) => {
      btn.addEventListener('click', () => LXD.closeModal(btn.dataset.close));
    });
    document.querySelectorAll('.modal-mask').forEach((mask) => {
      mask.addEventListener('click', (e) => { if (e.target === mask) mask.classList.remove('show'); });
    });

    // 重装
    document.getElementById('reinstallConfirm').addEventListener('click', submitReinstall);
    // 重置密码
    document.getElementById('resetPwdConfirm').addEventListener('click', submitResetPwd);
    // 分配 IP
    document.getElementById('allocIpConfirm').addEventListener('click', submitAllocIp);
    // 端口映射
    document.getElementById('mappingConfirm').addEventListener('click', submitMapping);
    // 反代
    document.getElementById('rpSSLType').addEventListener('change', (e) => {
      document.getElementById('rpCustomFields').style.display = e.target.value === 'custom' ? 'block' : 'none';
    });
    document.getElementById('rpConfirm').addEventListener('click', submitReverseProxy);

    // 防抖 resize 重绘走势图
    window.addEventListener('resize', LXD.debounce(() => { renderResHistory(); }, 200));
  }

  function initTabs() {
    const tabs = document.querySelectorAll('.tab');
    tabs.forEach((tab) => {
      tab.addEventListener('click', () => {
        const name = tab.dataset.tab;
        if (tab.disabled) return;
        switchTab(name);
      });
    });
  }

  function switchTab(name) {
    currentTab = name;
    document.querySelectorAll('.tab').forEach((t) => {
      t.classList.toggle('active', t.dataset.tab === name);
    });
    document.querySelectorAll('.tab-panel').forEach((p) => {
      p.style.display = p.id === 'tab-' + name ? 'block' : 'none';
    });
    // 按需加载
    if (name === 'ipv4Pool') loadIpv4Pool();
    if (name === 'ipv6Pool') loadIpv6Pool();
    if (name === 'ipv4Mapping') loadIpv4Mapping();
    if (name === 'ipv6Mapping') loadIpv6Mapping();
    if (name === 'reverseProxy') loadReverseProxy();
    if (name === 'dns') loadDNS();
  }

  // ---------- 容器信息 ----------
  function loadContainerInfo() {
    LXD.authRequest('/api/container/info').then((res) => {
      if (res.code !== 200) throw new Error(res.msg || '获取容器信息失败');
      containerData = res.data;
      if (!containerData) { LXD.toast('error', '容器不存在'); return; }
      containerName = containerData.name || '';
      ipPoolSettings = containerData;
      if (containerData.v4_port_start) portRangeConfig.v4_port_start = containerData.v4_port_start;
      if (containerData.v4_port_end) portRangeConfig.v4_port_end = containerData.v4_port_end;
      if (containerData.v6_port_start) portRangeConfig.v6_port_start = containerData.v6_port_start;
      if (containerData.v6_port_end) portRangeConfig.v6_port_end = containerData.v6_port_end;
      renderHeader(containerData);
      renderInfoTab(containerData);
      renderResourceCards(containerData);
      updateTabVisibility(containerData);
    }).catch((err) => {
      if (LXD.handleAuthError(err)) return;
      LXD.toast('error', err.message || '加载失败');
    });
  }

  function renderHeader(c) {
    const name = c.name || '未知容器';
    document.getElementById('pageContainerName').innerHTML = LXD.esc(name) + ' ' + LXD.statusBadge(c.status);
    document.getElementById('topContainerName').textContent = name;
    document.getElementById('containerSub').textContent =
      (c.image || '未知镜像') + ' · 更新于 ' + new Date().toLocaleTimeString('zh-CN', { hour12: false });

    // 顶部访问码展示（点击可复制）
    const chip = document.getElementById('hashChip');
    if (chip) chip.textContent = c.hash || hash || '-';

    // 操作按钮可用性
    const running = (c.status || '').toLowerCase() === 'running';
    document.getElementById('startBtn').disabled = running;
    document.getElementById('stopBtn').disabled = !running;
    document.getElementById('restartBtn').disabled = !running;
  }

  function renderInfoTab(c) {
    document.getElementById('infoName').textContent = c.name || '-';
    document.getElementById('infoStatus').innerHTML = LXD.statusBadge(c.status);
    document.getElementById('infoImage').textContent = c.image || '-';
    document.getElementById('infoPassword').textContent = c.password || '-';
    document.getElementById('infoHash').textContent = c.hash || hash || '-';
    document.getElementById('infoIPv4').innerHTML = (c.ipv4 && c.ipv4.length) ? c.ipv4.map((ip) => '<code class="mono">' + LXD.esc(ip) + '</code>').join(' ') : '-';
    document.getElementById('infoIPv6').innerHTML = (c.ipv6 && c.ipv6.length) ? c.ipv6.map((ip) => '<code class="mono">' + LXD.esc(ip) + '</code>').join(' ') : '-';
    document.getElementById('infoQuota').textContent =
      (c.cpu ? c.cpu + ' 核 · ' : '') + (c.memory || '-') + ' 内存 · ' + (c.disk || '-') + ' 磁盘' +
      (c.traffic_limit ? ' · ' + c.traffic_limit + ' GB 流量' : ' · 不限流量');
  }

  // ---------- 资源卡片 ----------
  function renderResourceCards(c) {
    const cpuPct = c.cpu_usage || 0;
    const memUsed = c.memory_usage_raw || 0;
    const memTotal = LXD.parseSize(c.memory) || 0;
    const diskUsed = c.disk_usage_raw || 0;
    const diskTotal = LXD.parseSize(c.disk) || 0;
    const trafficUsedBytes = (c.traffic_usage_raw || 0) * 1024 * 1024 * 1024;
    const trafficLimitBytes = (c.traffic_limit || 0) * 1024 * 1024 * 1024;

    const memPct = memTotal ? (memUsed / memTotal * 100) : 0;
    const diskPct = diskTotal ? (diskUsed / diskTotal * 100) : 0;
    const trafficPct = trafficLimitBytes ? (trafficUsedBytes / trafficLimitBytes * 100) : 0;

    const svgDefs = '<svg width="0" height="0" style="position:absolute"><defs><linearGradient id="gradRing" x1="0%" y1="0%" x2="100%" y2="100%"><stop offset="0%" stop-color="#38bdf8"/><stop offset="100%" stop-color="#6366f1"/></linearGradient></defs></svg>';

    const ringHtml = (pct) =>
      '<div class="ring">' + svgDefs +
      '<svg viewBox="0 0 66 66"><circle class="ring-bg" cx="33" cy="33" r="28" fill="none" stroke-width="6"></circle><circle class="ring-fg" cx="33" cy="33" r="28" fill="none" stroke-width="6"></circle></svg>' +
      '<span class="ring-val">0%</span></div>';

    document.getElementById('resGrid').innerHTML =
      resCard('CPU 使用率', ringHtml(cpuPct), formatPct(cpuPct), (c.cpu || 1) + ' 核', 'cpu') +
      resCard('内存', ringHtml(memPct), LXD.fmtBytes(memUsed) + ' / ' + LXD.fmtBytes(memTotal), formatPct(memPct), 'mem') +
      resCard('磁盘', ringHtml(diskPct), LXD.fmtBytes(diskUsed) + ' / ' + LXD.fmtBytes(diskTotal), formatPct(diskPct), 'disk') +
      resCard('流量', ringHtml(trafficPct), LXD.fmtBytes(trafficUsedBytes) + (trafficLimitBytes ? ' / ' + LXD.fmtBytes(trafficLimitBytes) : ''), trafficLimitBytes ? formatPct(trafficPct) : '不限', 'traffic');

    // 填充环
    LXD.ring(document.querySelector('#resGrid .res-card:nth-child(1) .ring'), cpuPct, '#38bdf8');
    LXD.ring(document.querySelector('#resGrid .res-card:nth-child(2) .ring'), memPct, '#6366f1');
    LXD.ring(document.querySelector('#resGrid .res-card:nth-child(3) .ring'), diskPct, '#fbbf24');
    LXD.ring(document.querySelector('#resGrid .res-card:nth-child(4) .ring'), trafficPct, '#34d399');

    // 历史数据（用于走势图）
    pushRes('cpu', cpuPct); pushRes('mem', memPct); pushRes('disk', diskPct); pushRes('traffic', trafficPct);
  }

  function resCard(name, ringHtml, value, detail, id) {
    return '<div class="card res-card">' + ringHtml +
      '<div class="res-info"><div class="name">' + name + '</div>' +
      '<div class="val">' + value + '</div>' +
      '<div class="sub">' + detail + '</div>' +
      '<div class="spark"><canvas id="spark-' + id + '"></canvas></div></div></div>';
  }

  function formatPct(v) { return (Math.round(v * 10) / 10) + '%'; }

  function pushRes(key, val) {
    const arr = resHistory[key];
    arr.push(val);
    if (arr.length > 30) arr.shift();
    renderResHistory();
  }

  function renderResHistory() {
    const colors = { cpu: '#38bdf8', mem: '#6366f1', disk: '#fbbf24', traffic: '#34d399' };
    Object.keys(colors).forEach((key) => {
      const canvas = document.getElementById('spark-' + key);
      if (canvas) LXD.sparkline(canvas, resHistory[key], colors[key]);
    });
  }

  // ---------- 标签可见性 ----------
  function updateTabVisibility(data) {
    const limits = {
      ipv4Pool: data.ipv4_pool_limit || 0,
      ipv4Mapping: data.ipv4_mapping_limit || 0,
      ipv6Pool: data.ipv6_pool_limit || 0,
      ipv6Mapping: data.ipv6_mapping_limit || 0,
      reverseProxy: data.reverse_proxy_limit || 0
    };
    Object.entries(limits).forEach(([name, limit]) => {
      const tab = document.querySelector('.tab[data-tab="' + name + '"]');
      if (tab) {
        tab.style.display = limit > 0 ? '' : 'none';
      }
    });
    // 若当前标签被隐藏，切回 info
    if (limits[currentTab] === 0 && currentTab !== 'info' && currentTab !== 'dns') {
      switchTab('info');
    }
  }

  // ---------- IPv4/IPv6 池 ----------
  function loadIpv4Pool() {
    document.getElementById('ipv4PoolList').innerHTML = '<div class="empty">加载中...</div>';
    LXD.authRequest('/api/container/ip?version=v4').then((res) => {
      renderPool(document.getElementById('ipv4PoolList'), res.data.ipv4 || [], 'v4');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) document.getElementById('ipv4PoolList').innerHTML = '<div class="empty">加载失败</div>';
    });
  }

  function loadIpv6Pool() {
    document.getElementById('ipv6PoolList').innerHTML = '<div class="empty">加载中...</div>';
    LXD.authRequest('/api/container/ip?version=v6').then((res) => {
      renderPool(document.getElementById('ipv6PoolList'), res.data.ipv6 || [], 'v6');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) document.getElementById('ipv6PoolList').innerHTML = '<div class="empty">加载失败</div>';
    });
  }

  function renderPool(el, bindings, version) {
    const countEl = version === 'v4' ? document.getElementById('ipv4PoolCount') : document.getElementById('ipv6PoolCount');
    countEl.textContent = bindings.length;
    if (!bindings.length) { el.innerHTML = '<div class="empty">暂无地址绑定</div>'; return; }

    let html = '<div class="table-wrap"><table><thead><tr><th>地址</th><th>状态</th><th>分配时间</th><th style="text-align:right">操作</th></tr></thead><tbody>';
    bindings.forEach((b) => {
      const ip = b.IPAddress || b.ip_address;
      const status = b.Status || b.status || 'allocated';
      const time = LXD.fmtTime(b.CreatedAt || b.created_at);
      const bound = status === 'bound';
      const canRelease = version === 'v4' ? ipPoolSettings.allow_container_release_ipv4 : ipPoolSettings.allow_container_release_ipv6;
      html += '<tr>' +
        '<td><code class="mono">' + LXD.esc(ip) + '</code></td>' +
        '<td><span class="badge ' + (bound ? 'badge-running' : 'badge-other') + '">' + (bound ? '已绑定' : '已分配') + '</span></td>' +
        '<td style="color:var(--text-2)">' + time + '</td>' +
        '<td style="text-align:right">' + (canRelease
          ? '<button class="btn btn-danger btn-sm" onclick="LXD_PANEL.releaseIp(\'' + version + '\',\'' + encodeURIComponent(ip) + '\')">释放</button>'
          : '<span style="color:var(--text-3);font-size:12px">无权限</span>') + '</td>' +
        '</tr>';
    });
    html += '</tbody></table></div>';
    el.innerHTML = html;
  }

  function releaseIp(version, ipEncoded) {
    const ip = decodeURIComponent(ipEncoded);
    LXD.confirmDialog('释放地址', '确定要释放地址 ' + ip + ' 吗？').then((ok) => {
      if (!ok) return;
      LXD.authRequest('/api/container/ip/release?version=' + version, {
        method: 'POST', body: { ip }
      }).then((res) => {
        if (res.code === 200) {
          LXD.toast('success', '地址已释放');
          version === 'v4' ? loadIpv4Pool() : loadIpv6Pool();
          loadContainerInfo();
        } else LXD.toast('error', res.msg || '释放失败');
      }).catch((err) => {
        if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '释放失败');
      });
    });
  }

  function openAllocIpModal(version) {
    document.getElementById('allocIpTitle').textContent = version === 'v4' ? '分配 IPv4 地址' : '分配 IPv6 地址';
    document.getElementById('allocIpCount').value = '1';
    document.getElementById('allocIpModal').dataset.version = version;
    LXD.openModal('allocIpModal');
  }

  function submitAllocIp() {
    const version = document.getElementById('allocIpModal').dataset.version;
    const count = parseInt(document.getElementById('allocIpCount').value) || 1;
    LXD.authRequest('/api/container/ip/allocate?version=' + version, {
      method: 'POST', body: { count }
    }).then((res) => {
      if (res.code === 200) {
        LXD.toast('success', '地址分配成功');
        LXD.closeModal('allocIpModal');
        version === 'v4' ? loadIpv4Pool() : loadIpv6Pool();
        loadContainerInfo();
      } else LXD.toast('error', res.msg || '分配失败');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '分配失败');
    });
  }

  // ---------- 端口映射 ----------
  function loadIpv4Mapping() {
    document.getElementById('ipv4MappingList').innerHTML = '<div class="empty">加载中...</div>';
    LXD.authRequest('/api/container/port-mapping?version=v4').then((res) => {
      renderMapping(document.getElementById('ipv4MappingList'), res.data.ipv4 || [], 'v4');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) document.getElementById('ipv4MappingList').innerHTML = '<div class="empty">加载失败</div>';
    });
  }

  function loadIpv6Mapping() {
    document.getElementById('ipv6MappingList').innerHTML = '<div class="empty">加载中...</div>';
    LXD.authRequest('/api/container/port-mapping?version=v6').then((res) => {
      renderMapping(document.getElementById('ipv6MappingList'), res.data.ipv6 || [], 'v6');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) document.getElementById('ipv6MappingList').innerHTML = '<div class="empty">加载失败</div>';
    });
  }

  function renderMapping(el, mappings, version) {
    const countEl = version === 'v4' ? document.getElementById('ipv4MappingCount') : document.getElementById('ipv6MappingCount');

    // 按 公网端口-容器端口-协议 分组
    const groups = {};
    mappings.forEach((m) => {
      const key = m.public_port + '-' + m.container_port + '-' + m.protocol;
      if (!groups[key]) {
        groups[key] = { public_port: m.public_port, public_port_end: m.public_port_end, container_port: m.container_port, container_port_end: m.container_port_end, protocol: m.protocol, description: m.description, ips: [], ids: [] };
      }
      groups[key].ips.push({ ip: m.public_ip, interface: m.interface, status: m.status, created_at: m.created_at });
      groups[key].ids.push(m.id);
    });

    const groupArray = Object.values(groups);
    let totalPorts = 0;
    groupArray.forEach((g) => {
      totalPorts += (g.public_port_end && g.public_port_end !== g.public_port) ? (g.public_port_end - g.public_port + 1) : 1;
    });
    countEl.textContent = totalPorts;

    if (!groupArray.length) { el.innerHTML = '<div class="empty">暂无端口映射规则</div>'; return; }

    let html = '<div style="display:flex;flex-direction:column;gap:12px">';
    groupArray.forEach((g) => {
      const protoText = g.protocol === 'both' ? 'TCP/UDP' : (g.protocol || 'tcp').toUpperCase();
      const pubPort = (g.public_port_end && g.public_port_end !== g.public_port) ? g.public_port + '-' + g.public_port_end : g.public_port;
      const conPort = (g.container_port_end && g.container_port_end !== g.container_port) ? g.container_port + '-' + g.container_port_end : g.container_port;
      const isRange = g.public_port_end && g.public_port_end !== g.public_port;
      const idsStr = g.ids.join(',');
      html += '<div class="card" style="padding:14px 16px">' +
        '<div style="display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap">' +
        '<div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">' +
        '<span style="font-family:var(--mono);font-size:15px;font-weight:700">' + pubPort + ' → ' + conPort + '</span>' +
        '<span class="badge badge-other">' + protoText + '</span>' +
        (isRange ? '<span class="badge" style="color:var(--purple);background:rgba(167,139,250,.12);border-color:rgba(167,139,250,.3)">' + (g.public_port_end - g.public_port + 1) + ' 个端口</span>' : '') +
        '<span style="font-size:12px;color:var(--text-3)">' + g.ips.length + ' 个公网 IP</span>' +
        '</div>' +
        '<button class="btn btn-danger btn-sm" onclick="LXD_PANEL.deleteMapping(\'' + version + '\',\'' + idsStr + '\')">删除</button>' +
        '</div>' +
        (g.description ? '<div style="font-size:12.5px;color:var(--text-2);margin-top:8px">' + LXD.esc(g.description) + '</div>' : '') +
        (g.ips.length ? '<div style="margin-top:10px"><details><summary style="cursor:pointer;font-size:12.5px;color:var(--primary)">查看关联公网 IP</summary><div style="margin-top:8px;display:flex;flex-direction:column;gap:6px">' +
        g.ips.map((ip) => '<div style="display:flex;align-items:center;gap:8px;font-size:12.5px"><span style="width:7px;height:7px;border-radius:50%;background:var(--green)"></span><code class="mono">' + LXD.esc(ip.ip) + '</code><span style="color:var(--text-3)">' + LXD.esc(ip.interface || '') + '</span></div>').join('') +
        '</div></details></div>' : '') +
        '</div>';
    });
    html += '</div>';
    el.innerHTML = html;
  }

  function deleteMapping(version, idsStr) {
    const ids = idsStr.split(',').map((id) => parseInt(id));
    LXD.confirmDialog('删除映射', '确定要删除这 ' + ids.length + ' 条端口映射规则吗？').then((ok) => {
      if (!ok) return;
      const reqs = ids.map((id) =>
        LXD.authRequest('/api/container/port-mapping/release?version=' + version, { method: 'POST', body: { ids: [id] } })
      );
      Promise.all(reqs).then(() => {
        LXD.toast('success', '映射规则已删除');
        version === 'v4' ? loadIpv4Mapping() : loadIpv6Mapping();
      }).catch((err) => {
        if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '删除失败');
      });
    });
  }

  function openMappingModal(version) {
    document.getElementById('mappingTitle').textContent = version === 'v4' ? '添加 IPv4 端口映射' : '添加 IPv6 端口映射';
    document.getElementById('mappingModal').dataset.version = version;
    const range = version === 'v4' ? portRangeConfig : portRangeConfig;
    document.getElementById('mappingPublicPort').placeholder = '如 8080 或 ' + range.v4_port_start + '-' + range.v4_port_end;
    document.getElementById('mappingPublicPort').value = '';
    document.getElementById('mappingContainerPort').value = '';
    document.getElementById('mappingDesc').value = '';
    document.getElementById('mappingProtocol').value = 'tcp';
    LXD.openModal('mappingModal');
  }

  function submitMapping() {
    const version = document.getElementById('mappingModal').dataset.version;
    const publicPortVal = document.getElementById('mappingPublicPort').value.trim();
    const containerPort = parseInt(document.getElementById('mappingContainerPort').value);
    const description = document.getElementById('mappingDesc').value.trim();
    if (!containerPort) return LXD.toast('warning', '请填写容器端口');

    const data = {
      public_port: publicPortVal ? parseInt(publicPortVal) : 0,
      container_port: containerPort,
      port_count: 1,
      description: description
    };
    LXD.authRequest('/api/container/port-mapping/allocate?version=' + version, {
      method: 'POST', body: data
    }).then((res) => {
      if (res.code === 200) {
        LXD.toast('success', '端口映射添加成功');
        LXD.closeModal('mappingModal');
        version === 'v4' ? loadIpv4Mapping() : loadIpv6Mapping();
      } else LXD.toast('error', res.msg || '添加失败');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '添加失败');
    });
  }

  // ---------- 反向代理 ----------
  function loadReverseProxy() {
    document.getElementById('reverseProxyList').innerHTML = '<div class="empty">加载中...</div>';
    LXD.authRequest('/api/container/nginx/proxies').then((res) => {
      renderReverseProxy(res.data || []);
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) document.getElementById('reverseProxyList').innerHTML = '<div class="empty">加载失败</div>';
    });
  }

  function renderReverseProxy(rules) {
    document.getElementById('reverseProxyCount').textContent = rules.length;
    if (!rules.length) { document.getElementById('reverseProxyList').innerHTML = '<div class="empty">暂无反向代理规则</div>'; return; }

    let html = '<div class="table-wrap"><table><thead><tr><th>域名</th><th>协议</th><th>目标端口</th><th>SSL</th><th>状态</th><th style="text-align:right">操作</th></tr></thead><tbody>';
    rules.forEach((r) => {
      const isHttps = (r.protocol || 'http') === 'https';
      const sslOn = !!r.enable_ssl;
      const active = (r.status || 'active') === 'active';
      html += '<tr>' +
        '<td><code class="mono">' + LXD.esc(r.domain) + '</code></td>' +
        '<td><span class="badge ' + (isHttps ? 'badge-running' : 'badge-other') + '">' + (isHttps ? 'HTTPS' : 'HTTP') + '</span></td>' +
        '<td>' + (r.target_port || '-') + '</td>' +
        '<td><span class="badge ' + (sslOn ? 'badge-running' : 'badge-stopped') + '">' + (sslOn ? '已启用' : '未启用') + '</span></td>' +
        '<td><span class="badge ' + (active ? 'badge-running' : 'badge-stopped') + '">' + (active ? '生效中' : '停用') + '</span></td>' +
        '<td style="text-align:right"><button class="btn btn-danger btn-sm" onclick="LXD_PANEL.deleteReverseProxy(' + r.id + ')">删除</button></td>' +
        '</tr>' +
        (r.description ? '<tr style="background:rgba(255,255,255,.02)"><td colspan="6" style="font-size:12px;color:var(--text-3);padding-top:0">备注：' + LXD.esc(r.description) + '</td></tr>' : '');
    });
    html += '</tbody></table></div>';
    document.getElementById('reverseProxyList').innerHTML = html;
  }

  function deleteReverseProxy(id) {
    LXD.confirmDialog('删除反代规则', '确定要删除此反向代理规则吗？').then((ok) => {
      if (!ok) return;
      LXD.authRequest('/api/container/nginx/proxies/' + id, { method: 'DELETE' }).then((res) => {
        if (res.code === 200) {
          LXD.toast('success', '反代规则已删除');
          loadReverseProxy();
        } else LXD.toast('error', res.msg || '删除失败');
      }).catch((err) => {
        if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '删除失败');
      });
    });
  }

  function submitReverseProxy() {
    const sslType = document.getElementById('rpSSLType').value;
    const sslEnabled = sslType !== 'none';
    const domain = document.getElementById('rpDomain').value.trim();
    const targetPort = parseInt(document.getElementById('rpTargetPort').value);
    const description = document.getElementById('rpDesc').value.trim();

    if (!domain) return LXD.toast('warning', '请填写域名');
    if (!targetPort) return LXD.toast('warning', '请填写目标端口');
    if (sslEnabled && sslType === 'custom') {
      const cert = document.getElementById('rpSSLCert').value.trim();
      const key = document.getElementById('rpSSLKey').value.trim();
      if (!cert || !key) return LXD.toast('warning', '自定义 SSL 需提供证书和私钥');
    }

    const data = {
      container_name: containerName,
      protocol: sslEnabled ? 'https' : 'http',
      domain: domain,
      target_port: targetPort,
      description: description,
      enable_ssl: sslEnabled,
      status: 'active'
    };
    if (sslEnabled && sslType === 'custom') {
      data.ssl_cert = document.getElementById('rpSSLCert').value;
      data.ssl_key = document.getElementById('rpSSLKey').value;
    }

    LXD.authRequest('/api/container/nginx/proxies', { method: 'POST', body: data }).then((res) => {
      if (res.code === 200) {
        LXD.toast('success', '反代规则添加成功');
        LXD.closeModal('reverseProxyModal');
        document.getElementById('rpCustomFields').style.display = 'none';
        loadReverseProxy();
      } else LXD.toast('error', res.msg || '添加失败');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '添加失败');
    });
  }

  // ---------- DNS ----------
  function loadDNS() {
    LXD.authRequest('/api/container/dns').then((res) => {
      document.getElementById('dnsInput').value = (res.data && res.data.dns) ? res.data.dns.join('\n') : '';
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) document.getElementById('dnsInput').value = '';
    });
  }

  function saveDNS() {
    const text = document.getElementById('dnsInput').value.trim();
    if (!text) return LXD.toast('warning', 'DNS 服务器不能为空');
    const dns = text.split('\n').map((s) => s.trim()).filter(Boolean);
    LXD.authRequest('/api/container/dns', { method: 'PUT', body: { dns } }).then((res) => {
      if (res.code === 200) LXD.toast('success', 'DNS 设置已保存');
      else LXD.toast('error', res.msg || '保存失败');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '保存失败');
    });
  }

  // ---------- 容器操作 ----------
  function confirmAction(action, title, text) {
    LXD.confirmDialog(title, text).then((ok) => {
      if (!ok) return;
      performAction(action, title.replace('容器', ''));
    });
  }

  function performAction(action, actionName) {
    const btn = document.getElementById(action === 'start' ? 'startBtn' : action === 'stop' ? 'stopBtn' : 'restartBtn');
    btn.disabled = true;
    const old = btn.textContent;
    btn.textContent = '执行中...';

    LXD.authRequest('/api/container/action?action=' + action, { method: 'POST', body: {} }).then((res) => {
      if (res.code === 200) {
        LXD.toast('success', actionName + '指令已提交');
        if (res.data && res.data.task_id) {
          pollTask(res.data.task_id, actionName);
        } else {
          setTimeout(loadContainerInfo, 1500);
        }
      } else LXD.toast('error', res.msg || actionName + '失败');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || actionName + '失败');
    }).finally(() => {
      btn.disabled = false;
      btn.textContent = old;
    });
  }

  function pollTask(taskId, actionName) {
    if (pollTimer) clearInterval(pollTimer);
    let polls = 0;
    pollTimer = setInterval(() => {
      polls++;
      LXD.authRequest('/api/admin/tasks/' + taskId).then((res) => {
        const status = res.data && res.data.status;
        if (status === 'success' || status === 'completed') {
          clearInterval(pollTimer);
          LXD.toast('success', actionName + '成功');
          loadContainerInfo();
        } else if (status === 'failed' || status === 'error') {
          clearInterval(pollTimer);
          LXD.toast('error', actionName + '失败：' + ((res.data && res.data.error_msg) || '未知错误'));
          loadContainerInfo();
        } else if (polls >= 60) {
          clearInterval(pollTimer);
          LXD.toast('warning', '操作仍在执行，请稍后刷新查看');
          loadContainerInfo();
        }
      }).catch(() => {
        if (polls >= 60) { clearInterval(pollTimer); loadContainerInfo(); }
      });
    }, 2000);
  }

  // ---------- 重装 ----------
  function openReinstallModal() {
    const select = document.getElementById('reinstallImage');
    select.innerHTML = '<option value="">加载中...</option>';
    LXD.openModal('reinstallModal');
    LXD.authRequest('/api/container/templates').then((res) => {
      const templates = res.data && res.data.templates ? res.data.templates : (Array.isArray(res.data) ? res.data : []);
      if (!templates.length) {
        select.innerHTML = '<option value="">暂无可用镜像</option>';
        return;
      }
      select.innerHTML = templates.map((t) => {
        const name = t.alias || t.fingerprint || t;
        return '<option value="' + LXD.esc(name) + '">' + LXD.esc(name) + '</option>';
      }).join('');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) select.innerHTML = '<option value="">镜像加载失败</option>';
    });
    document.getElementById('reinstallPassword').value = '';
  }

  function submitReinstall() {
    const image = document.getElementById('reinstallImage').value;
    const password = document.getElementById('reinstallPassword').value.trim();
    if (!image) return LXD.toast('warning', '请选择镜像');
    if (!password) return LXD.toast('warning', '请设置新密码');

    LXD.confirmDialog('确认重装', '重装将清空容器数据且不可恢复，确定继续吗？').then((ok) => {
      if (!ok) return;
      LXD.authRequest('/api/container/action?action=reinstall', {
        method: 'POST', body: { image, password }
      }).then((res) => {
        if (res.code === 200) {
          LXD.toast('success', '重装任务已提交');
          LXD.closeModal('reinstallModal');
          if (res.data && res.data.task_id) pollTask(res.data.task_id, '重装');
          else setTimeout(loadContainerInfo, 1500);
        } else LXD.toast('error', res.msg || '提交失败');
      }).catch((err) => {
        if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '提交失败');
      });
    });
  }

  // ---------- 重置密码 ----------
  function submitResetPwd() {
    const password = document.getElementById('resetPwdInput').value.trim();
    if (!password) return LXD.toast('warning', '请输入新密码');
    LXD.authRequest('/api/container/action?action=reset-password', {
      method: 'POST', body: { password }
    }).then((res) => {
      if (res.code === 200) {
        LXD.toast('success', '密码重置任务已提交');
        LXD.closeModal('resetPwdModal');
        document.getElementById('resetPwdInput').value = '';
        if (res.data && res.data.task_id) pollTask(res.data.task_id, '重置密码');
        else setTimeout(loadContainerInfo, 1500);
      } else LXD.toast('error', res.msg || '提交失败');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '提交失败');
    });
  }

  // ---------- 控制台 ----------
  function openConsole() {
    if (!containerName) return LXD.toast('warning', '容器信息未加载');
    LXD.authRequest('/api/container/console/create-token', {
      method: 'POST', body: { hostname: containerName }
    }).then((res) => {
      if (res.code === 200 && res.data && res.data.token) {
        const host = (containerData && containerData.ipv4 && containerData.ipv4.length) ? containerData.ipv4[0] : '';
        const pwd = (containerData && containerData.password) ? containerData.password : '';
        let qs = 'console.html?token=' + encodeURIComponent(res.data.token);
        if (host) qs += '&host=' + encodeURIComponent(host) + '&user=root';
        if (pwd) qs += '&pwd=' + encodeURIComponent(pwd);
        window.open(qs, '_blank');
      } else LXD.toast('error', res.msg || '创建控制台令牌失败');
    }).catch((err) => {
      if (!LXD.handleAuthError(err)) LXD.toast('error', err.message || '打开控制台失败');
    });
  }

  // ---------- Hash 复制 ----------
  function copyHash() {
    const h = containerData && containerData.hash ? containerData.hash : hash;
    if (!h) return LXD.toast('warning', '访问码不可用');
    const fallback = () => {
      const ta = document.createElement('textarea');
      ta.value = h;
      ta.style.cssText = 'position:fixed;left:-9999px';
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand('copy'); LXD.toast('success', '访问码已复制'); } catch (e) { LXD.toast('error', '复制失败'); }
      ta.remove();
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(h).then(() => LXD.toast('success', '访问码已复制')).catch(fallback);
    } else fallback();
  }

  // ---------- 导出给内联 onclick 使用 ----------
  window.LXD_PANEL = {
    releaseIp: releaseIp,
    deleteMapping: deleteMapping,
    deleteReverseProxy: deleteReverseProxy
  };

  // 定时刷新容器信息
  setInterval(() => { if (!document.hidden) loadContainerInfo(); }, 15000);

  init();
})();
