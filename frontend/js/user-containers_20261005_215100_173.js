/* ============================================================
   LXD Panel - User 我的容器
   ============================================================ */
(function () {
  'use strict';

  /** 配置项汉化映射 */
  const CFG_LABELS = {
    'image.description': '镜像描述',
    'image.name': '镜像名称',
    'image.os': '操作系统',
    'image.release': '系统版本',
    'image.architecture': '架构',
    'image.serial': '镜像序列号',
    'image.variant': '镜像变体',
    'limits.cpu': 'CPU 限制',
    'limits.cpu.allowance': 'CPU 份额',
    'limits.memory': '内存限制',
    'limits.memory.swap': '内存交换',
    'limits.memory.swap.priority': '交换优先级',
    'limits.processes': '进程数限制',
    'security.nesting': '嵌套虚拟化',
    'security.privileged': '特权模式',
    'volatile.eth0.host_name': '网卡主机名',
    'volatile.eth0.hwaddr': 'MAC 地址',
    'volatile.last_state.power': '上次电源状态',
    'volatile.uuid': '实例 UUID',
    'volatile.idmap.base': 'ID 映射基准'
  };
  function cfgLabel(k) { return CFG_LABELS[k] || k; }

  /** 执行容器操作 */
  function doAction(name, action, label) {
    if (!window.confirm('确定要' + label + '容器「' + name + '」吗？')) return;
    USER.request('/api/user/containers/' + encodeURIComponent(name) + '/action?action=' + action, { method: 'POST' }).then(() => {
      LXD.toast('success', label + '请求已提交');
      setTimeout(() => { if (window.__showContainerDetail) window.__showContainerDetail(name); }, 1500);
    }).catch((e) => LXD.toast('error', label + '失败：' + (e.message || '')));
  }

  /** 查询实例到期时间（instances 接口按容器名匹配） */
  function fetchExpire(name, cb) {
    USER.request('/api/user/instances').then((res) => {
      const list = (res.data && (res.data.instances || res.data.list)) || [];
      const hit = list.find((it) => it.name === name || it.container_name === name || (it.config_json && it.config_json.indexOf(name) >= 0));
      cb(hit && hit.expire_at ? hit.expire_at : null);
    }).catch(() => cb(null));
  }

  /** 容器详情弹窗 */
  function showDetail(name) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:620px"><h3>容器详情</h3><div id="ctDetailBody" style="max-height:60vh;overflow:auto">加载中...</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="close">关闭</button></div></div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="close"]').onclick = () => mask.remove();
    mask.addEventListener('click', (e) => { if (e.target === mask) mask.remove(); });

    USER.request('/api/user/containers/' + encodeURIComponent(name)).then((res) => {
      const d = res.data || {};
      const cfg = d.config || {};
      const fmtKv = (label, val) => '<tr><td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">' + label + '</td><td class="mono" style="padding:6px 0;word-break:break-all">' + (val == null || val === '' ? '-' : USER.esc(Array.isArray(val) ? val.join(', ') : String(val))) + '</td></tr>';

      let rows = '';
      rows += fmtKv('名称', d.name);
      rows += fmtKv('状态', d.status);
      rows += fmtKv('IPv4', d.ipv4 && d.ipv4.length ? d.ipv4 : (d.ip || ''));
      rows += fmtKv('IPv6', d.ipv6 && d.ipv6.length ? d.ipv6 : (d.ipv6_raw || ''));
      rows += fmtKv('系统', d.image || d.os || '');
      rows += fmtKv('CPU', d.cpu + (d.cpu_usage != null ? '（使用率 ' + d.cpu_usage + '%）' : ''));
      rows += fmtKv('内存', d.memory + (d.memory_usage ? '（已用 ' + d.memory_usage + '）' : ''));
      rows += fmtKv('磁盘', d.disk + (d.disk_usage ? '（已用 ' + d.disk_usage + '）' : ''));
      rows += fmtKv('流量', d.traffic_limit != null ? (d.traffic_usage || '0') + ' / ' + d.traffic_limit + ' GB' : '');
      rows += fmtKv('创建时间', d.created_at || '');

      // SSH / SFTP 连接信息
      const ip4 = (d.ipv4 && d.ipv4.length ? d.ipv4[0] : d.ip || '');
      const pwd = d.password || '';
      if (ip4) {
        rows += '<tr><td colspan="2" style="padding:10px 0 4px;font-weight:600">连接信息（SSH / SFTP）</td></tr>';
        rows += fmtKv('SSH 地址', 'ssh root@' + ip4);
        rows += fmtKv('SFTP 地址', 'sftp://root@' + ip4 + '（端口 22）');
        rows += fmtKv('登录用户', 'root');
        rows += fmtKv('登录密码', pwd || '详情接口未返回，可点击下方「重置密码」生成新密码');
      }

      // 管理操作
      const ACTIONS = [
        { act: 'start', label: '开机', cls: 'btn-primary' },
        { act: 'stop', label: '关机', cls: 'btn-danger-ghost' },
        { act: 'restart', label: '重启', cls: 'btn-ghost' },
        { act: 'reinstall', label: '重装系统', cls: 'btn-danger-ghost' },
        { act: 'reset-password', label: '重置密码', cls: 'btn-ghost' }
      ];
      const actHtml = ACTIONS.map((a) => '<button class="btn btn-sm ' + a.cls + '" data-act="' + a.act + '" data-label="' + a.label + '">' + a.label + '</button>').join('');
      rows += '<tr><td colspan="2" style="padding:10px 0 4px;font-weight:600">管理操作</td></tr>';
      rows += '<tr><td colspan="2" style="padding:4px 0 10px"><div style="display:flex;flex-wrap:wrap;gap:8px">' + actHtml + '</div></td></tr>';

      // 配置（汉化）
      const cfgKeys = Object.keys(cfg);
      if (cfgKeys.length) {
        const interesting = ['image.description', 'image.os', 'image.release', 'image.architecture', 'limits.cpu', 'limits.cpu.allowance', 'limits.memory', 'limits.memory.swap', 'limits.processes', 'security.nesting', 'security.privileged', 'volatile.eth0.hwaddr'];
        rows += '<tr><td colspan="2" style="padding:10px 0 4px;font-weight:600">配置</td></tr>';
        interesting.forEach((k) => { if (cfg[k] !== undefined) rows += fmtKv(cfgLabel(k), cfg[k]); });
      }

      document.getElementById('ctDetailBody').innerHTML = '<table style="width:100%;border-collapse:collapse;font-size:13px">' + rows + '</table>';

      // 绑定管理按钮
      mask.querySelectorAll('[data-act]').forEach((btn) => {
        if (btn.dataset.act === 'close') return;
        btn.addEventListener('click', () => doAction(name, btn.dataset.act, btn.dataset.label));
      });

      // 到期时间（异步补上）
      fetchExpire(name, (expire) => {
        const tbody = document.getElementById('ctDetailBody');
        if (!tbody || !expire) return;
        const tr = document.createElement('tr');
        tr.innerHTML = '<td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">到期时间</td><td class="mono" style="padding:6px 0;word-break:break-all">' + USER.esc(String(expire).replace('T', ' ').slice(0, 16)) + '（到期后请在「我的实例」续费）</td>';
        tbody.querySelector('table').insertBefore(tr, tbody.querySelector('table').firstChild.nextSibling ? tbody.querySelector('table').rows[2] : null);
      });
    }).catch((e) => {
      document.getElementById('ctDetailBody').innerHTML = '<div class="empty">加载失败：' + USER.esc(e.message || '') + '</div>';
    });
  }

  window.__showContainerDetail = showDetail;

  USER_GENERIC.page({
    active: 'containers',
    title: '我的容器',
    sub: '数据来自 /api/user/containers',
    api: '/api/user/containers',
    searchKeys: ["name", "status", "ip", "ipv4", "ipv6", "remark"],
    columns: [
      { "label": "名称", "render": (row) => '<a href="javascript:void(0)" onclick="__showContainerDetail(\'' + row.name + '\')" style="color:var(--accent);text-decoration:none">' + LXD.esc(row.name) + '</a>' },
      { "label": "状态", "keys": ["status", "Status"], "type": "status" },
      { "label": "IP 地址", "keys": ["ipv4", "ipv6", "ip", "IP", "ip_address", "IPAddress"], "mono": true },
      { "label": "系统", "keys": ["os", "system", "distribution", "image", "Os"] },
      { "label": "备注", "keys": ["remark", "Remark"] },
      { "label": "创建时间", "keys": ["created_at", "CreatedAt", "create_time"], "type": "time" }
    ],
    buttons: '<button class="btn btn-ghost btn-sm" onclick="__gReload()">↻ 刷新</button>',
    emptyText: '暂无容器'
  });
})();
