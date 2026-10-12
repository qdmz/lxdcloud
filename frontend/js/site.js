/* ============================================================
   LXD Cloud - 前台统一页眉 / 页脚
   用法：页面放 <header id="siteHeader"></header> 与 <footer id="siteFooter"></footer>，
   引入 css/home.css 与 js/site.js 即可；用户中心外壳调用 SITE.footer()
   ============================================================ */
(function () {
  'use strict';
  var path = location.pathname;
  var B = /\/(user|container)\//.test(path) ? '../' : '';   // 相对根路径
  var page = (path.split('/').pop() || 'index.html').replace(/\?.*$/, '') || 'index.html';
  var inUser = /\/user\//.test(path);
  var $ = function (id) { return document.getElementById(id); };
  var esc = function (s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  };
  var getJSON = function (url) {
    return fetch(url, { credentials: 'same-origin' }).then(function (r) { if (!r.ok) throw new Error(r.status); return r.json(); });
  };
  var each = function (sel, fn) { Array.prototype.forEach.call(document.querySelectorAll(sel), fn); };

  var NAV = [
    { href: 'index.html', text: '首页', on: !inUser && (page === 'index.html' || page === '') },
    { href: 'index.html#pricing', text: '产品价格' },
    { href: 'index.html#features', text: '产品特性' },
    { href: 'help.html', text: '帮助中心', on: page === 'help.html' },
    { href: 'container/login.html', text: '容器面板' }
  ];

  var cache = {};
  function brand() { return cache.brand || (cache.brand = getJSON('/api/public/brand').then(function (r) { return (r && r.data) || {}; }).catch(function () { return {}; })); }
  function auth() {
    return cache.auth || (cache.auth = getJSON('/api/user/info').then(function (r) {
      if (!r || (r.code && r.code !== 200)) return null;
      var d = r.data || {}; var u = d.user || d; return { name: u.username || u.Username || '' };
    }).catch(function () { return null; }));
  }
  function siteInfo() { return cache.si || (cache.si = getJSON('/api/public/site-info').then(function (r) { return (r && r.data) || {}; }).catch(function () { return {}; })); }

  function applyBrand(root) {
    brand().then(function (b) {
      var name = (b.site_name || '').split(/\s+-\s+/)[0] || 'LXD Cloud';
      each('[data-site="name"]', function (el) { el.textContent = name; });
      if (b.footer_text) each('[data-site="footer"]', function (el) { el.textContent = b.footer_text; });
      each('[data-site="logo"]', function (el) {
        if (b.logo_url && /^(https?:)?\/\//.test(b.logo_url)) el.innerHTML = '<img src="' + esc(b.logo_url) + '" alt="">';
        else el.textContent = name.charAt(0).toUpperCase();
      });
      if (b.favicon_url && !document.querySelector('link[data-fav]')) {
        var icon = document.createElement('link'); icon.rel = 'icon'; icon.href = b.favicon_url; icon.setAttribute('data-fav', '1');
        document.head.appendChild(icon);
      }
      document.dispatchEvent(new CustomEvent('site:brand', { detail: { name: name, brand: b } }));
    });
  }

  function header() {
    var el = $('siteHeader');
    if (!el) return;
    el.className = 'h-nav';
    el.id = 'hNav';
    var links = NAV.map(function (n) { return '<a href="' + B + n.href + '"' + (n.on ? ' class="on"' : '') + '>' + n.text + '</a>'; }).join('');
    el.innerHTML =
      '<div class="h-container h-nav-inner">' +
      '<a class="h-brand" href="' + B + 'index.html"><span class="h-logo" data-site="logo">L</span><span class="h-brand-name" data-site="name">LXD Cloud</span></a>' +
      '<nav class="h-links" aria-label="主导航">' + links + '</nav>' +
      '<div class="h-actions">' +
      '<button type="button" class="theme-toggle" onclick="LXDTheme.toggle()" aria-label="切换主题"></button>' +
      '<a class="h-btn h-btn-ghost h-hide-sm" href="' + B + 'user/login.html" data-auth="login">登录</a>' +
      '<a class="h-btn h-btn-primary h-hide-sm" href="' + B + 'user/register.html" data-auth="register">免费注册</a>' +
      '<button type="button" class="h-burger" aria-label="打开菜单" aria-expanded="false"><span></span><span></span><span></span></button>' +
      '</div></div>' +
      '<div class="h-mobile">' + links +
      '<div class="h-mobile-cta"><a class="h-btn h-btn-ghost" href="' + B + 'user/login.html" data-auth="login">登录</a>' +
      '<a class="h-btn h-btn-primary" href="' + B + 'user/register.html" data-auth="register">免费注册</a></div></div>';
    if (window.LXDTheme) LXDTheme.apply(LXDTheme.get());

    var burger = el.querySelector('.h-burger');
    var close = function () { el.classList.remove('open'); burger.setAttribute('aria-expanded', 'false'); };
    burger.addEventListener('click', function () {
      var open = !el.classList.contains('open');
      el.classList.toggle('open', open); burger.setAttribute('aria-expanded', open ? 'true' : 'false');
    });
    each('.h-mobile a', function (a) { a.addEventListener('click', close); });
    window.addEventListener('resize', function () { if (window.innerWidth > 900) close(); });
    var onScroll = function () { el.classList.toggle('scrolled', window.scrollY > 8); };
    window.addEventListener('scroll', onScroll, { passive: true }); onScroll();

    siteInfo().then(function (d) {
      if (d.register_enabled === false) each('[data-auth="register"]', function (a) { a.style.display = 'none'; });
    });
    auth().then(function (u) {
      if (!u) return;
      each('[data-auth="login"]', function (a) { a.style.display = 'none'; });
      each('[data-auth="register"]', function (a) { a.style.display = ''; a.href = B + 'user/index.html'; a.textContent = '用户中心'; });
      document.dispatchEvent(new CustomEvent('site:auth', { detail: u }));
    });
  }

  function footer() {
    var el = $('siteFooter');
    if (!el) return;
    el.classList.add('h-footer');
    el.innerHTML =
      '<div class="h-container h-footer-inner">' +
      '<div class="h-footer-brand"><span class="h-logo sm" data-site="logo">L</span><div><b data-site="name">LXD Cloud</b><p data-site="footer">基于 LXD 的轻量云主机平台，秒级开通，自动交付。</p></div></div>' +
      '<div class="h-footer-links">' +
      '<div><h4>产品</h4><a href="' + B + 'index.html#pricing">产品价格</a><a href="' + B + 'index.html#features">产品特性</a><a href="' + B + 'user/store.html">在线选购</a></div>' +
      '<div><h4>控制台</h4><a href="' + B + 'user/login.html">用户登录</a><a href="' + B + 'user/register.html">免费注册</a><a href="' + B + 'container/login.html">容器面板</a></div>' +
      '<div><h4>支持</h4><a href="' + B + 'help.html">帮助中心</a><a href="' + B + 'help.html#billing">购买与续费</a><a href="' + B + 'user/tickets.html">提交工单</a></div>' +
      '</div></div>' +
      '<div class="h-container h-copy">© ' + new Date().getFullYear() + ' <span data-site="name">LXD Cloud</span> · <span data-site="ver"></span></div>';
    getJSON('/api/public/version').then(function (r) {
      var v = r && r.data && r.data.version; if (v) each('[data-site="ver"]', function (s) { s.textContent = 'v' + v; });
    }).catch(function () {});
  }

  window.SITE = { base: B, header: header, footer: footer, brand: brand, auth: auth, applyBrand: applyBrand };
  function boot() { header(); footer(); applyBrand(); }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})();
