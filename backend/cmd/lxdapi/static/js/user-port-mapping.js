/* ============================================================
   LXD Panel - User 端口映射
   ============================================================ */
(function () {
  'use strict';
  USER_GENERIC.page({
    active: 'port_mapping',
    title: '端口映射',
    sub: '数据来自 /api/user/port-mapping',
    api: '/api/user/port-mapping?version=all',
    dataKeys: ['ipv4', 'ipv6'],
    searchKeys: ["container", "ip", "port", "remark"],
    columns: [
      { "label": "容器", "keys": ["container_name", "container", "Container", "name"], "mono": true },
      { "label": "容器 IP", "keys": ["container_ip", "ContainerIP", "ip"], "mono": true },
      { "label": "容器端口", "keys": ["container_port", "ContainerPort", "port"], "mono": true },
      { "label": "公网 IP", "keys": ["public_ip", "PublicIP", "forward_ip"], "mono": true },
      { "label": "公网端口", "keys": ["public_port", "PublicPort"], "mono": true },
      { "label": "协议", "keys": ["protocol", "Protocol"] },
      { "label": "状态", "keys": ["status", "Status"], "type": "status" },
      { "label": "创建时间", "keys": ["created_at", "CreatedAt"], "type": "time" }
    ],
    buttons: '<button class="btn btn-ghost btn-sm" onclick="__gReload()">↻ 刷新</button>',
    emptyText: '暂无数据'
  });
})();
