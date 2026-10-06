/* ============================================================
   LXD Panel - User 模板
   ============================================================ */
(function () {
  'use strict';
  USER_GENERIC.page({
    active: 'templates',
    title: '模板',
    sub: '数据来自 /api/user/templates',
    api: '/api/user/templates',
    searchKeys: ["name", "alias", "description"],
    columns: [
      { "label": "名称", "keys": ["name", "alias", "Name"], "mono": true },
      { "label": "架构", "keys": ["arch", "Arch"], "mono": true },
      { "label": "描述", "keys": ["description", "Description"] },
      { "label": "大小", "keys": ["size", "Size"], "type": "bytes" },
      { "label": "创建时间", "keys": ["created_at", "CreatedAt"], "type": "time" }
    ],
    buttons: '<button class="btn btn-ghost btn-sm" onclick="__gReload()">↻ 刷新</button>',
    emptyText: '暂无模板'
  });
})();
