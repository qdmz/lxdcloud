/* ============================================================
   LXD Panel - Admin 用户创建/编辑弹窗
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
      (opts.req ? 'required ' : '') +
      (opts.readonly ? 'readonly ' : '') +
      'class="input" style="width:100%;margin-top:5px"></div>';
  }

  function quotaSection() {
    return '' +
      '<div class="m-section">资源配置</div>' +
      '<div class="m-grid">' +
      fieldRow('CPU 核心数', 'cpu', '1', { type: 'number', min: 0 }) +
      fieldRow('内存 MB', 'memory', '256', { type: 'number', min: 0 }) +
      fieldRow('硬盘 MB', 'disk', '512', { type: 'number', min: 0 }) +
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
      '</div>';
  }

  /** 兼容大小写混合字段 */
  function uget(u, keys, def) {
    for (const k of keys) {
      if (u[k] !== undefined && u[k] !== null) return u[k];
    }
    return def;
  }

  /* ================= 创建用户 ================= */
  function openCreate(reload) {
    const html =
      '<div class="m-head"><div class="m-title">＋ 新建用户</div><button class="btn btn-ghost btn-sm" onclick="__lxdCloseModal()">✕</button></div>' +
      '<div class="m-body"><form id="userForm">' +
      '<div class="m-section">账号信息</div>' +
      '<div class="m-grid" style="grid-template-columns:repeat(auto-fit,minmax(200px,1fr))">' +
      fieldRow('用户名', 'username', '', { req: true, placeholder: '登录用户名' }) +
      fieldRow('初始密码', 'password', '', { placeholder: '留空自动生成' }) +
      '</div>' +
      '<div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-top:10px">' +
      '<div><label style="font-size:12px;color:var(--text-3)">状态</label>' +
      '<select name="status" class="input" style="width:100%;margin-top:5px">' +
      '<option value="enabled">正常</option><option value="disabled">禁用</option></select></div>' +
      fieldRow('备注', 'remark', '', { placeholder: '最多 20 字符' }) +
      '</div>' +
      quotaSection() +
      '</form></div>' +
      '<div class="m-foot"><button class="btn btn-ghost btn-sm" onclick="__lxdCloseModal()">取消</button>' +
      '<button class="btn btn-primary btn-sm" id="userSubmit" type="submit" form="userForm">创建用户</button></div>';

    openModal(html, (modal) => {
      modal.querySelector('#userForm').addEventListener('submit', (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const data = {};
        const NUM = ['cpu', 'memory', 'disk', 'ingress', 'egress', 'traffic_limit', 'ipv4_pool_limit', 'ipv6_pool_limit', 'ipv4_mapping_limit', 'ipv6_mapping_limit', 'reverse_proxy_limit'];
        fd.forEach((v, k) => { data[k] = (k === 'password' && !v) ? '' : (NUM.includes(k) ? (v === '' ? 0 : Number(v)) : v); });
        const btn = modal.querySelector('#userSubmit');
        btn.disabled = true; btn.textContent = '创建中...';
        ADMIN.request('/api/admin/users', { method: 'POST', body: data })
          .then((res) => {
            LXD.toast('success', '用户创建成功' + (res.data && res.data.username ? '：' + res.data.username : ''));
            __lxdCloseModal();
            reload && reload();
          })
          .catch((err) => { LXD.toast('error', err.message || '创建失败'); btn.disabled = false; btn.textContent = '创建用户'; });
      });
    });
  }

  /* ================= 编辑用户 ================= */
  function openEdit(id, reload) {
    const safe = ADMIN.esc(String(id));
    const html =
      '<div class="m-head"><div class="m-title">编辑用户 <span class="mono" style="color:var(--primary)">#' + safe + '</span></div><button class="btn btn-ghost btn-sm" onclick="__lxdCloseModal()">✕</button></div>' +
      '<div class="m-body"><div id="usrLoading" style="color:var(--text-3);font-size:13px;padding:20px;text-align:center">加载用户信息中...</div>' +
      '<form id="userForm" style="display:none">' +
      '<div class="m-section">账号信息</div>' +
      '<div class="m-grid" style="grid-template-columns:repeat(auto-fit,minmax(200px,1fr))">' +
      fieldRow('用户名', 'username', '', { readonly: true, placeholder: '-' }) +
      fieldRow('重置密码', 'password', '', { placeholder: '留空不修改' }) +
      '</div>' +
      '<div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-top:10px">' +
      '<div><label style="font-size:12px;color:var(--text-3)">状态</label>' +
      '<select name="status" id="usrStatus" class="input" style="width:100%;margin-top:5px">' +
      '<option value="enabled">正常</option><option value="disabled">禁用</option></select></div>' +
      fieldRow('备注', 'remark', '', { placeholder: '最多 20 字符' }) +
      '</div>' +
      quotaSection() +
      '</form></div>' +
      '<div class="m-foot"><button class="btn btn-ghost btn-sm" onclick="__lxdCloseModal()">取消</button>' +
      '<button class="btn btn-primary btn-sm" id="userSubmit" type="submit" form="userForm" disabled>保存</button></div>';

    openModal(html, (modal) => {
      ADMIN.request('/api/admin/users/' + encodeURIComponent(id)).then((res) => {
        const d = res.data || {};
        const u = d.user || d.User || d;
        const set = (n, v) => { const el = modal.querySelector('[name="' + n + '"]'); if (el && v != null) el.value = v; };
        set('username', uget(u, ['username', 'Username', 'name'], ''));
        set('remark', uget(u, ['remark', 'Remark', 'Nickname'], ''));
        set('status', String(uget(u, ['status', 'Status'], 'enabled')).toLowerCase() === 'disabled' ? 'disabled' : 'enabled');
        set('cpu', uget(u, ['cpu', 'CPU', 'cpu_quota', 'CPUQuota'], null));
        set('memory', uget(u, ['memory', 'memory_quota', 'MemoryQuota'], null));
        set('disk', uget(u, ['disk', 'disk_quota', 'DiskQuota'], null));
        set('ingress', uget(u, ['ingress', 'Ingress'], null));
        set('egress', uget(u, ['egress', 'Egress'], null));
        set('traffic_limit', uget(u, ['traffic_limit', 'TrafficLimit'], null));
        set('ipv4_pool_limit', uget(u, ['ipv4_pool_limit', 'IPv4PoolLimit'], null));
        set('ipv6_pool_limit', uget(u, ['ipv6_pool_limit', 'IPv6PoolLimit'], null));
        set('ipv4_mapping_limit', uget(u, ['ipv4_mapping_limit', 'IPv4MappingLimit'], null));
        set('ipv6_mapping_limit', uget(u, ['ipv6_mapping_limit', 'IPv6MappingLimit'], null));
        set('reverse_proxy_limit', uget(u, ['reverse_proxy_limit', 'ReverseProxyLimit'], null));
        modal.querySelector('#usrLoading').style.display = 'none';
        modal.querySelector('#userForm').style.display = 'block';
        modal.querySelector('#userSubmit').disabled = false;
      }).catch((err) => {
        modal.querySelector('#usrLoading').innerHTML = '<span style="color:var(--red)">加载失败：' + ADMIN.esc(err.message || '未知错误') + '</span>';
      });

      modal.querySelector('#userForm').addEventListener('submit', (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const data = {};
        const NUM = ['cpu', 'memory', 'disk', 'ingress', 'egress', 'traffic_limit', 'ipv4_pool_limit', 'ipv6_pool_limit', 'ipv4_mapping_limit', 'ipv6_mapping_limit', 'reverse_proxy_limit'];
        fd.forEach((v, k) => { if (k !== 'username' && !(k === 'password' && !v)) data[k] = NUM.includes(k) ? (v === '' ? 0 : Number(v)) : v; });
        const btn = modal.querySelector('#userSubmit');
        btn.disabled = true; btn.textContent = '保存中...';
        ADMIN.request('/api/admin/users/' + encodeURIComponent(id), { method: 'POST', body: data })
          .then(() => { LXD.toast('success', '已保存'); __lxdCloseModal(); reload && reload(); })
          .catch((err) => { LXD.toast('error', err.message || '保存失败'); btn.disabled = false; btn.textContent = '保存'; });
      });
    });
  }

  window.openUserModalSafe = (mode, id, reload) => {
    if (mode === 'edit') openEdit(id, reload);
    else openCreate(reload);
  };
})();
