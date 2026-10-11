/* SMTP 与邮件模板 */
(function () {
  'use strict';
  ADMIN.shell('mail', '邮件配置', 'SMTP 发送与模板管理');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  function renderSMTP(s) {
    c.innerHTML =
      '<div class="grid-2">' +
      '<div class="card"><div class="card-title">SMTP 服务配置</div>' +
      '<div class="field"><label>SMTP 主机</label><input class="input" id="mHost" value="' + LXD.esc(s.host || '') + '" placeholder="smtp.example.com"></div>' +
      '<div class="field"><label>端口</label><input class="input" id="mPort" type="number" value="' + (s.port || 465) + '"></div>' +
      '<div class="field"><label>用户名</label><input class="input" id="mUser" value="' + LXD.esc(s.username || '') + '"></div>' +
      '<div class="field"><label>密码/授权码</label><input class="input" id="mPass" type="password" value="' + LXD.esc(s.password || '') + '"></div>' +
      '<div class="field"><label>发件邮箱</label><input class="input" id="mFrom" value="' + LXD.esc(s.from_email || s.from || '') + '"></div>' +
      '<div class="field"><label>发件人名称</label><input class="input" id="mFromName" value="' + LXD.esc(s.from_name || '') + '"></div>' +
      '<div class="field"><label>加密方式</label><select class="input" id="mSSL">' +
      '<option value="true"' + (s.enable_ssl || s.ssl ? ' selected' : '') + '>SSL/TLS (465)</option>' +
      '<option value="false"' + (!(s.enable_ssl || s.ssl) ? ' selected' : '') + '>STARTTLS/无 (587/25)</option></select></div>' +
      '<div class="field"><label>启用</label><select class="input" id="mEnabled"><option value="true"' + (s.enabled ? ' selected' : '') + '>启用</option><option value="false"' + (!s.enabled ? ' selected' : '') + '>停用</option></select></div>' +
      '<div class="row-actions"><button class="btn btn-primary" id="btnSave">保存配置</button>' +
      '<button class="btn" id="btnTest">发送测试邮件</button></div></div>' +
      '<div class="card"><div class="card-title">邮件模板</div><div id="tplList">加载中...</div></div>' +
      '</div>';

    document.getElementById('btnSave').addEventListener('click', () => {
      ADMIN.request('/api/admin/smtp', { method: 'POST', body: {
        host: document.getElementById('mHost').value,
        port: parseInt(document.getElementById('mPort').value, 10) || 0,
        username: document.getElementById('mUser').value,
        password: document.getElementById('mPass').value,
        from: document.getElementById('mFrom').value,
        from_name: document.getElementById('mFromName').value,
        ssl: document.getElementById('mSSL').value === 'true',
        enabled: document.getElementById('mEnabled').value === 'true'
      }}).then(() => { LXD.toast('success', 'SMTP 配置已保存'); }).catch(e => LXD.toast('error', e.message));
    });
    document.getElementById('btnTest').addEventListener('click', () => {
      const to = window.prompt('输入测试收件邮箱：', '');
      if (!to) return;
      ADMIN.request('/api/admin/smtp/test', { method: 'POST', body: { to: to } })
        .then(() => LXD.toast('success', '测试邮件已发送，请查收')).catch(e => LXD.toast('error', e.message));
    });
    loadTpl();
  }

  function loadTpl() {
    ADMIN.request('/api/admin/mail-templates').then(res => {
      const list = res.data && res.data.templates || [];
      const box = document.getElementById('tplList');
      box.innerHTML = list.length ? list.map(t =>
        '<div class="tpl-row"><div><b>' + LXD.esc(t.code || '') + '</b>' +
        '<div class="sub">' + LXD.esc(t.subject || '') + '</div></div>' +
        '<button class="btn btn-sm" data-tpl="' + LXD.esc(t.code || '') + '">编辑</button></div>').join('')
        : '暂无模板';
      box.querySelectorAll('[data-tpl]').forEach(b => b.addEventListener('click', () => editTpl(b.dataset.tpl)));
    }).catch(() => {});
  }

  function editTpl(code) {
    ADMIN.request('/api/admin/mail-templates').then(res => {
      const list = res.data && res.data.templates || [];
      const t = list.find(x => x.code === code) || {};
      const card = document.querySelector('.card:last-child');
      card.innerHTML = '<div class="card-title">编辑模板 ' + LXD.esc(code) + '</div>' +
        '<div class="field"><label>主题</label><input class="input" id="tSubject" value="' + LXD.esc(t.subject || '') + '"></div>' +
        '<div class="field"><label>正文（支持 {{.Username}} {{.SiteName}} {{.Link}} 等变量）</label><textarea class="input" id="tBody" rows="12">' + LXD.esc(t.body || '') + '</textarea></div>' +
        '<div class="row-actions"><button class="btn btn-primary" id="btnTplSave">保存</button>' +
        '<button class="btn btn-ghost" id="btnTplBack">返回</button></div>';
      document.getElementById('btnTplSave').addEventListener('click', () => {
        ADMIN.request('/api/admin/mail-templates/' + encodeURIComponent(code), { method: 'POST', body: {
          subject: document.getElementById('tSubject').value,
          body: document.getElementById('tBody').value
        }}).then(() => { LXD.toast('success', '模板已保存'); load(); }).catch(e => LXD.toast('error', e.message));
      });
      document.getElementById('btnTplBack').addEventListener('click', load);
    }).catch(() => {});
  }

  function load() {
    ADMIN.request('/api/admin/smtp').then(res => renderSMTP(res.data && (res.data.config || res.data.smtp || res.data) || {})).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
