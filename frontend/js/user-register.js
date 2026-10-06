/* 用户注册 */
(function () {
  'use strict';
  LXD.loadBrand().then(LXD.applyBrand).catch(() => {});
  document.getElementById('registerForm').addEventListener('submit', function (e) {
    e.preventDefault();
    const d = new FormData(this);
    if (d.get('password') !== d.get('password2')) { LXD.toast('error', '两次密码不一致'); return; }
    LXD.toast('info', '正在注册...');
    fetch('/api/public/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: d.get('username'), email: d.get('email'), password: d.get('password') })
    }).then(r => r.json()).then(res => {
      if (res.code !== 200) { LXD.toast('error', res.msg || '注册失败'); return; }
      if (res.data && res.data.need_activate) {
        LXD.toast('success', '注册成功，激活邮件已发送，请查收邮箱完成激活');
      } else {
        LXD.toast('success', '注册成功，请登录');
      }
      setTimeout(() => { window.location.href = 'login.html'; }, 1500);
    }).catch(err => LXD.toast('error', err.message || '网络错误'));
  });
})();
