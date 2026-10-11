/* 我的实例：列表 / 续费 / 删除 */
(function () {
  'use strict';
  USER.shell('instances', '我的实例', '已购容器与虚拟机');
  const c = document.getElementById('userContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const STATUS = { creating: '开通中', active: '运行中', suspended: '已暂停', expired: '已到期', deleting: '删除中' };
  const PERIOD = { monthly: '月付', quarterly: '季付', half_year: '半年付', yearly: '年付' };

  function fmtTime(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '-'; }

  function render(list) {
    if (!list.length) { c.innerHTML = '<div class="empty">暂无实例，去 <a href="/user/store" style="color:var(--accent)">产品订购</a> 看看吧</div>'; return; }
    c.innerHTML = list.map(it => {
      let cfg = {};
      try { cfg = it.config_json ? JSON.parse(it.config_json) : {}; } catch (e) {}
      const productName = it.product_name || (cfg.name || ('商品#' + it.product_id));
      const expired = it.status === 'active' && it.expire_at && new Date(it.expire_at) < new Date();
      const statusText = expired ? '已到期' : (STATUS[it.status] || it.status);
      return '<div class="card inst-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(productName) + ' <span class="tag">' + (it.type === 'vm' ? 'VM' : '容器') + '</span></div>' +
        '<div class="sub">' + LXD.esc(it.name || '') + ' · 节点#' + (it.node_id || 0) + '</div>' +
        '<div class="sub">到期：' + fmtTime(it.expire_at) + (it.auto_renew ? ' · 自动续费' : '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div class="inst-status ' + (expired || it.status === 'suspended' ? 'warn' : '') + '">' + statusText + '</div>' +
        '<div class="row-actions">' +
        '<button class="btn btn-sm" data-renew="' + it.id + '" data-name="' + LXD.esc(it.name || '') + '">续费</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + it.id + '" data-name="' + LXD.esc(it.name || '') + '">删除</button>' +
        '</div></div></div>';
    }).join('');

    c.querySelectorAll('[data-renew]').forEach(b => b.addEventListener('click', () => renew(b.dataset.renew, b.dataset.name)));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => del(b.dataset.del, b.dataset.name)));
  }

  /* ============ 续费弹窗：选择周期 → 选择支付方式 → 下单跳转支付 ============ */
  const RENEW_PERIODS = [
    { key: 'monthly', text: '月付' },
    { key: 'quarterly', text: '季付' },
    { key: 'half_year', text: '半年付' },
    { key: 'yearly', text: '年付' }
  ];
  const PAY_TYPES = [
    { key: 'alipay', text: '支付宝' },
    { key: 'wxpay', text: '微信支付' },
    { key: 'qqpay', text: 'QQ 钱包' }
  ];

  const RENEW_MODAL_CSS = '.lxd-modal{position:fixed;inset:0;z-index:60;display:none;align-items:center;justify-content:center;padding:16px}' +
    '.lxd-modal.show{display:flex}.lxd-modal .m-overlay{position:absolute;inset:0;background:rgba(2,6,17,.72);backdrop-filter:blur(4px)}' +
    '.lxd-modal .m-card{position:relative;width:100%;max-width:440px;border:1px solid var(--border-strong);border-radius:18px;background:var(--card-strong);box-shadow:0 24px 80px rgba(0,0,0,.6)}' +
    '.lxd-modal .m-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border)}' +
    '.lxd-modal .m-title{font-size:16px;font-weight:700;color:var(--text-1)}' +
    '.lxd-modal .m-body{padding:18px 20px}' +
    '.lxd-modal .m-foot{display:flex;justify-content:flex-end;gap:10px;padding:14px 20px;border-top:1px solid var(--border)}' +
    '.lxd-modal .m-section{font-size:12px;font-weight:700;color:var(--primary);margin:14px 0 10px;padding-bottom:6px;border-bottom:1px dashed var(--border)}' +
    '.lxd-modal .m-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}' +
    '.lxd-modal .m-pay{display:flex;align-items:center;gap:8px;font-size:13px;color:var(--text-2);padding:8px 10px;border:1px solid var(--border);border-radius:10px;background:var(--card);cursor:pointer}' +
    '.lxd-modal .m-pay:has(input:checked){border-color:var(--accent);color:var(--accent);background:rgba(255,255,255,.06)}' +
    '.lxd-modal input[type=radio]{accent-color:#38bdf8;width:15px;height:15px}';

  function renewModalCss() {
    if (document.getElementById('renewModalCss')) return;
    const s = document.createElement('style');
    s.id = 'renewModalCss'; s.textContent = RENEW_MODAL_CSS;
    document.head.appendChild(s);
  }

  function renew(id, name) {
    renewModalCss();
    const host = document.createElement('div');
    host.innerHTML = '<div class="lxd-modal show">' +
      '<div class="m-overlay"></div><div class="m-card">' +
      '<div class="m-head"><div class="m-title">续费实例</div></div>' +
      '<div class="m-body">' +
      '<div class="m-section" style="margin-top:0">实例</div>' +
      '<div style="font-size:14px;font-weight:600;color:var(--text-1)">' + LXD.esc(name || '实例#' + id) + '</div>' +
      '<div class="m-section">续费周期</div>' +
      '<div class="period-row" style="margin-bottom:0">' + RENEW_PERIODS.map((p, i) =>
        '<button class="chip' + (i === 0 ? ' active' : '') + '" data-per="' + p.key + '">' + p.text + '</button>').join('') + '</div>' +
      '<div class="m-section">支付方式</div>' +
      '<div class="m-grid">' + PAY_TYPES.map((p, i) =>
        '<label class="m-pay"><input type="radio" name="renewPay" value="' + p.key + '"' + (i === 0 ? ' checked' : '') + '>' + p.text + '</label>').join('') + '</div>' +
      '</div>' +
      '<div class="m-foot"><button class="btn btn-ghost" data-act="cancel">取消</button>' +
      '<button class="btn btn-primary" data-act="save">确认续费</button></div>' +
      '</div></div>';
    const modal = host.firstElementChild;
    document.body.appendChild(modal);

    function close() { modal.remove(); }
    modal.querySelector('.m-overlay').addEventListener('click', close);
    modal.querySelector('[data-act="cancel"]').addEventListener('click', close);

    modal.querySelectorAll('.chip').forEach(ch => ch.addEventListener('click', () => {
      modal.querySelectorAll('.chip').forEach(x => x.classList.remove('active'));
      ch.classList.add('active');
    }));

    modal.querySelector('[data-act="save"]').addEventListener('click', () => {
      const period = (modal.querySelector('.chip.active') || {}).dataset ? modal.querySelector('.chip.active').dataset.per : 'monthly';
      const payType = (modal.querySelector('input[name="renewPay"]:checked') || {}).value || 'alipay';
      close();
      LXD.toast('info', '正在创建续费订单...');
      USER.request('/api/user/instances/' + id + '/renew', { method: 'POST', body: { period: period } }).then(res => {
        const order = res.data && res.data.order ? res.data.order : {};
        LXD.toast('success', '续费订单创建成功，正在跳转支付...');
        setTimeout(() => { window.location.href = '/user/orders?pay=' + (order.id || order.ID || '') + '&type=' + encodeURIComponent(payType); }, 800);
      }).catch(e => LXD.toast('error', e.message));
    });
  }

  function del(id, name) {
    if (!window.confirm('确定删除实例「' + name + '」？此操作不可恢复，底层容器/VM 将被移除。')) return;
    USER.request('/api/user/instances/' + id, { method: 'DELETE' }).then(() => {
      LXD.toast('success', '实例已删除');
      load();
    }).catch(e => LXD.toast('error', e.message));
  }

  function load() {
    USER.request('/api/user/instances').then(res => render(res.data && res.data.instances || [])).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
