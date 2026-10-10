# LXD Cloud

![version](https://img.shields.io/badge/version-1.1.5-38bdf8) ![go](https://img.shields.io/badge/Go-1.23%2B-00ADD8) ![license](https://img.shields.io/badge/UI-responsive%20%7C%20dark%2Flight-6366f1)

基于 **LXD** 的云主机管理面板（商业化版），由 Go 后端（lxdapi）与原生静态前端（lxdpanel）组成，提供 **管理后台 / 用户中心 / 容器面板** 三端能力，支持容器全生命周期管理、IP/端口映射、NAT 配置、反向代理、在线文件管理、商品订单与支付等模块。

- 当前版本：见 [`VERSION`](VERSION)，更新记录见 [`CHANGELOG.md`](CHANGELOG.md)
- 在线演示（无 LXD 的演示模式，容器操作会提示"当前节点未安装LXD"）：<https://lxd.freehost.ypvps.com>
  - 管理后台 `/admin/login.html` · 用户中心 `/user/login.html`（可自行注册） · 容器面板 `/container/login.html`

## 界面特性（v1.1）

- 响应式卡片布局：桌面 / 平板 / 手机自适应；管理后台手机端抽屉菜单、桌面端侧栏可收起；用户中心手机端折叠导航
- 深色 / 浅色主题一键切换，选择记忆在浏览器本地，未选择时跟随系统
- 统计卡片、表格窄屏横向滚动、Toast 提示、危险操作确认框、提交防重复点击、IP / 访问码一键复制
- 三端共享一套设计系统：`frontend/css/style.css`（主题变量）+ `frontend/js/theme.js`（主题初始化）+ `frontend/js/app.js`（通用增强）

## 项目概览

| 组件 | 目录 | 技术栈 | 说明 |
|---|---|---|---|
| 后端 API | `backend/` | Go + Gin + LXD + SQLite/MySQL/PostgreSQL | 编译产物为单文件二进制 `lxdapi` |
| 前端面板 | `frontend/` | 原生 HTML/CSS/JS + xterm.js | 纯静态，Nginx 或 Go 代理托管 |
| 配置示例 | `configs/` | YAML | 后端配置模板（脱敏） |
| Docker | `Dockerfile` + `docker-compose.yml` | 多阶段构建 | 一键容器化部署 |
| 文档 | `docs/` | Markdown | 部署文档、使用说明 |

## 目录结构

```
lxdcloud/
├── backend/                  # lxdapi Go 后端源码
│   ├── cmd/lxdapi/           # 主程序（main.go、内嵌模板/静态资源）
│   ├── internal/api/         # 各端 API 实现
│   │   ├── admin/            # 管理后台接口（含在线文件管理）
│   │   ├── container/        # 容器面板接口（含在线文件管理）
│   │   ├── user/             # 用户中心接口（含在线文件管理）
│   │   ├── console/          # Web 控制台（WebSocket）
│   │   ├── public/           # 公开接口（注册/登录/商品）
│   │   └── system/           # 系统级接口（节点/模板/网络）
│   ├── internal/service/     # 业务逻辑
│   ├── models/ pkg/ plugins/ # 数据模型 / 工具库 / 插件（nginx 反代、防火墙等）
│   └── build.sh              # 编译打包脚本（amd64/arm64，v1.1.0 已修复）
├── frontend/                 # lxdpanel 静态前端
│   ├── admin/                # 管理后台（容器/用户/订单/模板/IP池/端口映射/Nginx/防火墙…）
│   ├── user/                 # 用户中心（我的容器/订单/工单/个人资料…）
│   ├── container/            # 容器面板（资源监控/文件管理/Web终端…）
│   ├── js/ css/              # 公共 JS 与设计系统
│   └── deploy.sh             # 前端一键部署脚本（生成 Nginx 站点配置）
├── configs/config.yaml       # 后端配置示例（部署时按需修改）
├── scripts/release.sh        # 版本发布：递增版本号 → 更新 CHANGELOG → 提交 → 打 tag → 推送
├── VERSION / CHANGELOG.md    # 版本号与更新日志
├── Dockerfile                # 后端多阶段构建
├── docker-compose.yml        # 一键 Docker 部署（含前端 Nginx）
└── docs/
    ├── 部署文档.md           # 二进制部署 + Docker 部署 + 升级流程
    └── 使用说明.md           # 三端使用指南
```

## 快速开始

### 方式一：Docker（一键）

```bash
# 1. 准备配置（首次）
cp configs/config.yaml configs/config.prod.yaml
# 编辑 config.prod.yaml：修改 admin.password（bcrypt 哈希）、api_hash、session_secret

# 2. 启动
docker compose up -d --build

# 3. 访问
# http://<服务器IP>/admin/login.html
```

> Docker 部署要求宿主机已安装 LXD（挂载 unix socket）。详见 `docs/部署文档.md` 第四章。

### 方式二：二进制 + Nginx（传统）

```bash
# 1. 编译后端（纯 Go，无需 CGO，可交叉编译）
cd backend && ARCHS=amd64 bash build.sh      # 产物：backend/release/lxdapi-v<版本>-linux-amd64.tar.gz
# 或：CGO_ENABLED=0 go build -o lxdapi ./cmd/lxdapi

# 2. 部署
mkdir -p /opt/lxdcloud && cp lxdapi-amd64 /opt/lxdcloud/lxdapi
cp -r ../configs /opt/lxdcloud/
cp -r ../frontend /opt/lxdcloud/

# 3. 前端 Nginx（或用 Go 代理，见部署文档）
cd /opt/lxdcloud/frontend && sudo bash deploy.sh your-domain.com
```

## 三端功能

| 端 | 入口 | 面向 | 主要能力 |
|---|---|---|---|
| Admin | `admin/login.html` | 运维/管理员 | 容器管理、用户管理、商品/订单、模板、存储池、IP 池、端口映射（含 NAT 配置/端口范围/网络 NAT）、Nginx 站点、任务、防火墙、品牌设置、主机资源、在线文件管理 |
| User | `user/login.html` | 平台用户 | 概览统计、我的容器（含强制同步）、端口映射、反向代理、模板、任务、在线文件管理 |
| Container | `container/login.html` | 容器用户 | 资源监控、在线文件管理、Web 终端、IP 管理、端口映射、反向代理、DNS、重装、重置密码 |

## 安全特性

- **管理员密码 bcrypt 哈希存储**：`config.yaml` 中存放哈希而非明文（兼容旧明文配置，启动时告警）；占位密码（`CHANGE_ME` 等）拒绝登录
- **Session Cookie**：`HttpOnly` + `SameSite=Lax`，`Secure` 按请求是否 HTTPS 自动设置（可配置）；未配置会话密钥时随机生成
- **系统级 API**：`api_hash` 未配置时整体禁用，常量时间比较
- **越权防护**：Web 终端令牌校验容器归属（用户 / 容器访问码只能连自己的容器）
- **支付回调**：验签 + 商户号 + 金额校验，订单状态原子更新，防重复开通
- **公开接口不泄露 TLS 私钥**；文件管理路径转义 + 控制字符过滤；DNS 仅接受合法 IP；用户名 / 邮箱 / 容器名格式校验
- **登录限流**只信任本机 / 内网反代的 `X-Forwarded-For`，防伪造 IP 绕过
- **无 LXD 友好降级**：未安装 LXD 的节点上，容器操作返回中文友好提示而非底层 exec 错误

## 鉴权机制

| 端 | 鉴权方式 | 说明 |
|---|---|---|
| Admin | Cookie Session（`lxdapi_session`） | 登录 `POST /api/admin/login`（username/password/captcha，可在 config 关闭验证码） |
| User | 用户名 + 密码 / API Key | 登录 `POST /api/user/login`；自助注册用户用注册密码，管理员创建的老用户密码字段填 API Key |
| Container | 请求头 `X-Container-Hash` | 容器访问码 Hash（详情页可查看/复制/快捷连接） |

## 在线文件管理

三端均提供在线文件管理，支持：**目录浏览 / 面包屑导航 / 新建目录 / 上传 / 下载 / 重命名 / 删除**。

安全限制：路径必须为绝对路径且禁止 `..` 跳转（后端强制校验）。

## 环境要求

- Linux 宿主机（Debian/Ubuntu 推荐）
- LXD 已安装并运行（`snap install lxd && lxd init --auto`）
- Go 1.23+（仅编译需要，纯 Go 无需 CGO）、Nginx（前端托管）或 Docker

## 版本与发布

```bash
bash scripts/release.sh            # patch：1.1.0 → 1.1.1
bash scripts/release.sh minor      # 1.1.x → 1.2.0
bash scripts/release.sh major      # 1.x.y → 2.0.0
```

## 版本历史

- **v1.1.0** — 前端响应式重构 + 深浅色主题 + 通用交互增强；修复 TLS 私钥泄露、终端越权、系统 API 空密钥绕过、支付回调金额校验等安全问题；修复 build.sh；新增 CHANGELOG / VERSION / 发布脚本。详见 [CHANGELOG.md](CHANGELOG.md)
- **v1.0** — 安全加固（bcrypt/Secure Cookie/友好报错）+ 前端补齐（NAT 配置/端口范围/网络 NAT/强制同步）
