/* ============================================================
   LXD Panel - 控制台页逻辑
   协议：WS /ws/console?token= ；onmessage 直接 term.write；
   term.onData 发送 {type:'input',data}；令牌一次性不可重连
   ============================================================ */
(function () {
  'use strict';

  const urlParams = new URLSearchParams(window.location.search);
  const token = urlParams.get('token');

  if (!token) {
    alert('缺少访问令牌');
    window.close();
    return;
  }

  // 品牌信息（尽力而为，失败不阻塞终端）
  LXD.loadBrand().then((brand) => {
    document.title = (brand.site_name || 'LXD 容器面板') + ' - Shell 控制台';
    LXD.applyBrand(brand);
    const cn = document.getElementById('containerName');
    if (cn && brand.container_name) cn.textContent = brand.container_name;
  }).catch(() => {});

  const term = new Terminal({
    cursorBlink: true,
    cursorStyle: 'block',
    fontSize: 14,
    fontFamily: 'Consolas, Monaco, "Courier New", monospace',
    scrollback: 999999999,
    convertEol: false,
    allowProposedApi: true,
    drawBoldTextInBrightColors: true,
    allowTransparency: false,
    theme: {
      background: '#000000',
      foreground: '#e6edf3',
      cursor: '#ffffff',
      cursorAccent: '#000000',
      selection: 'rgba(56, 189, 248, 0.3)',
    },
    windowsMode: false,
    fastScrollModifier: 'shift',
    fastScrollSensitivity: 5,
    scrollSensitivity: 1,
    smoothScrollDuration: 0,
  });

  const fitAddon = new FitAddon.FitAddon();
  term.loadAddon(fitAddon);
  term.open(document.getElementById('terminal'));
  fitAddon.fit();

  const statusDot = document.getElementById('status-dot');
  const statusText = document.getElementById('status-text');
  const connectBtn = document.getElementById('connect-btn');
  const disconnectOverlay = document.getElementById('disconnect-overlay');
  const overlayClose = document.getElementById('overlay-close');

  let ws = null;
  let currentFontSize = 14;
  let tokenUsed = false;

  function setStatus(state) {
    statusDot.className = 'status-dot' + (state ? ' ' + state : '');
    const textMap = { connecting: '连接中...', connected: '已连接', closed: '已断开' };
    statusText.textContent = textMap[state] || '未连接';
  }

  function connect() {
    if (ws && ws.readyState === WebSocket.OPEN) return;
    if (tokenUsed) {
      alert('令牌已使用，无法重新连接。请关闭窗口并重新打开控制台。');
      return;
    }

    setStatus('connecting');
    connectBtn.textContent = '连接中...';
    connectBtn.disabled = true;
    disconnectOverlay.classList.add('hidden');

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = protocol + '//' + window.location.host + '/ws/console?token=' + encodeURIComponent(token);

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      tokenUsed = true;
      setStatus('connected');
      connectBtn.textContent = '断开';
      connectBtn.className = 'btn btn-danger btn-sm';
      connectBtn.disabled = false;
    };

    ws.onmessage = (event) => {
      term.write(event.data);
    };

    ws.onclose = () => {
      setStatus('closed');
      connectBtn.textContent = '已失效';
      connectBtn.className = 'btn btn-ghost btn-sm';
      connectBtn.disabled = true;
      disconnectOverlay.classList.remove('hidden');
    };

    ws.onerror = (e) => {
      console.error('WebSocket 错误:', e);
    };
  }

  function disconnect() {
    if (ws && ws.readyState === WebSocket.OPEN) ws.close();
  }

  connectBtn.addEventListener('click', () => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      disconnect();
    } else {
      connect();
    }
  });

  overlayClose.addEventListener('click', () => window.close());

  term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data: data }));
    }
  });

  document.getElementById('font-decrease').addEventListener('click', () => {
    if (currentFontSize > 10) {
      currentFontSize--;
      term.options.fontSize = currentFontSize;
      fitAddon.fit();
    }
  });

  document.getElementById('font-increase').addEventListener('click', () => {
    if (currentFontSize < 24) {
      currentFontSize++;
      term.options.fontSize = currentFontSize;
      fitAddon.fit();
    }
  });

  document.getElementById('clear-terminal').addEventListener('click', () => term.clear());

  document.getElementById('fullscreen').addEventListener('click', () => {
    if (!document.fullscreenElement) {
      (document.documentElement.requestFullscreen || document.documentElement.webkitRequestFullscreen).call(document.documentElement);
    } else {
      (document.exitFullscreen || document.webkitExitFullscreen).call(document);
    }
  });

  // ---------- SFTP 连接信息 ----------
  document.getElementById('sftp-info').addEventListener('click', () => {
    const host = urlParams.get('host');
    const user = urlParams.get('user') || 'root';
    const pwd = urlParams.get('pwd') || '';
    const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, (m) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[m]));

    const mask = document.createElement('div');
    mask.className = 'modal-mask show';
    const kv = (k, v) => '<tr><td style="color:var(--text-2);padding:6px 10px 6px 0;white-space:nowrap;vertical-align:top">' + k + '</td><td class="mono" style="padding:6px 0;word-break:break-all">' + esc(v) + '</td></tr>';
    let body = '<table style="width:100%;border-collapse:collapse;font-size:13px">' +
      kv('SFTP 地址', host ? 'sftp://' + user + '@' + host + '（端口 22）' : '未获取到容器 IP，请返回容器管理页查看') +
      kv('用户名', user) +
      kv('密码', pwd || '未随控制台令牌传递，请以容器详情/管理页展示的 SSH 密码为准') +
      '</table>';
    body += '<div style="font-size:12px;color:var(--text-3);line-height:1.7;margin-top:8px;padding-top:10px;border-top:1px solid rgba(255,255,255,.08)">' +
      '可使用任意 SFTP 客户端（WinSCP / FileZilla / Transmit 等）连接：协议选择 <b>SFTP</b>，主机 ' + esc(host || '-') + '，端口 <b>22</b>，用户名 <b>root</b>，密码为上述 SSH 密码。' +
      '</div>';
    mask.innerHTML = '<div class="modal" style="max-width:520px"><h3>SFTP 连接信息</h3><div style="max-height:50vh;overflow:auto">' + body + '</div>' +
      '<div class="modal-actions"><button class="btn btn-ghost" data-act="close">关闭</button></div></div>';
    document.body.appendChild(mask);
    mask.querySelector('[data-act="close"]').onclick = () => mask.remove();
    mask.addEventListener('click', (e) => { if (e.target === mask) mask.remove(); });
  });

  window.addEventListener('resize', () => fitAddon.fit());

  // 移动端横竖屏切换也重算
  if (window.visualViewport) {
    window.visualViewport.addEventListener('resize', () => fitAddon.fit());
  }

  connect();
})();
