/* ============================================================
   LXD Panel - 主题初始化（放在 <head> 中尽早执行，避免闪烁）
   - 深色 / 浅色主题，选择记忆在 localStorage('lxd_theme')
   - 未手动选择时跟随系统 prefers-color-scheme
   - 管理后台桌面端侧边栏收起状态记忆在 localStorage('lxd_side_collapsed')
   ============================================================ */
(function () {
  'use strict';
  var KEY = 'lxd_theme';
  var root = document.documentElement;

  function stored() {
    try { return localStorage.getItem(KEY); } catch (e) { return null; }
  }
  function system() {
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  }
  function current() { return root.getAttribute('data-theme') || stored() || system(); }
  function apply(t) {
    root.setAttribute('data-theme', t === 'light' ? 'light' : 'dark');
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute('content', t === 'light' ? '#f3f6fb' : '#070b14');
    var btns = document.querySelectorAll('.theme-toggle');
    for (var i = 0; i < btns.length; i++) {
      btns[i].textContent = t === 'light' ? '☾' : '☀';
      btns[i].title = t === 'light' ? '切换到深色模式' : '切换到浅色模式';
    }
  }
  function set(t) {
    try { localStorage.setItem(KEY, t); } catch (e) {}
    apply(t);
  }
  function toggle() { set(current() === 'light' ? 'dark' : 'light'); }

  apply(stored() || system());
  try {
    if (localStorage.getItem('lxd_side_collapsed') === '1') root.classList.add('side-collapsed');
  } catch (e) {}

  // 未手动选择时，系统主题变化实时跟随
  if (window.matchMedia) {
    var mq = window.matchMedia('(prefers-color-scheme: light)');
    var onChange = function () { if (!stored()) apply(system()); };
    if (mq.addEventListener) mq.addEventListener('change', onChange);
    else if (mq.addListener) mq.addListener(onChange);
  }

  window.LXDTheme = { get: current, set: set, toggle: toggle, apply: apply };
})();
