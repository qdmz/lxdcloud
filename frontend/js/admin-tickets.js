/* 工单管理 */
(function () {
  'use strict';
  ADMIN.shell('tickets', '工单管理', '处理用户工单');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const STATUS = { open: '待处理', replied: '已回复', closed: '已关闭' };

  function renderList(list) {
    c.innerHTML = list.length ? list.map(t =>
      '<div class="card ticket-row">' +
      '<div><div style="font-weight:600">' + LXD.esc(t.subject || '') + ' <span class="tag">' + LXD.esc(t.category || '') + '</span></div>' +
      '<div class="sub">#' + (t.ID || t.id) + ' · 用户#' + t.user_id + ' · ' + String(t.created_at || '').replace('T', ' ').slice(0, 16) + '</div></div>' +
      '<div style="text-align:right"><div class="inst-status ' + (t.status === 'closed' ? '' : 'warn') + '">' + (STATUS[t.status] || t.status) + '</div>' +
      '<button class="btn btn-sm" data-view="' + (t.ID || t.id) + '">处理</button></div></div>').join('') : '<div class="empty">暂无工单</div>';

    c.querySelectorAll('[data-view]').forEach(b => b.addEventListener('click', () => showDetail(b.dataset.view)));
  }

  function showDetail(id) {
    ADMIN.request('/api/admin/tickets/' + id).then(res => {
      const t = res.data.ticket || {};
      const replies = res.data.replies || [];
      c.innerHTML =
        '<div class="card">' +
        '<div class="card-title">' + LXD.esc(t.subject || '') + ' <span class="tag">' + (STATUS[t.status] || t.status) + '</span></div>' +
        '<div class="sub">#' + (t.ID || t.id) + ' · ' + LXD.esc(t.category || '') + ' · 用户#' + t.user_id + ' · ' + String(t.created_at || '').replace('T', ' ').slice(0, 16) + '</div>' +
        replies.map(r => '<div class="reply-box ' + (r.is_staff ? 'staff' : '') + '">' +
          '<div class="sub">' + (r.is_staff ? '【管理员】' : '【用户】') + ' ' + String(r.created_at || '').replace('T', ' ').slice(0, 16) + '</div>' +
          '<div style="white-space:pre-wrap">' + LXD.esc(r.content || '') + '</div></div>').join('') +
        (t.status !== 'closed' ?
          '<div class="field" style="margin-top:12px"><textarea class="input" id="rContent" rows="4" placeholder="回复内容"></textarea></div>' +
          '<div class="row-actions"><button class="btn btn-primary" id="btnReply">回复</button>' +
          '<button class="btn btn-ghost" id="btnClose">关闭工单</button>' +
          '<button class="btn btn-ghost" id="btnBack">返回</button></div>' :
          '<div class="row-actions"><button class="btn btn-ghost" id="btnBack">返回</button></div>') +
        '</div>';
      if (document.getElementById('btnReply')) document.getElementById('btnReply').addEventListener('click', () => {
        const content = document.getElementById('rContent').value;
        if (!content) { LXD.toast('error', '请输入回复内容'); return; }
        ADMIN.request('/api/admin/tickets/' + id + '/reply', { method: 'POST', body: { content: content } })
          .then(() => { LXD.toast('success', '已回复'); showDetail(id); }).catch(e => LXD.toast('error', e.message));
      });
      if (document.getElementById('btnClose')) document.getElementById('btnClose').addEventListener('click', () => {
        ADMIN.request('/api/admin/tickets/' + id + '/close', { method: 'POST' })
          .then(() => { LXD.toast('success', '工单已关闭'); load(); }).catch(e => LXD.toast('error', e.message));
      });
      document.getElementById('btnBack').addEventListener('click', load);
    }).catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }

  function load() {
    ADMIN.request('/api/admin/tickets').then(res => renderList(res.data && res.data.tickets || [])).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
