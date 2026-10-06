/* ============================================================
   LXD Panel - Admin 端口映射 IPv6（对齐 v3 后端 /api/admin/port-mapping?version=v6）
   - 列表 / 分配映射 / 批量释放 / 单个释放
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('port_mapping_v6', '端口映射', 'IPv6 端口映射管理');
  const V = 'v6', KEY = 'ipv6';
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';
  let all = [];
  const selected = new Set();

  function render(list) {
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

    c.innerHTML =
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
      .then(res => { all = (res.data && res.data[KEY]) || []; render(all); })
      .catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
