/* ============================================================
   LXD Panel - Admin 反向代理（对齐 nginx 插件 /api/admin/nginx）
   - Nginx状态 / 代理列表 / 新建 / 编辑 / 单个删除 / 批量删除 / 域名/IP限制设置
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('nginx', '反向代理', 'Nginx 反向代理管理');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';
  let all = [];
  const selected = new Set();

  function render(list, status) {
    const st = status || {};
    const rows = list.map(p => {
      const checked = selected.has(p.id) ? ' checked' : '';
      const stBadge = p.status === 'active'
        ? '<span class="badge badge-running">生效</span>'
        : '<span class="badge badge-other">' + LXD.esc(p.status || '-') + '</span>';
      const sslBadge = p.enable_ssl || p.protocol === 'https'
        ? '<span class="badge badge-info">SSL</span>' : '<span class="badge badge-other">HTTP</span>';
      return '<tr>' +
        '<td><input type="checkbox" class="px-check" data-id="' + p.id + '"' + checked + '></td>' +
        '<td class="mono">#' + p.id + '</td>' +
        '<td class="mono"><strong>' + LXD.esc(p.container_name || '-') + '</strong></td>' +
        '<td>' + sslBadge + '</td>' +
        '<td class="mono">' + LXD.esc(p.protocol || '-') + '://' + LXD.esc(p.domain || '-') + (p.public_port && p.public_port !== 80 && p.public_port !== 443 ? ':' + p.public_port : '') + '</td>' +
        '<td class="mono">' + LXD.esc(p.target_ip || '-') + ':' + (p.target_port || '-') + '</td>' +
        '<td>' + stBadge + '</td>' +
        '<td>' + LXD.esc(p.description || '-') + '</td>' +
        '<td class="mono">' + LXD.esc(p.created_at || '-') + '</td>' +
        '<td class="row-actions">' +
        '<button class="btn btn-sm" data-edit="' + p.id + '">编辑</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + p.id + '" data-name="' + LXD.esc(p.domain || '') + '">删除</button>' +
        '</td></tr>';
    }).join('');

    const statusBar = (st.running != null)
      ? '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
        (st.running ? '<span class="badge badge-running">Nginx 运行中</span>' : '<span class="badge badge-other">Nginx 未运行</span>') +
        '<span class="badge badge-info">版本 ' + LXD.esc(st.version || '-') + '</span>' +
        '<span class="badge">代理 ' + (st.active_proxies || 0) + '/' + (st.total_proxies || 0) + '</span>' +
        (st.uptime ? '<span class="badge">运行时长 ' + LXD.esc(st.uptime) + '</span>' : '') +
        '<span class="grow"></span>' +
        '<button class="btn btn-sm" id="btnReloadNginx">reload</button>' +
        '</div></div>'
      : '';

    c.innerHTML = statusBar +
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnNew">＋ 新建代理</button>' +
      '<button class="btn btn-ghost" id="btnRestrict">⛔ 域名/IP限制设置</button>' +
      '<button class="btn btn-danger-ghost" id="btnBatchDel" ' + (selected.size ? '' : 'disabled') + '>批量删除(' + selected.size + ')</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReload">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
        '<th style="width:32px"></th><th>ID</th><th>容器</th><th>协议</th><th>域名</th><th>目标</th><th>状态</th><th>描述</th><th>创建时间</th><th style="width:120px">操作</th>' +
        '</tr></thead><tbody>' + rows + '</tbody></table></div>' +
        '<div style="display:flex;justify-content:flex-end;margin-top:12px;font-size:13px;color:var(--text-3)">共 ' + list.length + ' 条代理规则</div>'
        : '<div class="empty">暂无反向代理，点击「新建代理」创建</div>') +
      '</div>';

    document.getElementById('btnNew').addEventListener('click', () => formProxy(null));
    document.getElementById('btnRestrict').addEventListener('click', formRestrictions);
    document.getElementById('btnReload').addEventListener('click', load);
    if (document.getElementById('btnReloadNginx')) {
      document.getElementById('btnReloadNginx').addEventListener('click', () => {
        ADMIN.request('/api/admin/nginx/reload', { method: 'POST' })
          .then(() => LXD.toast('success', 'Nginx 已重载')).catch(e => LXD.toast('error', e.message));
      });
    }
    const batchBtn = document.getElementById('btnBatchDel');
    batchBtn.addEventListener('click', () => batchDelete(Array.from(selected)));
    c.querySelectorAll('.px-check').forEach(ch => ch.addEventListener('change', () => {
      const id = parseInt(ch.dataset.id, 10);
      if (ch.checked) selected.add(id); else selected.delete(id);
      batchBtn.disabled = !selected.size;
      batchBtn.textContent = '批量删除(' + selected.size + ')';
    }));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => {
      if (!window.confirm('确定删除代理规则「' + b.dataset.name + '」？')) return;
      ADMIN.request('/api/admin/nginx/proxies/' + b.dataset.del, { method: 'DELETE' })
        .then(() => { LXD.toast('success', '已删除'); selected.delete(parseInt(b.dataset.del, 10)); load(); })
        .catch(e => LXD.toast('error', e.message));
    }));
    c.querySelectorAll('[data-edit]').forEach(b => b.addEventListener('click', () => {
      const p = all.find(x => x.id === parseInt(b.dataset.edit, 10)) || {};
      formProxy(p);
    }));
  }

  function modal(html, onSave) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:520px">' + html + '</div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="cancel"]').onclick = () => mask.remove();
    mask.addEventListener('click', e => { if (e.target === mask) mask.remove(); });
    mask.querySelector('[data-act="save"]').onclick = () => onSave(mask);
    return mask;
  }

  function formProxy(p) {
    const isEdit = p && p.id;
    const p2 = p || {};
    modal(
      '<h3>' + (isEdit ? '编辑代理 #' + p2.id : '新建反向代理') + '</h3>' +
      '<div class="form-grid">' +
      '<div class="field"><label>容器名称 *</label><input class="input" id="fCtn" value="' + LXD.esc(p2.container_name || '') + '"' + (isEdit ? ' disabled' : '') + '></div>' +
      '<div class="field"><label>协议 *</label><select class="input" id="fProto"><option value="http"' + (p2.protocol !== 'https' ? ' selected' : '') + '>http</option><option value="https"' + (p2.protocol === 'https' ? ' selected' : '') + '>https</option></select></div>' +
      '<div class="field" style="grid-column:1/-1"><label>域名 *</label><input class="input" id="fDomain" value="' + LXD.esc(p2.domain || '') + '" placeholder="例如 www.example.com"></div>' +
      '<div class="field"><label>目标 IP</label><input class="input" id="fTargetIP" value="' + LXD.esc(p2.target_ip || '') + '" placeholder="留空自动取容器IP"></div>' +
      '<div class="field"><label>目标端口</label><input class="input" id="fTargetPort" type="number" min="1" value="' + (p2.target_port || 80) + '"></div>' +
      '<div class="field"><label>状态</label><select class="input" id="fStatus"><option value="active"' + (p2.status !== 'disabled' ? ' selected' : '') + '>生效</option><option value="disabled"' + (p2.status === 'disabled' ? ' selected' : '') + '>停用</option></select></div>' +
      '<div class="field" style="grid-column:1/-1"><label>描述</label><input class="input" id="fDesc" value="' + LXD.esc(p2.description || '') + '"></div>' +
      '<div class="field" style="grid-column:1/-1"><label>自定义配置（可选，Nginx location 片段）</label><textarea class="input" id="fCustom" rows="3">' + LXD.esc(p2.custom_config || '') + '</textarea></div>' +
      '<div class="field" style="grid-column:1/-1;display:flex;align-items:center;gap:8px"><input type="checkbox" id="fSSL" ' + (p2.enable_ssl || p2.protocol === 'https' ? 'checked' : '') + '> <label style="margin:0">启用 SSL（https 时必填证书）</label></div>' +
      '<div class="field" style="grid-column:1/-1"><label>SSL 证书（PEM）</label><textarea class="input" id="fCert" rows="3">' + LXD.esc(p2.ssl_cert || '') + '</textarea></div>' +
      '<div class="field" style="grid-column:1/-1"><label>SSL 私钥（PEM）</label><textarea class="input" id="fKey" rows="3">' + LXD.esc(p2.ssl_key || '') + '</textarea></div>' +
      '</div>' +
      '<div style="font-size:12px;color:var(--text-3)">http 固定监听 80，https 固定监听 443；公网端口由协议自动决定</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">' + (isEdit ? '保存' : '创建') + '</button></div>',
      mask => {
        const proto = mask.querySelector('#fProto').value;
        const body = {
          container_name: mask.querySelector('#fCtn').value.trim(),
          protocol: proto,
          domain: mask.querySelector('#fDomain').value.trim(),
          public_port: proto === 'https' ? 443 : 80,
          target_ip: mask.querySelector('#fTargetIP').value.trim(),
          target_port: parseInt(mask.querySelector('#fTargetPort').value, 10) || 80,
          enable_ssl: mask.querySelector('#fSSL').checked,
          ssl_cert: mask.querySelector('#fCert').value,
          ssl_key: mask.querySelector('#fKey').value,
          custom_config: mask.querySelector('#fCustom').value,
          description: mask.querySelector('#fDesc').value.trim(),
          status: mask.querySelector('#fStatus').value
        };
        if (!body.container_name || !body.domain) { LXD.toast('error', '容器名称与域名必填'); return; }
        if (proto === 'https' && (!body.ssl_cert || !body.ssl_key)) { LXD.toast('error', 'https 需要填写 SSL 证书与私钥'); return; }
        const url = isEdit ? '/api/admin/nginx/proxies/' + p2.id : '/api/admin/nginx/proxies';
        ADMIN.request(url, { method: isEdit ? 'PUT' : 'POST', body: body })
          .then(() => { LXD.toast('success', isEdit ? '代理已更新' : '代理已创建'); mask.remove(); load(); })
          .catch(e => LXD.toast('error', e.message));
      });
  }

  /** 域名/IP限制设置（GET/PUT /api/admin/nginx/restrictions） */
  function formRestrictions() {
    const m = modal('<h3>域名/IP限制设置</h3>' +
      '<div class="loading" style="padding:20px">加载中...</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save" disabled>保存</button></div>', () => {});
    ADMIN.request('/api/admin/nginx/restrictions').then(res => {
      const d = res.data || {};
      m.querySelector('.loading').outerHTML =
        '<div class="form-grid">' +
        '<div class="field" style="grid-column:1/-1"><label>受限域名（每行一个，命中则禁止创建代理）</label><textarea class="input" id="fDomains" rows="6" placeholder="例如：&#10;example.com&#10;test.cn"></textarea></div>' +
        '<div class="field" style="grid-column:1/-1"><label>受限 IP（每行一个，命中目标 IP 则禁止创建代理）</label><textarea class="input" id="fIPs" rows="6" placeholder="例如：&#10;1.2.3.4&#10;5.6.7.8"></textarea></div>' +
        '</div>' +
        '<div style="font-size:12px;color:var(--text-3)">限制生效于新建代理时；修改后请点击「reload」应用配置</div>';
      m.querySelector('#fDomains').value = d.restricted_domains || '';
      m.querySelector('#fIPs').value = d.restricted_ips || '';
      const saveBtn = m.querySelector('[data-act="save"]');
      saveBtn.disabled = false;
      saveBtn.onclick = () => {
        ADMIN.request('/api/admin/nginx/restrictions', {
          method: 'PUT',
          body: {
            restricted_domains: m.querySelector('#fDomains').value,
            restricted_ips: m.querySelector('#fIPs').value
          }
        }).then(() => { LXD.toast('success', '限制设置已保存'); m.remove(); })
          .catch(e => LXD.toast('error', e.message));
      };
    }).catch(e => {
      m.querySelector('.loading').outerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }

  function batchDelete(ids) {
    if (!ids.length) return;
    LXD.confirmDialog('批量删除', '确定删除选中的 ' + ids.length + ' 条代理规则？').then(ok => {
      if (!ok) return;
      ADMIN.request('/api/admin/nginx/proxies/batch-delete', { method: 'POST', body: { ids: ids } })
        .then(() => { LXD.toast('success', '批量删除完成'); selected.clear(); load(); })
        .catch(e => LXD.toast('error', e.message));
    });
  }

  function load() {
    ADMIN.request('/api/admin/nginx/proxies')
      .then(res => { all = Array.isArray(res.data) ? res.data : []; return ADMIN.request('/api/admin/nginx/status').catch(() => ({ data: null })); })
      .then(r2 => render(all, r2.data || {}))
      .catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
