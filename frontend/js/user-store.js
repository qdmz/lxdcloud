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
  function priceText(n) { return Number(n || 0) === 0 ? '免费' : '¥' + fmt(n); }
  // 与后端一致：0 元周期仅在商品全部免费或月付为 0 时开放
  function periodOpen(p, per) {
    const v = Number(p[per.field] || 0);
    if (v > 0) return true;
    const allFree = PERIODS.every(x => Number(p[x.field] || 0) === 0);
    return allFree || per.key === 'monthly';
  }

  // 机房切换（v1.1.17）：多机房时按机房筛选商品
  let regionSel = '';
  try { regionSel = localStorage.getItem('lxd_store_region') || ''; } catch (e) {}

  function load() {
  USER.request('/api/user/products' + (regionSel ? '?region_id=' + encodeURIComponent(regionSel) : '')).then(res => {
    const list = res.data && res.data.products ? res.data.products : [];
    const purchased = (res.data && res.data.purchased) || {};
    const regions = ((res.data && res.data.regions) || []).filter(r => (r.product_count || 0) > 0);
    const regionBar = regions.length > 1 || regionSel ?
      '<div class="region-row" id="regionBar" style="margin-bottom:12px;display:flex;gap:6px;flex-wrap:wrap">' +
      '<span style="align-self:center;margin-right:6px;color:var(--text-2)">机房：</span>' +
      '<button class="chip' + (regionSel ? '' : ' active') + '" data-region="">全部</button>' +
      regions.map(r => '<button class="chip' + (String(r.ID || r.id) === regionSel ? ' active' : '') + '" data-region="' + (r.ID || r.id) + '" title="' + LXD.esc(r.description || '') + '">' + LXD.esc(r.name) + '</button>').join('') +
      '</div>' : '';
    const bindRegion = () => c.querySelectorAll('#regionBar [data-region]').forEach(b => b.addEventListener('click', () => {
      regionSel = b.dataset.region;
      try { localStorage.setItem('lxd_store_region', regionSel); } catch (e) {}
      load();
    }));
    if (!list.length) { c.innerHTML = regionBar + '<div class="empty">' + (regionSel ? '该机房暂无可售商品' : '暂无可售商品') + '</div>'; bindRegion(); return; }
    c.innerHTML = regionBar + list.map(p => {
      const pid = p.id || p.ID;
      const stock = (p.stock === undefined || p.stock === null) ? -1 : Number(p.stock);
      const limit = Number(p.per_user_limit || 0);
      const bought = Number(purchased[pid] || 0);
      const soldOut = stock === 0;
      const limitReached = limit > 0 && bought >= limit;
      // 库存 / 限购提示（库存 -1 = 不限，不显示）
      const tips = [];
      if (soldOut) tips.push('<span class="tag" style="color:var(--danger,#e5484d)">已售罄</span>');
      else if (stock > 0) tips.push('<span class="tag">剩余库存 ' + stock + '</span>');
      if (limit > 0) tips.push('<span class="tag">' + (bought > 0 ? '您已购买 ' + bought + '/' + limit : '每人限购 ' + limit + ' 个') + '</span>');
      const blocked = soldOut ? '已售罄' : (limitReached ? '已达限购数量' : '');
      const prices = PERIODS.filter(per => periodOpen(p, per)).map(per => {
        const val = p[per.field];
        return '<button class="chip" data-p="' + per.key + '" data-price="' + (val || 0) + '" data-text="' + per.text + '">' + per.text + ' ' + priceText(val) + '</button>';
      }).join('');
      return '<div class="card store-card">' +
        '<div class="card-title">' + LXD.esc(p.name || '') +
        (p.type === 'vm' ? ' <span class="tag tag-vm">VM</span>' : '') +
        ' <span class="tag">' + LXD.esc(p.region_name || p.node_name || (p.node_id ? '节点#' + p.node_id : '默认机房')) + '</span></div>' +
        '<div class="store-desc">' + LXD.esc(p.description || '') + '</div>' +
        '<div class="store-specs">' +
        '<span>CPU ' + (p.cpu || 0) + ' 核</span><span>内存 ' + (p.memory || 0) + ' MB</span>' +
        '<span>硬盘 ' + (p.disk || 0) + ' GB</span><span>流量 ' + (p.traffic_limit || 0) + ' GB</span>' +
        '<span>IPv4 ' + (p.ipv4_count || 0) + '</span><span>IPv6 ' + (p.ipv6_count || 0) + '</span>' +
        '</div>' +
        (tips.length ? '<div class="store-stock" style="margin:6px 0;display:flex;gap:6px;flex-wrap:wrap">' + tips.join('') + '</div>' : '') +
        '<div class="period-row" data-id="' + pid + '"' + (blocked ? ' data-blocked="1"' : '') + '>' + prices + '</div>' +
        '<button class="btn btn-primary btn-block btn-buy" data-id="' + pid + '"' + (blocked ? ' data-blocked="1"' : '') + ' disabled>' + (blocked || '请选择周期') + '</button>' +
        '</div>';
    }).join('');

    bindRegion();

    // 周期选择
    c.querySelectorAll('.period-row').forEach(row => {
      row.querySelectorAll('.chip').forEach(ch => {
        ch.addEventListener('click', () => {
          if (row.dataset.blocked) return; // 已售罄 / 已达限购：不可选择
          row.querySelectorAll('.chip').forEach(x => x.classList.remove('active'));
          ch.classList.add('active');
          const btn = row.parentElement.querySelector('.btn-buy');
          btn.disabled = false;
          btn.dataset.period = ch.dataset.p;
          btn.textContent = (Number(ch.dataset.price) === 0 ? '免费开通 ' : '立即购买 ') + ch.dataset.text + ' ' + priceText(ch.dataset.price);
        });
      });
    });

    // 下单
    c.querySelectorAll('.btn-buy').forEach(btn => {
      btn.addEventListener('click', () => {
        if (btn.disabled || btn.dataset.blocked) return;
        const id = parseInt(btn.dataset.id, 10);
        const period = btn.dataset.period;
        LXD.toast('info', '正在创建订单...');
        USER.request('/api/user/orders', { method: 'POST', body: { product_id: id, period: period } }).then(res => {
          const order = res.data && res.data.order ? res.data.order : {};
          if (order.status === 'paid') {
            LXD.toast('success', '免费订单已完成，实例正在开通...');
            setTimeout(() => { window.location.href = 'instances.html'; }, 1200);
            return;
          }
          LXD.toast('success', '订单创建成功，正在跳转支付...');
          setTimeout(() => { window.location.href = 'orders.html?pay=' + (order.id || order.ID || ''); }, 800);
        }).catch(e => LXD.toast('error', e.message));
      });
    });
  }).catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
