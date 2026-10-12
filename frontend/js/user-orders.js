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
        '<div class="sub">订单号：' + LXD.esc(o.order_no || '') + ' · ' + (o.type === 'renew' ? '<span class="tag">续费</span> ' : '') + (PERIOD[o.period] || o.period || '') + (o.status === 'paid' && o.pay_channel ? ' · ' + ({ balance: '余额支付', epay: '在线支付', free: '免费' }[o.pay_channel] || o.pay_channel) : '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div style="font-weight:700;color:var(--accent)">¥' + Number(amount || 0).toFixed(2) + '</div>' +
        '<div class="sub">' + (STATUS[o.status] || o.status) + '</div>' +
        (o.status === 'pending' ? '<div class="row-actions"><button class="btn btn-sm btn-primary" data-pay="' + o.id + '" data-amt="' + Number(amount || 0) + '">去支付</button>' +
          '<button class="btn btn-sm btn-ghost" data-cancel="' + o.id + '">取消</button></div>' : '') +
        '</div></div>';
    }).join('');

    c.querySelectorAll('[data-pay]').forEach(b => b.addEventListener('click', () => choosePay(b.dataset.pay, Number(b.dataset.amt || 0))));
    c.querySelectorAll('[data-cancel]').forEach(b => b.addEventListener('click', () => {
      LXD.confirmDialog('取消订单', '确定取消该订单吗？取消后需重新下单。').then(ok => {
        if (!ok) return;
        USER.request('/api/user/orders/' + b.dataset.cancel + '/cancel', { method: 'POST' })
          .then(() => { LXD.toast('success', '订单已取消'); load(); }).catch(e => LXD.toast('error', e.message));
      });
    }));
  }

  // 选择支付方式：余额充足时可选余额支付，否则在线支付
  function choosePay(id, amt) {
    if (amt == null) { const o = ORDERS[id]; amt = o ? Number(o.amount || 0) : 0; }
    if (!amt) { pay(id); return; }
    USER.request('/api/user/balance').then(r => {
      const bal = Number(r.data && r.data.balance || 0);
      if (bal < amt) { pay(id, 'alipay'); return; }
      LXD.confirmDialog('选择支付方式', '账户余额 ¥' + bal.toFixed(2) + '，是否使用余额支付 ¥' + amt.toFixed(2) + '？<br><span class="sub">点「取消」改用在线支付</span>', false)
        .then(ok => pay(id, ok ? 'balance' : 'alipay'));
    }).catch(() => pay(id, 'alipay'));
  }

  function pay(id, type) {
    USER.request('/api/user/orders/' + id + '/pay', { method: 'POST', body: type ? { type: type } : {} }).then(res => {
      const url = res.data && res.data.pay_url;
      if (url) { window.location.href = url; return; }
      LXD.toast('success', '订单已支付，实例正在开通，可在「我的实例」查看');
      setTimeout(load, 1000);
    }).catch(e => LXD.toast('error', e.message));
  }

  let ORDERS = {};
  function load() {
    USER.request('/api/user/orders').then(res => { const l = res.data && res.data.orders || []; ORDERS = {}; l.forEach(o => { ORDERS[o.id || o.ID] = o; }); render(l); }).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();

  // 若从下单页带 pay=id，自动发起支付
  const qs = new URLSearchParams(window.location.search);
  const payId = qs.get('pay');
  if (payId && /^\d+$/.test(payId)) setTimeout(() => choosePay(payId, null), 900);
})();
