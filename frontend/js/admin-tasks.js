/* ============================================================
   LXD Panel - Admin 任务管理
   ============================================================ */
(function () {
  'use strict';
  GENERIC.page({
    active: 'tasks',
    title: '任务管理',
    sub: '数据来自 /api/admin/tasks',
    api: '/api/admin/tasks',
    dataKey: 'tasks',
    searchKeys: ["id", "type", "status"],
    columns: [{"label": "ID", "keys": ["id", "task_id", "ID"], "mono": true}, {"label": "类型", "keys": ["type", "action", "Type"]}, {"label": "状态", "keys": ["status", "Status"], "type": "status"}, {"label": "进度", "render": "PROGRESS"}, {"label": "创建时间", "keys": ["created_at", "CreatedAt"], "type": "time"}],
    buttons: '<button class="btn btn-ghost btn-sm" onclick="__gReload()">↻ 刷新</button>',
    emptyText: '暂无数据'
  });
})();
