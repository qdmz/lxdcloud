# LXD Cloud

基于 **LXD** 的云主机管理面板（商业化版），由 Go 后端（lxdapi）与原生静态前端（lxdpanel）组成，提供 **管理后台 / 用户中心 / 容器面板** 三端能力，支持容器全生命周期管理、IP/端口映射、反向代理、在线文件管理、商品订单与支付等模块。

## 项目概览

| 组件 | 目录 | 技术栈 | 说明 |
|---|---|---|---|
| 后端 API | `backend/` | Go + Gin + LXD REST + SQLite/MySQL/PostgreSQL | 编译产物为单文件二进制 `lxdapi-amd64` |
| 前端面板 | `frontend/` | 原生 HTML/CSS/JS + xterm.js + ECharts | 纯静态，Nginx 托管，三端共享设计系统 |
| 配置示例 | `configs/` | YAML | 后端配置模板（脱敏） |
| 文档 | `docs/` | Markdown | 部署文档、使用说明 |

## 目录结构

```
lxdcloud/
├── backend/                  # lxdapi Go 后端源码
│   ├── cmd/lxdapi/           # 主程序（main.go、内嵌模板/静态资源）
│   ├── internal/api/         # 各端 API 实现
│   │   ├── admin/            # 管理后台接口（含 files.go 在线文件管理）
│   │   ├── container/        # 容器面板接口（含 files.go）
│   │   ├── user/             # 用户中心接口（含 files.go）
│   │   ├── console/          # Web 控制台（WebSocket）
│   │   ├── public/           # 公开接口（注册/登录/商品）
│   │   └── system/           # 系统级接口（节点/模板/网络）
│   ├── handlers/ models/ pkg/ plugins/   # 业务逻辑 / 数据模型 / 工具库 / 插件（nginx 反代、防火墙等）
│   └── build.sh              # 一键编译脚本（含 Swagger 生成）
├── frontend/                 # lxdpanel 静态前端
│   ├── admin/                # 管理后台（容器/用户/订单/模板/IP池/端口映射/Nginx/防火墙…）
│   ├── user/                 # 用户中心（我的容器/订单/工单/个人资料…）
│   ├── container/            # 容器面板（资源监控/文件管理/Web终端…）
│   ├── js/ css/              # 公共 JS 与设计系统
│   └── deploy.sh             # 前端一键部署脚本（生成 Nginx 站点配置）
├── configs/config.yaml       # 后端配置示例（部署时按需修改）
└── docs/
    ├── 部署文档.md           # 后端编译部署 + 前端托管 + 升级流程
    └── 使用说明.md           # 三端使用指南（含在线文件管理）
```

## 快速开始

```bash
# 1. 编译后端（详见 docs/部署文档.md）
cd backend && bash build.sh

# 2. 配置 config.yaml 并启动后端
mkdir -p /opt/lxdapi && cp -r configs /opt/lxdapi/
cp cmd/lxdapi/lxdapi-amd64 /opt/lxdapi/
/opt/lxdapi/lxdapi-amd64

# 3. 部署前端
cd frontend && sudo bash deploy.sh your-domain.com 9443 /srv/lxdpanel
```

## 三端功能

| 端 | 入口 | 面向 | 主要能力 |
|---|---|---|---|
| Admin | `admin/login.html` | 运维/管理员 | 容器管理、用户管理、商品/订单、模板、存储池、IP 池、端口映射、Nginx 站点、任务、防火墙、品牌设置、主机资源、**在线文件管理** |
| User | `user/login.html` | 平台用户 | 概览统计、我的容器、端口映射、反向代理、模板、任务、**在线文件管理**、SSH/SFTP 连接信息 |
| Container | `container/login.html` | 容器用户 | 资源监控、**在线文件管理**、Web 终端、IP 管理、端口映射、反向代理、DNS、重装、重置密码 |

## 鉴权机制

| 端 | 鉴权方式 | 说明 |
|---|---|---|
| Admin | Cookie Session（`lxdapi_session`） | 登录 `POST /api/admin/login`（username/password/captcha，可在 config 关闭验证码） |
| User | Basic 头（`X-User-Username` + `X-User-Password`） | 用户名 + API Key 作为密码 |
| Container | 请求头 `X-Container-Hash` | 容器访问码 Hash（详情页可查看/复制/快捷连接） |

## 在线文件管理（类 WebSFTP）

三端均提供在线文件管理，支持：**目录浏览 / 面包屑导航 / 新建目录 / 上传 / 下载 / 重命名 / 删除**。

实现采用双通道：
- **列目录 / 删除 / 重命名**：通过 `lxc exec` 执行 `ls -la --full-time` / `rm -rf` / `mv`
- **上传 / 下载 / 建目录**：通过 LXD REST files 接口（unix socket，建目录带 `X-LXD-type: directory` 头）

安全限制：路径必须为绝对路径且禁止 `..` 跳转；上传/下载/建目录走 REST，删除/重命名走 exec（LXD REST 不支持 PATCH 重命名与目录删除）。

## 环境要求

- Linux 宿主机（Debian/Ubuntu/CentOS/TencentOS 等）
- LXD 已安装并运行（`snap install lxd` 或系统包）
- Go 1.20+（仅编译需要）、Nginx（前端托管）、SQLite/MySQL/PostgreSQL
