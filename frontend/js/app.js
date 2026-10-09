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

  /** 加载品牌信息，返回 Promise<brand>（同一页面内只请求一次） */
  loadBrand() {
    if (!LXD._brandP) {
      LXD._brandP = LXD.request('/api/public/brand').then((res) => {
        const b = Object.assign({}, res.data || {});
        // 按所在端选用对应的系统名称
        const p = window.location.pathname;
        if (p.indexOf('/user/') >= 0 && b.user_system_name) { b.site_name = b.user_system_name; b.page_title = b.user_system_title || b.page_title; }
        else if (p.indexOf('/container/') >= 0 && b.container_system_name) { b.site_name = b.container_system_name; b.page_title = b.container_system_title || b.page_title; }
        return b;
      });
    }
    return LXD._brandP;
  },

  applyBrand(brand) {
    brand = brand || {};
    document.title = (brand.site_name || 'LXD 容器面板') + ' - ' + (brand.page_title || '容器管理');
    const logo = document.getElementById('brandLogo');
    if (logo) {
      if (brand.logo_url && /^(https?:)?\/\//.test(brand.logo_url)) {
        logo.innerHTML = '<img src="' + LXD.esc(brand.logo_url) + '" alt="logo">';
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
    // 消息统一转义，防止后端错误信息中的 HTML 被执行
    el.innerHTML = '<span style="flex-shrink:0;font-weight:700">' + (icons[type] || 'ℹ') + '</span><span>' + LXD.esc(msg) + '</span>';
    el.setAttribute('role', type === 'error' ? 'alert' : 'status');
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
      const onKey = (e) => { if (e.key === 'Escape') close(false); else if (e.key === 'Enter') close(true); };
      const close = (val) => { document.removeEventListener('keydown', onKey); mask.remove(); resolve(val); };
      mask.querySelector('[data-act="cancel"]').onclick = () => close(false);
      mask.querySelector('[data-act="ok"]').onclick = () => close(true);
      mask.addEventListener('click', (e) => { if (e.target === mask) close(false); });
      document.addEventListener('keydown', onKey);
      setTimeout(() => { const b = mask.querySelector('[data-act="cancel"]'); if (b) b.focus(); }, 30);
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

/* ============================================================
   v1.1 通用增强（三端共享，自动生效，不改动各页面业务逻辑）
   - 主题切换按钮（深色/浅色，记忆选择）
   - 复制：LXD.copy(text) + [data-copy] 委托 + 详情值自动加复制按钮
   - 表格自动包裹横向滚动容器
   - 防重复提交：表单提交 / 主要按钮点击后，在其触发的请求完成前锁定
   - 登录页：后端关闭验证码时自动隐藏验证码输入
   ============================================================ */
(function () {
  'use strict';

  /* ---------- 请求追踪（用于防重复提交） ---------- */
  const inflight = new Set();
  let fetchSeq = 0;
  if (window.fetch && !window.fetch.__lxdWrapped) {
    const raw = window.fetch.bind(window);
    const wrapped = function () {
      const p = raw.apply(null, arguments);
      fetchSeq++;
      inflight.add(p);
      const done = () => inflight.delete(p);
      p.then(done, done);
      return p;
    };
    wrapped.__lxdWrapped = true;
    window.fetch = wrapped;
  }

  function lockUntilIdle(el) {
    if (!el || el.dataset.lxdLocked === '1') return;
    el.dataset.lxdLocked = '1';
    el.classList.add('is-loading');
    el.setAttribute('aria-busy', 'true');
    const started = Date.now();
    const release = () => {
      delete el.dataset.lxdLocked;
      el.classList.remove('is-loading');
      el.removeAttribute('aria-busy');
    };
    const check = () => {
      if (!inflight.size || Date.now() - started > 30000) { setTimeout(release, 120); return; }
      Promise.allSettled(Array.from(inflight)).then(() => setTimeout(check, 60));
    };
    setTimeout(check, 30);
  }

  // 已锁定的按钮/表单：拦截重复点击与重复提交（捕获阶段，先于业务处理器）
  document.addEventListener('click', (e) => {
    const btn = e.target.closest && e.target.closest('button, .btn');
    if (!btn) return;
    if (btn.dataset.lxdLocked === '1') { e.preventDefault(); e.stopImmediatePropagation(); return; }
    if (btn.hasAttribute('data-no-lock') || btn.classList.contains('btn-ghost') || btn.classList.contains('theme-toggle') || btn.classList.contains('copy-btn')) return;
    if (btn.type === 'submit' && btn.form) return; // 交给 submit 处理
    const seq = fetchSeq;
    // 业务处理器同步发起请求 → 锁定按钮直到请求结束
    setTimeout(() => { if (fetchSeq !== seq && document.body.contains(btn)) lockUntilIdle(btn); }, 0);
  }, true);

  document.addEventListener('submit', (e) => {
    const form = e.target;
    if (!(form instanceof HTMLFormElement)) return;
    if (form.dataset.lxdLocked === '1') { e.preventDefault(); e.stopImmediatePropagation(); return; }
    const seq = fetchSeq;
    setTimeout(() => {
      if (fetchSeq === seq) return;
      form.dataset.lxdLocked = '1';
      const btn = form.querySelector('button[type="submit"], button:not([type]), input[type="submit"]');
      if (btn) lockUntilIdle(btn);
      const unlock = () => { if (!inflight.size) delete form.dataset.lxdLocked; else Promise.allSettled(Array.from(inflight)).then(() => setTimeout(unlock, 60)); };
      setTimeout(unlock, 30);
    }, 0);
  }, true);

  /* ---------- 复制 ---------- */
  LXD.copy = function (text, okMsg) {
    text = String(text == null ? '' : text);
    const ok = () => { LXD.toast('success', okMsg || '已复制到剪贴板'); return true; };
    const fallback = () => {
      const ta = document.createElement('textarea');
      ta.value = text; ta.setAttribute('readonly', '');
      ta.style.cssText = 'position:fixed;top:-1000px;opacity:0';
      document.body.appendChild(ta); ta.select();
      let r = false;
      try { r = document.execCommand('copy'); } catch (e) { r = false; }
      ta.remove();
      if (r) return ok();
      LXD.toast('error', '复制失败，请手动复制');
      return false;
    };
    if (navigator.clipboard && window.isSecureContext) {
      return navigator.clipboard.writeText(text).then(ok, fallback);
    }
    return Promise.resolve(fallback());
  };

  document.addEventListener('click', (e) => {
    const el = e.target.closest && e.target.closest('[data-copy]');
    if (!el) return;
    e.preventDefault();
    const v = el.getAttribute('data-copy') || (el.previousElementSibling && el.previousElementSibling.textContent) || '';
    Promise.resolve(LXD.copy(v.trim())).then(() => {
      if (el.classList.contains('copy-btn')) {
        el.classList.add('copied'); const t = el.textContent; el.textContent = '已复制';
        setTimeout(() => { el.classList.remove('copied'); el.textContent = t; }, 1400);
      }
    });
  });

  // 为详情区 / 表格中的等宽值（IP、访问码、密钥等）自动追加复制按钮
  function addCopyButtons(root) {
    const nodes = (root.querySelectorAll ? root : document).querySelectorAll('.kv-item .v code.mono, .kv-item .v .mono, td code.mono, [data-copyable]');
    nodes.forEach((n) => {
      if (n.dataset.lxdCopy || n.closest('button') || n.closest('.copy-btn')) return;
      const txt = (n.textContent || '').trim();
      if (!txt || txt === '-' || txt === '···' || txt.length < 3 || txt.length > 300) return;
      n.dataset.lxdCopy = '1';
      const b = document.createElement('button');
      b.type = 'button'; b.className = 'copy-btn'; b.textContent = '复制'; b.title = '复制';
      b.setAttribute('data-copy', txt);
      n.insertAdjacentElement('afterend', b);
    });
  }

  /* ---------- 表格横向滚动 ---------- */
  function wrapTables(root) {
    (root.querySelectorAll ? root : document).querySelectorAll('table').forEach((t) => {
      const p = t.parentElement;
      if (!p || t.closest('.xterm')) return;
      if (p.classList.contains('tbl-wrap') || p.classList.contains('table-wrap') || getComputedStyle(p).overflowX === 'auto') return;
      const w = document.createElement('div');
      w.className = 'tbl-wrap';
      p.insertBefore(w, t);
      w.appendChild(t);
    });
  }

  /* ---------- 主题切换按钮 ---------- */
  LXD.mountThemeToggle = function (container) {
    if (!window.LXDTheme) return null;
    const host = container ||
      document.querySelector('.admin-topbar .topbar-actions') ||
      document.querySelector('.user-topbar .u-actions') ||
      document.querySelector('.topbar .topbar-right') ||
      document.querySelector('.console-bar');
    let btn = document.querySelector('.theme-toggle');
    if (btn && (!host || host.contains(btn))) return btn;
    if (btn) btn.remove();
    btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'theme-toggle';
    btn.addEventListener('click', () => LXDTheme.toggle());
    if (host) host.insertBefore(btn, host.firstChild);
    else { btn.classList.add('theme-toggle-float'); document.body.appendChild(btn); }
    LXDTheme.apply(LXDTheme.get());
    return btn;
  };

  /* ---------- 登录页：按后端配置显示/隐藏验证码 ---------- */
  function adaptCaptcha() {
    const row = document.querySelector('.captcha-row');
    if (!row) return;
    fetch('/api/public/site-info').then((r) => r.json()).then((res) => {
      const d = res && res.data;
      if (!d || d.captcha_enabled !== false) return;
      const field = row.closest('.field') || row;
      field.style.display = 'none';
      const input = row.querySelector('input');
      if (input) { input.removeAttribute('required'); if (!input.value) input.value = '0000'; }
    }).catch(() => {});
  }

  function enhance(root) { try { wrapTables(root); addCopyButtons(root); } catch (e) { /* ignore */ } }

  let pending = false;
  const mo = new MutationObserver(() => {
    if (pending) return;
    pending = true;
    requestAnimationFrame(() => { pending = false; enhance(document); if (!document.querySelector('.theme-toggle')) LXD.mountThemeToggle(); });
  });

  function init() {
    enhance(document);
    LXD.mountThemeToggle();
    adaptCaptcha();
    mo.observe(document.body, { childList: true, subtree: true });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else setTimeout(init, 0);
})();
