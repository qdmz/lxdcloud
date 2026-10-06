/* 订单管理 */
(function () {
  'use strict';
  ADMIN.shell('orders', '订单管理', '全站订单与收款记录');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const STATUS = { pending: '待支付', paid: '已支付', cancelled: '已取消', refunded: '已退款' };
  const PERIOD = { monthly: '月付', quarterly: '季付', half_year: '半年付', yearly: '年付' };

  function render(list) {
    c.innerHTML = list.length ? list.map(o => {
      let info = {};
      try { info = o.info_json ? JSON.parse(o.info_json) : {}; } catch (e) {}
      const uName = o.username || info.username || ('用户#' + o.user_id);
      return '<div class="card order-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(info.product_name || ('商品#' + o.product_id)) + '</div>' +
        '<div class="sub">订单号：' + LXD.esc(o.order_no || '') + ' · ' + LXD.esc(uName) + ' · ' + (PERIOD[o.period] || o.period || '') + '</div>' +
        '<div class="sub">' + String(o.created_at || '').replace('T', ' ').slice(0, 16) + (o.paid_at ? ' · 支付 ' + String(o.paid_at).replace('T', ' ').slice(0, 16) : '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div style="font-weight:700;color:var(--accent)">¥' + Number(o.amount || 0).toFixed(2) + '</div>' +
        '<div class="sub">' + (STATUS[o.status] || o.status) + '</div></div></div>';
    }).join('') : '<div class="empty">暂无订单</div>';
  }

  function load() {
    ADMIN.request('/api/admin/orders').then(res => render(res.data && res.data.orders || [])).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
