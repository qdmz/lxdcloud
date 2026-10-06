/* 我的实例：列表 / 续费 / 删除 */
(function () {
  'use strict';
  USER.shell('instances', '我的实例', '已购容器与虚拟机');
  const c = document.getElementById('userContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const STATUS = { creating: '开通中', active: '运行中', suspended: '已暂停', expired: '已到期', deleting: '删除中' };
  const PERIOD = { monthly: '月付', quarterly: '季付', half_year: '半年付', yearly: '年付' };

  function fmtTime(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '-'; }

  function render(list) {
    if (!list.length) { c.innerHTML = '<div class="empty">暂无实例，去 <a href="store.html" style="color:var(--accent)">产品订购</a> 看看吧</div>'; return; }
    c.innerHTML = list.map(it => {
      let cfg = {};
      try { cfg = it.config_json ? JSON.parse(it.config_json) : {}; } catch (e) {}
      const productName = it.product_name || (cfg.name || ('商品#' + it.product_id));
      const expired = it.status === 'active' && it.expire_at && new Date(it.expire_at) < new Date();
      const statusText = expired ? '已到期' : (STATUS[it.status] || it.status);
      return '<div class="card inst-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(productName) + ' <span class="tag">' + (it.type === 'vm' ? 'VM' : '容器') + '</span></div>' +
        '<div class="sub">' + LXD.esc(it.name || '') + ' · 节点#' + (it.node_id || 0) + '</div>' +
        '<div class="sub">到期：' + fmtTime(it.expire_at) + (it.auto_renew ? ' · 自动续费' : '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div class="inst-status ' + (expired || it.status === 'suspended' ? 'warn' : '') + '">' + statusText + '</div>' +
        '<div class="row-actions">' +
        '<button class="btn btn-sm" data-renew="' + it.id + '">续费</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + it.id + '" data-name="' + LXD.esc(it.name || '') + '">删除</button>' +
        '</div></div></div>';
    }).join('');

    c.querySelectorAll('[data-renew]').forEach(b => b.addEventListener('click', () => renew(b.dataset.renew)));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => del(b.dataset.del, b.dataset.name)));
  }

  function renew(id) {
    const per = prompt('选择续费周期：monthly / quarterly / half_year / yearly');
    if (!per) return;
    USER.request('/api/user/instances/' + id + '/renew', { method: 'POST', body: { period: per } }).then(res => {
      const url = res.data && res.data.pay_url;
      if (url) { window.location.href = url; return; }
      LXD.toast('success', '续费成功');
      load();
    }).catch(e => LXD.toast('error', e.message));
  }

  function del(id, name) {
    if (!window.confirm('确定删除实例「' + name + '」？此操作不可恢复，底层容器/VM 将被移除。')) return;
    USER.request('/api/user/instances/' + id, { method: 'DELETE' }).then(() => {
      LXD.toast('success', '实例已删除');
      load();
    }).catch(e => LXD.toast('error', e.message));
  }

  function load() {
    USER.request('/api/user/instances').then(res => render(res.data && res.data.instances || [])).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
