/* ============================================================
   LXD Panel - User 任务详情
   后端仅提供 /api/user/tasks/:id 单任务查询
   ============================================================ */
(function () {
  'use strict';
  USER.shell('tasks', '任务详情', '数据来自 /api/user/tasks/:id');
  const content = document.getElementById('userContent');
  content.innerHTML =
    '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
    '<input id="taskIdInput" class="input" placeholder="输入任务 ID..." style="width:260px" onkeydown="if(event.key===\'Enter\')queryTask()">' +
    '<button class="btn btn-primary btn-sm" onclick="queryTask()">查询</button>' +
    '</div></div>' +
    '<div class="card" style="padding:14px" id="taskArea">' +
    USER.empty('请输入任务 ID 查询') +
    '</div>';

  function kv(label, value) {
    return '<div style="display:flex;justify-content:space-between;gap:16px;padding:8px 0;border-bottom:1px solid var(--line)"><span style="color:var(--text-3)">' + label + '</span><span class="mono">' + USER.esc(value) + '</span></div>';
  }

  window.queryTask = function () {
    const id = (document.getElementById('taskIdInput').value || '').trim();
    const area = document.getElementById('taskArea');
    if (!id) { area.innerHTML = USER.empty('请输入任务 ID'); return; }
    area.innerHTML = '<div class="empty"><div class="empty-icon">…</div>查询中...</div>';
    USER.request('/api/user/tasks/' + encodeURIComponent(id)).then((res) => {
      const t = res.data || {};
      if (!t || typeof t !== 'object' || (Array.isArray(t) && !t.length)) {
        area.innerHTML = USER.empty('未找到该任务');
        return;
      }
      let h = '<div style="padding:6px 4px">';
      h += kv('任务 ID', t.id || t.task_id || '-');
      h += kv('类型', t.type || t.action || '-');
      h += kv('状态', t.status || '-');
      h += kv('进度', (t.progress != null ? t.progress : (t.percent != null ? t.percent : '-')) + (t.progress != null || t.percent != null ? '%' : ''));
      if (t.error_msg || t.message) h += kv('信息', t.error_msg || t.message);
      h += kv('创建时间', t.created_at ? USER.fmt.time(t.created_at) : '-');
      h += kv('更新时间', t.updated_at ? USER.fmt.time(t.updated_at) : '-');
      h += '</div>';
      area.innerHTML = h;
    }).catch(() => { area.innerHTML = USER.empty('加载失败'); });
  };
})();
