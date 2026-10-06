/* ============================================================
   LXD Panel - Admin 品牌设置（全字段版）
   GET /api/admin/brand-settings
   POST /api/admin/brand-settings
   ============================================================ */
(function () {
  'use strict';

  ADMIN.shell('brand_settings', '品牌设置', '自定义站点名称、登录页、Logo、页脚与 TLS 等全局品牌信息');

  const content = document.getElementById('adminContent');

  // 分组定义：title 为分组标题，fields 为字段数组
  const GROUPS = [
    {
      title: '管理后台',
      fields: [
        { key: 'admin_system_name', label: '系统名称', type: 'text', placeholder: '例如：LXD API - 管理后台', hint: '显示在管理后台侧边栏品牌区与登录页' },
        { key: 'admin_system_title', label: '浏览器标题', type: 'text', placeholder: '例如：管理后台 - LXD容器管理系统', hint: '管理后台浏览器标签页标题' },
        { key: 'admin_login_title', label: '登录标题', type: 'text', placeholder: '例如：管理员登录', hint: '管理后台登录表单标题' },
        { key: 'admin_bg_image', label: '背景图 URL', type: 'text', placeholder: 'https://...', hint: '登录页背景图，留空使用默认渐变' },
        { key: 'admin_bg_opacity', label: '背景不透明度(%)', type: 'number', placeholder: '75', hint: '0-100，背景图遮罩透明度' },
        { key: 'admin_content_opacity', label: '内容不透明度(%)', type: 'number', placeholder: '85', hint: '0-100，内容卡片透明度' }
      ]
    },
    {
      title: '用户中心',
      fields: [
        { key: 'user_system_name', label: '系统名称', type: 'text', placeholder: '例如：LXD API - 用户中心' },
        { key: 'user_system_title', label: '浏览器标题', type: 'text', placeholder: '例如：用户中心 - LXD容器管理系统' },
        { key: 'user_login_title', label: '登录标题', type: 'text', placeholder: '例如：用户登录' },
        { key: 'user_bg_image', label: '背景图 URL', type: 'text', placeholder: 'https://...' },
        { key: 'user_bg_opacity', label: '背景不透明度(%)', type: 'number', placeholder: '75' },
        { key: 'user_content_opacity', label: '内容不透明度(%)', type: 'number', placeholder: '85' },
        { key: 'user_notice', label: '用户公告', type: 'textarea', placeholder: '登录后用户中心展示的公告内容', rows: 3 },
        { key: 'user_notice_opacity', label: '公告不透明度(%)', type: 'number', placeholder: '85' }
      ]
    },
    {
      title: '容器面板',
      fields: [
        { key: 'container_system_name', label: '系统名称', type: 'text', placeholder: '例如：LXD API - 容器控制' },
        { key: 'container_system_title', label: '浏览器标题', type: 'text', placeholder: '例如：容器管理 - LXD容器管理系统' },
        { key: 'container_login_title', label: '登录标题', type: 'text', placeholder: '例如：容器登录' },
        { key: 'container_bg_image', label: '背景图 URL', type: 'text', placeholder: 'https://...' },
        { key: 'container_bg_opacity', label: '背景不透明度(%)', type: 'number', placeholder: '75' },
        { key: 'container_content_opacity', label: '内容不透明度(%)', type: 'number', placeholder: '85' },
        { key: 'container_notice', label: '容器公告', type: 'textarea', placeholder: '容器控制台展示的公告内容', rows: 3 },
        { key: 'container_notice_opacity', label: '公告不透明度(%)', type: 'number', placeholder: '85' }
      ]
    },
    {
      title: '全局',
      fields: [
        { key: 'favicon_url', label: 'Favicon URL', type: 'text', placeholder: 'https://...', hint: '浏览器标签页图标' },
        { key: 'footer_text', label: '页脚文本', type: 'text', placeholder: '例如：LXD API 容器管理平台' },
        { key: 'container_lite_template', label: 'Lite 模板', type: 'text', placeholder: 'lite1', hint: '容器面板精简模板名' },
        { key: 'container_base_template', label: 'Base 模板', type: 'text', placeholder: 'base1', hint: '容器面板基础模板名' },
        { key: 'use_local_cdn', label: '使用本地 CDN', type: 'checkbox', hint: '开启后页面资源使用本地静态文件，不依赖外网 CDN' }
      ]
    },
    {
      title: 'TLS 证书（https 部署）',
      fields: [
        { key: 'tls_cert_content', label: '证书内容 (PEM)', type: 'textarea', placeholder: '-----BEGIN CERTIFICATE-----', rows: 5 },
        { key: 'tls_key_content', label: '私钥内容 (PEM)', type: 'textarea', placeholder: '-----BEGIN PRIVATE KEY-----', rows: 5 }
      ]
    }
  ];

  function fieldHtml(f) {
    let input;
    if (f.type === 'checkbox') {
      input = '<input id="bf_' + f.key + '" type="checkbox" style="width:18px;height:18px;margin-top:8px">';
    } else if (f.type === 'number') {
      input = '<input id="bf_' + f.key + '" type="number" min="0" max="100" class="input" style="width:100%;margin-top:8px" placeholder="' + f.placeholder + '">';
    } else if (f.type === 'textarea') {
      input = '<textarea id="bf_' + f.key + '" class="input" rows="' + (f.rows || 3) + '" style="width:100%;margin-top:8px" placeholder="' + f.placeholder + '"></textarea>';
    } else {
      input = '<input id="bf_' + f.key + '" class="input" style="width:100%;margin-top:8px" placeholder="' + f.placeholder + '">';
    }
    return '<div class="card" style="padding:16px 18px">' +
      '<label style="font-size:13px;font-weight:600;color:var(--text-1)">' + f.label + '</label>' +
      input +
      (f.hint ? '<div style="font-size:12px;color:var(--text-3);margin-top:6px">' + f.hint + '</div>' : '') +
      '</div>';
  }

  content.innerHTML =
    GROUPS.map(g =>
      '<div class="section-head" style="margin:4px 0 10px"><h3>' + g.title + '</h3></div>' +
      '<div class="grid-2" style="margin-bottom:16px">' + g.fields.map(fieldHtml).join('') + '</div>'
    ).join('') +
    '<div class="card" style="padding:16px 18px;display:flex;gap:12px;align-items:center;flex-wrap:wrap">' +
    '<span style="font-size:13px;color:var(--text-3)">修改后需刷新页面生效；留空表示使用默认值。</span>' +
    '<span class="grow"></span>' +
    '<button class="btn btn-ghost btn-sm" onclick="loadBrandCfg()">↻ 重新加载</button>' +
    '<button class="btn btn-danger-ghost btn-sm" onclick="resetBrandCfg()">恢复默认</button>' +
    '<button class="btn btn-primary btn-sm" id="brandSave">保存设置</button></div>';

  function uget(u, keys, def) {
    for (const k of keys) {
      if (u[k] !== undefined && u[k] !== null) return u[k];
    }
    return def;
  }

  function allFields() {
    const arr = [];
    GROUPS.forEach(g => g.fields.forEach(f => arr.push(f)));
    return arr;
  }

  window.loadBrandCfg = function () {
    ADMIN.request('/api/admin/brand-settings').then((res) => {
      const d = res.data || {};
      allFields().forEach((f) => {
        const el = document.getElementById('bf_' + f.key);
        if (!el) return;
        if (f.type === 'checkbox') {
          el.checked = !!d[f.key];
        } else {
          const v = uget(d, [f.key], '');
          el.value = (v === null || v === undefined) ? '' : v;
        }
      });
      LXD.toast('success', '已加载');
    }).catch((err) => LXD.toast('error', err.message || '加载失败'));
  };

  window.resetBrandCfg = function () {
    if (!window.confirm('确定恢复品牌设置为默认值？当前自定义配置将被覆盖。')) return;
    ADMIN.request('/api/admin/brand-settings?reset=true', { method: 'POST', body: {} })
      .then(() => { LXD.toast('success', '已恢复默认'); loadBrandCfg(); })
      .catch((err) => LXD.toast('error', err.message || '重置失败'));
  };

  document.getElementById('brandSave').addEventListener('click', () => {
    const data = {};
    allFields().forEach((f) => {
      const el = document.getElementById('bf_' + f.key);
      if (!el) return;
      if (f.type === 'checkbox') {
        data[f.key] = el.checked;
      } else if (f.type === 'number') {
        data[f.key] = el.value === '' ? 0 : parseInt(el.value, 10);
      } else {
        data[f.key] = el.value.trim();
      }
    });
    const btn = document.getElementById('brandSave');
    btn.disabled = true; btn.textContent = '保存中...';
    ADMIN.request('/api/admin/brand-settings', { method: 'POST', body: data })
      .then(() => {
        LXD.toast('success', '保存成功');
        try {
          LXD.applyBrand(data);
          const name = document.getElementById('brandName');
          if (name) name.textContent = data.admin_system_name || 'LXD 管理后台';
          const logo = document.getElementById('brandLogo');
          if (logo) {
            if (data.admin_bg_image) logo.innerHTML = '<img src="' + data.admin_bg_image + '" alt="logo">';
            else logo.textContent = (data.admin_system_name || 'L').charAt(0).toUpperCase();
          }
          document.title = (data.admin_system_name || 'LXD 管理后台') + ' - 品牌设置';
        } catch (e) { /* 忽略局部刷新失败 */ }
        btn.disabled = false; btn.textContent = '保存设置';
      })
      .catch((err) => { LXD.toast('error', err.message || '保存失败'); btn.disabled = false; btn.textContent = '保存设置'; });
  });

  loadBrandCfg();
})();
