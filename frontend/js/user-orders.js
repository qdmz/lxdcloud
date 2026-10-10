/* 我的订单 */
(function () {
  'use strict';
  USER.shell('orders', '我的订单', '订单列表与支付');
  const c = document.getElementById('userContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const STATUS = { pending: '待支付', paid: '已支付', cancelled: '已取消', refunded: '已退款' };
  const PERIOD = { monthly: '月付', quarterly: '季付', half_year: '半年付', halfyear: '半年付', yearly: '年付' };

  function render(list) {
    if (!list.length) { c.innerHTML = '<div class="empty">暂无订单</div>'; return; }
    c.innerHTML = list.map(o => {
      o.id = o.id || o.ID;
      let info = {};
      try { info = o.info_json ? JSON.parse(o.info_json) : {}; } catch (e) {}
      const amount = o.amount !== undefined ? o.amount : info.amount;
      return '<div class="card order-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(info.product_name || o.product_name || ('商品#' + o.product_id)) + '</div>' +
        '<div class="sub">订单号：' + LXD.esc(o.order_no || '') + ' · ' + (PERIOD[o.period] || o.period || '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div style="font-weight:700;color:var(--accent)">¥' + Number(amount || 0).toFixed(2) + '</div>' +
        '<div class="sub">' + (STATUS[o.status] || o.status) + '</div>' +
        (o.status === 'pending' ? '<div class="row-actions"><button class="btn btn-sm btn-primary" data-pay="' + o.id + '">去支付</button>' +
          '<button class="btn btn-sm btn-ghost" data-cancel="' + o.id + '">取消</button></div>' : '') +
        '</div></div>';
    }).join('');

    c.querySelectorAll('[data-pay]').forEach(b => b.addEventListener('click', () => pay(b.dataset.pay)));
    c.querySelectorAll('[data-cancel]').forEach(b => b.addEventListener('click', () => {
      LXD.confirmDialog('取消订单', '确定取消该订单吗？取消后需重新下单。').then(ok => {
        if (!ok) return;
        USER.request('/api/user/orders/' + b.dataset.cancel + '/cancel', { method: 'POST' })
          .then(() => { LXD.toast('success', '订单已取消'); load(); }).catch(e => LXD.toast('error', e.message));
      });
    }));
  }

  function pay(id) {
    USER.request('/api/user/orders/' + id + '/pay', { method: 'POST' }).then(res => {
      const url = res.data && res.data.pay_url;
      if (url) { window.location.href = url; return; }
      LXD.toast('success', '订单已支付，实例正在开通，可在「我的实例」查看');
      setTimeout(load, 1000);
    }).catch(e => LXD.toast('error', e.message));
  }

  function load() {
    USER.request('/api/user/orders').then(res => render(res.data && res.data.orders || [])).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();

  // 若从下单页带 pay=id，自动发起支付
  const qs = new URLSearchParams(window.location.search);
  const payId = qs.get('pay');
  if (payId && /^\d+$/.test(payId)) setTimeout(() => pay(payId), 600);
})();
