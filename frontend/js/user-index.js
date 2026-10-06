/* ============================================================
   LXD Panel - User 概览页
   GET /api/user/containers → data[] 统计容器 total/running/stopped
   GET /api/user/info       → data.user 配额 + data.stats 用量
   ============================================================ */
(function () {
  'use strict';

  USER.shell('index', '概览', '我的资源概览');

  const content = document.getElementById('userContent');

  const colors = { purple: '#a78bfa', green: '#34d399', gray: '#64748b', orange: '#fbbf24' };

  content.innerHTML =
    '<div class="stat-grid">' +
    statCard('purple', '▣', '容器总数', 'cTotal') +
    statCard('green', '▶', '运行中', 'cRunning') +
    statCard('gray', '■', '已停止', 'cStopped') +
    statCard('orange', '◔', '配额用量', 'quotaUse') +
    '</div>' +
    '<div class="grid-2">' +
    '  <div class="card" style="padding:16px">' +
    '    <div class="section-head"><h3>我的账户</h3></div>' +
    '    <div id="accInfo" style="font-size:13px;color:var(--text-2)">加载中...</div>' +
    '    <div id="quotaInfo" style="margin-top:14px;display:flex;flex-direction:column;gap:10px">加载中...</div>' +
    '  </div>' +
    '  <div class="card" style="padding:16px">' +
    '    <div class="section-head"><h3>快捷入口</h3></div>' +
    '    <div style="display:grid;gap:10px">' +
    quickLink('containers.html', '▤', '#38bdf8', '我的容器', '查看与管理我的 LXD 容器') +
    quickLink('port_mapping.html', '⇄', '#a78bfa', '端口映射', '查看我的端口映射规则') +
    quickLink('nginx.html', '⇌', '#34d399', '反向代理', '查看我的反向代理配置') +
    quickLink('templates.html', '▣', '#fbbf24', '模板', '浏览可用的系统模板') +
    quickLink('tasks.html', '◷', '#f87171', '任务', '查看我的后台任务状态') +
    quickLink('store.html', '◈', '#c084fc', '产品订购', '订购新容器或虚拟机套餐') +
    quickLink('instances.html', '▤', '#22d3ee', '我的实例', '续费、删除已购实例') +
    quickLink('orders.html', '◉', '#34d399', '我的订单', '订单列表与支付') +
    quickLink('tickets.html', '✉', '#fbbf24', '工单中心', '提交问题获取支持') +
    quickLink('messages.html', '☰', '#fb7185', '站内消息', '查看系统通知') +
    '    </div>' +
    '  </div>' +
    '</div>';

  function statCard(color, icon, name, id) {
    return '<div class="card stat-card">' +
      '<div class="stat-icon" style="background:rgba(255,255,255,.06);color:' + colors[color] + '">' + icon + '</div>' +
      '<div class="stat-val" id="' + id + '" style="font-size:24px;font-weight:700;letter-spacing:-.3px">-</div>' +
      '<div class="stat-name" style="font-size:12.5px;color:var(--text-3);font-weight:600">' + name + '</div></div>';
  }

  function quickLink(href, icon, color, title, desc) {
    return '<a href="' + href + '" class="card card-hover" style="display:flex;align-items:center;gap:12px;padding:12px 14px;text-decoration:none">' +
      '<div style="width:38px;height:38px;border-radius:10px;background:' + color + '22;border:1px solid ' + color + '55;display:flex;align-items:center;justify-content:center;color:' + color + ';font-size:17px">' + icon + '</div>' +
      '<div style="flex:1"><div style="font-size:13.5px;font-weight:600;color:var(--text-1)">' + title + '</div>' +
      '<div style="font-size:12px;color:var(--text-3);margin-top:2px">' + desc + '</div></div>' +
      '<span style="color:var(--text-3)">›</span></a>';
  }

  function pick(row, keys, def) {
    for (const k of keys) {
      if (row[k] !== undefined && row[k] !== null && row[k] !== '') return row[k];
    }
    return def;
  }

  function loadDashboard() {
    USER.request('/api/user/containers').then((res) => {
      const d = res.data;
      const list = (d && Array.isArray(d.data)) ? d.data : (Array.isArray(d) ? d : []);
      const total = list.length;
      const running = list.filter((it) => it.status === 'running').length;
      const stopped = list.filter((it) => it.status === 'stopped').length;
      set('cTotal', total);
      set('cRunning', running);
      set('cStopped', stopped);
    }).catch(() => { set('cTotal', '-'); set('cRunning', '-'); set('cStopped', '-'); set('quotaUse', '-'); });
  }

  function loadInfo() {
    USER.request('/api/user/info').then((res) => {
      const d = res.data || {};
      const u = d.user || {};
      const s = d.stats || {};
      const name = pick(u, ['username', 'Username', 'name'], '-');
      const email = pick(u, ['email', 'Email'], null);
      const role = pick(u, ['role', 'Role', 'level'], null);
      const created = pick(u, ['created_at', 'CreatedAt', 'create_time'], null);
      let html = kv('用户名', name);
      if (email) html += kv('邮箱', email);
      if (role) html += kv('角色', role);
      if (created != null) html += kv('注册时间', USER.fmt.time(created));
      document.getElementById('accInfo').innerHTML = html || USER.empty('暂无信息');

      const qInfo = document.getElementById('quotaInfo');
      if (qInfo) {
        const MB = 1024 * 1024;
        const GB = 1024 * 1024 * 1024;
        const cpuQ = u.cpu_quota != null ? u.cpu_quota : null;
        const memQ = u.memory_quota != null ? u.memory_quota : null;
        const diskQ = u.disk_quota != null ? u.disk_quota : null;
        const trafQ = u.traffic_limit != null ? u.traffic_limit : null;
        const usedMem = s.used_memory != null ? s.used_memory : null;
        const usedDisk = s.used_disk != null ? s.used_disk : null;
        const usedTraffic = s.used_traffic != null ? s.used_traffic : null;
        let qhtml = '';
        if (cpuQ != null) qhtml += bar('CPU 核心', cpuQ, '#38bdf8', cpuQ > 0 ? cpuQ + ' 核' : '未设置');
        if (memQ != null) qhtml += bar('内存', memQ, '#a78bfa', memQ > 0 ? (usedMem != null ? USER.fmt.bytes(usedMem * MB) + ' / ' : '') + USER.fmt.bytes(memQ * MB) : '未设置');
        if (diskQ != null) qhtml += bar('磁盘', diskQ, '#fbbf24', diskQ > 0 ? (usedDisk != null ? USER.fmt.bytes(usedDisk * MB) + ' / ' : '') + USER.fmt.bytes(diskQ * MB) : '未设置');
        if (trafQ != null) qhtml += bar('流量', trafQ, '#34d399', trafQ > 0 ? (usedTraffic != null ? USER.fmt.bytes(usedTraffic) + ' / ' : '') + USER.fmt.bytes(trafQ * GB) : '未设置');
        qInfo.innerHTML = qhtml || '<span style="color:var(--text-3)">暂无配额信息</span>';
        if (usedDisk != null) set('quotaUse', USER.fmt.bytes(usedDisk * MB) + (diskQ > 0 ? ' / ' + USER.fmt.bytes(diskQ * MB) : ''));
        else if (trafQ != null && usedTraffic != null) set('quotaUse', USER.fmt.bytes(usedTraffic));
      }
    }).catch(() => { document.getElementById('accInfo').innerHTML = '<span style="color:var(--red)">加载失败</span>'; });
  }

  function set(id, v) { const el = document.getElementById(id); if (el) el.textContent = v; }

  function kv(k, v) {
    return '<div style="display:flex;justify-content:space-between;padding:5px 0"><span style="color:var(--text-3)">' + k + '</span><span style="font-weight:500;color:var(--text-1)">' + USER.esc(v) + '</span></div>';
  }

  function bar(name, val, color, right) {
    if (val == null) return '';
    const p = parseFloat(val) || 0;
    const pct = Math.max(0, Math.min(100, p)).toFixed(1);
    let rightText = pct + '%';
    if (right != null) rightText = (typeof right === 'number') ? USER.fmt.bytes(right) : String(right);
    return '<div>' +
      '<div style="display:flex;justify-content:space-between;margin-bottom:6px">' +
      '<span style="font-size:12.5px;color:var(--text-2)">' + name + '</span>' +
      '<span style="font-size:12.5px;font-weight:600;color:' + color + '">' + rightText + '</span></div>' +
      '<div class="progress"><i style="width:' + pct + '%;background:' + color + '"></i></div></div>';
  }

  loadDashboard();
  loadInfo();
  setInterval(() => { loadDashboard(); }, 30000);
})();
