/* 我的工单 */
(function () {
  'use strict';
  USER.shell('tickets', '工单中心', '提交问题，获取支持');
  const c = document.getElementById('userContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const STATUS = { open: '待处理', replied: '已回复', closed: '已关闭' };

  function renderList(list) {
    if (!list.length) {
      c.innerHTML =
        '<div class="row-actions" style="margin-bottom:12px"><button class="btn btn-primary" id="btnNew">新建工单</button></div>' +
        USER.empty('暂无工单');
      document.getElementById('btnNew').addEventListener('click', showNew);
      return;
    }
    c.innerHTML =
      '<div class="row-actions" style="margin-bottom:12px"><button class="btn btn-primary" id="btnNew">新建工单</button></div>' +
      list.map(t => '<div class="card ticket-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(t.subject || '') + ' <span class="tag">' + LXD.esc(t.category || '') + '</span></div>' +
        '<div class="sub">#' + (t.ID || t.id) + ' · ' + String(t.created_at || '').replace('T', ' ').slice(0, 16) + '</div></div>' +
        '<div style="text-align:right"><div class="inst-status">' + (STATUS[t.status] || t.status) + '</div>' +
        '<button class="btn btn-sm" data-view="' + (t.ID || t.id) + '">查看</button></div></div>').join('');
    document.getElementById('btnNew').addEventListener('click', showNew);
    c.querySelectorAll('[data-view]').forEach(b => b.addEventListener('click', () => showDetail(b.dataset.view)));
  }

  function showNew() {
    c.innerHTML =
      '<div class="card">' +
      '<div class="card-title">新建工单</div>' +
      '<div class="field"><label>标题</label><input class="input" id="tSubject" placeholder="一句话描述问题"></div>' +
      '<div class="field"><label>分类</label><select class="input" id="tCategory">' +
      '<option value="咨询">咨询</option><option value="订单">订单</option><option value="产品">产品</option>' +
      '<option value="故障">故障</option><option value="其他">其他</option></select></div>' +
      '<div class="field"><label>内容</label><textarea class="input" id="tContent" rows="6" placeholder="详细描述问题，方便我们快速处理"></textarea></div>' +
      '<div class="row-actions"><button class="btn btn-primary" id="btnSubmit">提交工单</button>' +
      '<button class="btn btn-ghost" id="btnBack">返回</button></div></div>';
    document.getElementById('btnSubmit').addEventListener('click', () => {
      const body = {
        subject: document.getElementById('tSubject').value,
        category: document.getElementById('tCategory').value,
        content: document.getElementById('tContent').value
      };
      if (!body.subject || !body.content) { LXD.toast('error', '标题与内容必填'); return; }
      USER.request('/api/user/tickets', { method: 'POST', body: body }).then(res => {
        LXD.toast('success', '工单已提交');
        const id = res.data && res.data.ticket && (res.data.ticket.ID || res.data.ticket.id);
        showDetail(id);
      }).catch(e => LXD.toast('error', e.message));
    });
    document.getElementById('btnBack').addEventListener('click', load);
  }

  function showDetail(id) {
    USER.request('/api/user/tickets/' + id).then(res => {
      const t = res.data.ticket || {};
      const replies = res.data.replies || [];
      c.innerHTML =
        '<div class="card">' +
        '<div class="card-title">' + LXD.esc(t.subject || '') + ' <span class="tag">' + (STATUS[t.status] || t.status) + '</span></div>' +
        '<div class="sub">#' + (t.ID || t.id) + ' · ' + LXD.esc(t.category || '') + ' · 创建于 ' + String(t.created_at || '').replace('T', ' ').slice(0, 16) + '</div>' +
        replies.map(r => '<div class="reply-box ' + (r.is_staff ? 'staff' : '') + '">' +
          '<div class="sub">' + (r.is_staff ? '【管理员】' : '【我】') + ' ' + String(r.created_at || '').replace('T', ' ').slice(0, 16) + '</div>' +
          '<div style="white-space:pre-wrap">' + LXD.esc(r.content || '') + '</div></div>').join('') +
        (t.status !== 'closed' ?
          '<div class="field" style="margin-top:12px"><textarea class="input" id="rContent" rows="4" placeholder="回复内容"></textarea></div>' +
          '<div class="row-actions"><button class="btn btn-primary" id="btnReply">回复</button>' +
          '<button class="btn btn-ghost" id="btnClose">关闭工单</button>' +
          '<button class="btn btn-ghost" id="btnBack2">返回</button></div>' :
          '<div class="row-actions"><button class="btn btn-ghost" id="btnBack2">返回</button></div>') +
        '</div>';
      if (document.getElementById('btnReply')) document.getElementById('btnReply').addEventListener('click', () => {
        const content = document.getElementById('rContent').value;
        if (!content) { LXD.toast('error', '请输入回复内容'); return; }
        USER.request('/api/user/tickets/' + id + '/reply', { method: 'POST', body: { content: content } })
          .then(() => { LXD.toast('success', '已回复'); showDetail(id); }).catch(e => LXD.toast('error', e.message));
      });
      if (document.getElementById('btnClose')) document.getElementById('btnClose').addEventListener('click', () => {
        LXD.confirmDialog('关闭工单', '确定关闭该工单吗？关闭后无法继续回复。').then(ok => {
          if (!ok) return;
          USER.request('/api/user/tickets/' + id + '/close', { method: 'POST' })
            .then(() => { LXD.toast('success', '工单已关闭'); load(); }).catch(e => LXD.toast('error', e.message));
        });
      });
      document.getElementById('btnBack2').addEventListener('click', load);
    }).catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }

  function load() {
    USER.request('/api/user/tickets').then(res => renderList(res.data && res.data.tickets || [])).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
