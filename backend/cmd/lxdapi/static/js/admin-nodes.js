/* 节点管理 */
(function () {
  'use strict';
  ADMIN.shell('nodes', '节点管理', '子节点接入与状态检测');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  function render(list) {
    c.innerHTML =
      '<div class="row-actions" style="margin-bottom:12px"><button class="btn btn-primary" id="btnNew">新增节点</button></div>' +
      (list.length ? list.map(n =>
        '<div class="card node-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(n.name || '') + ' <span class="tag">#' + (n.ID || n.id) + '</span></div>' +
        '<div class="sub">' + LXD.esc(n.api_base_url || '') + (n.region ? ' · ' + LXD.esc(n.region) : '') + '</div>' +
        '<div class="sub">' + LXD.esc(n.remark || '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div class="row-actions">' +
        '<button class="btn btn-sm" data-test="' + (n.ID || n.id) + '">测试</button>' +
        '<button class="btn btn-sm" data-edit="' + (n.ID || n.id) + '">编辑</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + (n.ID || n.id) + '" data-name="' + LXD.esc(n.name || '') + '">删除</button>' +
        '</div></div></div>').join('') : '<div class="empty">暂无节点</div>');

    document.getElementById('btnNew').addEventListener('click', () => editNode(null));
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
      '<div class="field"><label>地区</label><input class="input" id="nRegion" value="' + LXD.esc(item.region || '') + '" placeholder="如：上海"></div>' +
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
    ADMIN.request('/api/admin/nodes').then(res => {
      window.__nodes = res.data && res.data.nodes || [];
      render(window.__nodes);
    }).catch(e => { c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>'; });
  }
  load();
})();
