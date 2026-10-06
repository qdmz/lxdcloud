/* 找回密码 */
(function () {
  'use strict';
  LXD.loadBrand().then(LXD.applyBrand).catch(() => {});
  document.getElementById('forgotForm').addEventListener('submit', function (e) {
    e.preventDefault();
    const email = new FormData(this).get('email');
    LXD.toast('info', '正在发送...');
    fetch('/api/public/forgot', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    }).then(r => r.json()).then(res => {
      if (res.code !== 200) { LXD.toast('error', res.msg || '发送失败'); return; }
      LXD.toast('success', '重置邮件已发送，请查收邮箱');
      setTimeout(() => { window.location.href = 'login.html'; }, 1800);
    }).catch(err => LXD.toast('error', err.message || '网络错误'));
  });
})();
