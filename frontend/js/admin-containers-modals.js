/* ============================================================
   LXD Panel - Admin 容器创建/配置弹窗
   ============================================================ */
(function () {
  'use strict';

  const MODAL_CSS = '.lxd-modal{position:fixed;inset:0;z-index:60;display:none;align-items:center;justify-content:center;padding:16px}' +
    '.lxd-modal.show{display:flex}.lxd-modal .m-overlay{position:absolute;inset:0;background:rgba(2,6,17,.72);backdrop-filter:blur(4px)}' +
    '.lxd-modal .m-card{position:relative;width:100%;max-width:720px;max-height:88vh;overflow-y:auto;border:1px solid var(--border-strong);border-radius:18px;background:var(--card-strong);box-shadow:0 24px 80px rgba(0,0,0,.6)}' +
    '.lxd-modal .m-head{position:sticky;top:0;z-index:2;display:flex;align-items:center;justify-content:space-between;padding:16px 20px;background:var(--card-strong);border-bottom:1px solid var(--border)}' +
    '.lxd-modal .m-title{font-size:16px;font-weight:700;color:var(--text-1)}' +
    '.lxd-modal .m-body{padding:18px 20px}' +
    '.lxd-modal .m-foot{display:flex;justify-content:flex-end;gap:10px;padding:14px 20px;border-top:1px solid var(--border)}' +
    '.lxd-modal .m-section{font-size:12px;font-weight:700;color:var(--primary);margin:16px 0 10px;padding-bottom:6px;border-bottom:1px dashed var(--border);text-transform:uppercase;letter-spacing:.05em}' +
    '.lxd-modal .m-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px}' +
    '.lxd-modal .m-chk{display:flex;align-items:center;gap:8px;font-size:13px;color:var(--text-2);padding:8px 10px;border:1px solid var(--border);border-radius:10px;background:var(--card)}' +
    '.lxd-modal input[type=checkbox]{accent-color:#38bdf8;width:15px;height:15px}';

  function ensureStyle() {
    if (document.getElementById('lxdModalCss')) return;
    const s = document.createElement('style');
    s.id = 'lxdModalCss'; s.textContent = MODAL_CSS;
    document.head.appendChild(s);
  }

  function openModal(html, onMount) {
    ensureStyle();
    const host = document.createElement('div');
    host.innerHTML = '<div class="lxd-modal show">' +
      '<div class="m-overlay"></div><div class="m-card">' + html + '</div></div>';
    const modal = host.firstElementChild;
    modal.querySelector('.m-overlay').addEventListener('click', close);
    document.body.appendChild(modal);
    function close() { modal.remove(); }
    window.__lxdCloseModal = close;
    if (onMount) onMount(modal, close);
  }

  function fieldRow(label, name, val, opts) {
    opts = opts || {};
    return '<div><label style="font-size:12px;color:var(--text-3)">' + label + (opts.req ? ' <span style="color:var(--red)">*</span>' : '') + '</label>' +
      '<input type="' + (opts.type || 'text') + '" name="' + name + '" value="' + (val == null ? '' : ADMIN.esc(String(val))) + '" ' +
      (opts.placeholder ? 'placeholder="' + opts.placeholder + '" ' : '') +
      (opts.min != null ? 'min="' + opts.min + '" ' : '') +
      (opts.step ? 'step="' + opts.step + '" ' : '') +
      (opts.req ? 'required ' : '') +
      'class="input" style="width:100%;margin-top:5px"></div>';
  }

  /* ================= 创建容器 ================= */
  function openCreateModal(reload) {
    const html =
      '<div class="m-head"><div class="m-title">＋ 创建容器</div><button class="btn btn-ghost btn-sm" onclick="__lxdCloseModal()">✕</button></div>' +
      '<div class="m-body"><form id="createForm">' +
      '<div class="m-section">基础配置</div>' +
      '<div class="m-grid" style="grid-template-columns:repeat(auto-fit,minmax(200px,1fr))">' +
      fieldRow('容器名称', 'name', '', { req: true, placeholder: '例如 web-server-01' }) +
      fieldRow('Root 密码', 'password', '', { placeholder: '留空自动生成' }) +
      '</div>' +
      '<div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-top:10px">' +
      '<div><label style="font-size:12px;color:var(--text-3)">镜像 <span style="color:var(--red)">*</span></label>' +
      '<select name="image" id="createImage" required class="input" style="width:100%;margin-top:5px"><option value="">加载中...</option></select></div>' +
      fieldRow('所属用户', 'username', '', { req: true, placeholder: '容器所属用户' }) +
      '</div>' +
      fieldRow('备注', 'remark', '', { placeholder: '最多 20 字符', }) +
      '<div class="m-section">资源配置</div>' +
      '<div class="m-grid">' +
      fieldRow('CPU 核心数', 'cpu', '1', { type: 'number', min: 1, req: true }) +
      fieldRow('内存 MB', 'memory', '256', { type: 'number', min: 1, req: true }) +
      fieldRow('硬盘 MB', 'disk', '512', { type: 'number', min: 1, req: true }) +
      '</div>' +
      '<div class="m-section">网络与配额</div>' +
      '<div class="m-grid">' +
      fieldRow('入站带宽 Mbit', 'ingress', '100', { type: 'number', min: 0 }) +
      fieldRow('出站带宽 Mbit', 'egress', '100', { type: 'number', min: 0 }) +
      fieldRow('月流量限制 GB', 'traffic_limit', '100', { type: 'number', min: 0, placeholder: '0 不限制' }) +
      fieldRow('IPv4 地址池配额', 'ipv4_pool_limit', '0', { type: 'number', min: 0 }) +
      fieldRow('IPv6 地址池配额', 'ipv6_pool_limit', '0', { type: 'number', min: 0 }) +
      fieldRow('IPv4 端口映射配额', 'ipv4_mapping_limit', '0', { type: 'number', min: 0 }) +
      fieldRow('IPv6 端口映射配额', 'ipv6_mapping_limit', '0', { type: 'number', min: 0 }) +
      fieldRow('反向代理配额', 'reverse_proxy_limit', '0', { type: 'number', min: 0 }) +
      '</div>' +
      '<div class="m-section">资源限制</div>' +
      '<div class="m-grid">' +
      fieldRow('CPU 使用限制 %', 'cpu_allowance', '50', { type: 'number', min: 1, max: 100, placeholder: '留空不配置' }) +
      fieldRow('I/O 读取限制 MB', 'io_read', '100', { type: 'number', min: 1, placeholder: '留空不配置' }) +
      fieldRow('I/O 写入限制 MB', 'io_write', '100', { type: 'number', min: 1, placeholder: '留空不配置' }) +
      fieldRow('最大进程数', 'processes_limit', '512', { type: 'number', min: 1, placeholder: '留空不配置' }) +
      '</div>' +
      '<div class="m-grid" style="margin-top:10px">' +
      '<label class="m-chk"><input type="checkbox" name="allow_nesting" checked> 启用嵌套虚拟化</label>' +
      '<label class="m-chk"><input type="checkbox" name="memory_swap" checked> 启用 Swap 虚拟内存</label>' +
      '<label class="m-chk"><input type="checkbox" name="privileged"> 特权模式容器</label>' +
      '</div>' +
      '</form></div>' +
      '<div class="m-foot"><button class="btn btn-ghost btn-sm" onclick="__lxdCloseModal()">取消</button>' +
      '<button class="btn btn-primary btn-sm" id="createSubmit" type="submit" form="createForm">创建容器</button></div>';

    openModal(html, (modal) => {
      const sel = modal.querySelector('#createImage');
      // 镜像列表：本地已缓存镜像（数据库为空时后端自动从 LXD 同步）+ 常用远程镜像
      ADMIN.request('/api/admin/image-options').then((res) => {
        const list = (res.data && res.data.images) || [];
        if (!list.length) { sel.innerHTML = '<option value="">无可用镜像，请先在模板管理中同步</option>'; return; }
        const opt = (t) => '<option value="' + ADMIN.esc(t.value) + '">' + ADMIN.esc(t.label || t.value) + (t.arch ? ' (' + ADMIN.esc(t.arch) + ')' : '') + '</option>';
        const local = list.filter((t) => t.source === 'local');
        const remote = list.filter((t) => t.source !== 'local');
        sel.innerHTML = '<option value="">请选择镜像</option>' +
          (local.length ? '<optgroup label="本地镜像">' + local.map(opt).join('') + '</optgroup>' : '') +
          (remote.length ? '<optgroup label="远程镜像（首次使用需下载）">' + remote.map(opt).join('') + '</optgroup>' : '');
        if (local.length) sel.value = local[0].value;
      }).catch((err) => { sel.innerHTML = '<option value="">加载镜像列表失败：' + ADMIN.esc(err.message || '') + '</option>'; });

      modal.querySelector('#createForm').addEventListener('submit', (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const data = {};
        const NUM = ['cpu', 'memory', 'disk', 'ingress', 'egress', 'traffic_limit', 'ipv4_pool_limit', 'ipv6_pool_limit', 'ipv4_mapping_limit', 'ipv6_mapping_limit', 'reverse_proxy_limit', 'cpu_allowance', 'io_read', 'io_write', 'processes_limit'];
        fd.forEach((v, k) => { data[k] = (k === 'password' && !v) ? '' : (NUM.includes(k) ? (v === '' ? 0 : Number(v)) : v); });
        data.allow_nesting = !!fd.get('allow_nesting');
        data.memory_swap = !!fd.get('memory_swap');
        data.privileged = !!fd.get('privileged');
        const btn = modal.querySelector('#createSubmit');
        btn.disabled = true; btn.textContent = '创建中...';
        ADMIN.request('/api/admin/containers/create', { method: 'POST', body: data })
          .then((res) => {
            LXD.toast('success', '创建任务已提交' + (res.data && res.data.task_id ? '（任务 ' + res.data.task_id + '）' : ''));
            __lxdCloseModal();
            reload && reload();
          })
          .catch((err) => { LXD.toast('error', err.message || '创建失败'); btn.disabled = false; btn.textContent = '创建容器'; });
      });
    });
  }

  /* ================= 修改配置 ================= */
  function openConfigModal(name, reload) {
    const safe = ADMIN.esc(name);
    const html =
      '<div class="m-head"><div class="m-title">配置容器 <span class="mono" style="color:var(--primary)">' + safe + '</span></div><button class="btn btn-ghost btn-sm" onclick="__lxdCloseModal()">✕</button></div>' +
      '<div class="m-body"><div id="cfgLoading" style="color:var(--text-3);font-size:13px;padding:20px;text-align:center">加载配置中...</div>' +
      '<form id="cfgForm" style="display:none">' +
      '<div class="m-section">资源配置</div>' +
      '<div class="m-grid">' +
      fieldRow('CPU 核心数', 'cpu', '') +
      fieldRow('内存 MB', 'memory', '') +
      fieldRow('硬盘 MB', 'disk', '') +
      '</div>' +
      '<div class="m-section">网络与配额</div>' +
      '<div class="m-grid">' +
      fieldRow('入站带宽 Mbit', 'ingress', '') +
      fieldRow('出站带宽 Mbit', 'egress', '') +
      fieldRow('月流量限制 GB', 'traffic_limit', '') +
      fieldRow('IPv4 地址池配额', 'ipv4_pool_limit', '') +
      fieldRow('IPv4 端口映射配额', 'ipv4_mapping_limit', '') +
      fieldRow('IPv6 地址池配额', 'ipv6_pool_limit', '') +
      fieldRow('IPv6 端口映射配额', 'ipv6_mapping_limit', '') +
      fieldRow('反向代理配额', 'reverse_proxy_limit', '') +
      '</div>' +
      '<div class="m-section">资源限制</div>' +
      '<div class="m-grid">' +
      fieldRow('CPU 使用限制 %', 'cpu_allowance', '') +
      fieldRow('I/O 读取限制 MB', 'io_read', '') +
      fieldRow('I/O 写入限制 MB', 'io_write', '') +
      fieldRow('最大进程数', 'processes_limit', '') +
      '</div>' +
      fieldRow('备注', 'remark', '', { placeholder: '最多 20 字符' }) +
      '<div class="m-grid" style="margin-top:10px">' +
      '<label class="m-chk"><input type="checkbox" name="allow_nesting" id="cfgNesting"> 启用嵌套虚拟化</label>' +
      '<label class="m-chk"><input type="checkbox" name="memory_swap" id="cfgSwap"> 启用 Swap 虚拟内存</label>' +
      '<label class="m-chk"><input type="checkbox" name="privileged" id="cfgPriv"> 特权模式容器</label>' +
      '</div>' +
      '</form></div>' +
      '<div class="m-foot"><button class="btn btn-ghost btn-sm" onclick="__lxdCloseModal()">取消</button>' +
      '<button class="btn btn-primary btn-sm" id="cfgSubmit" type="submit" form="cfgForm" disabled>保存配置</button></div>';

    openModal(html, (modal) => {
      ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/config').then((res) => {
        const d = res.data || {};
        const set = (n, v) => { const el = modal.querySelector('[name="' + n + '"]'); if (el && v != null) el.value = v; };
        set('cpu', d.cpu); set('memory', d.memory); set('disk', d.disk);
        set('ingress', d.ingress); set('egress', d.egress); set('traffic_limit', d.traffic_limit);
        set('ipv4_pool_limit', d.ipv4_pool_limit); set('ipv4_mapping_limit', d.ipv4_mapping_limit);
        set('ipv6_pool_limit', d.ipv6_pool_limit); set('ipv6_mapping_limit', d.ipv6_mapping_limit);
        set('reverse_proxy_limit', d.reverse_proxy_limit);
        set('cpu_allowance', d.cpu_allowance); set('io_read', d.io_read); set('io_write', d.io_write); set('processes_limit', d.processes_limit);
        set('remark', d.remark);
        modal.querySelector('#cfgNesting').checked = !!d.allow_nesting;
        modal.querySelector('#cfgSwap').checked = d.memory_swap !== false;
        modal.querySelector('#cfgPriv').checked = !!d.privileged;
        modal.querySelector('#cfgLoading').style.display = 'none';
        modal.querySelector('#cfgForm').style.display = 'block';
        modal.querySelector('#cfgSubmit').disabled = false;
      }).catch((err) => {
        modal.querySelector('#cfgLoading').innerHTML = '<span style="color:var(--red)">加载失败：' + ADMIN.esc(err.message || '未知错误') + '</span>';
      });

      modal.querySelector('#cfgForm').addEventListener('submit', (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const data = {};
        const NUM = ['cpu', 'memory', 'disk', 'ingress', 'egress', 'traffic_limit', 'ipv4_pool_limit', 'ipv6_pool_limit', 'ipv4_mapping_limit', 'ipv6_mapping_limit', 'reverse_proxy_limit', 'cpu_allowance', 'io_read', 'io_write', 'processes_limit'];
        fd.forEach((v, k) => { if (!['allow_nesting', 'memory_swap', 'privileged'].includes(k)) data[k] = NUM.includes(k) ? (v === '' ? 0 : Number(v)) : v; });
        data.allow_nesting = modal.querySelector('#cfgNesting').checked;
        data.memory_swap = modal.querySelector('#cfgSwap').checked;
        data.privileged = modal.querySelector('#cfgPriv').checked;
        const btn = modal.querySelector('#cfgSubmit');
        btn.disabled = true; btn.textContent = '保存中...';
        ADMIN.request('/api/admin/containers/' + encodeURIComponent(name) + '/config', { method: 'POST', body: data })
          .then(() => { LXD.toast('success', '配置已提交'); __lxdCloseModal(); reload && reload(); })
          .catch((err) => { LXD.toast('error', err.message || '保存失败'); btn.disabled = false; btn.textContent = '保存配置'; });
      });
    });
  }

  window.openCreateModal = openCreateModal;
  window.openConfigModal = openConfigModal;
})();
