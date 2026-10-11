/* ============================================================
   LXD Panel - User 反向代理
   ============================================================ */
(function () {
  'use strict';
  USER_GENERIC.page({
    active: 'nginx',
    title: '反向代理',
    sub: '数据来自 /api/user/nginx/proxies',
    api: '/api/user/nginx/proxies',
    searchKeys: ["domain", "target", "remark"],
    columns: [
      { "label": "域名", "keys": ["domain", "Domain", "name"], "mono": true },
      { "label": "协议", "keys": ["protocol", "Protocol"] },
      { "label": "目标", "render": (row) => (row.target_ip || row.TargetIP || '-') + (row.target_port ? ':' + row.target_port : '') },
      { "label": "SSL", "render": (row) => (row.enable_ssl || row.EnableSSL) ? '<span class="badge badge-running"><span class="dot"></span>已启用</span>' : '<span class="badge badge-stopped">未启用</span>' },
      { "label": "证书", "render": (row) => (row.ssl_cert || row.SSLCert) ? '已配置' : '未配置' },
      { "label": "状态", "keys": ["status", "Status"], "type": "status" },
      { "label": "创建时间", "keys": ["created_at", "CreatedAt"], "type": "time" }
    ],
    buttons: '<button class="btn btn-ghost btn-sm" onclick="__gReload()">↻ 刷新</button>',
    emptyText: '暂无数据'
  });
})();
