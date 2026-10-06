/* ============================================================
   LXD Panel - 共享工具库
   ============================================================ */

const LXD = {
  API_BASE: '',

  /** 统一 AJAX */
  request(url, options = {}) {
    options.method = options.method || 'GET';
    options.headers = Object.assign(
      { 'Content-Type': 'application/json' },
      options.headers || {}
    );
    if (options.body && typeof options.body !== 'string') {
      options.body = JSON.stringify(options.body);
    }
    return fetch(LXD.API_BASE + url, options).then(async (res) => {
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        const err = new Error(data.msg || data.message || '请求失败');
        err.status = res.status;
        err.data = data;
        throw err;
      }
      return data;
    });
  },

  /** 带容器 Hash 的请求 */
  authRequest(url, options = {}) {
    const hash = LXD.getHash();
    if (!hash) return Promise.reject(new Error('未找到访问码'));
    options.headers = Object.assign({ 'X-Container-Hash': hash }, options.headers || {});
    return LXD.request(url, options);
  },

  getHash() {
    const params = new URLSearchParams(window.location.search);
    const fromUrl = params.get('hash');
    if (fromUrl) {
      localStorage.setItem('container_hash', fromUrl);
      history.replaceState(null, '', window.location.pathname);
      return fromUrl;
    }
    return localStorage.getItem('container_hash') || '';
  },

  clearHash() {
    localStorage.removeItem('container_hash');
  },

  /** 加载品牌信息，返回 Promise<brand> */
  loadBrand() {
    return LXD.request('/api/public/brand').then((res) => res.data || {});
  },

  applyBrand(brand) {
    document.title = (brand.site_name || 'LXD 容器面板') + ' - ' + (brand.page_title || '容器管理');
    const logo = document.getElementById('brandLogo');
    if (logo) {
      if (brand.logo_url) {
        logo.innerHTML = '<img src="' + brand.logo_url + '" alt="logo">';
      } else {
        logo.textContent = (brand.site_name || 'L').charAt(0).toUpperCase();
      }
    }
    const name = document.getElementById('brandName');
    if (name) name.textContent = brand.site_name || 'LXD 容器面板';
    const fav = document.querySelector('link[rel="icon"]');
    if (brand.favicon_url && fav) fav.href = brand.favicon_url;
  },

  /** Toast */
  toast(type, msg) {
    let wrap = document.querySelector('.toast-wrap');
    if (!wrap) {
      wrap = document.createElement('div');
      wrap.className = 'toast-wrap';
      document.body.appendChild(wrap);
    }
    const el = document.createElement('div');
    el.className = 'toast toast-' + type;
    const icons = { success: '✓', error: '✕', warning: '!', info: 'ℹ' };
    el.innerHTML = '<span style="flex-shrink:0;font-weight:700">' + (icons[type] || 'ℹ') + '</span><span>' + msg + '</span>';
    wrap.appendChild(el);
    setTimeout(() => {
      el.style.transition = 'opacity .3s, transform .3s';
      el.style.opacity = '0';
      el.style.transform = 'translateY(-8px)';
      setTimeout(() => el.remove(), 320);
    }, 3600);
  },

  /** 确认对话框（Promise<boolean>） */
  confirmDialog(title, text, danger = true) {
    return new Promise((resolve) => {
      const mask = document.createElement('div');
      mask.className = 'modal-mask show';
      mask.innerHTML =
        '<div class="modal" style="max-width:400px">' +
        '<h3>' + title + '</h3>' +
        '<p style="color:var(--text-2);font-size:14px">' + text + '</p>' +
        '<div class="modal-actions">' +
        '<button class="btn btn-ghost" data-act="cancel">取消</button>' +
        '<button class="btn ' + (danger ? 'btn-danger' : 'btn-primary') + '" data-act="ok">确定</button>' +
        '</div></div>';
      document.body.appendChild(mask);
      const close = (val) => { mask.remove(); resolve(val); };
      mask.querySelector('[data-act="cancel"]').onclick = () => close(false);
      mask.querySelector('[data-act="ok"]').onclick = () => close(true);
      mask.addEventListener('click', (e) => { if (e.target === mask) close(false); });
    });
  },

  /** 通用模态框：打开/关闭 */
  openModal(id) { document.getElementById(id).classList.add('show'); },
  closeModal(id) { document.getElementById(id).classList.remove('show'); },

  /** 工具函数 */
  fmtBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  },
  parseSize(str) {
    if (!str) return 0;
    const m = String(str).match(/^(\d+(?:\.\d+)?)\s*(KB|MB|GB|TB|B)?$/i);
    if (!m) return 0;
    const units = { B: 1, KB: 1024, MB: 1024 * 1024, GB: 1024 * 1024 * 1024, TB: 1024 ** 4 };
    return parseFloat(m[1]) * (units[(m[2] || 'B').toUpperCase()] || 1);
  },
  fmtTime(t) {
    if (!t) return '-';
    const d = new Date(t);
    if (isNaN(d.getTime())) return '-';
    return d.toLocaleString('zh-CN', { hour12: false });
  },
  esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  },
  debounce(fn, wait = 300) {
    let t;
    return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), wait); };
  },

  /** 轻量折线走势图（canvas） */
  sparkline(canvas, values, color) {
    if (!canvas || !values || !values.length) return;
    const ctx = canvas.getContext('2d');
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth, h = canvas.clientHeight;
    canvas.width = w * dpr; canvas.height = h * dpr;
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, w, h);
    const max = Math.max(...values, 1), min = Math.min(...values, 0);
    const range = max - min || 1;
    const step = w / (values.length - 1 || 1);
    // 渐变填充
    const grad = ctx.createLinearGradient(0, 0, 0, h);
    grad.addColorStop(0, color + '55');
    grad.addColorStop(1, color + '00');
    ctx.beginPath();
    values.forEach((v, i) => {
      const x = i * step, y = h - ((v - min) / range) * (h - 8) - 4;
      i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
    });
    ctx.strokeStyle = color; ctx.lineWidth = 2; ctx.lineJoin = 'round';
    ctx.stroke();
    // 填充
    ctx.lineTo(w, h); ctx.lineTo(0, h); ctx.closePath();
    ctx.fillStyle = grad; ctx.fill();
  },

  /** 环形进度（SVG） */
  ring(el, percent, color) {
    const r = el.querySelector('.ring-fg');
    if (!r) return;
    const circ = 2 * Math.PI * r.r.baseVal.value;
    const p = Math.max(0, Math.min(100, percent));
    r.style.strokeDasharray = circ + 'px';
    r.style.strokeDashoffset = (circ * (1 - p / 100)) + 'px';
    r.style.stroke = color || 'var(--primary)';
    el.querySelector('.ring-val').textContent = Math.round(p) + '%';
  },

  /** 状态徽章 */
  statusBadge(status) {
    status = (status || '').toLowerCase();
    const map = {
      running: ['badge-running', '运行中', true],
      stopped: ['badge-stopped', '已停止', false],
      frozen: ['badge-frozen', '已暂停', false]
    };
    const m = map[status] || ['badge-other', (status || '未知'), false];
    return '<span class="badge ' + m[0] + '">' + (m[2] ? '<span class="dot"></span>' : '') + m[1] + '</span>';
  },

  /** 处理 401/403：清 hash 跳登录 */
  handleAuthError(err) {
    if (err && (err.status === 401 || err.status === 403)) {
      LXD.clearHash();
      window.location.href = 'login.html';
      return true;
    }
    return false;
  }
};
