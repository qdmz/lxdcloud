# LXD Panel 前端套件（v2 商业化版）

面向 `lxdapi-web-server`（v3 商业化二次开发后端）的独立静态前端，覆盖 **容器面板（Container）**、**管理后台（Admin）**、**用户中心（User）** 三端，新增多节点/商品/订单/实例/工单/消息/邮件/支付等商业化模块。

- 纯 HTML / CSS / JS 实现，不修改 Go 后端代码，通过 REST API + WebSocket 对接
- 深色科技风 + 玻璃拟态，手机优先的响应式设计，桌面端自动适配
- 三端共享同一套设计系统（`css/style.css`）与公共工具库（`js/app.js`）

---

## 目录

1. [项目概览](#项目概览)
2. [目录结构](#目录结构)
3. [快速开始](#快速开始)
4. [三端功能说明](#三端功能说明)
5. [鉴权机制](#鉴权机制)
6. [API 对接要点](#api-对接要点)
7. [控制台（WebSocket）协议](#控制台websocket协议)
8. [脚本与页面对照表](#脚本与页面对照表)
9. [注意事项](#注意事项)

---

## 项目概览

| 端 | 入口 | 面向 | 主要能力 |
|---|---|---|---|
| Container | `container/login.html` → `container/dashboard.html` / `container/console.html` | 终端用户 | 资源监控、IP 管理、端口映射、反向代理、DNS、重装、重置密码、Web 终端 |
| Admin | `admin/login.html` → `admin/dashboard.html` | 运维/管理员 | 容器管理、用户管理、模板、存储池、IP 池、端口映射、Nginx、任务、防火墙、品牌设置、主机资源 |
| User | `user/login.html` → `user/index.html` | 平台用户 | 概览统计、我的容器、端口映射、反向代理、模板、任务 |

技术栈：原生 HTML/CSS/JS、Fetch API、WebSocket（xterm.js）、Nginx 反向代理。

---

## 目录结构

```
lxdpanel/
├── index.html                  # 首页（三端入口介绍）
├── container/                  # 容器面板端
│   ├── login.html              # 访问码登录
│   ├── dashboard.html          # 仪表盘
│   ├── console.html            # Web 终端
│   └── js/
│       ├── login.js
│       ├── dashboard.js
│       └── console.js
├── deploy.sh                   # 一键部署脚本（Nginx 反代）
├── README.md
├── css/
│   ├── style.css               # 共享设计系统（三端通用）
│   ├── admin.css               # Admin 端样式
│   └── user.css                # User 端样式
├── js/
│   ├── app.js                  # 公共库（请求封装/品牌/toast/确认框/格式化）
│   ├── admin.js                # Admin 外壳（侧边栏/顶栏/Cookie 会话）
│   ├── admin-login.js          # Admin 登录
│   ├── admin-dashboard.js      # Admin 概览
│   ├── admin-containers.js     # Admin 容器管理
│   ├── admin-containers-modals.js
│   ├── admin-users.js          # Admin 用户管理
│   ├── admin-users-modals.js
│   ├── admin-generic.js        # Admin 通用列表页渲染
│   ├── admin-templates.js      # 模板管理
│   ├── admin-storage_pools.js  # 存储池
│   ├── admin-ip_pool_v4.js / admin-ip_pool_v6.js
│   ├── admin-port_mapping_v4.js / admin-port_mapping_v6.js
│   ├── admin-nginx.js          # 反向代理
│   ├── admin-tasks.js          # 任务管理
│   ├── admin-firewall.js       # 防火墙
│   ├── admin-brand-settings.js # 品牌设置
│   ├── user.js                 # User 外壳（顶部导航/Cookie 会话）
│   ├── user-login.js
│   ├── user-index.js           # 概览
│   ├── user-containers.js
│   ├── user-port-mapping.js
│   ├── user-nginx.js
│   ├── user-templates.js
│   └── user-tasks.js
├── admin/
│   ├── login.html
│   ├── dashboard.html
│   ├── containers.html
│   ├── users.html
│   ├── templates.html
│   ├── storage_pools.html
│   ├── ip_pool_v4.html
│   ├── ip_pool_v6.html
│   ├── port_mapping_v4.html
│   ├── port_mapping_v6.html
│   ├── nginx.html
│   ├── tasks.html
│   ├── firewall.html
│   └── brand_settings.html
└── user/
    ├── login.html
    ├── index.html
    ├── containers.html
    ├── port_mapping.html
    ├── nginx.html
    ├── templates.html
    └── tasks.html
```

---

## 快速开始

### 方式一：一键部署（推荐）

项目根目录提供 `deploy.sh`，一条命令完成前端静态托管 + `/api/`、`/ws/` 反向代理：

```bash
sudo bash deploy.sh                       # 默认：任意域名 + 后端 8080 + 当前目录
sudo bash deploy.sh panel.example.com     # 指定域名
sudo bash deploy.sh panel.example.com 8080 /srv/lxdpanel
```

脚本行为：生成 `/etc/nginx/conf.d/lxdpanel.conf` → `nginx -t` 校验 → `systemctl reload nginx` → 打印访问地址与 HTTPS（certbot）建议。要求 nginx + systemd 环境。

### 方式二：手动 Nginx

将 `lxdpanel/` 作为静态根目录部署，并配置反向代理：

```nginx
server {
    listen 80;
    server_name panel.example.com;
    root /path/to/lxdpanel;
    index index.html;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
    }
    location /ws/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
    # 三端静态目录优先（避免与后端 API/模板路由冲突）
    location ^~ /container {
        index login.html;
        try_files $uri $uri/ /container/login.html;
    }
    location ^~ /admin {
        index login.html;
        try_files $uri $uri/ /admin/login.html;
    }
    location ^~ /user {
        index login.html;
        try_files $uri $uri/ /user/login.html;
    }

    # SPA 路由回退
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

### 方式三：替换 Go 模板（同目录）

后端入口 `lxdapi/cmd/lxdapi/main.go` 使用 `//go:embed templates docs static` 嵌入模板。将 `lxdpanel/` 下文件放入模板目录（如 `lxdweb/templates/`）与静态目录后重新编译，保持文件名与路由一致即可。

---

## 三端功能说明

### Container 容器面板

| 页面 | 功能 |
|---|---|
| `container/login.html` | 访问码（X-Container-Hash）登录 + 图形验证码，支持 `?hash=` 预填 |
| `container/dashboard.html` | 资源监控（CPU/内存/磁盘/流量）、IP 分配与释放（v4/v6）、端口映射、反向代理、DNS、系统模板、重装、重置密码 |
| `container/console.html` | WebSocket 终端（xterm.js），令牌单次使用 |

### Admin 管理后台

| 页面 | 功能 |
|---|---|
| `login.html` | 管理员账号密码 + 验证码登录 |
| `dashboard.html` | 容器/用户统计、主机资源 |
| `containers.html` | 容器列表、创建、编辑配置、启停/重启/暂停/恢复、删除 |
| `users.html` | 用户列表、创建、编辑、重新生成 Key、批量删除 |
| `templates.html` | 系统模板管理 |
| `storage_pools.html` | 存储池管理 |
| `ip_pool_v4.html` / `ip_pool_v6.html` | IP 地址池 |
| `port_mapping_v4.html` / `port_mapping_v6.html` | 端口映射管理 |
| `nginx.html` | 反向代理管理 |
| `tasks.html` | 后台任务管理 |
| `firewall.html` | 防火墙规则 |
| `brand_settings.html` | 品牌设置（站点名/Logo/页脚/标题） |

### User 用户中心

| 页面 | 功能 |
|---|---|
| `login.html` | 用户账号密码 + 验证码登录 |
| `index.html` | 统计卡 + 账户信息 + 快捷入口 |
| `containers.html` | 我的容器列表 |
| `port_mapping.html` | 端口映射 |
| `nginx.html` | 反向代理 |
| `templates.html` | 可用模板 |
| `tasks.html` | 我的任务 |

---

## 鉴权机制

| 端 | 方式 | 说明 |
|---|---|---|
| Container | Header `X-Container-Hash` | 登录后写入 `localStorage.container_hash`，所有容器接口携带该头 |
| Admin | Cookie 会话 | `request()` 对 401/403 自动跳转 `admin/login.html` |
| User | Cookie 会话 | `request()` 对 401/403 自动跳转 `user/login.html` |

登录流程：

- Container：`GET /api/container/captcha`（返回 `captcha_id/code/image`）→ `POST /api/container/verify`，body `{ hash, captcha }`
- Admin：`GET /api/admin/captcha` → `POST /api/admin/login`，body `{ username, password, captcha }`，成功返回 `res.data.redirect`
- User：`GET /api/user/captcha` → `POST /api/user/login`

品牌信息：三端登录页与外壳自动调用 `LXD.loadBrand/applyBrand` 应用 `GET /api/public/brand` 返回的 `site_name / logo_url / favicon_url / footer_text / page_title`。

---

## API 对接要点

### Container 核心 API

| 功能 | 方法 | 路径 |
|---|---|---|
| 品牌信息 | GET | `/api/public/brand` |
| 容器信息 | GET | `/api/container/info` |
| 容器操作 | POST | `/api/container/action?action=start\|stop\|restart\|reinstall\|reset-password` |
| 系统模板 | GET | `/api/container/templates` |
| IP 列表 | GET | `/api/container/ip?version=v4\|v6` |
| IP 分配 / 释放 | POST | `/api/container/ip/allocate`、`/api/container/ip/release` |
| 端口映射列表 | GET | `/api/container/port-mapping?version=v4\|v6` |
| 端口映射分配 / 释放 | POST | `/api/container/port-mapping/allocate`、`/api/container/port-mapping/release?version=v4\|v6` |
| 反向代理列表 | GET / POST | `/api/container/nginx/proxies` |
| 删除反代 | DELETE | `/api/container/nginx/proxies/{id}` |
| DNS 配置 | GET / PUT | `/api/container/dns` |
| 控制台令牌 | POST | `/api/container/console/create-token` → `data.token` |

### Admin API 一览

| 模块 | 端点 |
|---|---|
| 概览/主机 | `/api/admin/dashboard`、`/api/admin/host/stats`、`/api/admin/cache/containers`、`/api/admin/cache/refresh?name=` |
| 容器 | `POST /api/admin/containers/create`、`POST /api/admin/containers/{name}/action?action=stop\|restart\|pause\|resume`、`POST /api/admin/containers/{name}/config`、`DELETE /api/admin/containers/{name}` |
| 用户 | `GET/POST /api/admin/users`、`POST /api/admin/users/{id}`、`POST /api/admin/users/{id}/regenerate-key`、`POST /api/admin/users/batch-delete`、`DELETE /api/admin/users/{id}` |
| 品牌 | `GET/POST /api/admin/brand-settings` |
| 其它 | `/api/admin/templates`、`/api/admin/tasks`、`/api/admin/firewall`、`/api/admin/ip-pool`、`/api/admin/port-mapping`、`/api/admin/nginx`、`/api/admin/storage-pools`、`/api/admin/cache`、`/api/admin/console`、`/api/admin/logout` |

### User API 一览

| 端点 | 说明 |
|---|---|
| `/api/user/dashboard` | 用户概览（代码内兼容 `data.containers/total/running/stopped` 与 `data.quota` 大小写混合） |
| `/api/user/info` | 账户信息 |
| `/api/user/containers` | 我的容器 |
| `/api/user/port-mapping` | 端口映射 |
| `/api/user/nginx` | 反向代理 |
| `/api/user/templates` | 可用模板 |
| `/api/user/tasks` | 我的任务 |
| `/api/user/cache` | 缓存 |
| `/api/user/logout` | 退出 |

---

## 控制台（WebSocket）协议

- 打开方式：仪表盘「控制台」按钮 → `POST /api/container/console/create-token` 获取一次性 token → `window.open('console.html?token=...')`
- WebSocket 地址：`ws(s)://<host>/ws/console?token=<token>`
- 数据流：服务端消息 `onmessage` 直接 `term.write`；键盘输入 `term.onData` 发送 `JSON {type:'input', data}`
- 依赖：xterm@5.3.0 + xterm-addon-fit@0.8.0（jsDelivr CDN）
- 注意：token 单次使用、不可重连；断开后需关闭窗口重新打开

---

## 脚本与页面对照表

| 层级 | 公共库 | 页面脚本 |
|---|---|---|
| Container | `js/app.js` | `container/js/login.js`、`container/js/dashboard.js`、`container/js/console.js` |
| Admin | `js/admin.js`（shell/request/MENUS）、`js/admin-generic.js`（GENERIC.page 通用列表） | `admin-login.js`、`admin-dashboard.js`、`admin-containers.js`、`admin-users.js`、`admin-users-modals.js`、`admin-templates.js`、`admin-storage_pools.js`、`admin-ip_pool_v4/v6.js`、`admin-port_mapping_v4/v6.js`、`admin-nginx.js`、`admin-tasks.js`、`admin-firewall.js`、`admin-brand-settings.js` |
| User | `js/user.js`（shell/request/NAVS + USER_GENERIC.page） | `user-login.js`、`user-index.js`、`user-containers.js`、`user-port-mapping.js`、`user-nginx.js`、`user-templates.js`、`user-tasks.js` |

通用列表页：`GENERIC.page`（admin）与 `USER_GENERIC.page`（user）支持 `columns` 配置，内置状态徽章（running/stopped/other）、时间、字节、PROGRESS 进度条渲染，字段兼容后端大小写混合（如 `name/Name`、`created_at/CreatedAt`）。

---

## 注意事项

1. **字段大小写**：后端接口存在大小写混合字段（如 `ipv4_pool_limit` / `IPv4PoolLimit`、`Username/Status/Remark/CreatedAt`、`CPUQuota/MemoryQuota/DiskQuota`），前端已做兼容处理。
2. **任务轮询**：重装 / 重置密码等长任务依赖轮询接口，容器端路由需按实际后端确认（原模板使用 `/api/admin/tasks/detail?task_id=`，Container 端未必同路由）。
3. **CDN 依赖**：控制台页依赖 jsDelivr 的 xterm@5.3.0 与 xterm-addon-fit@0.8.0，离线环境请自行本地化。
4. **登录入口**：首次访问建议指向对应端 `login.html`；Container 支持 `?hash=<访问码>` 自动预填并存储。
5. **会话跳转**：admin/user 的 `request()` 对 401/403 自动跳转本端 `login.html`。
6. **三端目录与后端路由冲突**：`container/`、`admin/`、`user/` 目录名与后端 API（`/api/container|admin|user`）及 Go 模板路由（`/admin`、`/user`、`/container`）存在同名路径。部署请使用 Nginx 静态托管（方式一/二）；v1.1 起 `deploy.sh` 已内置三端静态优先规则（`location ^~`）。切勿将前端直接放入后端模板目录覆盖，否则会被后端模板/API 路由拦截。

---

*（内容由AI生成，仅供参考）*
