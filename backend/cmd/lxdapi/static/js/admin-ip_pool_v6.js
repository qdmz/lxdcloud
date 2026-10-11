/* ============================================================
   LXD Panel - Admin IP地址池 IPv6（对齐 v3 后端 /api/admin/ip/pool?version=v6）
   - 列表 / 添加 / 批量添加 / 批量删除 / 编辑备注 / 单个删除
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('ip_pool_v6', 'IP地址池', 'IPv6 地址管理');
  const V = 'v6';
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';
  let all = [];
  const selected = new Set();

  function render(list) {
    const rows = list.map(p => {
      const addr = p.ip_address || p.IPAddress || '';
      const status = (p.status || p.Status || 'available').toLowerCase();
      const stBadge = status === 'available' ? '<span class="badge badge-running">可用</span>'
        : status === 'used' || status === 'in_use' ? '<span class="badge badge-warn">使用中</span>'
        : '<span class="badge badge-other">' + LXD.esc(status) + '</span>';
      const checked = selected.has(addr) ? ' checked' : '';
      return '<tr>' +
        '<td><input type="checkbox" class="ip-check" data-ip="' + LXD.esc(addr) + '"' + checked + '></td>' +
        '<td class="mono"><strong>' + LXD.esc(addr) + '</strong></td>' +
        '<td class="mono">' + LXD.esc(p.interface || p.Interface || '-') + '</td>' +
        '<td>' + stBadge + '</td>' +
        '<td>' + LXD.esc(p.note || p.Note || '-') + '</td>' +
        '<td class="row-actions">' +
        '<button class="btn btn-sm" data-edit="' + LXD.esc(addr) + '" data-note="' + LXD.esc(p.note || '') + '">备注</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + LXD.esc(addr) + '">删除</button>' +
        '</td></tr>';
    }).join('');

    c.innerHTML =
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnAdd">＋ 添加IP</button>' +
      '<button class="btn" id="btnBatchAdd">批量添加</button>' +
      '<button class="btn btn-danger-ghost" id="btnBatchDel" ' + (selected.size ? '' : 'disabled') + '>批量删除(' + selected.size + ')</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReload">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
        '<th style="width:32px"></th><th>IP 地址</th><th>接口</th><th>状态</th><th>备注</th><th style="width:120px">操作</th>' +
        '</tr></thead><tbody>' + rows + '</tbody></table></div>' +
        '<div style="display:flex;justify-content:flex-end;margin-top:12px;font-size:13px;color:var(--text-3)">共 ' + list.length + ' 个地址</div>'
        : '<div class="empty">暂无 IP，点击「添加IP」或「批量添加」录入</div>') +
      '</div>';

    document.getElementById('btnAdd').addEventListener('click', () => formAdd());
    document.getElementById('btnBatchAdd').addEventListener('click', () => formBatchAdd());
    document.getElementById('btnReload').addEventListener('click', load);
    const batchBtn = document.getElementById('btnBatchDel');
    batchBtn.addEventListener('click', () => batchDelete(Array.from(selected)));
    c.querySelectorAll('.ip-check').forEach(ch => ch.addEventListener('change', () => {
      const ip = ch.dataset.ip;
      if (ch.checked) selected.add(ip); else selected.delete(ip);
      batchBtn.disabled = !selected.size;
      batchBtn.textContent = '批量删除(' + selected.size + ')';
    }));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => {
      if (!window.confirm('确定从IP池删除 ' + b.dataset.del + '？')) return;
      ADMIN.request('/api/admin/ip/pool?version=' + V, { method: 'DELETE', body: { ip_address: b.dataset.del } })
        .then(() => { LXD.toast('success', '已删除'); selected.delete(b.dataset.del); load(); })
        .catch(e => LXD.toast('error', e.message));
    }));
    c.querySelectorAll('[data-edit]').forEach(b => b.addEventListener('click', () => formEdit(b.dataset.edit, b.dataset.note)));
  }

  function modal(html, onSave) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:440px">' + html + '</div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="cancel"]').onclick = () => mask.remove();
    mask.addEventListener('click', e => { if (e.target === mask) mask.remove(); });
    mask.querySelector('[data-act="save"]').onclick = () => onSave(mask);
    return mask;
  }

  function formAdd() {
    modal(
      '<h3>添加 IP 地址</h3>' +
      '<div class="form-grid">' +
      '<div class="field" style="grid-column:1/-1"><label>IP 地址</label><input class="input" id="fIP" placeholder="例如 2001:db8::1"></div>' +
      '<div class="field"><label>接口</label><input class="input" id="fIface" placeholder="例如 eth0"></div>' +
      '<div class="field"><label>备注</label><input class="input" id="fNote" placeholder="可选"></div>' +
      '</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">添加</button></div>',
      mask => {
        const ip = mask.querySelector('#fIP').value.trim();
        if (!ip) { LXD.toast('error', 'IP 地址必填'); return; }
        ADMIN.request('/api/admin/ip/pool?version=' + V, {
          method: 'POST',
          body: { ip_address: ip, interface: mask.querySelector('#fIface').value.trim(), note: mask.querySelector('#fNote').value.trim() }
        }).then(() => { LXD.toast('success', '添加成功'); mask.remove(); load(); }).catch(e => LXD.toast('error', e.message));
      });
  }

  function formBatchAdd() {
    modal(
      '<h3>批量添加 IP 地址</h3>' +
      '<div class="form-grid">' +
      '<div class="field"><label>起始 IP</label><input class="input" id="fStart" placeholder="例如 2001:db8::1"></div>' +
      '<div class="field"><label>数量</label><input class="input" id="fCount" type="number" min="1" placeholder="例如 100"></div>' +
      '<div class="field"><label>接口</label><input class="input" id="fIface" placeholder="例如 eth0"></div>' +
      '<div class="field"><label>备注</label><input class="input" id="fNote" placeholder="可选"></div>' +
      '</div>' +
      '<div style="font-size:12px;color:var(--text-3)">将从起始地址开始连续生成指定数量的 IPv6 地址</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">批量添加</button></div>',
      mask => {
        const start = mask.querySelector('#fStart').value.trim();
        const count = parseInt(mask.querySelector('#fCount').value, 10);
        if (!start || !count || count < 1) { LXD.toast('error', '起始 IP 与数量必填'); return; }
        ADMIN.request('/api/admin/ip/pool/batch?version=' + V, {
          method: 'POST',
          body: { start_ip: start, count: count, interface: mask.querySelector('#fIface').value.trim(), note: mask.querySelector('#fNote').value.trim() }
        }).then(res => {
          const d = res.data || {};
          LXD.toast('success', '批量添加完成：成功 ' + (d.added || 0) + ' 个');
          mask.remove(); load();
        }).catch(e => LXD.toast('error', e.message));
      });
  }

  function formEdit(ip, note) {
    modal(
      '<h3>编辑备注</h3>' +
      '<p style="color:var(--text-2);font-size:13px;margin-bottom:10px">' + LXD.esc(ip) + '</p>' +
      '<input class="input" id="fNote" value="' + LXD.esc(note) + '">' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-primary" data-act="save">保存</button></div>',
      mask => {
        ADMIN.request('/api/admin/ip/pool?version=' + V, {
          method: 'PUT',
          body: { ip_address: ip, note: mask.querySelector('#fNote').value.trim() }
        }).then(() => { LXD.toast('success', '备注已更新'); mask.remove(); load(); }).catch(e => LXD.toast('error', e.message));
      });
  }

  function batchDelete(ips) {
    if (!ips.length) return;
    LXD.confirmDialog('批量删除', '确定删除选中的 ' + ips.length + ' 个 IP 地址？').then(ok => {
      if (!ok) return;
      ADMIN.request('/api/admin/ip/pool/batch-delete?version=' + V, { method: 'POST', body: { ip_addresses: ips } })
        .then(() => { LXD.toast('success', '批量删除完成'); selected.clear(); load(); })
        .catch(e => LXD.toast('error', e.message));
    });
  }

  function load() {
    ADMIN.request('/api/admin/ip/pool?version=' + V)
      .then(res => { all = (res.data && res.data.pools) || []; render(all); })
      .catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
