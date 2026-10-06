/* 个人资料 / 修改密码 */
(function () {
  'use strict';
  USER.shell('profile', '个人资料', '账号信息与安全');
  const c = document.getElementById('userContent');

  c.innerHTML =
    '<div class="grid-2">' +
    '  <div class="card"><div class="card-title">基本资料</div>' +
    '    <div class="field"><label>用户名</label><input class="input" id="pUsername" disabled></div>' +
    '    <div class="field"><label>邮箱</label><input class="input" id="pEmail" disabled></div>' +
    '    <div class="field"><label>昵称</label><input class="input" id="pNickname" placeholder="昵称（可选）"></div>' +
    '    <div class="field"><label>账号状态</label><span id="pStatus"></span></div>' +
    '    <button class="btn btn-primary" id="btnSave">保存资料</button>' +
    '  </div>' +
    '  <div class="card"><div class="card-title">修改密码</div>' +
    '    <div class="field"><label>当前密码</label><input class="input" id="oldPwd" type="password" autocomplete="current-password"></div>' +
    '    <div class="field"><label>新密码</label><input class="input" id="newPwd" type="password" placeholder="至少 6 位" autocomplete="new-password"></div>' +
    '    <div class="field"><label>确认新密码</label><input class="input" id="newPwd2" type="password" autocomplete="new-password"></div>' +
    '    <button class="btn btn-primary" id="btnPwd">修改密码</button>' +
    '  </div>' +
    '</div>';

  const STATUS = { pending: '待激活', active: '正常', disabled: '已禁用' };
  function loadInfo() {
    return USER.request('/api/user/info').then(res => {
      const u = (res.data && res.data.user) || res.data || {};
      document.getElementById('pUsername').value = u.username || '';
      document.getElementById('pEmail').value = u.email || '';
      document.getElementById('pNickname').value = u.nickname || '';
      document.getElementById('pStatus').textContent = STATUS[u.status] || u.status || '正常';
      return u;
    }).catch(() => ({}));
  }
  loadInfo();

  document.getElementById('btnSave').addEventListener('click', function () {
    USER.request('/api/user/profile', { method: 'POST', body: { nickname: document.getElementById('pNickname').value } })
      .then(() => { LXD.toast('success', '资料已保存'); return loadInfo(); })
      .catch(e => LXD.toast('error', e.message));
  });
  document.getElementById('btnPwd').addEventListener('click', function () {
    const oldPwd = document.getElementById('oldPwd').value;
    const n1 = document.getElementById('newPwd').value;
    const n2 = document.getElementById('newPwd2').value;
    if (!oldPwd || !n1) { LXD.toast('error', '请填写完整'); return; }
    if (n1 !== n2) { LXD.toast('error', '两次新密码不一致'); return; }
    USER.request('/api/user/change-password', { method: 'POST', body: { old_password: oldPwd, new_password: n1 } })
      .then(() => LXD.toast('success', '密码修改成功')).catch(e => LXD.toast('error', e.message));
  });
})();
