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

  /** 重装系统：镜像选择弹窗 */
  function showReinstallModal(name) {
    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    mask.innerHTML = '<div class="modal" style="max-width:580px"><h3>重装系统 - ' + LXD.esc(name) + '</h3>' +
      '<div id="riBody" style="max-height:60vh;overflow:auto;padding:4px 0">加载镜像列表...</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="cancel">取消</button><button class="btn btn-danger" data-act="ok" disabled>确认重装</button></div></div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="cancel"]').onclick = () => mask.remove();
    mask.addEventListener('click', (e) => { if (e.target === mask) mask.remove(); });

    const body = mask.querySelector('#riBody');
    const okBtn = mask.querySelector('[data-act="ok"]');
    let chosen = '';

    USER.request('/api/user/templates').then((res) => {
      const list = Array.isArray(res.data) ? res.data : (res.data && (res.data.templates || res.data.list || res.data.images)) || [];
      if (!list.length) {
        body.innerHTML = '<div class="empty">暂无可用镜像，请联系管理员</div>';
        return;
      }
      let html = '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:8px;margin-bottom:12px">';
      list.forEach((t) => {
        const v = t.fingerprint || t.alias || t.name || '';
        const label = [t.os, t.release, t.alias || t.name].filter(Boolean).join(' ') || v;
        html += '<label class="ri-item" style="display:block;border:1px solid var(--border);border-radius:8px;padding:10px 12px;cursor:pointer">' +
          '<input type="radio" name="ri-img" value="' + USER.esc(v) + '" style="margin-right:6px">' +
          '<span>' + USER.esc(label) + '</span>' +
          (t.fingerprint ? '<div style="font-size:12px;color:var(--text-2);word-break:break-all">' + USER.esc(t.fingerprint.slice(0, 24)) + '</div>' : '') +
          '</label>';
      });
      html += '</div>';
      html += '<div style="margin:8px 0 4px;font-weight:600">新 root 密码</div>';
      html += '<input type="text" id="riPwd" class="input" placeholder="留空则保持原密码" style="width:100%;padding:8px 10px;border-radius:6px;border:1px solid var(--border);background:var(--bg-card);color:var(--text)">';
      html += '<div style="font-size:12px;color:var(--text-2);margin-top:6px">重装将清空容器磁盘数据，操作不可逆，请提前备份。</div>';
      body.innerHTML = html;

      body.querySelectorAll('.ri-item input').forEach((inp) => {
        inp.addEventListener('change', () => { chosen = inp.value; okBtn.disabled = false; });
      });
    }).catch((e) => {
      body.innerHTML = '<div class="empty">加载镜像失败：' + USER.esc(e.message || '') + '</div>';
    });

    okBtn.addEventListener('click', () => {
      if (!chosen) return;
      if (!window.confirm('确认使用所选镜像重装容器「' + name + '」吗？此操作将清空磁盘数据。')) return;
      const pwd = (mask.querySelector('#riPwd') || {}).value || '';
      okBtn.disabled = true;
      okBtn.textContent = '重装中...';
      USER.request('/api/user/containers/' + encodeURIComponent(name) + '/action?action=reinstall', {
        method: 'POST',
        body: { image: chosen, password: pwd }
      }).then(() => {
        LXD.toast('success', '重装请求已提交');
        mask.remove();
        setTimeout(() => { if (window.__showContainerDetail) window.__showContainerDetail(name); }, 1500);
      }).catch((e) => {
        LXD.toast('error', '重装失败：' + (e.message || ''));
        okBtn.disabled = false;
        okBtn.textContent = '确认重装';
      });
    });
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

      // SSH / SFTP 连接信息（优先展示公网IP:端口 转发模式，端口映射异步补齐）
      const ip4 = (d.ipv4 && d.ipv4.length ? d.ipv4[0] : d.ip || '');
      const pwd = d.password || '';
      if (ip4) {
        rows += '<tr><td colspan="2" style="padding:10px 0 4px;font-weight:600">连接信息（SSH / SFTP）</td></tr>';
        rows += '<tr><td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">SSH 地址</td><td class="mono" style="padding:6px 0;word-break:break-all" id="ctSshAddr">查询转发端口...</td></tr>';
        rows += '<tr><td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">SFTP 地址</td><td class="mono" style="padding:6px 0;word-break:break-all" id="ctSftpAddr">查询转发端口...</td></tr>';
        rows += fmtKv('登录用户', 'root');
        rows += fmtKv('登录密码', pwd || '详情接口未返回，可点击下方「重置密码」生成新密码');
        rows += '<tr><td colspan="2" style="padding:8px 0 2px"><div style="display:flex;flex-wrap:wrap;gap:8px">' +
          '<a class="btn btn-sm btn-primary" href="file-manager.html?name=' + encodeURIComponent(name) + '" target="_blank">◈ 在线管理文件</a>' +
          '<button class="btn btn-sm btn-ghost" id="ctThirdSsh">三方软件连接 SSH/SFTP</button>' +
          '</div></td></tr>';
        (function fillConn(m, n, ip) {
          const set = (ssh, sftp) => {
            const a = m.querySelector('#ctSshAddr'), b = m.querySelector('#ctSftpAddr');
            if (a) a.textContent = ssh; if (b) b.textContent = sftp;
            const t = m.querySelector('#ctThirdSsh');
            if (t) {
              t.addEventListener('click', () => {
                const win = document.createElement('div');
                win.className = 'modal-mask show';
                win.innerHTML = '<div class="modal" style="max-width:560px"><h3>三方软件连接 SSH / SFTP</h3>' +
                  '<div style="font-size:13px;color:var(--text-2);margin-bottom:10px">使用 XShell / PuTTY / WinSCP / FileZilla 等工具连接容器：</div>' +
                  '<div class="mono" style="background:var(--bg-card);border:1px solid var(--border);border-radius:8px;padding:10px 12px;margin-bottom:8px;word-break:break-all">' + LXD.esc(ssh) + '</div>' +
                  '<div class="mono" style="background:var(--bg-card);border:1px solid var(--border);border-radius:8px;padding:10px 12px;margin-bottom:12px;word-break:break-all">' + LXD.esc(sftp) + '</div>' +
                  '<div style="font-size:13px;color:var(--text-2);margin-bottom:4px">登录用户：root，密码见上方「登录密码」。</div>' +
                  '<div class="modal-actions"><button class="btn btn-ghost" data-act="close">关闭</button></div></div>';
                document.body.appendChild(win);
                win.querySelector('[data-act="close"]').onclick = () => win.remove();
                win.addEventListener('click', (e) => { if (e.target === win) win.remove(); });
              });
            }
          };
          const fallback = () => set('ssh root@' + ip, 'sftp://root@' + ip + '（端口 22，未配置公网转发）');
          USER.request('/api/user/port-mapping?version=all').then((r) => {
            let list = (r.data && (r.data.list || r.data.mappings || r.data.items || r.data.ipv4)) || (Array.isArray(r.data) ? r.data : []);
            const hit = list.find((x) => (x.container_name || x.name || x.container) === n && String(x.container_port || x.port || '') === '22');
            if (hit && hit.public_ip && hit.public_port) {
              set('ssh root@' + hit.public_ip + ' -p ' + hit.public_port, 'sftp://root@' + hit.public_ip + ':' + hit.public_port);
            } else fallback();
          }).catch(fallback);
        })(mask, name, ip4);
      }

      // 容器访问码与快捷连接
      rows += '<tr><td colspan="2" style="padding:10px 0 4px;font-weight:600">容器访问</td></tr>';
      rows += '<tr><td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">访问码 Hash</td><td class="mono" style="padding:6px 0;word-break:break-all"><span id="ctHashCode">加载中...</span> <button class="btn btn-ghost btn-xs" data-hash-copy="1">复制</button></td></tr>';
      rows += '<tr><td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">快捷连接</td><td style="padding:6px 0"><a class="btn btn-sm btn-primary" id="ctQuickLink" href="javascript:;" target="_blank">打开容器管理面板</a></td></tr>';

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

      // 加载容器访问码并绑定复制/重新生成
      (function loadHash(n, m) {
        const setHash = (h) => {
          const el = document.getElementById('ctHashCode');
          if (el) el.textContent = h || '-';
          const link = document.getElementById('ctQuickLink');
          if (link) link.href = (h ? '../container/dashboard.html?hash=' + encodeURIComponent(h) : 'javascript:;');
        };
        USER.request('/api/user/containers/' + encodeURIComponent(n) + '/credential').then((r) => {
          setHash((r.data && r.data.hash) || '');
        }).catch(() => setHash(''));
        m.querySelectorAll('[data-hash-copy]').forEach((b) => b.addEventListener('click', () => {
          const el = document.getElementById('ctHashCode');
          const h = el ? el.textContent : '';
          if (!h || h === '-') return LXD.toast('warn', '暂无访问码可复制');
          (navigator.clipboard ? navigator.clipboard.writeText(h) : Promise.reject()).then(
            () => LXD.toast('success', '访问码已复制'),
            () => LXD.toast('success', '访问码：' + h)
          );
        }));
      })(name, mask);

      // 绑定管理按钮（重装走镜像选择弹窗，其余直接执行）
      mask.querySelectorAll('[data-act]').forEach((btn) => {
        if (btn.dataset.act === 'close') return;
        btn.addEventListener('click', () => {
          if (btn.dataset.act === 'reinstall') showReinstallModal(name);
          else doAction(name, btn.dataset.act, btn.dataset.label);
        });
      });

      // 到期时间（异步补上）
      fetchExpire(name, (expire) => {
        const tbody = document.getElementById('ctDetailBody');
        if (!tbody) return;
        const tr = document.createElement('tr');
        if (expire) {
          tr.innerHTML = '<td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">到期时间</td><td class="mono" style="padding:6px 0;word-break:break-all">' + USER.esc(String(expire).replace('T', ' ').slice(0, 16)) + '（到期后请在「我的实例」续费）</td>';
        } else {
          tr.innerHTML = '<td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">到期时间</td><td style="padding:6px 0;word-break:break-all">未关联实例记录（可能由管理员开通）。产品到期与续费入口在「我的实例」页；如需查看请确认已通过产品订购下单支付。</td>';
        }
        tbody.querySelector('table').insertBefore(tr, tbody.querySelector('table').rows[2]);
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
      { "label": "创建时间", "keys": ["created_at", "CreatedAt", "create_time"], "type": "time" },
      { "label": "操作", "render": (row) => '<button class="btn btn-ghost btn-sm" onclick="__showContainerDetail(\'' + row.name + '\')">详情</button>' }
    ],
    buttons: '<button class="btn btn-ghost btn-sm" onclick="__gReload()">↻ 刷新</button>',
    emptyText: '暂无容器'
  });
})();
