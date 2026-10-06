/* 商品列表与订购 */
(function () {
  'use strict';
  USER.shell('store', '产品订购', '选择套餐，下单并在线支付');
  const c = document.getElementById('userContent');
  c.innerHTML = '<div class="loading">加载商品中...</div>';

  const PERIODS = [
    { key: 'monthly', text: '月付', field: 'price_monthly' },
    { key: 'quarterly', text: '季付', field: 'price_quarterly' },
    { key: 'half_year', text: '半年付', field: 'price_half_year' },
    { key: 'yearly', text: '年付', field: 'price_yearly' }
  ];

  function fmt(n) { return Number(n || 0).toFixed(2); }

  USER.request('/api/user/products').then(res => {
    const list = res.data && res.data.products ? res.data.products : [];
    if (!list.length) { c.innerHTML = '<div class="empty">暂无可售商品</div>'; return; }
    c.innerHTML = list.map(p => {
      const prices = PERIODS.map(per => {
        const val = p[per.field];
        return '<button class="chip" data-p="' + per.key + '" data-price="' + (val || 0) + '" data-text="' + per.text + '">' + per.text + ' ¥' + fmt(val) + '</button>';
      }).join('');
      return '<div class="card store-card">' +
        '<div class="card-title">' + LXD.esc(p.name || '') +
        (p.type === 'vm' ? ' <span class="tag tag-vm">VM</span>' : '') +
        ' <span class="tag">' + (p.node_id ? '节点#' + p.node_id : '主控') + '</span></div>' +
        '<div class="store-desc">' + LXD.esc(p.description || '') + '</div>' +
        '<div class="store-specs">' +
        '<span>CPU ' + (p.cpu || 0) + ' 核</span><span>内存 ' + (p.memory || 0) + ' MB</span>' +
        '<span>硬盘 ' + (p.disk || 0) + ' GB</span><span>流量 ' + (p.traffic_limit || 0) + ' GB</span>' +
        '<span>IPv4 ' + (p.ipv4_count || 0) + '</span><span>IPv6 ' + (p.ipv6_count || 0) + '</span>' +
        '</div>' +
        '<div class="period-row" data-id="' + (p.id || p.ID) + '">' + prices + '</div>' +
        '<button class="btn btn-primary btn-block btn-buy" data-id="' + (p.id || p.ID) + '" disabled>请选择周期</button>' +
        '</div>';
    }).join('');

    // 周期选择
    c.querySelectorAll('.period-row').forEach(row => {
      row.querySelectorAll('.chip').forEach(ch => {
        ch.addEventListener('click', () => {
          row.querySelectorAll('.chip').forEach(x => x.classList.remove('active'));
          ch.classList.add('active');
          const btn = row.parentElement.querySelector('.btn-buy');
          btn.disabled = false;
          btn.dataset.period = ch.dataset.p;
          btn.textContent = '立即购买 ' + ch.dataset.text + ' ¥' + fmt(ch.dataset.price);
        });
      });
    });

    // 下单
    c.querySelectorAll('.btn-buy').forEach(btn => {
      btn.addEventListener('click', () => {
        if (btn.disabled) return;
        const id = parseInt(btn.dataset.id, 10);
        const period = btn.dataset.period;
        LXD.toast('info', '正在创建订单...');
        USER.request('/api/user/orders', { method: 'POST', body: { product_id: id, period: period } }).then(res => {
          const order = res.data && res.data.order ? res.data.order : {};
          LXD.toast('success', '订单创建成功，正在跳转支付...');
          setTimeout(() => { window.location.href = 'orders.html?pay=' + (order.id || order.ID || ''); }, 800);
        }).catch(e => LXD.toast('error', e.message));
      });
    });
  }).catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
})();
