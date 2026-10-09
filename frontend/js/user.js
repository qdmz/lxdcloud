/* ============================================================
   LXD Panel - User 用户端公共库
   - USER.shell() 渲染顶部导航+品牌+内容容器
   - USER.request() 请求封装（同源 Cookie 会话，401 跳登录）
   ============================================================ */
(function () {
  'use strict';

  const NAVS = [
    { id: 'index', text: '概览', href: 'index.html' },
    { id: 'store', text: '产品订购', href: 'store.html' },
    { id: 'instances', text: '我的实例', href: 'instances.html' },
    { id: 'orders', text: '我的订单', href: 'orders.html' },
    { id: 'containers', text: '我的容器', href: 'containers.html' },
    { id: 'port_mapping', text: '端口映射', href: 'port_mapping.html' },
    { id: 'nginx', text: '反向代理', href: 'nginx.html' },
    { id: 'templates', text: '模板', href: 'templates.html' },
    { id: 'tasks', text: '任务', href: 'tasks.html' },
    { id: 'tickets', text: '工单中心', href: 'tickets.html' },
    { id: 'messages', text: '站内消息', href: 'messages.html' },
    { id: 'profile', text: '个人资料', href: 'profile.html' }
  ];

  /** 渲染用户端外壳 */
  function shell(active, title, sub) {
    const app = document.getElementById('user-app');
    if (!app) return;

    const navHtml = NAVS.map((it) => {
      const cls = 'u-nav-item' + (it.id === active ? ' active' : '');
      return '<a class="' + cls + '" href="' + it.href + '">' + it.text + '</a>';
    }).join('');

    app.innerHTML =
      '<div class="user-shell">' +
      '<header class="user-topbar">' +
      '  <div class="brand-logo" id="brandLogo">L</div>' +
      '  <div><div class="user-topbar-name" id="brandName">LXD 容器面板</div>' +
      '    <div class="user-topbar-sub" id="userInfo">用户中心</div></div>' +
      '  <span class="u-spacer"></span>' +
      '  <div class="u-actions">' +
      '    <button class="btn btn-ghost btn-sm" id="btnLogout" title="退出登录">⎋ <span class="u-logout-text">退出</span></button>' +
      '    <button class="btn btn-ghost btn-sm u-menu-btn" id="btnUMenu" title="菜单" aria-label="菜单">☰</button>' +
      '  </div>' +
      '  <nav class="user-nav" id="userNav">' + navHtml + '</nav>' +
      '</header>' +
      '<main class="user-content" id="userContent">' +
      (title ? '<div class="page-head"><div><h2>' + title + '</h2>' + (sub ? '<div class="sub">' + sub + '</div>' : '') + '</div></div>' : '') +
      '</main></div>';

    document.getElementById('btnLogout').addEventListener('click', USER.logout);
    document.getElementById('btnUMenu').addEventListener('click', () => {
      document.getElementById('userNav').classList.toggle('open');
    });
    if (LXD.mountThemeToggle) LXD.mountThemeToggle(app.querySelector('.u-actions'));

    // 品牌
    LXD.loadBrand().then((b) => {
      LXD.applyBrand(b);
      document.title = (b.site_name || 'LXD 容器面板') + ' - ' + (title || '');
    }).catch(() => {});

    // 用户信息
    USER.request('/api/user/info').then((res) => {
      const d = res.data || {};
      const u = d.user || d; // /api/user/info 返回 {user:{...}, stats:{...}}
      const name = u.username || u.Username || u.name;
      const info = document.getElementById('userInfo');
      if (info && name) info.textContent = '已登录：' + name;
    }).catch(() => {});
  }

  /** 请求封装（同源 Cookie 会话；401/403 跳登录） */
  function request(url, options = {}) {
    options.method = options.method || 'GET';
    options.headers = Object.assign({ 'Content-Type': 'application/json' }, options.headers || {});
    if (options.body && typeof options.body !== 'string') options.body = JSON.stringify(options.body);
    return fetch(url, options).then(async (res) => {
      if (res.status === 401 || res.status === 403) {
        LXD.toast('error', '登录已失效，请重新登录');
        setTimeout(() => { window.location.href = 'login.html'; }, 800);
        throw new Error('unauthorized');
      }
      const data = await res.json().catch(() => ({}));
      if (data && data.code === 401) {
        LXD.toast('error', data.msg || '登录已失效，请重新登录');
        setTimeout(() => { window.location.href = 'login.html'; }, 800);
        throw new Error('unauthorized');
      }
      if (!res.ok || (data.code !== undefined && data.code !== 200)) {
        const err = new Error(data.msg || data.message || '请求失败');
        err.status = res.status; err.data = data;
        throw err;
      }
      return data;
    });
  }

  function logout() {
    fetch('/api/user/logout', { method: 'POST' }).catch(() => {});
    setTimeout(() => { window.location.href = 'login.html'; }, 200);
  }

  window.USER = { shell, request, logout, toast: LXD.toast, esc: LXD.esc, fmt: { bytes: LXD.fmtBytes, time: LXD.fmtTime }, empty: (t) => '<div class="empty"><div class="empty-icon">▫</div>' + LXD.esc(t || '暂无数据') + '</div>' };
})();

/* ============================================================
   LXD Panel - User 通用列表页
   用法：USER_GENERIC.page({
     api: '/api/user/xxx',
     title: '页面标题',
     sub: '副标题',
     searchKeys: ['name', 'remark'],
     columns: [
       { label: '名称', keys: ['name', 'Name'], mono: true },
       { label: '状态', keys: ['status', 'Status'], type: 'status' },
       { label: '时间', keys: ['created_at', 'CreatedAt'], type: 'time' },
       { label: '大小', keys: ['size'], type: 'bytes' },
       { label: '操作', render: (row) => '...' }
     ],
     buttons: '<button ...>'
   })
   ============================================================ */
(function () {
  'use strict';

  function get(row, keys, def) {
    for (const k of keys) {
      if (row[k] !== undefined && row[k] !== null) return row[k];
    }
    return def;
  }

  function renderCell(row, col) {
    if (col.render === 'PROGRESS') {
      const p = Math.max(0, Math.min(100, parseFloat(get(row, ['progress', 'percent', 'Progress'], 0)) || 0));
      return '<div class="progress" style="min-width:80px"><i style="width:' + p + '%"></i></div>' +
        '<div style="font-size:11px;color:var(--text-3);margin-top:3px">' + Math.round(p) + '%</div>';
    }
    if (typeof col.render === 'function') return col.render(row);
    let v = get(row, col.keys || [col.key], null);
    if (v == null || v === '') return '-';
    if (Array.isArray(v)) v = v.join(', '); // ipv4/ipv6 等多值字段
    switch (col.type) {
      case 'time': return '<span class="mono">' + USER.fmt.time(v) + '</span>';
      case 'bytes': return USER.fmt.bytes(v);
      case 'status': {
        const s = String(v).toLowerCase();
        if (['enabled', 'active', 'running', 'ok', 'success', 'normal', '1', 'true'].includes(s)) {
          return '<span class="badge badge-running"><span class="dot"></span>' + USER.esc(String(v)) + '</span>';
        }
        if (['disabled', 'stopped', 'failed', 'error', 'banned', '0', 'false'].includes(s)) {
          return '<span class="badge badge-stopped">' + USER.esc(String(v)) + '</span>';
        }
        return '<span class="badge badge-other">' + USER.esc(String(v)) + '</span>';
      }
      case 'text':
      default: return USER.esc(String(v));
    }
  }

  function page(opts) {
    USER.shell(opts.active || '', opts.title, opts.sub);
    const content = document.getElementById('userContent');
    const cols = opts.columns || [];
    let all = [];
    let filtered = [];
    let pageNo = 1;
    const pageSize = 20;

    content.innerHTML =
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      (opts.buttons || '') +
      '<span class="grow"></span>' +
      '<div class="search-box"><span class="sb-icon">⌕</span><input id="gSearch" class="input" placeholder="搜索..." oninput="doGFilter()"></div>' +
      '</div></div>' +
      '<div class="card" style="padding:14px" id="gArea"></div>';

    function render() {
      const area = document.getElementById('gArea');
      const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
      if (pageNo > totalPages) pageNo = totalPages;
      const rows = filtered.slice((pageNo - 1) * pageSize, pageNo * pageSize);
      if (!rows.length) { area.innerHTML = USER.empty(opts.emptyText || '暂无数据'); return; }
      let h = '<div class="tbl-wrap"><table class="tbl"><thead><tr>';
      cols.forEach((c) => { h += '<th>' + c.label + '</th>'; });
      h += '</tr></thead><tbody>';
      rows.forEach((row) => {
        h += '<tr>';
        cols.forEach((c) => { h += '<td' + (c.mono ? ' class="mono"' : '') + '>' + renderCell(row, c) + '</td>'; });
        h += '</tr>';
      });
      h += '</tbody></table></div>';
      const pager = '<div style="display:flex;gap:10px;align-items:center;justify-content:flex-end;margin-top:12px;font-size:13px">' +
        '<span style="color:var(--text-3)">共 ' + filtered.length + ' 条</span>' +
        '<button class="btn btn-ghost btn-sm" onclick="goGPage(' + (pageNo - 1) + ')" ' + (pageNo <= 1 ? 'disabled' : '') + '>‹ 上一页</button>' +
        '<span style="color:var(--text-2)">' + pageNo + ' / ' + totalPages + '</span>' +
        '<button class="btn btn-ghost btn-sm" onclick="goGPage(' + (pageNo + 1) + ')" ' + (pageNo >= totalPages ? 'disabled' : '') + '>下一页 ›</button></div>';
      area.innerHTML = h + pager;
    }

    function load() {
      let url = opts.api;
      if (opts.params) {
        const qs = Object.keys(opts.params).map((k) => encodeURIComponent(k) + '=' + encodeURIComponent(opts.params[k])).join('&');
        url += (url.indexOf('?') >= 0 ? '&' : '?') + qs;
      }
      USER.request(url).then((res) => {
        const data = res.data;
        let rows = null;
        if (opts.dataKey && data && Array.isArray(data[opts.dataKey])) rows = data[opts.dataKey];
        else if (opts.dataKeys && data) {
          rows = [];
          opts.dataKeys.forEach((k) => { if (Array.isArray(data[k])) rows = rows.concat(data[k]); });
        }
        if (rows == null) {
          if (Array.isArray(data)) rows = data;
          else if (data && Array.isArray(data.list)) rows = data.list;
          else if (data && Array.isArray(data.items)) rows = data.items;
          else if (data && Array.isArray(data.data)) rows = data.data;
          else rows = [];
        }
        all = rows;
        doFilter();
      }).catch(() => { document.getElementById('gArea').innerHTML = USER.empty('加载失败'); });
    }

    function doFilter() {
      const kw = (document.getElementById('gSearch').value || '').trim().toLowerCase();
      const keys = opts.searchKeys || [];
      filtered = kw ? all.filter((row) =>
        keys.some((k) => String(get(row, Array.isArray(k) ? k : [k], '')).toLowerCase().includes(kw))
      ) : all.slice();
      pageNo = 1;
      render();
    }

    window.doGFilter = doFilter;
    window.goGPage = (p) => {
      const tp = Math.max(1, Math.ceil(filtered.length / pageSize));
      if (p < 1 || p > tp) return;
      pageNo = p; render();
    };
    window.__gReload = load;

    load();
  }

  window.USER_GENERIC = { page, get };
})();
