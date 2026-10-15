/* 找回密码 */
(function () {
  'use strict';
  LXD.loadBrand().then(LXD.applyBrand).catch(() => {});
  const img = document.getElementById('captchaImg');
  function refreshCaptcha() {
    fetch('/api/public/captcha').then(r => r.json()).then(res => {
      if (res.code === 200) img.src = res.image;
    }).catch(() => {});
  }
  if (img) { img.addEventListener('click', refreshCaptcha); refreshCaptcha(); }
  window.refreshCaptcha = refreshCaptcha;
  document.getElementById('forgotForm').addEventListener('submit', function (e) {
    e.preventDefault();
    const d = new FormData(this);
    LXD.toast('info', '正在发送...');
    fetch('/api/public/forgot', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: d.get('email'), captcha: d.get('captcha') })
    }).then(r => r.json()).then(res => {
      if (res.code !== 200) { LXD.toast('error', res.msg || '发送失败'); refreshCaptcha(); return; }
      LXD.toast('success', '重置邮件已发送，请查收邮箱');
      setTimeout(() => { window.location.href = 'login.html'; }, 1800);
    }).catch(err => LXD.toast('error', err.message || '网络错误'));
  });
})();
