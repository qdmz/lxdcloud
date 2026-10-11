/* ============================================================
   LXD Panel - Admin 防火墙（对接 OpenGFW 插件 /api/admin/firewall）
   - 状态栏：运行状态 / 版本 / 规则数 / 拦截数 + 启动/停止/重启
   - 规则编辑：OpenGFW YAML rules 文本编辑，保存与应用
   - 日志面板：最近日志查看与清空
   ============================================================ */
(function () {
  'use strict';
  ADMIN.shell('firewall', '防火墙', 'OpenGFW 防火墙规则管理');

  const c = document.getElementById('adminContent');
  let statusTimer = null;

  c.innerHTML =
    '<div class="loading">加载中...</div>';

  function esc(s) { return LXD.esc ? LXD.esc(s || '') : String(s || ''); }

  function render(status) {
    const st = status || {};
    const running = !!st.running;
    const statusBar =
      '<div class="card" style="padding:12px 14px;margin-bottom:16px">' +
      '<div class="toolbar">' +
      (running
        ? '<span class="badge badge-running">防火墙运行中</span>'
        : '<span class="badge badge-other">防火墙未运行</span>') +
      (st.version ? '<span class="badge badge-info">版本 ' + esc(st.version) + '</span>' : '') +
      '<span class="badge">规则 ' + (st.active_rules != null ? st.active_rules : '-') + '</span>' +
      '<span class="badge">今日拦截 ' + (st.blocked_today != null ? st.blocked_today : '-') + '</span>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-sm" id="fwStart" ' + (running ? 'disabled' : '') + '>启动</button>' +
      '<button class="btn btn-sm" id="fwStop" ' + (running ? '' : 'disabled') + '>停止</button>' +
      '<button class="btn btn-sm" id="fwRestart" ' + (running ? '' : 'disabled') + '>重启</button>' +
      '<button class="btn btn-primary btn-sm" id="fwApply">应用配置</button>' +
      '</div></div>';

    c.innerHTML = statusBar +
      '<div class="card" style="padding:16px;margin-bottom:16px">' +
      '<div class="section-head"><h3>规则配置（OpenGFW YAML）</h3></div>' +
      '<textarea id="fwRules" class="input" rows="16" style="width:100%;margin-top:10px;font-family:ui-monospace,Consolas,monospace;font-size:12.5px" placeholder="# 示例：' + esc('- name: 阻止高危端口') + '&#10;' + esc('  action: reject') + '&#10;' + esc('  proto: tcp') + '&#10;' + esc('  dport: 23') + '"></textarea>' +
      '<div style="display:flex;gap:10px;margin-top:12px;flex-wrap:wrap;align-items:center">' +
      '<button class="btn btn-ghost btn-sm" id="fwLoadExample">插入示例规则</button>' +
      '<button class="btn btn-ghost btn-sm" id="fwSave">保存配置</button>' +
      '<span class="grow"></span>' +
      '<span class="badge" id="fwEnabledBadge">未启用</span>' +
      '</div></div>' +
      '<div class="card" style="padding:16px">' +
      '<div class="toolbar"><div class="section-head" style="margin:0"><h3>运行日志</h3></div>' +
      '<span class="grow"></span>' +
      '<button class="btn btn-ghost btn-sm" id="fwRefreshLogs">↻ 刷新</button>' +
      '<button class="btn btn-danger-ghost btn-sm" id="fwClearLogs">清空日志</button>' +
      '</div>' +
      '<pre id="fwLogs" class="log-view" style="margin-top:10px;background:rgba(0,0,0,.25);border-radius:10px;padding:12px;max-height:320px;overflow:auto;font-size:12px;line-height:1.6;white-space:pre-wrap">加载中...</pre>' +
      '</div>';

    const fwEnabled = document.getElementById('fwEnabledBadge');
    if (fwEnabled) fwEnabled.textContent = running ? '已启用' : '未启用';

    document.getElementById('fwStart').addEventListener('click', () => act('/api/admin/firewall/start', '服务已启动'));
    document.getElementById('fwStop').addEventListener('click', () => act('/api/admin/firewall/stop', '服务已停止'));
    document.getElementById('fwRestart').addEventListener('click', () => act('/api/admin/firewall/restart', '服务已重启'));
    document.getElementById('fwApply').addEventListener('click', () => {
      const btn = document.getElementById('fwApply');
      btn.disabled = true; btn.textContent = '应用中...';
      ADMIN.request('/api/admin/firewall/apply', { method: 'POST' })
        .then(r => { LXD.toast('success', (r && r.message) || '配置已应用'); load(); })
        .catch(e => LXD.toast('error', e.message))
        .finally(() => { btn.disabled = false; btn.textContent = '应用配置'; });
    });
    document.getElementById('fwLoadExample').addEventListener('click', () => {
      const t = document.getElementById('fwRules');
      t.value += (t.value && !t.value.endsWith('\n') ? '\n' : '') +
        '- name: 阻止高危端口\n  action: reject\n  proto: tcp\n  dport: 23\n' +
        '- name: 限制SSH来源\n  action: allow\n  proto: tcp\n  dport: 22\n  ipset: ["1.2.3.4"]\n';
    });
    document.getElementById('fwSave').addEventListener('click', () => {
      const btn = document.getElementById('fwSave');
      btn.disabled = true; btn.textContent = '保存中...';
      ADMIN.request('/api/admin/firewall/config', { method: 'PUT', body: { rules: document.getElementById('fwRules').value } })
        .then(r => { LXD.toast('success', (r && r.message) || '配置已保存，请点击应用配置使其生效'); })
        .catch(e => LXD.toast('error', e.message))
        .finally(() => { btn.disabled = false; btn.textContent = '保存配置'; });
    });
    document.getElementById('fwRefreshLogs').addEventListener('click', loadLogs);
    document.getElementById('fwClearLogs').addEventListener('click', () => {
      ADMIN.request('/api/admin/firewall/logs', { method: 'DELETE' })
        .then(() => { LXD.toast('success', '日志已清空'); document.getElementById('fwLogs').textContent = '(空)'; })
        .catch(e => LXD.toast('error', e.message));
    });
  }

  function act(url, okMsg) {
    ADMIN.request(url, { method: 'POST' })
      .then(r => { LXD.toast('success', (r && r.message) || okMsg); load(); })
      .catch(e => LXD.toast('error', e.message));
  }

  function loadLogs() {
    const pre = document.getElementById('fwLogs');
    if (!pre) return;
    pre.textContent = '加载中...';
    ADMIN.request('/api/admin/firewall/logs?lines=200')
      .then(res => {
        const logs = (res.data && res.data.logs) || [];
        pre.textContent = logs.length ? logs.join('\n') : '(暂无日志)';
      })
      .catch(e => { pre.textContent = '加载失败：' + e.message; });
  }

  function load() {
    Promise.all([
      ADMIN.request('/api/admin/firewall/config').catch(() => ({ data: null })),
      ADMIN.request('/api/admin/firewall/status').catch(() => ({ data: null }))
    ]).then(([cfgRes, stRes]) => {
      const cfg = (cfgRes && cfgRes.data) || {};
      const st = (stRes && stRes.data) || {};
      render(st);
      const t = document.getElementById('fwRules');
      if (t) t.value = cfg.rules || '';
      loadLogs();
    }).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + esc(e.message) + '</div>';
    });
  }

  load();
})();
