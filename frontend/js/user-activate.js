/* 邮箱激活 & 密码重置 */
(function () {
  'use strict';
  LXD.loadBrand().then(LXD.applyBrand).catch(() => {});
  const box = document.getElementById('resultBox');
  const qs = new URLSearchParams(window.location.search);
  const token = qs.get('token') || '';
  const mode = qs.get('mode') || 'activate';
  const api = mode === 'reset' ? '/api/public/reset' : '/api/public/activate';

  if (mode === 'reset') {
    box.innerHTML =
      '<div class="field"><label>新密码</label><input class="input" id="newPwd" type="password" placeholder="至少 6 位" minlength="6"></div>' +
      '<button class="btn btn-primary btn-block" id="btnReset" style="margin-top:12px">重置密码</button>';
    document.getElementById('btnReset').addEventListener('click', function () {
      const pwd = document.getElementById('newPwd').value;
      if (pwd.length < 6) { LXD.toast('error', '密码至少 6 位'); return; }
      fetch(api, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ token: token, password: pwd }) })
        .then(r => r.json()).then(res => {
          if (res.code !== 200) { box.innerHTML = '<div class="login-error">' + (res.msg || '重置失败') + '</div>'; return; }
          box.innerHTML = '<div style="color:var(--ok,#22c55e);font-weight:600">密码已重置，请前往登录</div>';
          setTimeout(() => { window.location.href = 'login.html'; }, 1500);
        }).catch(err => { box.innerHTML = '<div class="login-error">' + err.message + '</div>'; });
    });
    return;
  }

  fetch(api + (token ? '?token=' + encodeURIComponent(token) : '')).then(r => r.json()).then(res => {
    if (res.code !== 200) {
      box.innerHTML = '<div class="login-error">' + (res.msg || '激活失败，链接可能已失效') + '</div>';
      return;
    }
    box.innerHTML = '<div style="color:var(--ok,#22c55e);font-weight:600">账号激活成功，请前往登录</div>';
    setTimeout(() => { window.location.href = 'login.html'; }, 1500);
  }).catch(err => { box.innerHTML = '<div class="login-error">' + err.message + '</div>'; });
})();
