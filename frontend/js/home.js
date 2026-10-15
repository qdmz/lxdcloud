/* LXD Cloud 首页：品牌、版本、登录态、公开商品价格方案 */
(function () {
  'use strict';
  var $ = function (id) { return document.getElementById(id); };
  var esc = function (s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  };
  var getJSON = function (url) {
    return fetch(url, { credentials: 'same-origin' }).then(function (r) {
      if (!r.ok) throw new Error(r.status);
      return r.json();
    });
  };
  var setText = function (id, v) { var el = $(id); if (el) el.textContent = v; };

  /* 登录态（由 site.js 统一处理页眉，这里只影响购买按钮与入口链接） */
  var loggedIn = false;
  document.addEventListener('site:auth', function () {
    loggedIn = true;
    var hc = $('heroConsole'); if (hc) { hc.href = 'user/index.html'; hc.textContent = '进入用户中心'; }
    var pu = $('portalUser'); if (pu) pu.href = 'user/index.html';
    Array.prototype.forEach.call(document.querySelectorAll('main [data-auth="register"]'), function (a) { a.href = 'user/index.html'; a.textContent = '用户中心'; });
    renderPlans();
  });
  document.addEventListener('site:brand', function (e) { document.title = e.detail.name + ' - 高性能云主机 · 秒级开通'; });
  getJSON('/api/public/version').then(function (res) {
    var v = res && res.data && res.data.version; if (v) setText('statVersion', 'v' + v);
  }).catch(function () {});

  /* 价格方案 */
  var PERIODS = [
    { k: 'price_monthly', t: '月', key: 'monthly' },
    { k: 'price_quarterly', t: '季', key: 'quarterly' },
    { k: 'price_half_year', t: '半年', key: 'half_year' },
    { k: 'price_yearly', t: '年', key: 'yearly' }
  ];
  var period = 'price_monthly';
  var products = [];

  var periodOpen = function (p, per) {
    if (Number(p[per.k] || 0) > 0) return true;
    var allFree = PERIODS.every(function (x) { return Number(p[x.k] || 0) === 0; });
    return allFree || per.key === 'monthly';
  };
  var mem = function (mb) { mb = Number(mb || 0); return mb >= 1024 ? (Math.round(mb / 102.4) / 10) + ' GB' : mb + ' MB'; };
  var disk = function (gb) { gb = Number(gb || 0); return gb >= 1024 ? (gb / 1024) + ' TB' : gb + ' GB'; };
  var bw = function (p) {
    var i = Number(p.ingress || 0), e = Number(p.egress || 0);
    if (!i && !e) return '共享带宽';
    var m = Math.max(i, e);
    return m + ' Mbps 带宽';
  };
  var traffic = function (t) { t = Number(t || 0); return t > 0 ? t + ' GB 月流量' : '不限流量'; };

  function renderPlans() {
    var box = $('hPlans');
    if (!box) return;
    if (!products.length) {
      box.innerHTML = '<div class="h-empty">暂无在售套餐，敬请期待。</div>';
      return;
    }
    var per = PERIODS.filter(function (x) { return x.k === period; })[0];
    var featured = products.length >= 3 ? 1 : -1;
    box.innerHTML = products.map(function (p, idx) {
      var open = periodOpen(p, per);
      var price = Number(p[per.k] || 0);
      var stock = (p.stock === undefined || p.stock === null) ? -1 : Number(p.stock);
      var limit = Number(p.per_user_limit || 0);
      var soldOut = stock === 0;
      var badges = [];
      if (soldOut) badges.push('<span class="h-tag t-red">已售罄</span>');
      else if (stock > 0) badges.push('<span class="h-tag t-amber">仅剩 ' + stock + ' 台</span>');
      if (limit > 0) badges.push('<span class="h-tag">每人限购 ' + limit + '</span>');
      if (p.type === 'vm') badges.push('<span class="h-tag">虚拟机</span>');
      if (p.region_name) badges.push('<span class="h-tag">' + esc(p.region_name) + '</span>');
      var priceHtml = !open
        ? '<div class="h-price"><b class="h-na">—</b><span>该周期暂不提供</span></div>'
        : price === 0
          ? '<div class="h-price"><b>免费</b><span>/' + per.t + '</span></div>'
          : '<div class="h-price"><small>¥</small><b>' + (price % 1 === 0 ? price : price.toFixed(2)) + '</b><span>/' + per.t + '</span></div>';
      var specs = [
        '<li><em>' + (p.cpu || 1) + '</em> 核 CPU</li>',
        '<li><em>' + mem(p.memory) + '</em> 内存</li>',
        '<li><em>' + disk(p.disk) + '</em> SSD 存储</li>',
        '<li>' + esc(bw(p)) + '</li>',
        '<li>' + esc(traffic(p.traffic_limit)) + '</li>'
      ];
      var net = [];
      if (Number(p.ipv4_count) > 0) net.push(p.ipv4_count + ' 个 IPv4');
      if (Number(p.ipv6_count) > 0) net.push(p.ipv6_count + ' 个 IPv6');
      if (Number(p.port_mappings) > 0) net.push(p.port_mappings + ' 个端口映射');
      if (net.length) specs.push('<li>' + esc(net.join(' · ')) + '</li>');
      if (Number(p.backups) > 0) specs.push('<li>' + p.backups + ' 个免费备份</li>');
      var href = loggedIn ? 'user/store.html' : 'user/login.html';
      var btn = soldOut || !open
        ? '<span class="h-btn h-btn-block h-btn-disabled">' + (soldOut ? '已售罄' : '暂不可选') + '</span>'
        : '<a class="h-btn h-btn-block ' + (idx === featured ? 'h-btn-primary' : 'h-btn-soft') + '" href="' + href + '">立即购买</a>';
      return '<article class="h-plan' + (idx === featured ? ' featured' : '') + (soldOut ? ' soldout' : '') + '">' +
        (idx === featured ? '<span class="h-plan-flag">最受欢迎</span>' : '') +
        '<h3>' + esc(p.name) + '</h3>' +
        '<p class="h-plan-desc">' + esc(p.description || '高性能 LXD 容器实例') + '</p>' +
        priceHtml +
        (badges.length ? '<div class="h-tags">' + badges.join('') + '</div>' : '') +
        '<ul class="h-specs">' + specs.join('') + '</ul>' + btn +
        '</article>';
    }).join('');
  }

  Array.prototype.forEach.call(document.querySelectorAll('#hPeriod button'), function (b) {
    b.addEventListener('click', function () {
      period = b.getAttribute('data-k');
      Array.prototype.forEach.call(document.querySelectorAll('#hPeriod button'), function (x) { x.classList.toggle('on', x === b); });
      renderPlans();
    });
  });

  getJSON('/api/public/products').then(function (res) {
    products = (res && res.data && res.data.products) || [];
    setText('statProducts', products.length);
    renderPlans();
  }).catch(function () {
    products = [];
    setText('statProducts', '—');
    renderPlans();
  });

  /* 进入视口渐显 */
  if ('IntersectionObserver' in window) {
    var io = new IntersectionObserver(function (es) {
      es.forEach(function (e) { if (e.isIntersecting) { e.target.classList.add('in'); io.unobserve(e.target); } });
    }, { threshold: 0.12 });
    Array.prototype.forEach.call(document.querySelectorAll('.h-feat,.h-steps li,.h-portal,.h-head'), function (el) {
      el.classList.add('reveal'); io.observe(el);
    });
  }
})();
