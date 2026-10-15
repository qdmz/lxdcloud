/* ============================================================
   LXD Panel - User 任务日志
   ============================================================ */
(function () {
  'use strict';
  USER_GENERIC.page({
    active: 'tasks',
    title: '任务日志',
    sub: '数据来自 /api/user/tasks',
    api: '/api/user/tasks',
    searchKeys: ["container_name", "action", "status"],
    columns: [
      { "label": "ID", "keys": ["id"], "mono": true },
      { "label": "容器", "keys": ["container_name"] },
      { "label": "操作", "keys": ["action"] },
      { "label": "类型", "keys": ["type"] },
      { "label": "状态", "keys": ["status"], "type": "status" },
      { "label": "耗时", "render": function (r) { return (r.duration != null ? (r.duration / 1000).toFixed(1) + 's' : '-'); } },
      { "label": "信息", "keys": ["error_msg"] },
      { "label": "创建时间", "keys": ["created_at"], "type": "time" }
    ],
    buttons: '<button class="btn btn-ghost btn-sm" onclick="__gReload()">↻ 刷新</button>',
    emptyText: '暂无任务'
  });
})();
