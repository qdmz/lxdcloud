/* 实例管理 */
(function () {
  'use strict';
  ADMIN.shell('instances', '实例管理', '已开通实例与生命周期');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const STATUS = { creating: '开通中', active: '运行中', suspended: '已暂停', expired: '已到期', deleting: '删除中' };

  function fmtTime(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '-'; }

  function render(list) {
    c.innerHTML = list.length ? list.map(it => {
      let cfg = {};
      try { cfg = it.config_json ? JSON.parse(it.config_json) : {}; } catch (e) {}
      return '<div class="card inst-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(it.product_name || cfg.name || ('商品#' + it.product_id)) + ' <span class="tag">' + (it.type === 'vm' ? 'VM' : '容器') + '</span></div>' +
        '<div class="sub">' + LXD.esc(it.name || '') + ' · 用户#' + it.user_id + ' · ' + LXD.esc((it.region_name ? it.region_name + ' / ' : '') + (it.node_name || ('节点#' + (it.node_id || 0)))) + '</div>' +
        '<div class="sub">到期：' + fmtTime(it.expire_at) + (it.auto_renew ? ' · 自动续费' : '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div class="inst-status ' + (it.status === 'suspended' || it.status === 'expired' ? 'warn' : '') + '">' + (STATUS[it.status] || it.status) + '</div>' +
        '<div class="row-actions"><button class="btn btn-sm btn-danger-ghost" data-del="' + (it.ID || it.id) + '" data-name="' + LXD.esc(it.name || '') + '">删除</button></div>' +
        '</div></div>';
    }).join('') : '<div class="empty">暂无实例</div>';

    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => {
      if (!window.confirm('确定强制删除实例「' + b.dataset.name + '」？底层资源将被移除。')) return;
      ADMIN.request('/api/admin/instances/' + b.dataset.del, { method: 'DELETE' })
        .then(() => { LXD.toast('success', '实例已删除'); load(); }).catch(e => LXD.toast('error', e.message));
    }));
  }

  let nodeFilter = '';
  function mountFilter() {
    ADMIN.request('/api/admin/nodes').then(r => {
      const nodes = (r.data && r.data.nodes) || [];
      if (!nodes.length) return;
      const box = document.querySelector('.topbar-actions');
      if (!box || document.getElementById('instNodeFilter')) return;
      const sel = document.createElement('select');
      sel.id = 'instNodeFilter';
      sel.className = 'form-control';
      sel.style.cssText = 'width:auto;min-width:120px;height:30px;padding:2px 8px;font-size:12.5px';
      sel.innerHTML = '<option value="">全部节点</option>' + nodes.map(n => '<option value="' + (n.ID || n.id) + '">' + LXD.esc((n.region ? n.region + ' / ' : '') + n.name) + '</option>').join('');
      sel.addEventListener('change', () => { nodeFilter = sel.value; load(); });
      box.insertBefore(sel, box.firstChild);
    }).catch(() => {});
  }
  mountFilter();

  function load() {
    ADMIN.request('/api/admin/instances' + (nodeFilter ? '?filter_node=' + encodeURIComponent(nodeFilter) : '')).then(res => render(res.data && res.data.instances || [])).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
