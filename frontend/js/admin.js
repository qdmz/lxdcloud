/* ============================================================
   LXD Panel - Admin 管理端公共库
   - ADMIN.shell() 渲染侧边栏+顶栏+内容容器
   - ADMIN.request() 请求封装（同源 Cookie 会话，401 跳登录）
   ============================================================ */
(function () {
  'use strict';

  const ICONS = {
    dashboard: '◈', containers: '▤', templates: '▣', storage: '◫',
    users: '◉', tasks: '◷', brand: '✎', firewall: '◬', nginx: '⛨',
    ip: '◍', port: '⇄', menu: '☰', refresh: '↻', logout: '⎋', chevron: '▾',
    nodes: '⌘', products: '◈', orders: '◉', instances: '▣', tickets: '✉', mail: '✉', pay: '¤'
  };

  const MENUS = [
    { label: '概览', items: [{ id: 'dashboard', text: '仪表盘', href: 'dashboard.html', icon: ICONS.dashboard }] },
    {
      label: '商业化', items: [
        { id: 'nodes', text: '节点管理', href: 'nodes.html', icon: ICONS.nodes },
        { id: 'products', text: '商品管理', href: 'products.html', icon: ICONS.products },
        { id: 'orders', text: '订单管理', href: 'orders.html', icon: ICONS.orders },
        { id: 'instances', text: '实例管理', href: 'instances.html', icon: ICONS.instances },
        { id: 'tickets', text: '工单管理', href: 'tickets.html', icon: ICONS.tickets },
        { id: 'mail', text: '邮件配置', href: 'mail.html', icon: ICONS.mail },
        { id: 'pay', text: '支付配置', href: 'pay.html', icon: ICONS.pay }
      ]
    },
    {
      label: '容器', items: [
        { id: 'containers', text: '容器管理', href: 'containers.html', icon: ICONS.containers },
        { id: 'templates', text: '模板管理', href: 'templates.html', icon: ICONS.templates },
        { id: 'storage_pools', text: '存储池', href: 'storage_pools.html', icon: ICONS.storage }
      ]
    },
    { label: '用户', items: [{ id: 'users', text: '用户管理', href: 'users.html', icon: ICONS.users }] },
    {
      label: '网络', items: [
        { id: 'ip_pool_v4', text: 'IPv4 地址池', href: 'ip_pool_v4.html', icon: ICONS.ip },
        { id: 'ip_pool_v6', text: 'IPv6 地址池', href: 'ip_pool_v6.html', icon: ICONS.ip },
        { id: 'port_mapping_v4', text: '端口映射 IPv4', href: 'port_mapping_v4.html', icon: ICONS.port },
        { id: 'port_mapping_v6', text: '端口映射 IPv6', href: 'port_mapping_v6.html', icon: ICONS.port },
        { id: 'nginx', text: '反向代理', href: 'nginx.html', icon: ICONS.nginx }
      ]
    },
    {
      label: '系统', items: [
        { id: 'tasks', text: '任务管理', href: 'tasks.html', icon: ICONS.tasks },
        { id: 'brand_settings', text: '品牌设置', href: 'brand_settings.html', icon: ICONS.brand },
        { id: 'firewall', text: '防火墙', href: 'firewall.html', icon: ICONS.firewall }
      ]
    }
  ];

  /** 渲染管理端外壳 */
  function shell(active, title, sub) {
    const app = document.getElementById('admin-app');
    if (!app) return;

    const menuHtml = MENUS.map((group) => {
      const items = group.items.map((it) => {
        const cls = 'menu-item' + (it.id === active ? ' active' : '');
        return '<a class="' + cls + '" href="' + it.href + '">' +
          '<span class="mi-icon">' + it.icon + '</span>' + it.text + '</a>';
      }).join('');
      return '<div class="menu-group-label">' + group.label + '</div>' + items;
    }).join('');

    app.innerHTML =
      '<div class="admin-shell">' +
      '<div class="side-overlay" id="sideOverlay"></div>' +
      '<aside class="admin-sidebar" id="adminSidebar">' +
      '  <div class="side-brand"><div class="brand-logo" id="brandLogo">L</div>' +
      '    <div><div class="side-brand-name" id="brandName">LXD 管理后台</div><div class="side-brand-sub">Admin Panel</div></div>' +
      '  </div>' +
      '  <nav class="side-nav">' + menuHtml + '</nav>' +
      '  <div class="side-foot">' +
      '    <div class="brand-logo" id="footLogo" style="width:26px;height:26px;font-size:12px">A</div>' +
      '    <div style="flex:1;min-width:0"><div style="font-size:12.5px;font-weight:600;color:var(--text-1)">管理员</div>' +
      '      <div style="font-size:11px;color:var(--text-3)">LXD Panel</div></div>' +
      '    <button class="btn btn-ghost btn-sm" id="adminLogout" title="退出登录">' + ICONS.logout + '</button>' +
      '  </div>' +
      '</aside>' +
      '<div class="admin-main">' +
      '  <header class="admin-topbar">' +
      '    <button class="btn btn-ghost btn-sm btn-menu" id="btnMenu">' + ICONS.menu + '</button>' +
      '    <div class="topbar-title">' + (title || '') + (sub ? '<div class="sub">' + sub + '</div>' : '') + '</div>' +
      '    <span class="topbar-spacer"></span>' +
      '    <button class="btn btn-ghost btn-sm" id="btnRefresh" title="刷新">' + ICONS.refresh + '</button>' +
      '  </header>' +
      '  <main class="admin-content" id="adminContent"></main>' +
      '</div></div>';

    // 移动端菜单
    document.getElementById('btnMenu').addEventListener('click', () => {
      document.getElementById('adminSidebar').classList.add('open');
      document.getElementById('sideOverlay').classList.add('show');
    });
    document.getElementById('sideOverlay').addEventListener('click', () => {
      document.getElementById('adminSidebar').classList.remove('open');
      document.getElementById('sideOverlay').classList.remove('show');
    });
    document.getElementById('adminLogout').addEventListener('click', ADMIN.logout);
    document.getElementById('btnRefresh').addEventListener('click', () => window.location.reload());

    // 品牌
    LXD.loadBrand().then((b) => {
      LXD.applyBrand(b);
      document.title = (b.site_name || 'LXD 管理后台') + ' - ' + (title || '');
      const fn = document.getElementById('footLogo');
      if (fn) fn.textContent = (b.site_name || 'A').charAt(0).toUpperCase();
    }).catch(() => {});
  }

  /** 请求封装（同源 Cookie 会话；返回 data 字段） */
  function request(url, options = {}) {
    options.method = options.method || 'GET';
    options.headers = Object.assign({ 'Content-Type': 'application/json' }, options.headers || {});
    if (options.body && typeof options.body !== 'string') options.body = JSON.stringify(options.body);
    return fetch(url, options).then(async (res) => {
      if (res.status === 401 || res.status === 403) {
        ADMIN.toast('error', '登录已失效，请重新登录');
        setTimeout(() => { window.location.href = 'login.html'; }, 800);
        throw new Error('unauthorized');
      }
      const data = await res.json().catch(() => ({}));
      if (!res.ok || (data.code !== undefined && data.code !== 200)) {
        const err = new Error(data.msg || data.message || '请求失败');
        err.status = res.status; err.data = data;
        throw err;
      }
      return data;
    });
  }

  function logout() {
    fetch('/api/admin/logout', { method: 'POST' }).catch(() => {});
    setTimeout(() => { window.location.href = 'login.html'; }, 200);
  }

  /** 便捷工具 */
  const fmt = {
    bytes: LXD.fmtBytes,
    time: LXD.fmtTime,
    pct: (v) => (Math.round(v * 10) / 10) + '%'
  };

  /** 表格空态 */
  function empty(text) {
    return '<div class="empty"><div class="empty-icon">▫</div>' + LXD.esc(text || '暂无数据') + '</div>';
  }

  window.ADMIN = { shell, request, logout, fmt, empty, toast: LXD.toast, esc: LXD.esc, MENUS, ICONS };
})();
