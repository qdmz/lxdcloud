/* ============================================================
   LXD Panel - Admin 商业化仪表盘
   GET /api/admin/dashboard → users/nodes/orders/paid_orders/income/products/tickets/open_tickets/active_products
   GET /api/admin/host/stats → cpu/memory/disks/load/network
   ============================================================ */
(function () {
  'use strict';

  ADMIN.shell('dashboard', '仪表盘', '商业化概览与系统状态');

  const content = document.getElementById('adminContent');
  const colors = { purple: '#a78bfa', green: '#34d399', gray: '#64748b', orange: '#fbbf24', blue: '#38bdf8', red: '#f87171' };

  content.innerHTML =
    '<div class="stat-grid">' +
    statCard('purple', '◉', '注册用户', 'userCount') +
    statCard('green', '¤', '累计收入(元)', 'incomeVal') +
    statCard('blue', '◉', '已支付订单', 'paidOrders') +
    statCard('orange', '▤', '运行中实例', 'activeInstances') +
    statCard('cyan', '▣', '容器总数', 'containerTotal') +
    statCard('teal', '▶', '运行中容器', 'containerRunning') +
    statCard('gray', '⌘', '节点数', 'nodeCount') +
    statCard('red', '✉', '待处理工单', 'openTickets') +
    '</div>' +
    '<div class="grid-2">' +
    '  <div class="card" style="padding:16px">' +
    '    <div class="section-head"><h3>经营数据</h3></div>' +
    '    <div id="bizInfo" style="font-size:13px;color:var(--text-2)">加载中...</div>' +
    '  </div>' +
    '  <div class="card" style="padding:16px">' +
    '    <div class="section-head"><h3>主机资源</h3></div>' +
    '    <div id="hostResources" style="display:flex;flex-direction:column;gap:10px">加载中...</div>' +
    '  </div>' +
    '</div>' +
    '<div class="card" style="padding:16px;margin-top:14px">' +
    '  <div class="section-head"><h3>快捷操作</h3></div>' +
    '  <div style="display:grid;gap:10px;grid-template-columns:repeat(auto-fill,minmax(240px,1fr))">' +
    quickLink('products.html', '◈', '#38bdf8', '商品管理', '新增套餐、定价、上下架') +
    quickLink('nodes.html', '⌘', '#a78bfa', '节点管理', '接入子节点并检测连通') +
    quickLink('orders.html', '◉', '#34d399', '订单管理', '查看全站订单与收入') +
    quickLink('tickets.html', '✉', '#fbbf24', '工单管理', '处理用户工单') +
    quickLink('mail.html', '✉', '#f87171', '邮件配置', 'SMTP 与模板') +
    quickLink('pay.html', '¤', '#22d3ee', '支付配置', '易支付网关设置') +
    '  </div>' +
    '</div>';

  function statCard(color, icon, name, id) {
    return '<div class="card stat-card">' +
      '<div class="stat-icon" style="background:rgba(255,255,255,.06);color:' + colors[color] + '">' + icon + '</div>' +
      '<div class="stat-val" id="' + id + '">-</div>' +
      '<div class="stat-name">' + name + '</div></div>';
  }

  function quickLink(href, icon, color, name, desc) {
    return '<a href="' + href + '" style="display:flex;align-items:center;gap:10px;padding:12px;border-radius:12px;background:rgba(255,255,255,.045);color:var(--text-1);text-decoration:none">' +
      '<span style="width:34px;height:34px;border-radius:9px;display:flex;align-items:center;justify-content:center;background:rgba(255,255,255,.06);color:' + color + ';font-size:16px">' + icon + '</span>' +
      '<span><b style="font-size:13.5px">' + name + '</b><div style="font-size:11.5px;color:var(--text-3)">' + desc + '</div></span></a>';
  }

  function fmtMoney(v) { return Number(v || 0).toFixed(2); }

  // 经营数据
  ADMIN.request('/api/admin/dashboard').then(res => {
    const d = res.data || {};
    document.getElementById('userCount').textContent = d.users || 0;
    document.getElementById('incomeVal').textContent = fmtMoney(d.income);
    document.getElementById('paidOrders').textContent = d.paid_orders || 0;
    document.getElementById('activeInstances').textContent = d.active_products || 0;
    const ct = d.containers || {};
    document.getElementById('containerTotal').textContent = ct.total || 0;
    document.getElementById('containerRunning').textContent = ct.running || 0;
    document.getElementById('nodeCount').textContent = d.nodes || 0;
    document.getElementById('openTickets').textContent = d.open_tickets || 0;
    document.getElementById('bizInfo').innerHTML =
      '<div style="display:grid;gap:8px">' +
      '<div>商品总数：<b>' + (d.products || 0) + '</b></div>' +
      '<div>全部订单：<b>' + (d.orders || 0) + '</b>（已支付 ' + (d.paid_orders || 0) + '）</div>' +
      '<div>工单总数：<b>' + (d.tickets || 0) + '</b>（待处理 ' + (d.open_tickets || 0) + '）</div>' +
      '<div>激活实例：<b>' + (d.active_products || 0) + '</b></div>' +
      '</div>';
  }).catch(() => {});

  // 主机资源
  ADMIN.request('/api/admin/host/stats').then(res => {
    const d = res.data || {};
    const mem = d.memory || {};
    const memPct = mem.total ? Math.round(mem.used / mem.total * 100) : 0;
    const cpuPct = (typeof d.cpu === 'number') ? Math.round(d.cpu) : (d.cpu_percent || 0);
    const disks = (d.disks || []).map(dk => {
      const pct = dk.total ? Math.round(dk.used / dk.total * 100) : 0;
      return '<div style="display:flex;justify-content:space-between;font-size:12.5px"><span>' + LXD.esc(dk.mount || '') + '</span><span>' + fmtGB(dk.used) + ' / ' + fmtGB(dk.total) + ' (' + pct + '%)</span></div>';
    }).join('');
    document.getElementById('hostResources').innerHTML =
      '<div style="display:flex;justify-content:space-between;font-size:12.5px"><span>CPU</span><b>' + cpuPct + '%</b></div>' +
      '<div style="display:flex;justify-content:space-between;font-size:12.5px"><span>内存</span><b>' + fmtGB(mem.used) + ' / ' + fmtGB(mem.total) + ' (' + memPct + '%)</b></div>' +
      (disks || '<div style="font-size:12.5px;color:var(--text-3)">磁盘信息不可用</div>');
  }).catch(() => {});

  function fmtGB(b) {
    if (b === undefined || b === null) return '-';
    const g = b / 1024 / 1024 / 1024;
    return g >= 1024 ? (g / 1024).toFixed(1) + ' TB' : g.toFixed(1) + ' GB';
  }
})();
