/* 商品管理 */
(function () {
  'use strict';
  ADMIN.shell('products', '商品管理', '产品套餐与定价');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  // 列表筛选（按机房/节点）
  const filter = { region: '', node: '' };
  try { Object.assign(filter, JSON.parse(localStorage.getItem('lxd_admin_product_filter') || '{}')); } catch (e) {}

  function render(list, nodes, regions) {
    const nodeMap = {};
    (nodes || []).forEach(n => { nodeMap[n.ID || n.id] = n.name; });
    const regionOpts = '<option value="">全部机房</option>' + (regions || []).map(r =>
      '<option value="' + (r.ID || r.id) + '"' + (String(r.ID || r.id) === filter.region ? ' selected' : '') + '>' + LXD.esc(r.name) + '</option>').join('');
    const nodeFilterOpts = '<option value="">全部节点</option>' + (nodes || []).filter(n => !filter.region || String(n.region_id) === filter.region).map(n =>
      '<option value="' + (n.ID || n.id) + '"' + (String(n.ID || n.id) === filter.node ? ' selected' : '') + '>' + LXD.esc(n.name || '') + '</option>').join('');
    c.innerHTML =
      '<div class="row-actions" style="margin-bottom:12px;flex-wrap:wrap;gap:8px"><button class="btn btn-primary" id="btnNew">新增商品</button>' +
      '<select class="input" id="fRegion" style="width:auto;min-width:140px">' + regionOpts + '</select>' +
      '<select class="input" id="fNode" style="width:auto;min-width:140px">' + nodeFilterOpts + '</select>' +
      '<span class="sub" style="align-self:center">共 ' + list.length + ' 个商品</span></div>' +
      (list.length ? list.map(p =>
        '<div class="card product-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(p.name || '') +
        (p.type === 'vm' ? ' <span class="tag tag-vm">VM</span>' : ' <span class="tag">CT</span>') +
        ' <span class="tag">' + (p.status === 'active' ? '上架' : '下架') + '</span></div>' +
        '<div class="sub">#' + (p.ID || p.id) + ' · ' + LXD.esc(p.region_name || '未分机房') + ' / ' + LXD.esc(p.node_name || nodeMap[p.node_id] || ('节点#' + (p.node_id || 0))) + (p.storage_pool ? ' · 存储池 ' + LXD.esc(p.storage_pool) : '') + ' · 库存 ' + (p.stock < 0 ? '不限' : (p.stock === 0 ? '0（售罄）' : p.stock)) + (p.per_user_limit > 0 ? ' · 每人限购 ' + p.per_user_limit : '') + '</div>' +
        '<div class="sub">CPU ' + (p.cpu || 0) + ' 核 / 内存 ' + (p.memory || 0) + ' MB / 硬盘 ' + (p.disk || 0) + ' GB / 流量 ' + (p.traffic_limit || 0) + ' GB / IPv4 x' + (p.ipv4_count || 0) + '</div></div>' +
        '<div style="text-align:right">' +
        '<div style="font-weight:700;color:var(--accent)">¥' + Number(p.price_monthly || 0).toFixed(2) + '/月起</div>' +
        '<div class="row-actions">' +
        '<button class="btn btn-sm" data-edit="' + (p.ID || p.id) + '">编辑</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + (p.ID || p.id) + '" data-name="' + LXD.esc(p.name || '') + '">删除</button>' +
        '</div></div></div>').join('') : '<div class="empty">暂无商品</div>');

    document.getElementById('fRegion').addEventListener('change', (e) => { filter.region = e.target.value; filter.node = ''; saveFilter(); load(); });
    document.getElementById('fNode').addEventListener('change', (e) => { filter.node = e.target.value; saveFilter(); load(); });
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
    // 新建商品默认选中当前筛选的节点（或唯一节点）
    if (!id) {
      if (filter.node) p.node_id = parseInt(filter.node, 10);
      else if ((nodes || []).length === 1) p.node_id = nodes[0].ID || nodes[0].id;
    }
    const nodeOpts = (nodes || []).map(n => '<option value="' + (n.ID || n.id) + '"' + (p.node_id === (n.ID || n.id) ? ' selected' : '') + '>' + LXD.esc((n.region ? n.region + ' / ' : '') + (n.name || '')) + ' (#' + (n.ID || n.id) + ')</option>').join('');
    c.innerHTML =
      '<div class="card" style="max-width:760px">' +
      '<div class="card-title">' + (id ? '编辑商品' : '新增商品') + '</div>' +
      '<div class="form-grid">' +
      '  <div class="field"><label>名称</label><input class="input" id="pName" value="' + LXD.esc(p.name || '') + '"></div>' +
      '  <div class="field"><label>类型</label><select class="input" id="pType"><option value="container"' + (p.type !== 'vm' ? ' selected' : '') + '>容器</option><option value="vm"' + (p.type === 'vm' ? ' selected' : '') + '>虚拟机</option></select><div class="sub" id="pTypeHint" style="font-size:12px;color:var(--danger,#e5484d);display:none">节点开通接口只能创建 LXC 容器，所选节点不支持虚拟机；虚拟机仅支持具备 KVM 的主控本机</div></div>' +
      '  <div class="field"><label>所属节点（机房 / 节点）</label><select class="input" id="pNode"><option value="0">主控本机</option>' + nodeOpts + '</select><div class="sub" style="font-size:12px;color:var(--text-2)">镜像与存储池列表取自所选节点</div></div>' +
      '  <div class="field"><label>系统镜像</label><select class="input" id="pImage"><option value="">加载中...</option></select></div>' +
      '  <div class="field"><label>存储池</label><select class="input" id="pPool"><option value="">加载中...</option></select></div>' +
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
      '  <div class="field"><label>库存</label><input class="input" id="pStock" type="number" min="-1" value="' + (p.stock === undefined || p.stock === null ? -1 : p.stock) + '"><div class="sub" style="font-size:12px;color:var(--text-2)">-1 = 不限；0 = 售罄；下单即预占，取消未支付订单自动归还</div></div>' +
      '  <div class="field"><label>每人限购</label><input class="input" id="pLimit" type="number" min="0" value="' + (p.per_user_limit || 0) + '"><div class="sub" style="font-size:12px;color:var(--text-2)">0 = 不限；按用户当前持有的实例 + 待支付订单计数</div></div>' +
      '  <div class="field"><label>状态</label><select class="input" id="pStatus"><option value="active"' + (p.status === 'active' ? ' selected' : '') + '>上架</option><option value="disabled"' + (p.status !== 'active' ? ' selected' : '') + '>下架</option></select></div>' +
      '  <div class="field" style="grid-column:1/-1"><label>描述</label><textarea class="input" id="pDesc" rows="3">' + LXD.esc(p.description || '') + '</textarea></div>' +
      '</div>' +
      '<div class="row-actions"><button class="btn btn-primary" id="btnSave">保存</button>' +
      '<button class="btn btn-ghost" id="btnCancel">返回</button></div></div>';

    // 系统镜像 / 存储池下拉：从所选节点读取（node_id 由主控转发到节点）；留空则开通时使用节点默认值
    const imgSel = document.getElementById('pImage');
    const poolSel = document.getElementById('pPool');
    const nodeSel = document.getElementById('pNode');
    function nodeQuery() { const v = nodeSel.value; return v && v !== '0' ? '?node_id=' + encodeURIComponent(v) : ''; }
    function loadNodeOptions(keepCur) {
      const curImg = keepCur ? (p.image || '') : '';
      const curPool = keepCur ? (p.storage_pool || '') : '';
      imgSel.innerHTML = '<option value="">加载中...</option>';
      poolSel.innerHTML = '<option value="">加载中...</option>';
      ADMIN.request('/api/admin/image-options' + nodeQuery()).then(res => {
        const list = (res.data && res.data.images) || [];
        let html = '<option value="">自动（默认镜像）</option>' + list.map(t =>
          '<option value="' + LXD.esc(t.value) + '">' + LXD.esc(t.label || t.value) + (t.source === 'local' ? '' : '（远程）') + '</option>').join('');
        if (curImg && !list.some(t => t.value === curImg)) html += '<option value="' + LXD.esc(curImg) + '">' + LXD.esc(curImg) + '（节点上未找到）</option>';
        imgSel.innerHTML = html;
        imgSel.value = curImg;
      }).catch(e => { imgSel.innerHTML = '<option value="' + LXD.esc(curImg) + '">' + LXD.esc(curImg || '自动（默认镜像）') + '</option>'; LXD.toast('error', '读取节点镜像失败：' + e.message); });
      ADMIN.request('/api/admin/storage-pools' + nodeQuery()).then(res => {
        const list = (res.data && res.data.pools) || [];
        let html = '<option value="">自动（节点优先级默认池）</option>' + list.map(sp =>
          '<option value="' + LXD.esc(sp.name) + '">' + LXD.esc(sp.name) + ' · ' + LXD.esc(sp.driver || '') + (sp.priority > 0 ? ' · 优先级 ' + sp.priority : ' · 未启用') + '</option>').join('');
        if (curPool && !list.some(sp => sp.name === curPool)) html += '<option value="' + LXD.esc(curPool) + '">' + LXD.esc(curPool) + '（节点上未找到）</option>';
        poolSel.innerHTML = html;
        poolSel.value = curPool;
      }).catch(() => { poolSel.innerHTML = '<option value="' + LXD.esc(curPool) + '">' + LXD.esc(curPool || '自动（节点优先级默认池）') + '</option>'; });
    }
    loadNodeOptions(true);
    nodeSel.addEventListener('change', () => loadNodeOptions(String(p.node_id || 0) === nodeSel.value));
    // 虚拟机类型 + 远程节点：节点不支持，给出提示（保存时后端也会拒绝）
    const typeSel = document.getElementById('pType');
    function typeHint() { document.getElementById('pTypeHint').style.display = (typeSel.value === 'vm' && nodeSel.value !== '0') ? '' : 'none'; }
    typeSel.addEventListener('change', typeHint);
    nodeSel.addEventListener('change', typeHint);
    typeHint();

    document.getElementById('btnSave').addEventListener('click', () => {
      // 以原商品数据为基础，保留表单未展示的字段（带宽、存储池、嵌套等），避免保存时被清零
      const body = Object.assign({}, p, {
        image: imgSel.value,
        storage_pool: poolSel.value,
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
        stock: (function (v) { v = parseInt(v, 10); return isNaN(v) ? -1 : (v < 0 ? -1 : v); })(document.getElementById('pStock').value),
        per_user_limit: Math.max(0, parseInt(document.getElementById('pLimit').value, 10) || 0),
        status: document.getElementById('pStatus').value,
        description: document.getElementById('pDesc').value
      });
      if (!body.name) { LXD.toast('error', '商品名称必填'); return; }
      // 注意：不能在此块内再声明名为 p 的变量（会遮蔽上方商品对象 p，触发 TDZ 报错导致保存无反应）
      const url = id ? '/api/admin/products/' + id : '/api/admin/products';
      ADMIN.request(url, { method: id ? 'PUT' : 'POST', body: body })
        .then(() => { LXD.toast('success', '已保存'); load(); }).catch(e => LXD.toast('error', e.message));
    });
    document.getElementById('btnCancel').addEventListener('click', load);
  }

  function saveFilter() { try { localStorage.setItem('lxd_admin_product_filter', JSON.stringify(filter)); } catch (e) {} }

  function load() {
    const q = [];
    if (filter.region) q.push('filter_region=' + encodeURIComponent(filter.region));
    if (filter.node) q.push('filter_node=' + encodeURIComponent(filter.node));
    ADMIN.request('/api/admin/products' + (q.length ? '?' + q.join('&') : '')).then(res => {
      window.__products = res.data && res.data.products || [];
      return Promise.all([
        ADMIN.request('/api/admin/nodes'),
        ADMIN.request('/api/admin/regions').catch(() => ({ data: { regions: [] } }))
      ]).then(([r2, r3]) => ({ list: window.__products, nodes: r2.data && r2.data.nodes || [], regions: r3.data && r3.data.regions || [] }));
    }).then(x => render(x.list, x.nodes, x.regions)).catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
