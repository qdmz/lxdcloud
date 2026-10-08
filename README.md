# LXD Cloud

基于 **LXD** 的云主机管理面板（商业化版），由 Go 后端（lxdapi）与原生静态前端（lxdpanel）组成，提供 **管理后台 / 用户中心 / 容器面板** 三端能力，支持容器全生命周期管理、IP/端口映射、NAT 配置、反向代理、在线文件管理、商品订单与支付等模块。

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
│   └── build.sh              # 编译脚本（注意：有已知问题，见部署文档）
├── frontend/                 # lxdpanel 静态前端
│   ├── admin/                # 管理后台（容器/用户/订单/模板/IP池/端口映射/Nginx/防火墙…）
│   ├── user/                 # 用户中心（我的容器/订单/工单/个人资料…）
│   ├── container/            # 容器面板（资源监控/文件管理/Web终端…）
│   ├── js/ css/              # 公共 JS 与设计系统
│   └── deploy.sh             # 前端一键部署脚本（生成 Nginx 站点配置）
├── configs/config.yaml       # 后端配置示例（部署时按需修改）
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
# 1. 编译后端
cd backend/cmd/lxdapi && go build -o lxdapi .

# 2. 部署
mkdir -p /opt/lxdcloud && cp lxdapi /opt/lxdcloud/
cp -r ../../configs /opt/lxdcloud/
cp -r ../../frontend /opt/lxdcloud/

# 3. 前端 Nginx（或用 Go 代理，见部署文档）
cd /opt/lxdcloud/frontend && sudo bash deploy.sh your-domain.com
```

## 三端功能

| 端 | 入口 | 面向 | 主要能力 |
|---|---|---|---|
| Admin | `admin/login.html` | 运维/管理员 | 容器管理、用户管理、商品/订单、模板、存储池、IP 池、端口映射（含 NAT 配置/端口范围/网络 NAT）、Nginx 站点、任务、防火墙、品牌设置、主机资源、在线文件管理 |
| User | `user/login.html` | 平台用户 | 概览统计、我的容器（含强制同步）、端口映射、反向代理、模板、任务、在线文件管理 |
| Container | `container/login.html` | 容器用户 | 资源监控、在线文件管理、Web 终端、IP 管理、端口映射、反向代理、DNS、重装、重置密码 |

## 安全特性（v1.0+）

- **管理员密码 bcrypt 哈希存储**：`config.yaml` 中存放哈希而非明文（兼容旧明文配置，启动时告警）
- **Session Cookie `Secure` 标志**：仅 HTTPS 传输（配合 `HttpOnly` + `SameSite`）
- **无 LXD 友好降级**：未安装 LXD 的节点上，容器操作返回中文友好提示而非底层 exec 错误

## 鉴权机制

| 端 | 鉴权方式 | 说明 |
|---|---|---|
| Admin | Cookie Session（`lxdapi_session`） | 登录 `POST /api/admin/login`（username/password/captcha，可在 config 关闭验证码） |
| User | 用户名 + API Key | 登录 `POST /api/user/login`，密码字段填 API Key |
| Container | 请求头 `X-Container-Hash` | 容器访问码 Hash（详情页可查看/复制/快捷连接） |

## 在线文件管理

三端均提供在线文件管理，支持：**目录浏览 / 面包屑导航 / 新建目录 / 上传 / 下载 / 重命名 / 删除**。

安全限制：路径必须为绝对路径且禁止 `..` 跳转（后端强制校验）。

## 环境要求

- Linux 宿主机（Debian/Ubuntu 推荐）
- LXD 已安装并运行（`snap install lxd && lxd init --auto`）
- Go 1.21+（仅编译需要）、Nginx（前端托管）或 Docker

## 版本历史

- **v1.0** — 安全加固（bcrypt/Secure Cookie/友好报错）+ 前端补齐（NAT 配置/端口范围/网络 NAT/强制同步）
