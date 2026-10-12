/* 账户余额：余额 / 在线充值 / 收支记录 */
(function () {
  'use strict';
  USER.shell('balance', '账户余额', '余额充值与收支记录');
  const c = document.getElementById('userContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  const TYPE = { recharge: '充值', pay: '消费', refund: '退款', adjust: '调整' };
  const PRESETS = [10, 30, 50, 100, 200, 500];
  let payType = 'alipay';
  let amount = 50;

  function fmtTime(t) {
    if (!t) return '-';
    const d = new Date(t);
    if (isNaN(d)) return String(t).slice(0, 16);
    const z = n => String(n).padStart(2, '0');
    return d.getFullYear() + '-' + z(d.getMonth() + 1) + '-' + z(d.getDate()) + ' ' + z(d.getHours()) + ':' + z(d.getMinutes());
  }
  function money(n) { return '¥' + Number(n || 0).toFixed(2); }

  function render(d) {
    const logs = d.logs || [];
    let income = 0, spend = 0;
    logs.forEach(l => { if (l.amount > 0) income += l.amount; else spend += -l.amount; });
    const min = d.recharge_min || 1, max = d.recharge_max || 50000;

    const recharge = d.epay_enabled
      ? '<div class="card bal-recharge">' +
        '<div class="card-title">在线充值</div>' +
        '<div class="renew-label">充值金额</div>' +
        '<div class="renew-grid">' + PRESETS.map(v => '<button type="button" class="renew-opt' + (v === amount ? ' active' : '') + '" data-amt="' + v + '"><b>¥' + v + '</b></button>').join('') + '</div>' +
        '<div class="bal-custom"><span>自定义</span><input type="number" id="rcAmt" min="' + min + '" max="' + max + '" step="0.01" value="' + amount + '"><span class="sub">元（' + min + ' ~ ' + max + '）</span></div>' +
        '<div class="renew-label">支付方式</div>' +
        '<div class="renew-grid">' +
        '<button type="button" class="renew-opt' + (payType === 'alipay' ? ' active' : '') + '" data-pt="alipay"><b>支付宝</b><span>在线支付</span></button>' +
        '<button type="button" class="renew-opt' + (payType === 'wxpay' ? ' active' : '') + '" data-pt="wxpay"><b>微信支付</b><span>在线支付</span></button>' +
        '</div>' +
        '<button class="btn btn-primary btn-block" id="rcGo" style="margin-top:16px">立即充值 ' + money(amount) + '</button>' +
        '<div class="sub" style="margin-top:8px">支付成功后余额自动到账，可用于新购、续费和自动续费。</div>' +
        '</div>'
      : '<div class="card bal-recharge"><div class="card-title">在线充值</div><div class="sub">站点暂未开启在线支付，请联系管理员充值。</div></div>';

    c.innerHTML =
      '<div class="bal-top">' +
      '<div class="card bal-hero"><div class="sub">当前余额</div><div class="bal-num">' + money(d.balance) + '</div>' +
      '<div class="bal-mini"><span>累计入账 <b>' + money(income) + '</b></span><span>累计支出 <b>' + money(spend) + '</b></span></div></div>' +
      recharge + '</div>' +
      '<div class="card" style="margin-top:12px"><div class="card-title">收支记录</div>' +
      (logs.length
        ? '<div class="bal-logs">' + logs.map(l =>
          '<div class="bal-log"><div><div style="font-weight:600">' + (TYPE[l.type] || l.type) + (l.remark ? ' · ' + LXD.esc(l.remark) : '') + '</div>' +
          '<div class="sub">' + fmtTime(l.CreatedAt || l.created_at) + (l.order_no ? ' · ' + LXD.esc(l.order_no) : '') + '</div></div>' +
          '<div style="text-align:right"><div class="' + (l.amount >= 0 ? 'bal-in' : 'bal-out') + '">' + (l.amount >= 0 ? '+' : '') + Number(l.amount).toFixed(2) + '</div>' +
          '<div class="sub">余额 ' + money(l.balance) + '</div></div></div>').join('') + '</div>'
        : '<div class="empty">暂无记录</div>') +
      '</div>';

    if (!d.epay_enabled) return;
    const btn = c.querySelector('#rcGo');
    const inp = c.querySelector('#rcAmt');
    const sync = () => { btn.textContent = '立即充值 ' + money(amount); };
    c.querySelectorAll('[data-amt]').forEach(b => b.onclick = () => {
      amount = Number(b.dataset.amt); inp.value = amount;
      c.querySelectorAll('[data-amt]').forEach(x => x.classList.toggle('active', x === b)); sync();
    });
    inp.oninput = () => {
      amount = Math.round(Number(inp.value || 0) * 100) / 100;
      c.querySelectorAll('[data-amt]').forEach(x => x.classList.toggle('active', Number(x.dataset.amt) === amount)); sync();
    };
    c.querySelectorAll('[data-pt]').forEach(b => b.onclick = () => {
      payType = b.dataset.pt;
      c.querySelectorAll('[data-pt]').forEach(x => x.classList.toggle('active', x === b));
    });
    btn.onclick = () => {
      if (!(amount >= min && amount <= max)) { LXD.toast('error', '充值金额需在 ¥' + min + ' ~ ¥' + max + ' 之间'); return; }
      btn.disabled = true; btn.textContent = '正在创建充值订单...';
      USER.request('/api/user/balance/recharge', { method: 'POST', body: { amount: amount, type: payType } }).then(r => {
        const url = r.data && r.data.pay_url;
        if (url) { window.location.href = url; return; }
        LXD.toast('error', '未获取到支付地址'); btn.disabled = false; sync();
      }).catch(e => { LXD.toast('error', e.message); btn.disabled = false; sync(); });
    };
  }

  function load() {
    USER.request('/api/user/balance').then(r => render(r.data || {})).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  const done = new URLSearchParams(location.search).get('recharged');
  if (done) { LXD.toast('success', '支付成功，余额已到账（订单 ' + done + '）'); history.replaceState(null, '', location.pathname); }
  load();
})();
