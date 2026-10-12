/* 我的实例：列表 / 续费 / 删除 */
(function () {
  'use strict';
  USER.shell('instances', '我的实例', '已购容器与虚拟机');
  const c = document.getElementById('userContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const STATUS = { creating: '开通中', active: '运行中', suspended: '已暂停', expired: '已到期', deleting: '删除中' };
  const PERIOD = { monthly: '月付', quarterly: '季付', half_year: '半年付', yearly: '年付' };

  function fmtTime(t) {
    if (!t) return '-';
    const s = String(t);
    if (/^\d{4}-\d{2}-\d{2}T/.test(s)) {
      // ISO 时间（含时区）按浏览器本地时区显示
      const d = new Date(s);
      if (!isNaN(d)) { const z = n => String(n).padStart(2, '0'); return d.getFullYear() + '-' + z(d.getMonth() + 1) + '-' + z(d.getDate()) + ' ' + z(d.getHours()) + ':' + z(d.getMinutes()); }
    }
    // 后端时间格式如 "Mon 2026-11-09 11:16 AM CST (UTC+08:00)" 或 ISO
    const m = s.match(/(\d{4}-\d{2}-\d{2})[ T](\d{1,2}):(\d{2})(?::\d{2})?(?:\S*)?\s*(AM|PM)?/i);
    if (!m) return s.slice(0, 16);
    let h = parseInt(m[2], 10);
    if (m[4]) { const pm = m[4].toUpperCase() === 'PM'; if (pm && h < 12) h += 12; if (!pm && h === 12) h = 0; }
    return m[1] + ' ' + String(h).padStart(2, '0') + ':' + m[3];
  }
  function toDate(t) { const f = fmtTime(t); return f === '-' ? null : new Date(f.replace(' ', 'T') + ':00'); }

  function render(list) {
    if (!list.length) { c.innerHTML = '<div class="empty">暂无实例，去 <a href="/user/store" style="color:var(--accent)">产品订购</a> 看看吧</div>'; return; }
    c.innerHTML = list.map(it => {
      it.id = it.id || it.ID;
      let cfg = {};
      try { cfg = it.config_json ? JSON.parse(it.config_json) : {}; } catch (e) {}
      const productName = it.product_name || (cfg.name || ('商品#' + it.product_id));
      const expired = it.status === 'active' && it.expire_at && toDate(it.expire_at) && toDate(it.expire_at) < new Date();
      const statusText = expired ? '已到期' : (STATUS[it.status] || it.status);
      return '<div class="card inst-row">' +
        '<div><div style="font-weight:600">' + LXD.esc(productName) + ' <span class="tag">' + (it.type === 'vm' ? 'VM' : '容器') + '</span></div>' +
        '<div class="sub">' + LXD.esc(it.name || '') + ' · 节点#' + (it.node_id || 0) + '</div>' +
        '<div class="sub">到期：' + fmtTime(it.expire_at) + (it.auto_renew ? ' · <span style="color:var(--green)">自动续费(' + (PERIOD[it.renew_period] || '月付') + ')</span>' : '') + '</div></div>' +
        '<div style="text-align:right">' +
        '<div class="inst-status ' + (expired || it.status === 'suspended' ? 'warn' : '') + '">' + statusText + '</div>' +
        '<div class="row-actions">' +
        '<button class="btn btn-sm btn-primary" data-renew="' + it.id + '">续费</button>' +
        '<button class="btn btn-sm" data-auto="' + it.id + '" data-on="' + (it.auto_renew ? 1 : 0) + '" data-per="' + (it.renew_period || 'monthly') + '">' + (it.auto_renew ? '关闭自动续费' : '自动续费') + '</button>' +
        '<button class="btn btn-sm btn-danger-ghost" data-del="' + it.id + '" data-name="' + LXD.esc(it.name || '') + '">删除</button>' +
        '</div></div></div>';
    }).join('');

    c.querySelectorAll('[data-renew]').forEach(b => b.addEventListener('click', () => renew(b.dataset.renew)));
    c.querySelectorAll('[data-auto]').forEach(b => b.addEventListener('click', () => toggleAuto(b.dataset.auto, b.dataset.on !== '1', b.dataset.per)));
    c.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', () => del(b.dataset.del, b.dataset.name)));
  }

  /* ---------- 续费：选择周期 + 支付方式 → 下单 → 支付 → 自动延期 ---------- */
  function money(n) { return Number(n || 0) === 0 ? '免费' : '¥' + Number(n || 0).toFixed(2); }

  function renew(id) {
    USER.request('/api/user/instances/' + id + '/renew-options').then(res => {
      const d = res.data || {};
      const opts = d.options || [];
      if (!opts.length) { LXD.toast('error', '该实例暂无可续费周期'); return; }
      const inst = d.instance || {};
      const balance = Number(d.balance || 0);
      const epay = !!d.epay_enabled;
      let sel = opts[0];
      let pay = balance >= sel.price || !epay ? 'balance' : 'alipay';
      let auto = !!inst.auto_renew;

      const mask = document.createElement('div');
      mask.className = 'modal-mask show';
      document.body.appendChild(mask);
      const close = () => { document.removeEventListener('keydown', onKey); mask.remove(); };
      const onKey = (e) => { if (e.key === 'Escape') close(); };
      document.addEventListener('keydown', onKey);
      mask.addEventListener('click', e => { if (e.target === mask) close(); });

      function payMethods() {
        const m = [{ k: 'balance', t: '余额支付', sub: '余额 ¥' + balance.toFixed(2) }];
        if (epay) { m.push({ k: 'alipay', t: '支付宝', sub: '在线支付' }); m.push({ k: 'wxpay', t: '微信支付', sub: '在线支付' }); }
        return m;
      }
      function draw() {
        const free = Number(sel.price) === 0;
        const lack = !free && pay === 'balance' && balance < sel.price;
        mask.innerHTML =
          '<div class="modal renew-modal" style="max-width:480px;width:calc(100% - 24px)">' +
          '<h3 style="margin-bottom:4px">续费实例</h3>' +
          '<p class="sub" style="margin:0 0 14px">' + LXD.esc(inst.name || '') + ' · 当前到期 ' + fmtTime(inst.expire_at) + '</p>' +
          '<div class="renew-label">续费周期</div>' +
          '<div class="renew-grid">' + opts.map(o =>
            '<button type="button" class="renew-opt' + (o.period === sel.period ? ' active' : '') + '" data-per="' + o.period + '">' +
            '<b>' + o.text + '</b><span>' + money(o.price) + '</span></button>').join('') + '</div>' +
          (free ? '' :
            '<div class="renew-label">支付方式</div>' +
            '<div class="renew-grid">' + payMethods().map(m =>
              '<button type="button" class="renew-opt' + (m.k === pay ? ' active' : '') + '" data-pay="' + m.k + '">' +
              '<b>' + m.t + '</b><span>' + m.sub + '</span></button>').join('') + '</div>') +
          '<div class="renew-sum">' +
          '<div><span>续费后到期</span><b>' + fmtTime(sel.new_expire) + '</b></div>' +
          '<div><span>应付金额</span><b class="renew-amt">' + money(sel.price) + '</b></div>' +
          (lack ? '<div class="renew-warn">余额不足（还差 ¥' + (sel.price - balance).toFixed(2) + '），请选择在线支付或联系管理员充值</div>' : '') +
          '</div>' +
          '<label class="renew-auto"><input type="checkbox" id="rnAuto"' + (auto ? ' checked' : '') + '> 到期前 24 小时自动用余额续费（按所选周期）</label>' +
          '<div class="modal-actions">' +
          '<button class="btn btn-ghost" data-act="cancel">取消</button>' +
          '<button class="btn btn-primary" data-act="ok"' + (lack ? ' disabled' : '') + '>' + (free ? '免费续费' : '确认支付 ' + money(sel.price)) + '</button>' +
          '</div></div>';
        mask.querySelectorAll('[data-per]').forEach(b => b.onclick = () => {
          sel = opts.find(o => o.period === b.dataset.per) || sel;
          if (pay === 'balance' && balance < sel.price && epay) pay = 'alipay';
          draw();
        });
        mask.querySelectorAll('[data-pay]').forEach(b => b.onclick = () => { pay = b.dataset.pay; draw(); });
        mask.querySelector('#rnAuto').onchange = e => { auto = e.target.checked; };
        mask.querySelector('[data-act="cancel"]').onclick = close;
        mask.querySelector('[data-act="ok"]').onclick = submit;
      }

      function submit() {
        const btn = mask.querySelector('[data-act="ok"]');
        btn.disabled = true; btn.textContent = '处理中...';
        // 同步自动续费设置（失败不阻塞续费）
        const autoP = (auto !== !!inst.auto_renew || (auto && inst.renew_period !== sel.period))
          ? USER.request('/api/user/instances/' + id + '/auto-renew', { method: 'POST', body: { enabled: auto, period: sel.period } }).catch(() => {})
          : Promise.resolve();
        autoP.then(() => USER.request('/api/user/instances/' + id + '/renew', { method: 'POST', body: { period: sel.period } }))
          .then(r => {
            const order = (r.data && r.data.order) || {};
            if (order.status === 'paid') return { paid: true };
            return USER.request('/api/user/orders/' + (order.id || order.ID) + '/pay', { method: 'POST', body: { type: pay } }).then(p => p.data || {});
          })
          .then(p => {
            if (p.pay_url) { LXD.toast('info', '正在跳转支付...'); window.location.href = p.pay_url; return; }
            close();
            LXD.toast('success', '续费成功，到期时间已更新至 ' + fmtTime(sel.new_expire));
            setTimeout(load, 900);
          })
          .catch(e => { LXD.toast('error', e.message); btn.disabled = false; btn.textContent = '重试'; });
      }
      draw();
    }).catch(e => LXD.toast('error', e.message));
  }

  function toggleAuto(id, on, period) {
    USER.request('/api/user/instances/' + id + '/auto-renew', { method: 'POST', body: { enabled: on, period: period || 'monthly' } })
      .then(() => { LXD.toast('success', on ? '已开启自动续费' : '已关闭自动续费'); load(); })
      .catch(e => LXD.toast('error', e.message));
  }

  function del(id, name) {
    if (!window.confirm('确定删除实例「' + name + '」？此操作不可恢复，底层容器/VM 将被移除。')) return;
    USER.request('/api/user/instances/' + id, { method: 'DELETE' }).then(() => {
      LXD.toast('success', '实例已删除');
      load();
    }).catch(e => LXD.toast('error', e.message));
  }

  function load() {
    USER.request('/api/user/instances').then(res => {
      render(res.data && res.data.instances || []);
      USER.request('/api/user/balance').then(b => {
        const bal = Number(b.data && b.data.balance || 0);
        let bar = document.getElementById('balBar');
        if (!bar) { bar = document.createElement('div'); bar.id = 'balBar'; bar.className = 'card bal-bar'; c.insertBefore(bar, c.firstChild); }
        bar.innerHTML = '<span>账户余额</span><b>¥' + bal.toFixed(2) + '</b><span class="sub">可用于续费与自动续费</span>';
      }).catch(() => {});
    }).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  // 在线支付返回：提示续费结果
  const renewed = new URLSearchParams(location.search).get('renewed');
  if (renewed) { LXD.toast('success', '支付成功，续费已生效（订单 ' + renewed + '）'); history.replaceState(null, '', location.pathname); }
  load();
})();
