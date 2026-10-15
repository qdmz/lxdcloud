/* 节点管理 */
(function () {
  'use strict';
  ADMIN.shell('nodes', '节点管理', '子节点接入与状态检测');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  function statusTag(n) {
    if (n.status === 'online') return ' <span class="badge badge-running"><span class="dot"></span>在线</span>';
    if (n.status === 'offline') return ' <span class="badge badge-stopped">离线</span>';
    return ' <span class="tag">未检测</span>';
  }

  function regionsCard(regions) {
    return '<div class="card" style="margin-bottom:12px"><div class="card-title">机房 / 地域</div>' +
      '<div class="sub" style="margin-bottom:8px">节点按机房分组；商品自动归属其节点所在机房，用户下单时按机房筛选。编辑节点时填写新的机房名称会自动创建。</div>' +
      (regions.length ? regions.map(r =>
        '<div style="display:flex;align-items:center;gap:8px;padding:6px 0;border-top:1px solid var(--border)">' +
        '<div style="flex:1"><b>' + LXD.esc(r.name) + '</b> <span class="tag">#' + (r.ID || r.id) + '</span>' + (r.status === 'hidden' ? ' <span class="tag">前台隐藏</span>' : '') +
        '<span class="sub"> · 节点 ' + (r.node_count || 0) + ' · 商品 ' + (r.product_count || 0) + (r.description ? ' · ' + LXD.esc(r.description) : '') + '</span></div>' +
        '<button class="btn btn-sm" data-redit="' + (r.ID || r.id) + '">编辑</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-rdel="' + (r.ID || r.id) + '" data-name="' + LXD.esc(r.name) + '">删除</button></div>').join('') : '<div class="empty">暂无机房</div>') +
      '<div class="row-actions" style="margin-top:8px"><button class="btn btn-sm btn-primary" id="btnNewRegion">新增机房</button></div></div>';
  }

  function editRegion(r) {
    r = r || {};
    const name = window.prompt('机房名称', r.name || '');
    if (name === null) return;
    const desc = window.prompt('机房描述（可选，前台展示）', r.description || '');
    if (desc === null) return;
    const sort = window.prompt('排序（数字越小越靠前）', String(r.sort_order || 0));
    if (sort === null) return;
    const hidden = window.confirm('是否在前台显示该机房？\n确定=显示，取消=隐藏');
    const body = { name: name, description: desc, sort_order: parseInt(sort, 10) || 0, status: hidden ? 'active' : 'hidden' };
    const id = r.ID || r.id;
    ADMIN.request(id ? '/api/admin/regions/' + id : '/api/admin/regions', { method: id ? 'PUT' : 'POST', body: body })
      .then(() => { LXD.toast('success', '已保存'); load(); }).catch(e => LXD.toast('error', e.message));
  }

  function render(list, regions) {
    regions = regions || [];
    c.innerHTML = regionsCard(regions) +
      '<div class="row-actions" style="margin-bottom:12px"><button class="btn btn-primary" id="btnNew">新增节点</button></div>' +
      (list.length ? list.map(n =>
        '<div class="card node-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(n.name || '') + ' <span class="tag">#' + (n.ID || n.id) + '</span>' + statusTag(n) + '</div>' +
        '<div class="sub">机房：' + LXD.esc(n.region || '未分机房') + ' · ' + LXD.esc(n.api_base_url || '') + '</div>' +
        '<div class="sub">' + LXD.esc(n.remark || '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div class="row-actions">' +
        '<button class="btn btn-sm" data-test="' + (n.ID || n.id) + '">测试</button>' +
        '<button class="btn btn-sm" data-edit="' + (n.ID || n.id) + '">编辑</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + (n.ID || n.id) + '" data-name="' + LXD.esc(n.name || '') + '">删除</button>' +
        '</div></div></div>').join('') : '<div class="empty">暂无节点</div>');

    document.getElementById('btnNew').addEventListener('click', () => editNode(null));
    document.getElementById('btnNewRegion').addEventListener('click', () => editRegion(null));
    c.querySelectorAll('[data-redit]').forEach(b => b.addEventListener('click', () => editRegion(regions.find(r => String(r.ID || r.id) === b.dataset.redit))));
    c.querySelectorAll('[data-rdel]').forEach(b => b.addEventListener('click', () => {
      if (!window.confirm('确定删除机房「' + b.dataset.name + '」？（机房下不能有节点）')) return;
      ADMIN.request('/api/admin/regions/' + b.dataset.rdel, { method: 'DELETE' })
        .then(() => { LXD.toast('success', '机房已删除'); load(); }).catch(e => LXD.toast('error', e.message));
    }));
    c.querySelectorAll('[data-test]').forEach(b => b.addEventListener('click', () => {
      LXD.toast('info', '正在测试节点连通性...');
      ADMIN.request('/api/admin/nodes/' + b.dataset.test + '/test', { method: 'POST' }).then(res => {
        const d = res.data || {};
        if (d.status === 'online') {
          LXD.toast('success', '节点连接正常，延迟 ' + (d.latency || '-') + 'ms');
        } else {
          LXD.toast('error', d.msg || ('节点状态异常：' + (d.status || '未知')));
        }
      }).catch(e => LXD.toast('error', e.message));
    }));
    c.querySelectorAll('[data-edit]').forEach(b => b.addEventListener('click', () => editNode(parseInt(b.dataset.edit, 10))));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => {
      if (!window.confirm('确定删除节点「' + b.dataset.name + '」？该节点上的商品将不可用。')) return;
      ADMIN.request('/api/admin/nodes/' + b.dataset.del, { method: 'DELETE' })
        .then(() => { LXD.toast('success', '节点已删除'); load(); }).catch(e => LXD.toast('error', e.message));
    }));
  }

  function editNode(id) {
    let item = {};
    if (id) {
      const all = window.__nodes || [];
      item = all.find(n => n.ID === id || n.id === id) || {};
    }
    c.innerHTML =
      '<div class="card" style="max-width:640px">' +
      '<div class="card-title">' + (id ? '编辑节点' : '新增节点') + '</div>' +
      '<div class="field"><label>名称</label><input class="input" id="nName" value="' + LXD.esc(item.name || '') + '" placeholder="如：上海节点"></div>' +
      '<div class="field"><label>API 地址</label><input class="input" id="nURL" value="' + LXD.esc(item.api_base_url || '') + '" placeholder="http://子节点IP:端口"></div>' +
      '<div class="field"><label>API Hash（子节点 config.yaml 中 system.api_hash）</label><input class="input" id="nHash" value="' + LXD.esc(item.api_hash || '') + '"></div>' +
      '<div class="field"><label>机房 / 地域</label><input class="input" id="nRegion" list="regionList" value="' + LXD.esc(item.region || '') + '" placeholder="选择已有机房或输入新机房名称，如：香港">' +
      '<datalist id="regionList">' + (window.__regions || []).map(r => '<option value="' + LXD.esc(r.name) + '">').join('') + '</datalist></div>' +
      '<div class="field"><label>备注</label><input class="input" id="nRemark" value="' + LXD.esc(item.remark || '') + '"></div>' +
      '<div class="row-actions"><button class="btn btn-primary" id="btnSave">保存</button>' +
      '<button class="btn btn-ghost" id="btnCancel">返回</button></div></div>';
    document.getElementById('btnSave').addEventListener('click', () => {
      const body = {
        name: document.getElementById('nName').value,
        api_base_url: document.getElementById('nURL').value,
        api_hash: document.getElementById('nHash').value,
        region: document.getElementById('nRegion').value,
        remark: document.getElementById('nRemark').value
      };
      if (!body.name || !body.api_base_url || !body.api_hash) { LXD.toast('error', '名称/API地址/API Hash 必填'); return; }
      const p = id ? '/api/admin/nodes/' + id : '/api/admin/nodes';
      ADMIN.request(p, { method: id ? 'PUT' : 'POST', body: body })
        .then(() => { LXD.toast('success', '已保存'); load(); }).catch(e => LXD.toast('error', e.message));
    });
    document.getElementById('btnCancel').addEventListener('click', load);
  }

  function load() {
    Promise.all([
      ADMIN.request('/api/admin/nodes'),
      ADMIN.request('/api/admin/regions').catch(() => ({ data: { regions: [] } }))
    ]).then(([res, r2]) => {
      window.__nodes = res.data && res.data.nodes || [];
      window.__regions = r2.data && r2.data.regions || [];
      render(window.__nodes, window.__regions);
    }).catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
