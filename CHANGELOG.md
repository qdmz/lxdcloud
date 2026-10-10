# 更新日志

## v1.1.4 - 2026-10-10

- fix: 管理后台「容器管理 → 新建容器」系统镜像下拉为空：前端把 `/api/admin/templates` 返回的对象当数组解析且读取不存在的 `name` 字段。新增 `GET /api/admin/image-options`，返回本地镜像（模板表为空时自动从 LXD 同步一次；无别名时使用指纹）及常用远程镜像（`ubuntu:24.04`、`images:debian/12` 等，首次使用自动下载）
- fix: 0 元商品下单后不开通：原逻辑直接拒绝价格为 0 的订单。现在 0 元订单创建后立即标记为已支付，并与在线支付回调走同一开通流程；待支付的 0 元订单点「去支付」也直接完成
- fix: 自动开通的容器未归属下单用户（用户面板看不到）；商品未配置镜像时 `lxc init` 镜像为空导致开通失败，改为使用默认镜像；商品硬盘（GB）按 MB 传给容器创建导致磁盘过小
- fix: 自动开通实例的 root 密码由 `pwd-` + 6 位十六进制改为 16 位随机字母数字
- fix: 半年付（`half_year`）订单到期时间计算为 0，开通后立即被暂停
- fix: 宿主机内核缺少 tc 分类器（如部分云厂商/Freestyle 内核）时，设置带宽限制的容器无法启动；现在自动移除带宽限制后重试启动并记录警告
- fix: 支付回调与免费订单的开通改为后台执行，避免创建容器耗时导致回调超时
- fix: 删除用户实例（用户端/管理端）时同步清理容器记录、端口映射、IP 绑定与访问凭证，不再在容器列表残留
- fix: 用户「我的订单 → 去支付」请求 `/orders/undefined/pay`（订单 ID 字段为 `ID`）
- feat: 商品管理新增「系统镜像」选择；编辑商品保存时保留表单未展示的字段（带宽、存储池等），不再被清零
- feat: 产品订购页 0 元周期显示「免费」，未定价（0 元且非免费商品）的周期不再展示

## v1.1.3 - 2026-10-09

- fix: 后台创建容器未指定用户名时不再自动生成空用户名账号
- fix: 管理后台「用户管理 → 编辑」提示接口不存在：新增 `GET/POST /api/admin/users/:id`，兼容前端字段（cpu/memory/disk、enabled 状态、重置密码）
- fix: 新建用户时填写的初始密码可直接用于用户中心登录
- fix: Nginx 反向代理插件内置默认站点模板，未部署 `nginx-default.tmpl` 时也能启动
- feat: 插件加载失败时，前端提示中显示具体原因（如内核不支持 NFQUEUE）；内核内置 nfnetlink_queue 时不再误判

## v1.1.2 - 2026-10-09

- fix: 反向代理/防火墙插件未加载时返回明确提示，不再显示「接口不存在」

## v1.1.1 - 2026-10-09

- fix: 列表类卡片补内边距

## v1.1.0 - 2026-10-09

### 新增
- 前端（`frontend/` 三端）整体升级为响应式卡片布局：桌面 / 平板 / 手机自适应。
- 深色 / 浅色主题一键切换（`js/theme.js`，在 `<head>` 中提前执行避免闪烁），选择记忆在 localStorage，未选择时跟随系统主题。
- 管理后台：手机端抽屉菜单（点遮罩 / 菜单项 / Esc 收起），桌面端侧边栏可收起并记住状态。
- 用户中心：手机端 ☰ 折叠导航（两列网格），顶栏正确显示"已登录：用户名"。
- 通用增强（`js/app.js`，三端自动生效）：表格自动包裹横向滚动容器、IP / 访问码等等宽值一键复制（`LXD.copy` + `[data-copy]`）、表单与主要按钮在请求完成前锁定防重复提交、确认框支持 Esc / Enter、登录页在后端关闭验证码时自动隐藏验证码输入。
- 取消订单、删除站内消息、关闭工单（用户端 / 管理端）补充二次确认。
- 后端：`GET /healthz`、`GET /api/public/version`；`/api/public/site-info` 增加 `captcha_enabled`；`/api/public/brand` 别名路由（静态前端一直请求的地址）；用户登录记录最近登录时间。
- 配置项：`system.server.public_url`、`secure_cookie`、`trusted_proxies`。
- `VERSION`、`CHANGELOG.md`、`scripts/release.sh`（patch / minor / major 自动递增、同步前端缓存版本号与后端版本号、打 tag 并推送）。

### 修复
- `backend/build.sh` 会从不存在的 `../lxdweb` 复制并在编译后删除嵌入的模板 / 静态资源目录，导致仓库被破坏；重写为直接嵌入 `cmd/lxdapi/{templates,static,docs}`，支持 `ARCHS`，注入版本号，`CGO_ENABLED=0` 交叉编译。
- 静态前端请求 `/api/public/brand`，后端只有 `/api/public/brand-settings`，404 被 `NoRoute` 重定向到 `/`，品牌信息从未生效；现两者均可用，且按管理端 / 用户端 / 容器端分别取对应的系统名称与标题。
- `/api/*` 未匹配的路由返回 JSON 404，不再 302 到首页。
- 激活邮件链接指向返回 JSON 的 API、重置密码链接指向只接受 POST 的 API（点开即被重定向到首页）；现均指向前端 `user/activate.html` 页面。
- 易支付同步跳转回 `/user/orders`（不存在）改为 `/user/orders.html`；异步回调失败按协议返回纯文本 `fail`。
- 管理端 / 用户端请求封装只识别 HTTP 401，而后端以 HTTP 200 + `code:401` 返回未登录，导致会话过期后页面停留在"请求失败"；现自动跳转登录页。
- 会话 Cookie：全局固定 `Secure=true` 与登录时固定 `Secure=false` 互相矛盾（HTTP 访问时验证码会话丢失）；改为按请求是否 HTTPS（含 `X-Forwarded-Proto`）自动设置，`SameSite=Lax`。
- 关闭验证码时登录接口仍强制要求 `captcha` 字段。
- 后台品牌信息 `logo_url` 误映射为后台背景图。
- 首页接口版本号硬编码 `v2.1.3`，改为真实版本。
- Dockerfile / 文档中"需要 CGO（mattn/go-sqlite3）"的错误说明；删除仓库中的 `.bak-fixbrand` 与带时间戳的备份 JS。

### 安全
- **公开接口 `/api/public/brand-settings` 返回 TLS 证书与私钥内容**（任何人可下载后台配置的 HTTPS 私钥）——已从公开响应中移除。
- **Web 终端越权**：`/api/user/console/create-token`、`/api/container/console/create-token` 接受任意容器名，普通用户 / 容器访问码可获取他人容器的 root Shell；现校验归属（管理员 / 系统级不受限）。
- **系统级 API 空密钥绕过**：`api_hash` 为空时不带请求头即可通过 `/api/system/*`；现空值 / 占位值时禁用该组接口，并改为常量时间比较；示例配置中的真实 `api_hash` / `session_secret` 替换为占位符。
- 管理员密码为空或占位值（`CHANGE_ME` 等）时拒绝登录；明文密码比较改为常量时间。
- `session_secret` 未配置 / 过短时启动随机生成，不再用空密钥签名 Cookie。
- `?token=` 任意值即可绕过管理端 / 用户端页面登录校验；现必须是有效且类型匹配的访问令牌。
- 易支付回调：校验商户号与回调金额（防止低价支付冒充高价订单）、签名常量时间比较、下单跳转参数 URL 编码；订单 pending→paid 改为条件更新，异步通知与同步跳转并发时不再重复开通 / 续费。
- 邮件令牌（激活 / 重置）原子消费，防止并发重复使用；邮件链接优先使用 `public_url`，防 Host 头注入。
- 文件管理：LXD 文件接口的容器名 / 路径做 URL 转义（防 `&`、`#` 注入查询参数），拒绝控制字符与超长路径，下载文件名安全处理（防响应头注入）。
- DNS 设置只接受合法 IP（原实现拼接进 `sh -c`，可命令注入）；容器名格式校验（会被拼进设置主机名的 shell 命令）。
- 注册：用户名 / 邮箱格式校验、密码长度上限；修改资料时校验邮箱格式。
- 登录限流只信任本机 / 内网反代的 `X-Forwarded-For`（原先任意客户端可伪造 IP 绕过锁定）。
- 登录成功重建会话数据；新增 `X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy` 响应头；Toast 消息统一转义防 XSS。

## v1.0 - 2026-10-08

- 安全加固（bcrypt / Secure Cookie / 无 LXD 友好报错）+ 前端补齐（NAT 配置 / 端口范围 / 网络 NAT / 强制同步）。
