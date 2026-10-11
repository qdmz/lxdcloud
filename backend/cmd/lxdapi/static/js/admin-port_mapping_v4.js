/* ============================================================
   LXD Panel - Admin 端口映射 IPv4（对齐 v3 后端 /api/admin/port-mapping?version=v4）
   子功能：① 端口映射列表  ② 端口范围配置  ③ SSH(22) 自动分配
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('port_mapping_v4', '端口映射', 'IPv4 端口映射管理');
  const V = 'v4', KEY = 'ipv4';
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';
  let all = [];
  const selected = new Set();
  let currentTab = 'map';
  let rangeConfig = null;

  /* ---------- 子功能 Tab ---------- */
  function tabBar() {
    return '<div class="tabs" style="margin-bottom:14px">' +
      '<button class="tab' + (currentTab === 'map' ? ' active' : '') + '" data-tab="map">端口映射</button>' +
      '<button class="tab' + (currentTab === 'range' ? ' active' : '') + '" data-tab="range">端口范围配置</button>' +
      '<button class="tab' + (currentTab === 'ssh' ? ' active' : '') + '" data-tab="ssh">SSH(22) 自动分配</button>' +
      '</div>';
  }

  function bindTabs() {
    c.querySelectorAll('.tab[data-tab]').forEach(b => b.addEventListener('click', () => switchTab(b.dataset.tab)));
  }

  function switchTab(tab) {
    currentTab = tab;
    if (tab === 'range') renderRange();
    else if (tab === 'ssh') renderSsh();
    else { currentTab = 'map'; renderMap(all); }
  }

  function loadRangeConfig() {
    return ADMIN.request('/api/admin/port-range/config').then(res => {
      rangeConfig = res.data || {};
      return rangeConfig;
    });
  }

  /* ---------- ① 端口映射列表 ---------- */
  function renderMap(list) {
    const rows = list.map(m => {
      const pubPort = m.public_port_end && m.public_port_end !== m.public_port
        ? (m.public_port + '-' + m.public_port_end) : m.public_port;
      const ctnPort = m.container_port_end && m.container_port_end !== m.container_port
        ? (m.container_port + '-' + m.container_port_end) : m.container_port;
      const st = (m.status || 'active').toLowerCase();
      const stBadge = st === 'active' ? '<span class="badge badge-running">生效</span>'
        : '<span class="badge badge-other">' + LXD.esc(m.status || '-') + '</span>';
      const checked = selected.has(m.id) ? ' checked' : '';
      return '<tr>' +
        '<td><input type="checkbox" class="pm-check" data-id="' + m.id + '"' + checked + '></td>' +
        '<td class="mono">#' + m.id + '</td>' +
        '<td class="mono"><strong>' + LXD.esc(m.public_ip || '-') + ':' + pubPort + '</strong></td>' +
        '<td class="mono">' + LXD.esc(m.container_name || '-') + '</td>' +
        '<td class="mono">' + ctnPort + '</td>' +
        '<td class="mono">' + LXD.esc((m.protocol || 'tcp').toUpperCase()) + '</td>' +
        '<td>' + stBadge + '</td>' +
        '<td>' + LXD.esc(m.description || '-') + '</td>' +
        '<td class="mono">' + LXD.esc(m.created_at || '-') + '</td>' +
        '<td class="row-actions"><button class="btn btn-sm btn-danger-ghost" data-rel="' + m.id + '" data-info="' + LXD.esc(m.public_ip + ':' + pubPort + ' → ' + m.container_name + ':' + ctnPort) + '">释放</button></td>' +
        '</tr>';
    }).join('');

    c.innerHTML = tabBar() +
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnAlloc">＋ 分配端口映射</button>' +
      '<button class="btn btn-danger-ghost" id="btnBatchRel" ' + (selected.size ? '' : 'disabled') + '>批量释放(' + selected.size + ')</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReload">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
        '<th style="width:32px"></th><th>ID</th><th>公网地址</th><th>容器</th><th>容器端口</th><th>协议</th><th>状态</th><th>描述</th><th>创建时间</th><th style="width:90px">操作</th>' +
        '</tr></thead><tbody>' + rows + '</tbody></table></div>' +
        '<div style="display:flex;justify-content:flex-end;margin-top:12px;font-size:13px;color:var(--text-3)">共 ' + list.length + ' 条映射</div>'
        : '<div class="empty">暂无端口映射，点击「分配端口映射」创建</div>') +
      '</div>';

    bindTabs();
    document.getElementById('btnAlloc').addEventListener('click', formAlloc);
    document.getElementById('btnReload').addEventListener('click', load);
    const batchBtn = document.getElementById('btnBatchRel');
    batchBtn.addEventListener('click', () => batchRelease(Array.from(selected)));
    c.querySelectorAll('.pm-check').forEach(ch => ch.addEventListener('change', () => {
      const id = parseInt(ch.dataset.id, 10);
      if (ch.checked) selected.add(id); else selected.delete(id);
      batchBtn.disabled = !selected.size;
      batchBtn.textContent = '批量释放(' + selected.size + ')';
    }));
    c.querySelectorAll('[data-rel]').forEach(b => b.addEventListener('click', () => releaseOne(parseInt(b.dataset.rel, 10), b.dataset.info)));
  }

  /* ---------- ② 端口范围配置 ---------- */
  function renderRange() {
    c.innerHTML = tabBar() + '<div class="loading">加载中...</div>';
    bindTabs();
    loadRangeConfig().then(cfg => {
      const start = cfg.v4_port_start, end = cfg.v4_port_end;
      c.innerHTML = tabBar() +
        '<div class="card" style="padding:18px;max-width:520px">' +
        '<h3 style="margin-bottom:4px">IPv4 端口范围配置</h3>' +
        '<p style="font-size:13px;color:var(--text-3);margin-bottom:14px">配置 NAT 端口映射可自动分配的 IPv4 公网端口段，映射时留空公网端口将从此范围随机分配。</p>' +
        '<div class="form-grid">' +
        '<div class="field"><label>起始端口</label><input class="input" id="rStart" type="number" min="1" max="65535" value="' + start + '"></div>' +
        '<div class="field"><label>结束端口</label><input class="input" id="rEnd" type="number" min="1" max="65535" value="' + end + '"></div>' +
        '</div>' +
        '<div style="display:flex;gap:10px;margin-top:16px">' +
        '<button class="btn btn-primary" id="btnSaveRange">保存配置</button>' +
        '<button class="btn btn-ghost" id="btnReloadRange">↻ 刷新</button>' +
        '</div></div>';
      bindTabs();
      document.getElementById('btnSaveRange').addEventListener('click', saveRange);
      document.getElementById('btnReloadRange').addEventListener('click', renderRange);
    }).catch(e => { c.innerHTML = tabBar() + '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; bindTabs(); });
  }

  function saveRange() {
    const start = parseInt(c.querySelector('#rStart').value, 10);
    const end = parseInt(c.querySelector('#rEnd').value, 10);
    if (!start || !end || start >= end) { LXD.toast('error', '起始端口必须小于结束端口（1-65535）'); return; }
    if (!rangeConfig) rangeConfig = {};
    const body = {
      v4_port_start: start,
      v4_port_end: end,
      v6_port_start: rangeConfig.v6_port_start || 10000,
      v6_port_end: rangeConfig.v6_port_end || 65535,
      v4_auto_allocate_22: !!rangeConfig.v4_auto_allocate_22,
      v6_auto_allocate_22: !!rangeConfig.v6_auto_allocate_22
    };
    ADMIN.request('/api/admin/port-range/config', { method: 'POST', body: body })
      .then(res => { rangeConfig = res.data || body; LXD.toast('success', '端口范围已保存'); })
      .catch(e => LXD.toast('error', e.message));
  }

  /* ---------- ③ SSH(22) 自动分配 ---------- */
  function renderSsh() {
    c.innerHTML = tabBar() + '<div class="loading">加载中...</div>';
    bindTabs();
    loadRangeConfig().then(cfg => {
      const on = !!cfg.v4_auto_allocate_22;
      c.innerHTML = tabBar() +
        '<div class="card" style="padding:18px;max-width:520px">' +
        '<h3 style="margin-bottom:4px">SSH(22) 自动分配</h3>' +
        '<p style="font-size:13px;color:var(--text-3);margin-bottom:14px">开启后，新建 IPv4 容器时将自动从公网端口段分配 22 端口映射（SSH 直连），无需手动添加映射。</p>' +
        '<div style="display:flex;align-items:center;gap:12px;padding:12px;border:1px solid var(--border);border-radius:12px;background:var(--bg-2)">' +
        '<label class="toggle-switch"><input type="checkbox" id="sshToggle"' + (on ? ' checked' : '') + '><span class="toggle-slider"></span></label>' +
        '<div><div style="font-size:14px;font-weight:600">自动分配 22 端口</div>' +
        '<div style="font-size:12.5px;color:var(--text-3)">当前状态：' + (on ? '已开启' : '已关闭') + '</div></div>' +
        '</div>' +
        '<div style="display:flex;gap:10px;margin-top:16px">' +
        '<button class="btn btn-primary" id="btnSaveSsh">保存设置</button>' +
        '<button class="btn btn-ghost" id="btnReloadSsh">↻ 刷新</button>' +
        '</div></div>';
      bindTabs();
      document.getElementById('btnSaveSsh').addEventListener('click', saveSsh);
      document.getElementById('btnReloadSsh').addEventListener('click', renderSsh);
    }).catch(e => { c.innerHTML = tabBar() + '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; bindTabs(); });
  }

  function saveSsh() {
    const on = c.querySelector('#sshToggle').checked;
    if (!rangeConfig) rangeConfig = {};
    const body = {
      v4_port_start: rangeConfig.v4_port_start || 10000,
      v4_port_end: rangeConfig.v4_port_end || 65535,
      v6_port_start: rangeConfig.v6_port_start || 10000,
      v6_port_end: rangeConfig.v6_port_end || 65535,
      v4_auto_allocate_22: on,
      v6_auto_allocate_22: !!rangeConfig.v6_auto_allocate_22
    };
    ADMIN.request('/api/admin/port-range/config', { method: 'POST', body: body })
      .then(res => { rangeConfig = res.data || body; LXD.toast('success', '已' + (on ? '开启' : '关闭') + ' SSH(22) 自动分配'); })
      .catch(e => LXD.toast('error', e.message));
  }

  /* ---------- 分配 / 释放 ---------- */
  function modal(html, onSave) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:460px">' + html + '</div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="cancel"]').onclick = () => mask.remove();
    mask.addEventListener('click', e => { if (e.target === mask) mask.remove(); });
    mask.querySelector('[data-act="save"]').onclick = () => onSave(mask);
    return mask;
  }

  function formAlloc() {
    modal(
      '<h3>分配端口映射</h3>' +
      '<div class="form-grid">' +
      '<div class="field" style="grid-column:1/-1"><label>容器名称</label><input class="input" id="fCtn" placeholder="容器名"></div>' +
      '<div class="field"><label>容器端口</label><input class="input" id="fCtnPort" type="number" min="1" placeholder="如 80"></div>' +
      '<div class="field"><label>端口数量</label><input class="input" id="fCount" type="number" min="1" value="1"></div>' +
      '<div class="field" style="grid-column:1/-1"><label>公网端口（留空自动分配）</label><input class="input" id="fPubPort" type="number" min="1" placeholder="自动"></div>' +
      '<div class="field" style="grid-column:1/-1"><label>描述</label><input class="input" id="fDesc" placeholder="可选"></div>' +
      '</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">分配</button></div>',
      mask => {
        const body = {
          container_name: mask.querySelector('#fCtn').value.trim(),
          container_port: parseInt(mask.querySelector('#fCtnPort').value, 10) || 0,
          port_count: parseInt(mask.querySelector('#fCount').value, 10) || 1,
          public_port: parseInt(mask.querySelector('#fPubPort').value, 10) || 0,
          description: mask.querySelector('#fDesc').value.trim()
        };
        if (!body.container_name || !body.container_port) { LXD.toast('error', '容器名称与容器端口必填'); return; }
        ADMIN.request('/api/admin/port-mapping/allocate?version=' + V, { method: 'POST', body: body })
          .then(res => {
            const d = res.data || {};
            const n = (d.mappings && d.mappings.length) || d.total || 1;
            LXD.toast('success', '分配成功：' + n + ' 条');
            mask.remove(); load();
          }).catch(e => LXD.toast('error', e.message));
      });
  }

  function releaseOne(id, info) {
    LXD.confirmDialog('释放映射', '确定释放端口映射 [' + info + ']？').then(ok => {
      if (!ok) return;
      ADMIN.request('/api/admin/port-mapping/release?version=' + V, { method: 'POST', body: { id: id } })
        .then(() => { LXD.toast('success', '已释放'); selected.delete(id); load(); })
        .catch(e => LXD.toast('error', e.message));
    });
  }

  function batchRelease(ids) {
    if (!ids.length) return;
    LXD.confirmDialog('批量释放', '确定释放选中的 ' + ids.length + ' 条端口映射？').then(ok => {
      if (!ok) return;
      ADMIN.request('/api/admin/port-mapping/release?version=' + V, { method: 'POST', body: { ids: ids } })
        .then(() => { LXD.toast('success', '批量释放完成'); selected.clear(); load(); })
        .catch(e => LXD.toast('error', e.message));
    });
  }

  function load() {
    ADMIN.request('/api/admin/port-mapping?version=' + V)
      .then(res => { all = (res.data && res.data[KEY]) || []; renderMap(all); })
      .catch(e => { c.innerHTML = tabBar() + '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; bindTabs(); });
  }
  load();
})();
