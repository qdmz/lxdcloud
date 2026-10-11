/* ============================================================
   LXD Panel - Admin 用户管理
   GET  /api/admin/users
   POST /api/admin/users
   POST /api/admin/users/{id}
   POST /api/admin/users/{id}/regenerate-key
   POST /api/admin/users/batch-delete
   DELETE /api/admin/users/{id}
   ============================================================ */
(function () {
  'use strict';

  ADMIN.shell('users', '用户管理', '管理用户账号、资源配额与访问密钥');

  let all = [];
  let filtered = [];
  let page = 1, pageSize = 20;
  let selected = new Set();

  const content = document.getElementById('adminContent');
  const PAGE = { count: () => Math.max(1, Math.ceil(filtered.length / pageSize)) };

  /** 兼容大小写混合字段（原接口 ID/Username/Status/Remark/CreatedAt/CPUQuota 等） */
  function uget(u, keys, def) {
    for (const k of keys) {
      if (u[k] !== undefined && u[k] !== null) return u[k];
    }
    return def;
  }

  function userStatus(u) {
    const s = String(uget(u, ['status', 'Status'], 'enabled')).toLowerCase();
    if (['enabled', 'active', 'normal', 'ok', '1'].includes(s)) {
      return '<span class="badge badge-running"><span class="dot"></span>正常</span>';
    }
    if (['disabled', 'banned', 'suspend', 'suspended', '0'].includes(s)) {
      return '<span class="badge badge-stopped">禁用</span>';
    }
    return '<span class="badge badge-other">' + ADMIN.esc(s || '未知') + '</span>';
  }

  function statBar() {
    return '<div class="stat-grid" style="grid-template-columns:repeat(4,1fr);margin-bottom:16px">' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px" id="stTotal">-</div><div class="stat-name">用户总数</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#34d399" id="stOk">-</div><div class="stat-name">正常</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#f87171" id="stBlocked">-</div><div class="stat-name">已禁用</div></div>' +
      '<div class="card stat-card"><div class="stat-val" style="font-size:18px;color:#fbbf24" id="stKeys">-</div><div class="stat-name">待重置密钥</div></div>' +
      '</div>';
  }

  function toolbar() {
    return '<div class="card" style="padding:12px 14px;margin-bottom:16px">' +
      '<div class="toolbar">' +
      '  <button class="btn btn-primary btn-sm" onclick="openUserModal(\'create\')">＋ 新建用户</button>' +
      '  <button class="btn btn-ghost btn-sm" onclick="load()">↻ 刷新</button>' +
      '  <span class="grow"></span>' +
      '  <div class="search-box"><span class="sb-icon">⌕</span><input id="searchInput" class="input" placeholder="搜索用户名/备注..." oninput="doFilter()"></div>' +
      '</div>' +
      '<div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">' +
      '  <button class="btn btn-danger btn-sm" id="btnBatchDelete">✕ 批量删除</button>' +
      '  <span id="selInfo" style="font-size:12px;color:var(--text-3)"></span>' +
      '</div></div>';
  }

  function renderShell() {
    content.innerHTML = statBar() + toolbar() + '<div class="card" style="padding:14px" id="listArea"></div>';
    document.getElementById('btnBatchDelete').addEventListener('click', batchDelete);
  }

  function renderList() {
    const area = document.getElementById('listArea');
    const totalPages = PAGE.count();
    if (page > totalPages) page = totalPages;
    const rows = filtered.slice((page - 1) * pageSize, page * pageSize);
    if (!rows.length) { area.innerHTML = ADMIN.empty('暂无用户，点击「新建用户」创建'); return; }

    let h = '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
      '<th><input type="checkbox" onclick="toggleAll(this.checked)" id="ckAll"></th>' +
      '<th>ID</th><th>用户名</th><th>状态</th><th>配额</th><th>备注</th><th>创建时间</th><th>操作</th>' +
      '</tr></thead><tbody>';
    rows.forEach((u) => {
      const id = uget(u, ['id', 'ID'], '');
      const nm = uget(u, ['username', 'Username', 'name'], '');
      h += '<tr>' +
        '<td><input type="checkbox" class="ck" data-id="' + ADMIN.esc(String(id)) + '" ' + (selected.has(String(id)) ? 'checked' : '') + ' onclick="updateSel(this)"></td>' +
        '<td class="mono">' + ADMIN.esc(String(id)) + '</td>' +
        '<td style="color:var(--text-1);font-weight:600">' + ADMIN.esc(nm) + '</td>' +
        '<td>' + userStatus(u) + '</td>' +
        '<td>' + quotaStr(u) + '</td>' +
        '<td>' + ADMIN.esc(uget(u, ['remark', 'Remark'], '') || '-') + '</td>' +
        '<td class="mono">' + ADMIN.fmt.time(uget(u, ['created_at', 'CreatedAt', 'createdAt'], '')) + '</td>' +
        '<td><div class="row-actions">' +
        '<button class="btn btn-ghost btn-xs" onclick="openUserModal(\'edit\',\'' + ADMIN.esc(String(id)) + '\')">编辑</button>' +
        '<button class="btn btn-ghost btn-xs" onclick="regenerateKey(\'' + ADMIN.esc(String(id)) + '\')">重置密钥</button>' +
        '<button class="btn btn-danger btn-xs" onclick="delOne(\'' + ADMIN.esc(String(id)) + '\')">删除</button>' +
        '</div></td></tr>';
    });
    h += '</tbody></table></div>';

    const pager = '<div style="display:flex;gap:10px;align-items:center;justify-content:flex-end;margin-top:12px;font-size:13px">' +
      '<span style="color:var(--text-3)">共 ' + filtered.length + ' 条</span>' +
      '<button class="btn btn-ghost btn-sm" onclick="goPage(' + (page - 1) + ')" ' + (page <= 1 ? 'disabled' : '') + '>‹ 上一页</button>' +
      '<span style="color:var(--text-2)">' + page + ' / ' + totalPages + '</span>' +
      '<button class="btn btn-ghost btn-sm" onclick="goPage(' + (page + 1) + ')" ' + (page >= totalPages ? 'disabled' : '') + '>下一页 ›</button></div>';
    area.innerHTML = h + pager;
  }

  function quotaStr(u) {
    const cpu = uget(u, ['cpu', 'CPU', 'cpu_quota', 'CPUQuota'], null);
    const mem = uget(u, ['memory', 'memory_quota', 'MemoryQuota'], null);
    const disk = uget(u, ['disk', 'disk_quota', 'DiskQuota'], null);
    if (cpu == null && mem == null && disk == null) return '-';
    const parts = [];
    if (cpu != null) parts.push(cpu + ' 核');
    if (mem != null) parts.push(ADMIN.fmt.bytes(mem) || mem);
    if (disk != null) parts.push(ADMIN.fmt.bytes(disk) || disk);
    return parts.join(' / ');
  }

  function load() {
    ADMIN.request('/api/admin/users').then((res) => {
      all = (res.data && res.data.users) || [];
      selected.clear();
      doFilter();
    }).catch(() => { document.getElementById('listArea').innerHTML = ADMIN.empty('加载失败'); });
  }

  function doFilter() {
    const kw = (document.getElementById('searchInput').value || '').trim().toLowerCase();
    filtered = kw ? all.filter((u) =>
      String(uget(u, ['username', 'Username', 'name'], '')).toLowerCase().includes(kw) ||
      String(uget(u, ['remark', 'Remark'], '')).toLowerCase().includes(kw)
    ) : all.slice();
    page = 1;
    const st = { total: filtered.length, ok: 0, blocked: 0, keys: 0 };
    filtered.forEach((u) => {
      const s = String(uget(u, ['status', 'Status'], 'enabled')).toLowerCase();
      if (['disabled', 'banned', 'suspend', 'suspended', '0'].includes(s)) st.blocked++;
      else st.ok++;
    });
    const set = (id, v) => { const el = document.getElementById(id); if (el) el.textContent = v; };
    set('stTotal', st.total); set('stOk', st.ok); set('stBlocked', st.blocked); set('stKeys', st.keys);
    renderList();
  }

  function updateSel(ck) {
    const id = ck.dataset.id;
    if (ck.checked) selected.add(id); else selected.delete(id);
    document.getElementById('selInfo').textContent = selected.size ? '已选 ' + selected.size + ' 个' : '';
  }
  function toggleAll(on) {
    document.querySelectorAll('.ck').forEach((ck) => { ck.checked = on; selected[on ? 'add' : 'delete'](ck.dataset.id); });
    document.getElementById('selInfo').textContent = selected.size ? '已选 ' + selected.size + ' 个' : '';
  }

  function batchDelete() {
    const list = [...selected];
    if (!list.length) return LXD.toast('warn', '请先勾选用户');
    LXD.confirmDialog('批量删除', '确定删除选中的 ' + list.length + ' 个用户？其名下容器与映射将受影响，此操作不可恢复！', true).then((ok) => {
      if (!ok) return;
      ADMIN.request('/api/admin/users/batch-delete', { method: 'POST', body: { ids: list } })
        .then(() => { LXD.toast('success', '批量删除完成'); load(); })
        .catch((err) => LXD.toast('error', err.message || '批量删除失败'));
    });
  }

  function delOne(id) {
    LXD.confirmDialog('删除用户', '确定要删除用户 #' + id + ' 吗？此操作不可恢复！', true).then((ok) => {
      if (!ok) return;
      ADMIN.request('/api/admin/users/' + encodeURIComponent(id), { method: 'DELETE' })
        .then(() => { LXD.toast('success', '删除成功'); load(); })
        .catch(() => LXD.toast('error', '删除失败'));
    });
  }

  function regenerateKey(id) {
    LXD.confirmDialog('重置访问密钥', '确定重置用户 #' + id + ' 的访问密钥吗？旧密钥将立即失效。', false).then((ok) => {
      if (!ok) return;
      ADMIN.request('/api/admin/users/' + encodeURIComponent(id) + '/regenerate-key', { method: 'POST' })
        .then((res) => {
          const key = res.data && (res.data.key || res.data.hash || res.data.access_key || res.data.container_hash);
          LXD.toast('success', '密钥已重置');
          if (key) showKeyModal(key); else load();
        })
        .catch((err) => LXD.toast('error', err.message || '重置失败'));
    });
  }

  function showKeyModal(key) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML =
      '<div class="modal" style="max-width:480px">' +
      '<h3>新的访问密钥</h3>' +
      '<p style="color:var(--text-3);font-size:13px">请立即复制保存，关闭后不再显示。</p>' +
      '<div style="display:flex;gap:8px;margin:14px 0">' +
      '<input id="newKeyInput" readonly class="input mono" value="' + ADMIN.esc(key) + '" style="flex:1">' +
      '<button class="btn btn-primary btn-sm" id="copyKeyBtn">复制</button></div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="close">关闭</button></div></div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="close"]').onclick = () => mask.remove();
    mask.addEventListener('click', (e) => { if (e.target === mask) mask.remove(); });
    mask.querySelector('#copyKeyBtn').addEventListener('click', () => {
      const input = mask.querySelector('#newKeyInput');
      input.select();
      try { document.execCommand('copy'); LXD.toast('success', '已复制'); } catch (e) { LXD.toast('warn', '请手动复制'); }
    });
  }

  function goPage(p) {
    const tp = PAGE.count();
    if (p < 1 || p > tp) return;
    page = p; renderList();
  }

  window.openUserModal = (mode, id) => { try { openUserModalSafe(mode, id, load); } catch (e) { LXD.toast('error', '弹窗模块加载失败'); } };
  window.doFilter = doFilter;
  window.goPage = goPage;
  window.load = load;
  window.delOne = delOne;
  window.regenerateKey = regenerateKey;
  window.updateSel = updateSel;
  window.toggleAll = toggleAll;

  renderShell();
  load();
})();
