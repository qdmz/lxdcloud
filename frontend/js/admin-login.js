/* ============================================================
   LXD Panel - Admin 登录逻辑
   GET /api/admin/captcha → {code, image(base64)}
   POST /api/admin/login {username,password,captcha} → {code,data:{redirect}}
   ============================================================ */
(function () {
  'use strict';

  LXD.loadBrand().then((b) => {
    document.title = (b.site_name || 'LXD 管理后台') + ' - 管理员登录';
    LXD.applyBrand(b);
  }).catch(() => {});

  const img = document.getElementById('captchaImg');
  const errorMsg = document.getElementById('errorMsg');

  function refreshCaptcha() {
    fetch('/api/admin/captcha').then((r) => r.json()).then((res) => {
      if (res.code === 200) img.src = res.image;
      else LXD.toast('error', res.msg || '获取验证码失败');
    }).catch(() => LXD.toast('error', '获取验证码失败'));
  }
  window.refreshCaptcha = refreshCaptcha;
  refreshCaptcha();

  /** 后端 redirect 路径 → 静态页面文件 */
  function mapRedirect(path) {
    if (!path) return 'dashboard.html';
    const m = String(path).replace(/^\/admin\//, '');
    const map = {
      'ip-pool/v4': 'ip_pool_v4.html', 'ip-pool/v6': 'ip_pool_v6.html',
      'port-mapping/v4': 'port_mapping_v4.html', 'port-mapping/v6': 'port_mapping_v6.html',
      'brand-settings': 'brand_settings.html', 'storage-pools': 'storage_pools.html'
    };
    if (map[m]) return map[m];
    if (/\.html$/.test(m)) return m;
    return (m || 'dashboard').replace(/-/g, '_') + '.html';
  }

  document.getElementById('loginForm').addEventListener('submit', (e) => {
    e.preventDefault();
    const form = new FormData(e.target);
    const body = Object.fromEntries(form.entries());
    const btn = e.target.querySelector('button[type="submit"]');
    btn.disabled = true; btn.textContent = '登录中...';

    fetch('/api/admin/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    }).then((r) => r.json()).then((res) => {
      if (res.code === 200) {
        window.location.href = mapRedirect(res.data && res.data.redirect);
      } else {
        showError(res.msg || '登录失败');
        refreshCaptcha();
        if (form.get('captcha')) e.target.querySelector('input[name="captcha"]').value = '';
      }
    }).catch(() => {
      showError('登录失败，请重试');
      refreshCaptcha();
    }).finally(() => {
      btn.disabled = false; btn.textContent = '登 录';
    });
  });

  function showError(msg) {
    errorMsg.textContent = msg;
    errorMsg.classList.remove('hidden');
    setTimeout(() => errorMsg.classList.add('hidden'), 4000);
  }
})();
