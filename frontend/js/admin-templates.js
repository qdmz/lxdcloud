/* ============================================================
   LXD Panel - Admin 模板管理（对齐 v3 后端 /api/admin/templates）
   - 列表 / 从LXD同步 / 单个删除 / 批量删除 / 用户权限设置
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('templates', '模板管理', '系统镜像模板');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';
  let all = [];
  const selected = new Set();

  function render(list) {
    const rows = list.map(t => {
      const fp = t.fingerprint || '';
      const checked = selected.has(fp) ? ' checked' : '';
      const publicBadge = t.public ? '<span class="badge badge-running">公开</span>' : '<span class="badge badge-other">私有</span>';
      const autoBadge = t.auto_update ? '<span class="badge badge-running">自动</span>' : '<span class="badge badge-other">手动</span>';
      return '<tr>' +
        '<td><input type="checkbox" class="tpl-check" data-fp="' + LXD.esc(fp) + '"' + checked + '></td>' +
        '<td class="mono">' + LXD.esc(t.alias || '-') + '</td>' +
        '<td class="mono" style="max-width:180px;overflow:hidden;text-overflow:ellipsis">' + LXD.esc(fp.slice(0, 12) + (fp.length > 12 ? '…' : '')) + '</td>' +
        '<td class="mono">' + LXD.esc(t.architecture || '-') + '</td>' +
        '<td>' + LXD.esc(t.os || '-') + ' ' + LXD.esc(t.release || '') + '</td>' +
        '<td>' + LXD.esc(t.description || '-') + '</td>' +
        '<td class="mono">' + LXD.esc(t.size_human || (t.size ? ADMIN.fmt.bytes(t.size) : '-')) + '</td>' +
        '<td>' + publicBadge + ' ' + autoBadge + '</td>' +
        '<td class="mono">' + LXD.esc(t.created_at || '-') + '</td>' +
        '<td class="row-actions">' +
        '<button class="btn btn-sm" data-perm="' + LXD.esc(fp) + '" data-alias="' + LXD.esc(t.alias || fp) + '">权限</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + LXD.esc(fp) + '" data-alias="' + LXD.esc(t.alias || fp) + '">删除</button>' +
        '</td></tr>';
    }).join('');

    c.innerHTML =
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      '<button class="btn btn-primary" id="btnSync">⇄ 从LXD同步</button>' +
      '<button class="btn btn-danger-ghost" id="btnBatchDel" ' + (selected.size ? '' : 'disabled') + '>批量删除(' + selected.size + ')</button>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost" id="btnReload">↻ 刷新</button>' +
      '</div></div>' +
      '<div class="card" style="padding:14px">' +
      (list.length ?
        '<div class="tbl-wrap"><table class="tbl"><thead><tr>' +
        '<th style="width:32px"></th><th>名称</th><th>指纹</th><th>架构</th><th>系统</th><th>描述</th><th>大小</th><th>属性</th><th>创建时间</th><th style="width:120px">操作</th>' +
        '</tr></thead><tbody>' + rows + '</tbody></table></div>' +
        '<div style="display:flex;justify-content:flex-end;margin-top:12px;font-size:13px;color:var(--text-3)">共 ' + list.length + ' 个模板</div>'
        : '<div class="empty">暂无模板，点击「从LXD同步」拉取镜像</div>') +
      '</div>';

    bind();
  }

  function bind() {
    document.getElementById('btnSync').addEventListener('click', sync);
    document.getElementById('btnReload').addEventListener('click', load);
    const batchBtn = document.getElementById('btnBatchDel');
    batchBtn.addEventListener('click', () => batchDelete(Array.from(selected)));
    c.querySelectorAll('.tpl-check').forEach(ch => ch.addEventListener('change', () => {
      const fp = ch.dataset.fp;
      if (ch.checked) selected.add(fp); else selected.delete(fp);
      batchBtn.disabled = !selected.size;
      batchBtn.textContent = '批量删除(' + selected.size + ')';
    }));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => {
      const fp = b.dataset.del;
      if (!window.confirm('确定删除模板「' + b.dataset.alias + '」？此操作不可逆！')) return;
      ADMIN.request('/api/admin/templates/' + encodeURIComponent(fp), { method: 'DELETE' })
        .then(() => { LXD.toast('success', '模板已删除'); selected.delete(fp); load(); })
        .catch(e => LXD.toast('error', e.message));
    }));
    c.querySelectorAll('[data-perm]').forEach(b => b.addEventListener('click', () => showPermission(b.dataset.perm, b.dataset.alias)));
  }

  function sync() {
    LXD.confirmDialog('同步模板', '确认从LXD同步镜像模板？将获取所有本地镜像并更新数据库。', false).then(ok => {
      if (!ok) return;
      LXD.toast('info', '正在从LXD同步镜像模板...');
      ADMIN.request('/api/admin/templates/sync', { method: 'POST' })
        .then(res => {
          const d = res.data || {};
          LXD.toast('success', '同步完成：新增 ' + (d.added || 0) + '，更新 ' + (d.updated || 0) + '，删除 ' + (d.deleted || 0));
          setTimeout(load, 800);
        }).catch(e => LXD.toast('error', e.message));
    });
  }

  function batchDelete(fps) {
    if (!fps.length) return;
    LXD.confirmDialog('批量删除模板', '确定删除选中的 ' + fps.length + ' 个模板？此操作不可逆！').then(ok => {
      if (!ok) return;
      ADMIN.request('/api/admin/templates/batch-delete', { method: 'POST', body: { fingerprints: fps } })
        .then(res => {
          const d = res.data || {};
          LXD.toast('success', '批量删除完成：成功 ' + (d.deleted_count || 0) + ' 个');
          selected.clear();
          load();
        }).catch(e => LXD.toast('error', e.message));
    });
  }

  function showPermission(fp, alias) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML =
      '<div class="modal" style="max-width:420px">' +
      '<h3>模板权限设置</h3>' +
      '<p style="color:var(--text-2);font-size:13px;margin-bottom:14px">镜像：' + LXD.esc(alias) + '</p>' +
      '<label style="display:flex;align-items:center;gap:8px;font-size:14px;margin-bottom:10px">' +
      '<input type="radio" name="permMode" value="all" checked> 所有用户可用</label>' +
      '<label style="display:flex;align-items:center;gap:8px;font-size:14px;margin-bottom:10px">' +
      '<input type="radio" name="permMode" value="specific"> 仅指定用户可用</label>' +
      '<div id="permUsersWrap" style="display:none;margin-bottom:12px">' +
      '<input class="input" id="permUsers" placeholder="user1,user2,user3（逗号分隔）">' +
      '<div style="font-size:12px;color:var(--text-3);margin-top:4px">留空表示所有用户可用</div></div>' +
      '<div class="modal-actions">' +
      '<button class="btn btn-ghost" data-act="cancel">取消</button>' +
      '<button class="btn btn-primary" data-act="save">保存</button>' +
      '</div></div>';
    document.body.appendChild(mask);
    const radios = mask.querySelectorAll('input[name="permMode"]');
    radios.forEach(r => r.addEventListener('change', () => {
      mask.querySelector('#permUsersWrap').style.display = mask.querySelector('input[value="specific"]').checked ? '' : 'none';
    }));
    mask.querySelector('[data-act="cancel"]').onclick = () => mask.remove();
    mask.addEventListener('click', e => { if (e.target === mask) mask.remove(); });

    ADMIN.request('/api/admin/templates/' + encodeURIComponent(fp) + '/permission')
      .then(res => {
        const users = (res.data && res.data.allowed_users) || [];
        if (users.length) {
          mask.querySelector('input[value="specific"]').checked = true;
          mask.querySelector('#permUsers').value = users.join(',');
          mask.querySelector('#permUsersWrap').style.display = '';
        }
      }).catch(() => {});

    mask.querySelector('[data-act="save"]').onclick = () => {
      let allowed = [];
      if (mask.querySelector('input[value="specific"]').checked) {
        allowed = mask.querySelector('#permUsers').value.split(',').map(s => s.trim()).filter(Boolean);
      }
      ADMIN.request('/api/admin/templates/' + encodeURIComponent(fp) + '/permission',
        { method: 'PUT', body: { allowed_users: allowed } })
        .then(() => { LXD.toast('success', '权限设置成功'); mask.remove(); })
        .catch(e => LXD.toast('error', e.message));
    };
  }

  function load() {
    ADMIN.request('/api/admin/templates')
      .then(res => { all = (res.data && res.data.templates) || []; render(all); })
      .catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
