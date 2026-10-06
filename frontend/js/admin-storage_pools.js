/* ============================================================
   LXD Panel - Admin 存储池管理（对齐 v3 后端 /api/admin/storage-pools）
   - 列表 / 从LXD同步 / 设置优先级（数字越小优先级越高，0=禁用）
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('storage_pools', '存储池', 'LXD 存储池管理');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  function render(list) {
    const rows = list.map(p => {
      const usedPct = p.usage_percent != null ? p.usage_percent : (p.total_space ? Math.round((p.used_space / p.total_space) * 1000) / 10 : 0);
      const color = usedPct > 80 ? 'progress-red' : usedPct > 60 ? 'progress-yellow' : 'progress-green';
      const statusBadge = p.status === 'Created' ? '<span class="badge badge-running">正常</span>'
        : '<span class="badge badge-other">' + LXD.esc(p.status || '-') + '</span>';
      const priBadge = p.priority === 0
        ? '<span class="badge badge-other">禁用</span>'
        : '<span class="badge badge-info">P' + p.priority + '</span>';
      const total = typeof p.total_space === 'number' ? ADMIN.fmt.bytes(p.total_space) : p.total_space;
      const used = typeof p.used_space === 'number' ? ADMIN.fmt.bytes(p.used_space) : p.used_space;
      return '<tr>' +
        '<td><strong>' + LXD.esc(p.name || '-') + '</strong></td>' +
        '<td class="mono">' + LXD.esc(p.driver || '-') + '</td>' +
        '<td>' + LXD.esc(p.description || '-') + '</td>' +
        '<td>' + statusBadge + '</td>' +
        '<td class="mono">' + (p.used_by || 0) + '</td>' +
        '<td style="min-width:180px"><div class="progress"><i class="' + color + '" style="width:' + usedPct + '%"></i></div>' +
        '<div style="font-size:12px;color:var(--text-3)">' + used + ' / ' + total + ' · ' + usedPct + '%</div></td>' +
        '<td>' + priBadge + '</td>' +
        '<td class="row-actions"><button class="btn btn-sm" data-pri="' + LXD.esc(p.name || '') + '" data-val="' + (p.priority || 0) + '">设置优先级</button></td>' +
        '</tr>';
    }).join('');

    c.innerHTML =
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnSync">⇄ 从LXD同步</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReload">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
        '<th>名称</th><th>驱动</th><th>描述</th><th>状态</th><th>容器数</th><th>使用率</th><th>优先级</th><th style="width:120px">操作</th>' +
        '</tr></thead><tbody>' + rows + '</tbody></table></div>' +
        '<div style="display:flex;justify-content:flex-end;margin-top:12px;font-size:13px;color:var(--text-3)">共 ' + list.length + ' 个存储池</div>'
        : '<div class="empty">暂无存储池，点击「从LXD同步」拉取</div>') +
      '</div>';

    document.getElementById('btnSync').addEventListener('click', sync);
    document.getElementById('btnReload').addEventListener('click', load);
    c.querySelectorAll('[data-pri]').forEach(b => b.addEventListener('click', () => showPriority(b.dataset.pri, parseInt(b.dataset.val, 10) || 0)));
  }

  function sync() {
    LXD.confirmDialog('同步存储池', '确认从LXD同步存储池？', false).then(ok => {
      if (!ok) return;
      LXD.toast('info', '正在从LXD同步存储池...');
      ADMIN.request('/api/admin/storage-pools/sync', { method: 'POST' })
        .then(res => {
          const d = res.data || {};
          LXD.toast('success', '同步完成：新增 ' + (d.added || 0) + '，更新 ' + (d.updated || 0) + '，删除 ' + (d.deleted || 0));
          setTimeout(load, 800);
        }).catch(e => LXD.toast('error', e.message));
    });
  }

  function showPriority(name, cur) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML =
      '<div class="modal" style="max-width:380px">' +
      '<h3>设置优先级</h3>' +
      '<p style="color:var(--text-2);font-size:13px;margin-bottom:14px">存储池：' + LXD.esc(name) + '（数字越小优先级越高，0 表示禁用）</p>' +
      '<input class="input" id="priInput" type="number" min="0" step="1" value="' + cur + '">' +
      '<div class="modal-actions">' +
      '<button class="btn btn-ghost" data-act="cancel">取消</button>' +
      '<button class="btn btn-primary" data-act="save">保存</button>' +
      '</div></div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="cancel"]').onclick = () => mask.remove();
    mask.addEventListener('click', e => { if (e.target === mask) mask.remove(); });
    mask.querySelector('[data-act="save"]').onclick = () => {
      const v = parseInt(mask.querySelector('#priInput').value, 10);
      if (isNaN(v)) { LXD.toast('error', '请输入有效数值'); return; }
      ADMIN.request('/api/admin/storage-pools/' + encodeURIComponent(name) + '/priority',
        { method: 'PUT', body: { priority: v } })
        .then(() => { LXD.toast('success', '优先级已更新'); mask.remove(); load(); })
        .catch(e => LXD.toast('error', e.message));
    };
  }

  function load() {
    ADMIN.request('/api/admin/storage-pools')
      .then(res => { const pools = (res.data && res.data.pools) || []; render(pools); })
      .catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
