#!/bin/bash
# 安装/修复 LXD Cloud 插件运行环境：Nginx 反向代理 + OpenGFW 防火墙
# 用法：sudo bash scripts/install-plugins.sh [面板目录，默认 /opt/lxdapi 或 /opt/lxdcloud]
# 可选环境变量：
#   OPENGFW_URL  OpenGFW 二进制下载地址（默认尝试 GitHub Release，失败则用本机 go 编译 v0.4.1）
#   OPENGFW_VER  go 编译的版本（默认 v0.4.1）
set -uo pipefail
SRC="$(cd "$(dirname "$0")/.." && pwd)"
APP_DIR="${1:-}"
if [[ -z "$APP_DIR" ]]; then
  for d in /opt/lxdapi /opt/lxdcloud; do [[ -d "$d" ]] && APP_DIR="$d" && break; done
fi
[[ -n "$APP_DIR" && -d "$APP_DIR" ]] || { echo "[x] 未找到面板目录，请作为参数传入"; exit 1; }
ARCH="$(uname -m)"; case "$ARCH" in aarch64|arm64) GA=arm64;; *) GA=amd64;; esac
BIN_NAME="OpenGFW-linux-${GA}"
echo "[i] 面板目录: $APP_DIR"

# ---- Nginx 反向代理 ----
mkdir -p "$APP_DIR/plugins/nginx/conf/sites" "$APP_DIR/plugins/nginx/ssl"
if ! command -v nginx >/dev/null; then
  echo "[i] 安装 nginx..."; (apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nginx) >/dev/null 2>&1 || yum install -y nginx >/dev/null 2>&1
fi
if [[ -d /etc/nginx/conf.d ]]; then
  echo "include ${APP_DIR}/plugins/nginx/conf/sites/*.conf;" > /etc/nginx/conf.d/lxdcloud-proxies.conf
  if nginx -t >/dev/null 2>&1; then systemctl reload nginx 2>/dev/null || nginx -s reload 2>/dev/null; echo "[ok] Nginx 反代目录已就绪并接入 /etc/nginx/conf.d/lxdcloud-proxies.conf"
  else echo "[!] nginx -t 未通过，请检查 nginx 配置"; fi
fi

# ---- OpenGFW 防火墙 ----
G="$APP_DIR/plugins/opengfw"; mkdir -p "$G/bin" "$G/data"
for f in geoip.dat geosite.dat; do
  [[ -f "$G/data/$f" ]] || cp "$SRC/backend/plugins/opengfw/data/$f" "$G/data/" 2>/dev/null || true
done
if [[ ! -x "$G/bin/$BIN_NAME" ]]; then
  URL="${OPENGFW_URL:-https://github.com/apernet/OpenGFW/releases/latest/download/${BIN_NAME}}"
  echo "[i] 下载 OpenGFW: $URL"
  if curl -fsSL -m 120 -o "$G/bin/$BIN_NAME" "$URL"; then chmod +x "$G/bin/$BIN_NAME"
  else
    rm -f "$G/bin/$BIN_NAME"
    GO="$(command -v go || ls /usr/local/go/bin/go 2>/dev/null)"
    if [[ -n "$GO" ]]; then
      echo "[i] 下载失败，用 go 编译 OpenGFW ${OPENGFW_VER:-v0.4.1}..."
      T="$(mktemp -d)"; GOBIN="$T" CGO_ENABLED=0 GOPROXY="${GOPROXY:-https://goproxy.cn,https://proxy.golang.org,direct}" \
        "$GO" install "github.com/apernet/OpenGFW@${OPENGFW_VER:-v0.4.1}" && mv "$T/OpenGFW" "$G/bin/$BIN_NAME"; rm -rf "$T"
    fi
  fi
fi
if [[ -x "$G/bin/$BIN_NAME" ]]; then echo "[ok] OpenGFW: $G/bin/$BIN_NAME"
else echo "[!] OpenGFW 未安装：请设置 OPENGFW_URL 指向可下载的二进制，或安装 Go 后重试"; fi
modprobe nfnetlink_queue 2>/dev/null && echo "nfnetlink_queue" > /etc/modules-load.d/lxdcloud-nfqueue.conf 2>/dev/null
[[ -e /sys/module/nfnetlink_queue ]] && echo "[ok] 内核 NFQUEUE 可用" || echo "[!] 内核不支持 NFQUEUE，防火墙无法运行"
echo "[i] 完成后重启面板服务（systemctl restart lxdapi 或 lxdcloud）使插件生效"
