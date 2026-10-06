/* ============================================================
   LXD Panel - Admin 通用列表页
   用法：GENERIC.page({
     api: '/api/admin/xxx',
     title: '页面标题',
     sub: '副标题',
     searchKeys: ['name', 'remark'],
     columns: [
       { label: '名称', keys: ['name', 'Name'], mono: true },
       { label: '状态', keys: ['status', 'Status'], type: 'status' },
       { label: '时间', keys: ['created_at', 'CreatedAt'], type: 'time' },
       { label: '大小', keys: ['size'], type: 'bytes' },
       { label: '操作', render: (row) => '...' }
     ]
   })
   ============================================================ */
(function () {
  'use strict';

  function get(row, keys, def) {
    for (const k of keys) {
      if (row[k] !== undefined && row[k] !== null) return row[k];
    }
    return def;
  }

  function renderCell(row, col) {
    if (col.render === 'PROGRESS') {
      const p = Math.max(0, Math.min(100, parseFloat(get(row, ['progress', 'percent', 'Progress'], 0)) || 0));
      return '<div class="progress" style="min-width:80px"><i style="width:' + p + '%"></i></div>' +
        '<div style="font-size:11px;color:var(--text-3);margin-top:3px">' + Math.round(p) + '%</div>';
    }
    if (typeof col.render === 'function') return col.render(row);
    const v = get(row, col.keys || [col.key], null);
    if (v == null || v === '') return '-';
    switch (col.type) {
      case 'time': return '<span class="mono">' + ADMIN.fmt.time(v) + '</span>';
      case 'bytes': return ADMIN.fmt.bytes(v);
      case 'status': {
        const s = String(v).toLowerCase();
        if (['enabled', 'active', 'running', 'ok', 'success', 'normal', '1', 'true'].includes(s)) {
          return '<span class="badge badge-running"><span class="dot"></span>' + ADMIN.esc(String(v)) + '</span>';
        }
        if (['disabled', 'stopped', 'failed', 'error', 'banned', '0', 'false'].includes(s)) {
          return '<span class="badge badge-stopped">' + ADMIN.esc(String(v)) + '</span>';
        }
        return '<span class="badge badge-other">' + ADMIN.esc(String(v)) + '</span>';
      }
      case 'text':
      default: return ADMIN.esc(String(v));
    }
  }

  function page(opts) {
    ADMIN.shell(opts.active || '', opts.title, opts.sub);
    const content = document.getElementById('adminContent');
    const cols = opts.columns || [];
    let all = [];
    let filtered = [];
    let pageNo = 1;
    const pageSize = 20;

    content.innerHTML =
      '<div class="card" style="padding:12px 14px;margin-bottom:16px"><div class="toolbar">' +
      (opts.buttons || '') +
      '<span class="grow"></span>' +
      '<div class="search-box"><span class="sb-icon">⌕</span><input id="gSearch" class="input" placeholder="搜索..." oninput="doGFilter()"></div>' +
      '</div></div>' +
      '<div class="card" style="padding:14px" id="gArea"></div>';

    function render() {
      const area = document.getElementById('gArea');
      const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
      if (pageNo > totalPages) pageNo = totalPages;
      const rows = filtered.slice((pageNo - 1) * pageSize, pageNo * pageSize);
      if (!rows.length) { area.innerHTML = ADMIN.empty(opts.emptyText || '暂无数据'); return; }
      let h = '<div class="tbl-wrap"><table class="tbl"><thead><tr>';
      cols.forEach((c) => { h += '<th>' + c.label + '</th>'; });
      h += '</tr></thead><tbody>';
      rows.forEach((row) => {
        h += '<tr>';
        cols.forEach((c) => { h += '<td' + (c.mono ? ' class="mono"' : '') + '>' + renderCell(row, c) + '</td>'; });
        h += '</tr>';
      });
      h += '</tbody></table></div>';
      const pager = '<div style="display:flex;gap:10px;align-items:center;justify-content:flex-end;margin-top:12px;font-size:13px">' +
        '<span style="color:var(--text-3)">共 ' + filtered.length + ' 条</span>' +
        '<button class="btn btn-ghost btn-sm" onclick="goGPage(' + (pageNo - 1) + ')" ' + (pageNo <= 1 ? 'disabled' : '') + '>‹ 上一页</button>' +
        '<span style="color:var(--text-2)">' + pageNo + ' / ' + totalPages + '</span>' +
        '<button class="btn btn-ghost btn-sm" onclick="goGPage(' + (pageNo + 1) + ')" ' + (pageNo >= totalPages ? 'disabled' : '') + '>下一页 ›</button></div>';
      area.innerHTML = h + pager;
    }

    function load() {
      let url = opts.api;
      if (opts.params) {
        const qs = Object.keys(opts.params).map((k) => encodeURIComponent(k) + '=' + encodeURIComponent(opts.params[k])).join('&');
        url += (url.indexOf('?') >= 0 ? '&' : '?') + qs;
      }
      ADMIN.request(url).then((res) => {
        const data = res.data;
        let rows = null;
        if (opts.dataKey && data && Array.isArray(data[opts.dataKey])) rows = data[opts.dataKey];
        else if (opts.dataKeys && data) {
          rows = [];
          opts.dataKeys.forEach((k) => { if (Array.isArray(data[k])) rows = rows.concat(data[k]); });
        }
        if (rows == null) {
          if (Array.isArray(data)) rows = data;
          else if (data && Array.isArray(data.list)) rows = data.list;
          else if (data && Array.isArray(data.items)) rows = data.items;
          else if (data && Array.isArray(data.rules)) rows = data.rules;
          else rows = [];
        }
        all = rows;
        doFilter();
      }).catch(() => { document.getElementById('gArea').innerHTML = ADMIN.empty('加载失败'); });
    }

    function doFilter() {
      const kw = (document.getElementById('gSearch').value || '').trim().toLowerCase();
      const keys = opts.searchKeys || [];
      filtered = kw ? all.filter((row) =>
        keys.some((k) => String(get(row, Array.isArray(k) ? k : [k], '')).toLowerCase().includes(kw))
      ) : all.slice();
      pageNo = 1;
      render();
    }

    window.doGFilter = doFilter;
    window.goGPage = (p) => {
      const tp = Math.max(1, Math.ceil(filtered.length / pageSize));
      if (p < 1 || p > tp) return;
      pageNo = p; render();
    };
    window.load = load;
    window.__gReload = load;

    load();
  }

  window.GENERIC = { page, get };
})();
