/* 商品管理 */
(function () {
  'use strict';
  ADMIN.shell('products', '商品管理', '产品套餐与定价');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  function render(list, nodes) {
    const nodeMap = {};
    (nodes || []).forEach(n => { nodeMap[n.ID || n.id] = n.name; });
    c.innerHTML =
      '<div class="row-actions" style="margin-bottom:12px"><button class="btn btn-primary" id="btnNew">新增商品</button></div>' +
      (list.length ? list.map(p =>
        '<div class="card product-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(p.name || '') +
        (p.type === 'vm' ? ' <span class="tag tag-vm">VM</span>' : ' <span class="tag">CT</span>') +
        ' <span class="tag">' + (p.status === 'active' ? '上架' : '下架') + '</span></div>' +
        '<div class="sub">#' + (p.ID || p.id) + ' · 节点#' + (p.node_id || 0) + ' ' + LXD.esc(nodeMap[p.node_id] || '') + ' · 库存 ' + (p.stock || 0) + '</div>' +
        '<div class="sub">CPU ' + (p.cpu || 0) + ' 核 / 内存 ' + (p.memory || 0) + ' MB / 硬盘 ' + (p.disk || 0) + ' GB / 流量 ' + (p.traffic_limit || 0) + ' GB / IPv4 x' + (p.ipv4_count || 0) + '</div></div>' +
        '<div style="text-align:right">' +
        '<div style="font-weight:700;color:var(--accent)">¥' + Number(p.price_monthly || 0).toFixed(2) + '/月起</div>' +
        '<div class="row-actions">' +
        '<button class="btn btn-sm" data-edit="' + (p.ID || p.id) + '">编辑</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + (p.ID || p.id) + '" data-name="' + LXD.esc(p.name || '') + '">删除</button>' +
        '</div></div></div>').join('') : '<div class="empty">暂无商品</div>');

    document.getElementById('btnNew').addEventListener('click', () => editProduct(null, nodes));
    c.querySelectorAll('[data-edit]').forEach(b => b.addEventListener('click', () => editProduct(parseInt(b.dataset.edit, 10), nodes)));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => {
      if (!window.confirm('确定删除商品「' + b.dataset.name + '」？')) return;
      ADMIN.request('/api/admin/products/' + b.dataset.del, { method: 'DELETE' })
        .then(() => { LXD.toast('success', '商品已删除'); load(); }).catch(e => LXD.toast('error', e.message));
    }));
  }

  function editProduct(id, nodes) {
    let p = {};
    if (id) {
      const all = window.__products || [];
      p = all.find(x => x.ID === id || x.id === id) || {};
    }
    const nodeOpts = (nodes || []).map(n => '<option value="' + (n.ID || n.id) + '"' + (p.node_id === (n.ID || n.id) ? ' selected' : '') + '>#' + (n.ID || n.id) + ' ' + LXD.esc(n.name || '') + '</option>').join('');
    c.innerHTML =
      '<div class="card" style="max-width:760px">' +
      '<div class="card-title">' + (id ? '编辑商品' : '新增商品') + '</div>' +
      '<div class="form-grid">' +
      '  <div class="field"><label>名称</label><input class="input" id="pName" value="' + LXD.esc(p.name || '') + '"></div>' +
      '  <div class="field"><label>类型</label><select class="input" id="pType"><option value="container"' + (p.type !== 'vm' ? ' selected' : '') + '>容器</option><option value="vm"' + (p.type === 'vm' ? ' selected' : '') + '>虚拟机</option></select></div>' +
      '  <div class="field"><label>所属节点</label><select class="input" id="pNode"><option value="0">主控(本机)</option>' + nodeOpts + '</select></div>' +
      '  <div class="field"><label>CPU(核)</label><input class="input" id="pCPU" type="number" value="' + (p.cpu || 1) + '"></div>' +
      '  <div class="field"><label>内存(MB)</label><input class="input" id="pMem" type="number" value="' + (p.memory || 512) + '"></div>' +
      '  <div class="field"><label>硬盘(GB)</label><input class="input" id="pDisk" type="number" value="' + (p.disk || 10) + '"></div>' +
      '  <div class="field"><label>流量(GB/月)</label><input class="input" id="pTraffic" type="number" value="' + (p.traffic_limit || 0) + '"></div>' +
      '  <div class="field"><label>IPv4 数量</label><input class="input" id="pV4" type="number" value="' + (p.ipv4_count || 0) + '"></div>' +
      '  <div class="field"><label>IPv6 数量</label><input class="input" id="pV6" type="number" value="' + (p.ipv6_count || 0) + '"></div>' +
      '  <div class="field"><label>端口映射数</label><input class="input" id="pPorts" type="number" value="' + (p.port_mappings || 0) + '"></div>' +
      '  <div class="field"><label>月付(元)</label><input class="input" id="pM" type="number" step="0.01" value="' + (p.price_monthly || 0) + '"></div>' +
      '  <div class="field"><label>季付(元)</label><input class="input" id="pQ" type="number" step="0.01" value="' + (p.price_quarterly || 0) + '"></div>' +
      '  <div class="field"><label>半年付(元)</label><input class="input" id="pH" type="number" step="0.01" value="' + (p.price_half_year || 0) + '"></div>' +
      '  <div class="field"><label>年付(元)</label><input class="input" id="pY" type="number" step="0.01" value="' + (p.price_yearly || 0) + '"></div>' +
      '  <div class="field"><label>库存</label><input class="input" id="pStock" type="number" value="' + (p.stock || 0) + '"></div>' +
      '  <div class="field"><label>状态</label><select class="input" id="pStatus"><option value="active"' + (p.status === 'active' ? ' selected' : '') + '>上架</option><option value="disabled"' + (p.status !== 'active' ? ' selected' : '') + '>下架</option></select></div>' +
      '  <div class="field" style="grid-column:1/-1"><label>描述</label><textarea class="input" id="pDesc" rows="3">' + LXD.esc(p.description || '') + '</textarea></div>' +
      '</div>' +
      '<div class="row-actions"><button class="btn btn-primary" id="btnSave">保存</button>' +
      '<button class="btn btn-ghost" id="btnCancel">返回</button></div></div>';

    document.getElementById('btnSave').addEventListener('click', () => {
      const body = {
        name: document.getElementById('pName').value,
        type: document.getElementById('pType').value,
        node_id: parseInt(document.getElementById('pNode').value, 10),
        cpu: parseInt(document.getElementById('pCPU').value, 10) || 0,
        memory: parseInt(document.getElementById('pMem').value, 10) || 0,
        disk: parseInt(document.getElementById('pDisk').value, 10) || 0,
        traffic_limit: parseInt(document.getElementById('pTraffic').value, 10) || 0,
        ipv4_count: parseInt(document.getElementById('pV4').value, 10) || 0,
        ipv6_count: parseInt(document.getElementById('pV6').value, 10) || 0,
        port_mappings: parseInt(document.getElementById('pPorts').value, 10) || 0,
        price_monthly: parseFloat(document.getElementById('pM').value) || 0,
        price_quarterly: parseFloat(document.getElementById('pQ').value) || 0,
        price_half_year: parseFloat(document.getElementById('pH').value) || 0,
        price_yearly: parseFloat(document.getElementById('pY').value) || 0,
        stock: parseInt(document.getElementById('pStock').value, 10) || 0,
        status: document.getElementById('pStatus').value,
        description: document.getElementById('pDesc').value
      };
      if (!body.name) { LXD.toast('error', '商品名称必填'); return; }
      const p = id ? '/api/admin/products/' + id : '/api/admin/products';
      ADMIN.request(p, { method: id ? 'PUT' : 'POST', body: body })
        .then(() => { LXD.toast('success', '已保存'); load(); }).catch(e => LXD.toast('error', e.message));
    });
    document.getElementById('btnCancel').addEventListener('click', load);
  }

  function load() {
    ADMIN.request('/api/admin/products').then(res => {
      window.__products = res.data && res.data.products || [];
      return ADMIN.request('/api/admin/nodes').then(r2 => ({ list: window.__products, nodes: r2.data && r2.data.nodes || [] }));
    }).then(x => render(x.list, x.nodes)).catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
