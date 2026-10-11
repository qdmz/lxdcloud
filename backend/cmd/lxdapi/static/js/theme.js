/* CloudVM Pro 双主题切换（data-theme + localStorage） */
(function () {
  'use strict';
  var KEY = 'cvp_theme';
  var saved = null;
  try { saved = localStorage.getItem(KEY); } catch (e) {}
  if (!saved) {
    saved = (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) ? 'dark' : 'light';
  }
  var root = document.documentElement;
  root.setAttribute('data-theme', saved);
  var toggleBtns = function () {
    return document.querySelectorAll('[data-theme-toggle], #themeToggle, .theme-toggle');
  };
  function apply(theme) {
    root.setAttribute('data-theme', theme);
    try { localStorage.setItem(KEY, theme); } catch (e) {}
    toggleBtns().forEach(function (btn) {
      btn.setAttribute('aria-label', theme === 'dark' ? '切换到浅色' : '切换到深色');
      var sun = btn.querySelector('.sun-icon'); if (sun) sun.style.display = theme === 'dark' ? 'block' : 'none';
      var moon = btn.querySelector('.moon-icon'); if (moon) moon.style.display = theme === 'dark' ? 'none' : 'block';
      if (btn.querySelector('[data-theme-label]')) btn.querySelector('[data-theme-label]').textContent = theme === 'dark' ? '浅色' : '深色';
    });
  }
  function toggle() { apply(root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark'); }
  document.addEventListener('click', function (e) {
    var btn = e.target.closest('[data-theme-toggle], #themeToggle, .theme-toggle');
    if (btn) { e.preventDefault(); toggle(); }
  });
  window.LXDTheme = { apply: apply, toggle: toggle, current: function () { return root.getAttribute('data-theme'); } };
  apply(saved);
})();
