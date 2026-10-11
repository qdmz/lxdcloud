/* ============================================================
   LXD Panel - Admin 容器管理
   GET /api/admin/cache/containers
   POST /api/admin/containers/create
   POST /api/admin/containers/{name}/action?action=stop|restart|pause|resume
   POST /api/admin/containers/{name}/config
   DELETE /api/admin/containers/{name}
   GET  /api/admin/cache/refresh?name={name}
   ============================================================ */
(function () {
  'use strict';

  ADMIN.shell('containers', '容器管理', '创建、管理与监控所有容器');

  let all = [];
  let filtered = [];
  let page = 1, pageSize = 20;
  let viewMode = localStorage.getItem('lxd_admin_view') || 'table';
  let selected = new Set();

  const content = document.getElementById('adminContent');
  const PAGE = { count: () => Math.max(1, Math.ceil(filtered.length / pageSize)) };

  function statBar() {
    return '<div class="stat-grid" style="grid-template-columns:repeat(4,1fr);margin-bottom:16px">' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px" id="stTotal">-</div><div class="stat-name">容器总数</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#34d399" id="stRun">-</div><div class="stat-name">运行中</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#64748b" id="stStop">-</div><div class="stat-name">已停止</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#fbbf24" id="stPause">-</div><div class="stat-name">已暂停</div></div>' +
      '</div>';
  }

  function toolbar() {
    return '<div class="card" style="padding:12px 14px;margin-bottom:16px">' +
      '<div class="toolbar">' +
      '  <button class="btn btn-primary btn-sm" onclick="openCreate()">＋ 创建容器</button>' +
      '  <button class="btn btn-ghost btn-sm" onclick="refreshCache()">↻ 同步缓存</button>' +
      '  <button class="btn btn-ghost btn-sm" onclick="toggleView()">' + (viewMode === 'table' ? '▦ 卡片视图' : '☰ 列表视图') + '</button>' +
      '  <span class="grow"></span>' +
      '  <div class="search-box"><span class="sb-icon">⌕</span><input id="searchInput" class="input" placeholder="搜索名称/备注/用户..." oninput="doFilter()"></div>' +
      '</div>' +
      '<div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">' +
      '  <button class="btn btn-ghost btn-sm" id="btnBatchStop">■ 停止选中</button>' +
      '  <button class="btn btn-ghost btn-sm" id="btnBatchRestart">↻ 重启选中</button>' +
      '  <button class="btn btn-danger btn-sm" id="btnBatchDelete">✕ 批量删除</button>' +
      '  <span id="selInfo" style="font-size:12px;color:var(--text-3)"></span>' +
      '</div></div>';
  }

  function renderShell() {
    content.innerHTML = statBar() + toolbar() + '<div class="card" style="padding:14px" id="listArea"></div>';
    document.getElementById('btnBatchStop').addEventListener('click', () => batchOp('stop', '停止'));
    document.getElementById('btnBatchRestart').addEventListener('click', () => batchOp('restart', '重启'));
    document.getElementById('btnBatchDelete').addEventListener('click', batchDelete);
  }

  function renderList() {
    const area = document.getElementById('listArea');
    const totalPages = PAGE.count();
    if (page > totalPages) page = totalPages;
    const rows = filtered.slice((page - 1) * pageSize, page * pageSize);
    if (!rows.length) { area.innerHTML = ADMIN.empty('暂无容器，点击右上角「创建容器」新建'); return; }

    const html = viewMode === 'table' ? renderTable(rows) : renderCards(rows);
    const pager = '<div style="display:flex;gap:10px;align-items:center;justify-content:flex-end;margin-top:12px;font-size:13px">' +
      '<span style="color:var(--text-3)">共 ' + filtered.length + ' 条</span>' +
      '<button class="btn btn-ghost btn-sm" onclick="goPage(' + (page - 1) + ')" ' + (page <= 1 ? 'disabled' : '') + '>‹ 上一页</button>' +
      '<span style="color:var(--text-2)">' + page + ' / ' + totalPages + '</span>' +
      '<button class="btn btn-ghost btn-sm" onclick="goPage(' + (page + 1) + ')" ' + (page >= totalPages ? 'disabled' : '') + '>下一页 ›</button></div>';
    area.innerHTML = html + pager;
  }

  function renderTable(rows) {
    let h = '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
      '<th><input type="checkbox" onclick="toggleAll(this.checked)" id="ckAll"></th>' +
      '<th>名称</th><th>状态</th><th>镜像</th><th>用户</th><th>内存</th><th>磁盘</th><th>IP</th><th>备注</th><th>操作</th>' +
      '</tr></thead><tbody>';
    rows.forEach((c) => {
      const nm = c.name || '-';
      h += '<tr>' +
        '<td><input type="checkbox" class="ck" data-name="' + ADMIN.esc(nm) + '" ' + (selected.has(nm) ? 'checked' : '') + ' onclick="updateSel(this)"></td>' +
        '<td class="mono" style="color:var(--text-1);font-weight:600">' + ADMIN.esc(nm) + '</td>' +
        '<td>' + LXD.statusBadge(c.status) + '</td>' +
        '<td>' + ADMIN.esc(c.image || c.image_name || '-') + '</td>' +
        '<td>' + ADMIN.esc(c.username || '-') + '</td>' +
        '<td>' + ADMIN.esc(memStr(c)) + '</td>' +
        '<td>' + ADMIN.esc(diskStr(c)) + '</td>' +
        '<td class="mono">' + ADMIN.esc(ipStr(c)) + '</td>' +
        '<td>' + ADMIN.esc(c.remark || '-') + '</td>' +
        '<td><div class="row-actions">' +
        '<a class="btn btn-ghost btn-xs" href="/admin/container-detail.html?name=' + encodeURIComponent(nm) + '">详情</a>' +
        '<button class="btn btn-ghost btn-xs" onclick="syncOne(\'' + encodeURIComponent(nm) + '\')">同步</button>' +
        '<button class="btn btn-ghost btn-xs" onclick="openConfig(\'' + encodeURIComponent(nm) + '\')">配置</button>' +
        '<button class="btn btn-danger btn-xs" onclick="delOne(\'' + encodeURIComponent(nm) + '\')">删除</button>' +
        '</div></td></tr>';
    });
    return h + '</tbody></table></div>';
  }

  function renderCards(rows) {
    let h = '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(240px,1fr));gap:12px">';
    rows.forEach((c) => {
      const nm = c.name || '-';
      h += '<div class="card card-hover" style="padding:14px">' +
        '<div style="display:flex;justify-content:space-between;align-items:center;gap:8px">' +
        '<div class="mono" style="font-weight:700;color:var(--text-1)">' + ADMIN.esc(nm) + '</div>' +
        LXD.statusBadge(c.status) + '</div>' +
        '<div style="font-size:12px;color:var(--text-3);margin-top:8px;display:flex;flex-direction:column;gap:4px">' +
        '<span>镜像：' + ADMIN.esc(c.image || c.image_name || '-') + '</span>' +
        '<span>用户：' + ADMIN.esc(c.username || '-') + '</span>' +
        '<span>内存：' + ADMIN.esc(memStr(c)) + '　磁盘：' + ADMIN.esc(diskStr(c)) + '</span>' +
        '<span class="mono">IP：' + ADMIN.esc(ipStr(c)) + '</span>' +
        (c.remark ? '<span>备注：' + ADMIN.esc(c.remark) + '</span>' : '') +
        '</div>' +
        '<div class="row-actions" style="margin-top:12px">' +
        '<a class="btn btn-ghost btn-xs" href="/admin/container-detail.html?name=' + encodeURIComponent(nm) + '">详情</a>' +
        '<button class="btn btn-ghost btn-xs" onclick="syncOne(\'' + encodeURIComponent(nm) + '\')">同步</button>' +
        '<button class="btn btn-danger btn-xs" onclick="delOne(\'' + encodeURIComponent(nm) + '\')">删除</button>' +
        '</div></div>';
    });
    return h + '</div>';
  }

  function memStr(c) {
    if (c.memory_usage_raw || c.memory) {
      const used = c.memory_usage_raw ? (parseFloat(c.memory_usage_raw) / 1024 / 1024).toFixed(0) + 'M' : '?';
      return used + ' / ' + ADMIN.esc(c.memory || '?');
    }
    return c.memory || '-';
  }
  function diskStr(c) {
    if (c.disk_usage_raw || c.disk) {
      const used = c.disk_usage_raw ? (parseFloat(c.disk_usage_raw) / 1024 / 1024).toFixed(0) + 'M' : '?';
      return used + ' / ' + ADMIN.esc(c.disk || '?');
    }
    return c.disk || '-';
  }
  function ipStr(c) {
    const v4 = c.ipv4 || c.IPAddress || c.ip_address;
    const v6 = c.ipv6 || c.ipv6_address;
    return [v4, v6].filter(Boolean).join('<br>') || '-';
  }

  function load() {
    ADMIN.request('/api/admin/cache/containers').then((res) => {
      all = (res.data && res.data.data) || [];
      selected.clear();
      doFilter();
    }).catch(() => { document.getElementById('listArea').innerHTML = ADMIN.empty('加载失败'); });
  }

  function doFilter() {
    const kw = (document.getElementById('searchInput').value || '').trim().toLowerCase();
    filtered = kw ? all.filter((c) => (c.name || '').toLowerCase().includes(kw) || (c.remark || '').toLowerCase().includes(kw) || (c.username || '').toLowerCase().includes(kw)) : all.slice();
    page = 1;
    const total = filtered.length;
    const st = { total, run: 0, stop: 0, pause: 0 };
    filtered.forEach((c) => {
      const s = (c.status || '').toLowerCase();
      if (s === 'running') st.run++; else if (s === 'stopped') st.stop++; else if (s === 'paused') st.pause++;
    });
    const set = (id, v) => { const el = document.getElementById(id); if (el) el.textContent = v; };
    set('stTotal', total); set('stRun', st.run); set('stStop', st.stop); set('stPause', st.pause);
    renderList();
  }

  function updateSel(ck) {
    const name = ck.dataset.name;
    if (ck.checked) selected.add(name); else selected.delete(name);
    document.getElementById('selInfo').textContent = selected.size ? '已选 ' + selected.size + ' 个' : '';
  }
  function toggleAll(on) {
    document.querySelectorAll('.ck').forEach((ck) => { ck.checked = on; selected[on ? 'add' : 'delete'](ck.dataset.name); });
    document.getElementById('selInfo').textContent = selected.size ? '已选 ' + selected.size + ' 个' : '';
  }

  function batchOp(action, label) {
    const list = [...selected];
    if (!list.length) return LXD.toast('warn', '请先勾选容器');
    LXD.confirmDialog('批量' + label, '确定要' + label + '选中的 ' + list.length + ' 个容器吗？').then((ok) => {
      if (!ok) return;
      let i = 0;
      const next = () => {
        if (i >= list.length) { LXD.toast('success', '批量' + label + '完成'); load(); return; }
        const name = list[i++];
        ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/action?action=' + action, { method: 'POST' })
          .then(() => next()).catch(() => next());
      };
      next();
    });
  }

  function batchDelete() {
    const list = [...selected];
    if (!list.length) return LXD.toast('warn', '请先勾选容器');
    LXD.confirmDialog('批量删除', '确定删除选中的 ' + list.length + ' 个容器？此操作不可恢复！', true).then((ok) => {
      if (!ok) return;
      let i = 0;
      const next = () => {
        if (i >= list.length) { LXD.toast('success', '批量删除完成'); load(); return; }
        const name = list[i++];
        ADMIN.request('/api/admin/containers/' + encodeURIComponent(name), { method: 'DELETE' }).then(() => next()).catch(() => next());
      };
      next();
    });
  }

  function syncOne(name) {
    ADMIN.request('/api/admin/cache/refresh?name=' + name).then(() => LXD.toast('success', '同步成功')).catch(() => LXD.toast('error', '同步失败'));
  }
  function refreshCache() {
    ADMIN.request('/api/admin/cache/refresh').then(() => { LXD.toast('success', '缓存已刷新'); load(); }).catch(() => LXD.toast('error', '刷新失败'));
  }

  function delOne(name) {
    LXD.confirmDialog('删除容器', '确定要删除容器 ' + decodeURIComponent(name) + ' 吗？此操作不可恢复！', true).then((ok) => {
      if (!ok) return;
      ADMIN.request('/api/admin/containers/' + name, { method: 'DELETE' })
        .then(() => { LXD.toast('success', '删除成功'); load(); })
        .catch(() => LXD.toast('error', '删除失败'));
    });
  }

  function toggleView() {
    viewMode = viewMode === 'table' ? 'card' : 'table';
    localStorage.setItem('lxd_admin_view', viewMode);
    renderShell(); renderList();
  }
  function goPage(p) {
    const tp = PAGE.count();
    if (p < 1 || p > tp) return;
    page = p; renderList();
  }

  window.openCreate = () => { try { openCreateModal(load); } catch (e) { LXD.toast('error', '弹窗模块加载失败'); } };
  window.openConfig = (name) => { try { openConfigModal(name, load); } catch (e) { LXD.toast('error', '弹窗模块加载失败'); } };
  window.doFilter = doFilter;
  window.toggleView = toggleView;
  window.goPage = goPage;
  window.syncOne = syncOne;
  window.delOne = delOne;
  window.refreshCache = refreshCache;
  window.updateSel = updateSel;
  window.toggleAll = toggleAll;

  renderShell();
  load();
})();
