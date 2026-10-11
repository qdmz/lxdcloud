/* 易支付配置 */
(function () {
  'use strict';
  ADMIN.shell('pay', '支付配置', '易支付网关设置');
  const c = document.getElementById('adminContent');
  c.innerHTML = '<div class="loading">加载中...</div>';

  function render(p) {
    c.innerHTML =
      '<div class="card" style="max-width:720px">' +
      '<div class="card-title">易支付网关</div>' +
      '<div class="field"><label>启用支付</label><select class="input" id="yEnabled">' +
      '<option value="true"' + (p.enabled ? ' selected' : '') + '>启用</option>' +
      '<option value="false"' + (!p.enabled ? ' selected' : '') + '>停用</option></select></div>' +
      '<div class="field"><label>网关地址</label><input class="input" id="yGateway" value="' + LXD.esc(p.gateway_url || '') + '" placeholder="https://pay.example.com/submit.php"></div>' +
      '<div class="field"><label>商户ID (PID)</label><input class="input" id="yPid" value="' + LXD.esc(p.pid || '') + '"></div>' +
      '<div class="field"><label>商户密钥 (Key)</label><input class="input" id="yKey" type="password" value="' + LXD.esc(p.key || '') + '"></div>' +
      '<div class="field"><label>异步通知地址 (notify_url)</label><input class="input" id="yNotify" value="' + LXD.esc(p.notify_url || '') + '" placeholder="https://面板域名/api/public/pay/notify"></div>' +
      '<div class="field"><label>同步跳转地址 (return_url)</label><input class="input" id="yReturn" value="' + LXD.esc(p.return_url || '') + '" placeholder="https://面板域名/api/public/pay/return"></div>' +
      '<div class="row-actions"><button class="btn btn-primary" id="btnSave">保存配置</button></div>' +
      '<div class="sub" style="margin-top:10px">停用支付时，新订单将直接自动开通（测试模式）。</div>' +
      '</div>';
    document.getElementById('btnSave').addEventListener('click', () => {
      ADMIN.request('/api/admin/pay', { method: 'POST', body: {
        enabled: document.getElementById('yEnabled').value === 'true',
        gateway_url: document.getElementById('yGateway').value,
        pid: document.getElementById('yPid').value,
        key: document.getElementById('yKey').value,
        notify_url: document.getElementById('yNotify').value,
        return_url: document.getElementById('yReturn').value
      }}).then(() => { LXD.toast('success', '支付配置已保存'); }).catch(e => LXD.toast('error', e.message));
    });
  }

  function load() {
    ADMIN.request('/api/admin/pay').then(res => render(res.data && (res.data.config || res.data.pay || res.data) || {})).catch(e => {
      c.innerHTML = '<div class="empty">加载失败：' + LXD.esc(e.message) + '</div>';
    });
  }
  load();
})();
