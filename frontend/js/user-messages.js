/* 站内消息 */
(function () {
  'use strict';
  USER.shell('messages', '站内消息', '订单、实例、工单等系统通知');
  const c = document.getElementById('userContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const TYPE = { order: '订单', product: '实例', ticket: '工单', system: '系统' };

  function render(list) {
    if (!list.length) { c.innerHTML = '<div class="empty">暂无消息</div>'; return; }
    c.innerHTML =
      '<div class="row-actions" style="margin-bottom:12px"><button class="btn btn-sm" id="btnReadAll">全部已读</button></div>' +
      list.map(m => '<div class="card msg-row' + (m.is_read ? '' : ' unread') + '" data-id="' + (m.ID || m.id) + '">' +
        '<div class="msg-main"><div style="font-weight:600">' + LXD.esc(m.title || '') + ' <span class="tag">' + (TYPE[m.type] || m.type || '') + '</span>' + (m.is_read ? '' : ' <span class="tag tag-vm">新</span>') + '</div>' +
        '<div class="sub">' + LXD.esc(m.content || '') + '</div>' +
        '<div class="sub" style="font-size:11.5px">' + String(m.CreatedAt || m.created_at || '').replace('T', ' ').slice(0, 16) + '</div></div>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + (m.ID || m.id) + '">删除</button></div>').join('');

    document.getElementById('btnReadAll').addEventListener('click', () => {
      USER.request('/api/user/notifications/read-all', { method: 'POST' })
        .then(() => { LXD.toast('success', '已全部标记为已读'); load(); }).catch(e => LXD.toast('error', e.message));
    });
    c.querySelectorAll('.msg-row').forEach(row => row.addEventListener('click', () => {
      const id = row.dataset.id;
      USER.request('/api/user/notifications/' + id + '/read', { method: 'POST' }).then(() => {
        row.classList.remove('unread');
        const tag = row.querySelector('.tag-vm');
        if (tag) tag.remove();
      }).catch(() => {});
    }));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', (e) => {
      e.stopPropagation();
      LXD.confirmDialog('删除消息', '确定删除这条消息吗？').then(ok => {
        if (!ok) return;
        USER.request('/api/user/notifications/' + b.dataset.del, { method: 'DELETE' })
          .then(() => { LXD.toast('success', '已删除'); load(); }).catch(e2 => LXD.toast('error', e2.message));
      });
    }));
  }

  function load() {
    USER.request('/api/user/notifications').then(res => render(res.data && res.data.notifications || [])).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
