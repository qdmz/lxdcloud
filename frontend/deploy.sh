#!/usr/bin/env bash
# ============================================================
# LXD Panel - 一键部署脚本（Nginx 反向代理）
#
# 用法：
#   sudo bash deploy.sh                            # 默认：任意域名(_) + 后端 8080 + 本目录为前端根
#   sudo bash deploy.sh panel.example.com          # 指定域名
#   sudo bash deploy.sh panel.example.com 8080     # 域名 + 后端端口
#   sudo bash deploy.sh panel.example.com 8080 /srv/lxdpanel   # 全参数
#
# 功能：
#   1. 生成 Nginx 站点配置（前端静态 + /api/ 反代 + /ws/ WebSocket 升级）
#   2. 校验配置并重载 Nginx
#   3. 打印访问地址与后续 HTTPS 建议
# ============================================================
set -euo pipefail

DOMAIN="${1:-_}"
BACKEND_PORT="${2:-8080}"
FRONT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CONF="/etc/nginx/conf.d/lxdpanel.conf"
NGINX_BIN="$(command -v nginx || true)"

echo "==> LXD Panel 部署参数"
echo "    域名      : ${DOMAIN}"
echo "    后端端口  : ${BACKEND_PORT}"
echo "    前端目录  : ${FRONT_DIR}"
echo "    Nginx配置 : ${CONF}"

if [[ -z "${NGINX_BIN}" ]]; then
  echo "!! 未检测到 nginx，请先安装："
  echo "   Debian/Ubuntu: apt-get install -y nginx"
  echo "   CentOS/RHEL  : yum install -y nginx"
  exit 1
fi

if [[ ! -f "${FRONT_DIR}/index.html" ]]; then
  echo "!! 前端目录中没有 index.html，请确认 FRONT_DIR 指向 lxdpanel 解压目录"
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "!! 未检测到 systemd（systemctl），本脚本仅支持 systemd 环境"
  exit 1
fi

# 写入 Nginx 配置
cat > "${CONF}" <<EOF
# LXD Panel - 自动生成于 $(date '+%Y-%m-%d %H:%M:%S')
server {
    listen 80;
    server_name ${DOMAIN};

    root ${FRONT_DIR};
    index index.html;

    # 静态资源压缩
    gzip on;
    gzip_min_length 1k;
    gzip_types text/css application/javascript application/json image/svg+xml;

    # REST API 反向代理
    location /api/ {
        proxy_pass http://127.0.0.1:${BACKEND_PORT};
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 30s;
        proxy_read_timeout 60s;
    }

    # WebSocket 反向代理（容器控制台）
    location /ws/ {
        proxy_pass http://127.0.0.1:${BACKEND_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_read_timeout 3600s;
    }

    # 三端静态目录优先（避免与后端 API / 模板路由冲突，Nginx ^~ 优先级高于普通前缀）
    location ^~ /container {
        index login.html;
        try_files \$uri \$uri/ /container/login.html;
    }
    location ^~ /admin {
        index login.html;
        try_files \$uri \$uri/ /admin/login.html;
    }
    location ^~ /user {
        index login.html;
        try_files \$uri \$uri/ /user/login.html;
    }

    # 前端静态文件（未命中回退登录页）
    location / {
        try_files \$uri \$uri/ /index.html;
    }
}
EOF

echo "==> 校验 Nginx 配置"
if ! nginx -t; then
  echo "!! nginx -t 校验失败，请检查 ${CONF} 与 nginx 语法"
  exit 1
fi

echo "==> 重载 Nginx"
systemctl reload nginx

echo ""
echo "==> 部署完成"
if [[ "${DOMAIN}" == "_" ]]; then
  echo "    访问地址：http://<服务器IP>/"
else
  echo "    访问地址：http://${DOMAIN}/"
fi
echo ""
echo "==> 后续建议"
echo "  1. HTTPS：安装 certbot 后执行"
echo "     certbot --nginx -d ${DOMAIN}"
echo "  2. 确认后端服务监听 127.0.0.1:${BACKEND_PORT}"
echo "  3. 容器控制台依赖 WebSocket，需保证 /ws/ 反向代理配置存在"
