#!/usr/bin/env bash
# ============================================================
# LXD Cloud - 全功能一键部署/维护脚本
#
# 支持：
#   --all            全部安装配置（后端 + 前端 + 免费证书）
#   --backend        只安装配置后端（lxdapi + systemd）
#   --frontend       只安装配置前端（静态文件 + Nginx 反代）
#   --reinstall      重新安装配置（自动备份现有配置与数据）
#   --cert           申请/续期免费证书（Let's Encrypt）
#   --backup         备份配置和数据
#   --restore FILE   从备份恢复配置和数据
#   --status         查看部署状态
#   --uninstall      卸载（保留数据备份，需 --force）
#
# 常用选项：
#   --domain DOMAIN       绑定域名（证书与 Nginx server_name）
#   --backend-port PORT   后端监听端口（默认 9443）
#   --app-dir DIR         后端运行目录（默认 /opt/lxdapi）
#   --front-dir DIR       前端安装目录（默认 /opt/lxdpanel-frontend）
#   --email EMAIL         证书注册邮箱
#   --no-cert             跳过证书申请
#   --yes / -y            跳过所有交互确认
#   --force               强制覆盖/卸载，不二次确认
#
# 示例：
#   sudo bash install.sh --all --domain panel.example.com --email admin@example.com
#   sudo bash install.sh --backend --backend-port 9443
#   sudo bash install.sh --frontend --domain panel.example.com
#   sudo bash install.sh --reinstall --all --domain panel.example.com
#   sudo bash install.sh --backup /root/backup
#   sudo bash install.sh --restore /root/backup/lxdcloud-backup-2026-10-06.tar.gz
# ============================================================
set -euo pipefail

# ---------- 默认值 ----------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_SRC_DIR="${SCRIPT_DIR}/backend"
FRONTEND_SRC_DIR="${SCRIPT_DIR}/frontend"
APP_DIR="/opt/lxdapi"
FRONT_DIR="/opt/lxdpanel-frontend"
BACKEND_PORT="9443"
DOMAIN=""
EMAIL=""
CERT_PROVIDER="auto"
DO_CERT=1
ASSUME_YES=0
FORCE=0
ACTION=""

# ---------- 颜色 ----------
C_RED='\033[0;31m'; C_GREEN='\033[0;32m'; C_YELLOW='\033[1;33m'; C_CYAN='\033[0;36m'; C_NC='\033[0m'
info()  { echo -e "${C_CYAN}[INFO]${C_NC} $*"; }
ok()    { echo -e "${C_GREEN}[ OK ]${C_NC} $*"; }
warn()  { echo -e "${C_YELLOW}[WARN]${C_NC} $*"; }
err()   { echo -e "${C_RED}[FAIL]${C_NC} $*" >&2; }

# ---------- 帮助 ----------
usage() {
    sed -n '2,60p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
}

# ---------- 参数解析 ----------
parse_args() {
    if [[ $# -eq 0 ]]; then usage; fi
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --all|--backend|--frontend|--reinstall|--cert|--restore|--status|--uninstall)
                ACTION="$1"
                shift
                if [[ "$ACTION" == "--restore" ]]; then
                    RESTORE_FILE="${1:-}"; shift || true
                    [[ -n "${RESTORE_FILE}" ]] || { err "--restore 需要备份文件路径"; exit 1; }
                fi
                ;;
            --backup)
                ACTION="--backup"; shift
                BACKUP_DIR="${1:-/root/backup}"; shift || true
                ;;
            --domain) shift; DOMAIN="${1:-}"; shift || true ;;
            --backend-port) shift; BACKEND_PORT="${1:-}"; shift || true ;;
            --app-dir) shift; APP_DIR="${1:-}"; shift || true ;;
            --front-dir) shift; FRONT_DIR="${1:-}"; shift || true ;;
            --email) shift; EMAIL="${1:-}"; shift || true ;;
            --cert-provider) shift; CERT_PROVIDER="${1:-}"; shift || true ;;
            --no-cert) DO_CERT=0; shift ;;
            --yes|-y) ASSUME_YES=1; shift ;;
            --force) FORCE=1; shift ;;
            -h|--help) usage ;;
            *) err "未知参数: $1"; usage ;;
        esac
    done
    [[ -n "$ACTION" ]] || { err "缺少操作参数"; usage; }
    # 域名/证书相关校验
    if [[ "$ACTION" == "--all" || "$ACTION" == "--reinstall" || "$ACTION" == "--cert" ]]; then
        if [[ "$ACTION" == "--cert" && -z "$DOMAIN" ]]; then
            err "--cert 需要 --domain 指定域名"; exit 1
        fi
    fi
}

# ---------- 环境检测 ----------
check_env() {
    info "检测运行环境..."
    [[ "$(id -u)" -eq 0 ]] || { err "请以 root 运行（sudo）"; exit 1; }
    command -v systemctl >/dev/null 2>&1 || { err "未检测到 systemd，本脚本仅支持 systemd 环境"; exit 1; }
    if command -v apt-get >/dev/null 2>&1; then
        PKG_MGR="apt"; PKG_UPDATE="apt-get update -y"
        PKG_INSTALL="apt-get install -y"
    elif command -v dnf >/dev/null 2>&1; then
        PKG_MGR="dnf"; PKG_UPDATE="dnf check-update || true"
        PKG_INSTALL="dnf install -y"
    elif command -v yum >/dev/null 2>&1; then
        PKG_MGR="yum"; PKG_UPDATE="yum check-update || true"
        PKG_INSTALL="yum install -y"
    else
        err "不支持的包管理器（需要 apt/dnf/yum）"; exit 1
    fi
    if command -v nginx >/dev/null 2>&1; then
        NGINX_BIN="$(command -v nginx)"
    else
        NGINX_BIN=""
    fi
    if command -v go >/dev/null 2>&1 || [[ -x /usr/local/bin/go ]]; then
        GO_BIN="$(command -v go || echo /usr/local/bin/go)"
    else
        GO_BIN=""
    fi
    ok "环境：$(. /etc/os-release; echo "${PRETTY_NAME:-Linux}") / 包管理器 ${PKG_MGR}"
    info "    nginx: ${NGINX_BIN:-未安装(仅前端/证书需要)}"
    info "    go: ${GO_BIN:-未安装(仅源码编译需要，可用发布二进制替代)}"
}

ensure_nginx() {
    if [[ -z "${NGINX_BIN}" ]]; then
        info "安装 nginx..."
        eval "${PKG_UPDATE}"
        eval "${PKG_INSTALL} nginx"
        systemctl enable --now nginx
        NGINX_BIN="$(command -v nginx)"
    fi
    ok "nginx: ${NGINX_BIN}"
}

# ---------- 后端安装 ----------
build_backend() {
    if [[ -x "${BACKEND_SRC_DIR}/cmd/lxdapi/lxdapi-amd64" ]]; then
        info "使用已编译二进制 cmd/lxdapi/lxdapi-amd64"
        cp "${BACKEND_SRC_DIR}/cmd/lxdapi/lxdapi-amd64" "${APP_DIR}/lxdapi-amd64"
        chmod 755 "${APP_DIR}/lxdapi-amd64"
        return
    fi
    if [[ -z "${GO_BIN}" ]]; then
        err "未找到编译产物与 go 编译器；请先在 backend/cmd/lxdapi 目录编译，或放置发布二进制"
        exit 1
    fi
    info "使用 go 编译后端..."
    ( cd "${BACKEND_SRC_DIR}/cmd/lxdapi" && "${GO_BIN}" build -o lxdapi-amd64 . )
    cp "${BACKEND_SRC_DIR}/cmd/lxdapi/lxdapi-amd64" "${APP_DIR}/lxdapi-amd64"
    chmod 755 "${APP_DIR}/lxdapi-amd64"
    ok "后端编译完成"
}

gen_backend_config() {
    local cfg="${APP_DIR}/configs/config.yaml"
    mkdir -p "${APP_DIR}/configs" "${APP_DIR}/certs"
    if [[ -f "$cfg" ]]; then
        info "config.yaml 已存在，仅更新端口与证书路径（保留数据库等其余配置）"
        sed -i -E "s/^(\s*port:\s*).*/\1${BACKEND_PORT}/" "$cfg"
        sed -i -E "s|^(\s*cert_file:\s*).*|\1\"certs/server.crt\"|" "$cfg"
        sed -i -E "s|^(\s*key_file:\s*).*|\1\"certs/server.key\"|" "$cfg"
    else
        info "生成新 config.yaml（基于项目模板）"
        [[ -f "${SCRIPT_DIR}/configs/config.yaml" ]] || { err "缺少 configs/config.yaml 模板"; exit 1; }
        cp "${SCRIPT_DIR}/configs/config.yaml" "$cfg"
        sed -i -E "s/^(\s*port:\s*).*/\1${BACKEND_PORT}/" "$cfg"
        # 生成随机 api_hash（保留模板默认时取消注释下一行）
        local new_hash
        new_hash="$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')"
        sed -i -E "s/^(\s*api_hash:\s*).*/\1\"${new_hash}\"/" "$cfg"
    fi
    # 自签证书（若不存在）
    if [[ ! -f "${APP_DIR}/certs/server.crt" || ! -f "${APP_DIR}/certs/server.key" ]]; then
        info "生成后端自签证书（内网反代用；公网 HTTPS 由 Nginx + Let's Encrypt 承担）"
        openssl req -x509 -newkey rsa:2048 -nodes \
            -keyout "${APP_DIR}/certs/server.key" \
            -out "${APP_DIR}/certs/server.crt" \
            -days 3650 -subj "/CN=${DOMAIN:-lxdcloud.local}" >/dev/null 2>&1
    fi
    ok "config.yaml 就绪（端口 ${BACKEND_PORT}）"
}

install_backend_service() {
    cat > /etc/systemd/system/lxdapi.service <<EOF
[Unit]
Description=LXD API Server
After=network.target lxd.service
Wants=lxd.service

[Service]
Type=simple
User=root
WorkingDirectory=${APP_DIR}
Environment="PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin"
ExecStart=${APP_DIR}/lxdapi-amd64
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable lxdapi >/dev/null 2>&1 || true
    systemctl restart lxdapi
    sleep 2
    if systemctl is-active --quiet lxdapi; then
        ok "lxdapi 服务运行中"
    else
        err "lxdapi 服务启动失败，查看日志：journalctl -u lxdapi -n 50"
        exit 1
    fi
    # 健康检查
    local code
    code="$(curl -sk -o /dev/null -w '%{http_code}' "https://127.0.0.1:${BACKEND_PORT}/api/health" || true)"
    ok "后端健康检查 HTTP ${code:-N/A}（${APP_DIR}/lxdapi-amd64, 端口 ${BACKEND_PORT}）"
}

# ---------- 前端安装 ----------
install_frontend() {
    [[ -d "${FRONTEND_SRC_DIR}" ]] || { err "缺少前端源码目录 ${FRONTEND_SRC_DIR}"; exit 1; }
    ensure_nginx
    info "部署前端到 ${FRONT_DIR}"
    mkdir -p "$(dirname "${FRONT_DIR}")"
    if [[ -d "${FRONT_DIR}" && $FORCE -eq 0 && $ASSUME_YES -eq 0 ]]; then
        read -r -p "前端目录 ${FRONT_DIR} 已存在，覆盖安装？[y/N] " ans
        [[ "$ans" == "y" || "$ans" == "Y" ]] || { warn "已取消"; return 1; }
    fi
    # 保留旧版本差异：直接整目录覆盖（保留用户可能放置的文件由 --backup 负责）
    cp -a "${FRONTEND_SRC_DIR}/." "${FRONT_DIR}/"
    chown -R www-data:www-data "${FRONT_DIR}" 2>/dev/null || chown -R root:root "${FRONT_DIR}"
    find "${FRONT_DIR}" -type d -exec chmod 755 {} +
    find "${FRONT_DIR}" -type f -exec chmod 644 {} +
    ok "前端文件就绪"
}

gen_nginx_conf() {
    local conf="/etc/nginx/conf.d/lxdpanel.conf"
    local server_name="${DOMAIN:-_}"
    info "生成 Nginx 配置 → ${conf} (server_name=${server_name}, 后端端口 ${BACKEND_PORT})"
    cat > "${conf}" <<EOF
# LXD Cloud - 自动生成于 $(date '+%Y-%m-%d %H:%M:%S')
server {
    listen 80;
    server_name ${server_name};

    root ${FRONT_DIR};
    index index.html;

    client_max_body_size 100M;
    client_body_buffer_size 128k;

    gzip on;
    gzip_min_length 1k;
    gzip_types text/css application/javascript application/json image/svg+xml;

    location /api/ {
        proxy_pass https://127.0.0.1:${BACKEND_PORT};
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 30s;
        proxy_read_timeout 120s;
    }

    location /ws/ {
        proxy_pass https://127.0.0.1:${BACKEND_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_read_timeout 3600s;
    }

    location ^~ /container { index login.html; try_files \$uri \$uri/ /container/login.html; }
    location ^~ /admin    { index login.html; try_files \$uri \$uri/ /admin/login.html; }
    location ^~ /user     { index login.html; try_files \$uri \$uri/ /user/login.html; }

    location / { try_files \$uri \$uri/ /index.html; }
}
EOF
    nginx -t
    systemctl reload nginx
    ok "Nginx 配置生效"
}

# ---------- 证书 ----------
setup_cert() {
    [[ -n "$DOMAIN" ]] || { err "申请证书需要 --domain"; exit 1; }
    [[ -n "$EMAIL" ]] && EMAIL_ARGS=("--email" "$EMAIL") || EMAIL_ARGS=()
    info "申请 Let's Encrypt 免费证书：${DOMAIN}（provider=${CERT_PROVIDER}）"

    if command -v certbot >/dev/null 2>&1 && { [[ "$CERT_PROVIDER" == "auto" ]] || [[ "$CERT_PROVIDER" == "certbot" ]]; }; then
        certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos \
            "${EMAIL_ARGS[@]}" --redirect --keep-until-expiring || {
                err "certbot 申请失败"; exit 1; }
        ok "certbot 证书已安装并启用自动续期（systemd timer）"
    elif command -v acme.sh >/dev/null 2>&1 || [[ -f /root/.acme.sh/acme.sh ]]; then
        local acme="/root/.acme.sh/acme.sh"
        "$acme" --issue -d "$DOMAIN" --nginx "${EMAIL_ARGS[@]}" || {
            err "acme.sh 申请失败"; exit 1; }
        "$acme" --install-cert -d "$DOMAIN" \
            --key-file /etc/nginx/ssl/${DOMAIN}.key \
            --fullchain-file /etc/nginx/ssl/${DOMAIN}.crt \
            --reloadcmd "systemctl reload nginx"
        # 续期兜底 cron
        ( crontab -l 2>/dev/null | grep -v "acme.sh.*${DOMAIN}" ; \
          echo "0 3 * * * \"$acme\" --cron --home /root/.acme.sh >/dev/null 2>&1" ) | crontab -
        ok "acme.sh 证书已安装，续期 cron 已配置"
    else
        warn "未检测到 certbot/acme.sh，尝试安装 certbot..."
        eval "${PKG_UPDATE}"
        if [[ "${PKG_MGR}" == "apt" ]]; then
            eval "${PKG_INSTALL} certbot python3-certbot-nginx"
        else
            eval "${PKG_INSTALL} certbot python3-certbot-nginx"
        fi
        certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos "${EMAIL_ARGS[@]}" --redirect
        ok "certbot 已安装并申请证书"
    fi
    # 兜底续期 cron（certbot 自带 timer 的情况下再放一条也无害）
    ( crontab -l 2>/dev/null | grep -v "lxdcloud-cert-renew" ; \
      echo "17 3 * * * root certbot renew --nginx --quiet || acme.sh --cron --home /root/.acme.sh >/dev/null 2>&1 # lxdcloud-cert-renew" ) | crontab -
}

# ---------- 备份 ----------
do_backup() {
    local out_dir="${BACKUP_DIR:-/root/backup}"
    mkdir -p "$out_dir"
    local stamp
    stamp="$(date +%Y%m%d-%H%M%S)"
    local file="${out_dir}/lxdcloud-backup-${stamp}.tar.gz"
    local args=()
    [[ -f "${APP_DIR}/lxdapi.db" ]] && args+=("${APP_DIR}/lxdapi.db")
    [[ -d "${APP_DIR}/configs" ]] && args+=("${APP_DIR}/configs")
    [[ -d "${APP_DIR}/certs" ]] && args+=("${APP_DIR}/certs")
    [[ -f /etc/systemd/system/lxdapi.service ]] && args+=(/etc/systemd/system/lxdapi.service)
    [[ -f /etc/nginx/conf.d/lxdpanel.conf ]] && args+=(/etc/nginx/conf.d/lxdpanel.conf)
    if [[ -d "${FRONT_DIR}" ]]; then
        info "包含前端目录 ${FRONT_DIR}（体积较大，备份耗时）"
        args+=("${FRONT_DIR}")
    fi
    if [[ ${#args[@]} -eq 0 ]]; then
        warn "没有可备份内容（${APP_DIR} 不存在或为空）"
        exit 0
    fi
    info "备份配置与数据 → ${file}"
    tar -Pczf "$file" "${args[@]}"
    ok "备份完成：${file}"
}

# ---------- 恢复 ----------
do_restore() {
    [[ -n "${RESTORE_FILE:-}" ]] || { err "缺少备份文件"; exit 1; }
    [[ -f "$RESTORE_FILE" ]] || { err "备份文件不存在：$RESTORE_FILE"; exit 1; }
    if [[ $FORCE -eq 0 ]]; then
        read -r -p "恢复会覆盖当前配置与数据，继续？[y/N] " ans
        [[ "$ans" == "y" || "$ans" == "Y" ]] || { warn "已取消"; exit 0; }
    fi
    info "从 ${RESTORE_FILE} 恢复..."
    local tmp
    tmp="$(mktemp -d)"
    tar -Pxzf "$RESTORE_FILE" -C "$tmp"
    # 停服务，恢复文件，再启动
    systemctl stop lxdapi 2>/dev/null || true
    mkdir -p "${APP_DIR}/configs" "${APP_DIR}/certs" "${FRONT_DIR}"
    [[ -f "${tmp}${APP_DIR}/lxdapi.db" ]] && cp -a "${tmp}${APP_DIR}/lxdapi.db" "${APP_DIR}/"
    [[ -d "${tmp}${APP_DIR}/configs" ]] && cp -a "${tmp}${APP_DIR}/configs/." "${APP_DIR}/configs/"
    [[ -d "${tmp}${APP_DIR}/certs" ]] && cp -a "${tmp}${APP_DIR}/certs/." "${APP_DIR}/certs/"
    [[ -f "${tmp}/etc/systemd/system/lxdapi.service" ]] && cp -a "${tmp}/etc/systemd/system/lxdapi.service" /etc/systemd/system/
    [[ -f "${tmp}/etc/nginx/conf.d/lxdpanel.conf" ]] && cp -a "${tmp}/etc/nginx/conf.d/lxdpanel.conf" /etc/nginx/conf.d/
    [[ -d "${tmp}${FRONT_DIR}" ]] && cp -a "${tmp}${FRONT_DIR}/." "${FRONT_DIR}/"
    systemctl daemon-reload
    systemctl start lxdapi 2>/dev/null || true
    nginx -t 2>/dev/null && systemctl reload nginx 2>/dev/null || true
    rm -rf "$tmp"
    ok "恢复完成（已重启 lxdapi 并重载 nginx）"
}

# ---------- 状态 ----------
do_status() {
    echo "==== LXD Cloud 部署状态 ===="
    echo "后端服务: $(systemctl is-active lxdapi 2>/dev/null || echo 未安装) (${APP_DIR})"
    if systemctl is-active --quiet lxdapi 2>/dev/null; then
        local port
        port="$(grep -E '^\s*port:' "${APP_DIR}/configs/config.yaml" 2>/dev/null | awk '{print $2}' | head -1)"
        echo "后端端口: ${port:-${BACKEND_PORT}}"
        curl -sk -o /dev/null -w "健康检查: HTTP %{http_code}\n" "https://127.0.0.1:${port:-${BACKEND_PORT}}/api/health" || echo "健康检查: 失败"
    fi
    echo "Nginx: $(systemctl is-active nginx 2>/dev/null || echo 未安装)"
    if [[ -f /etc/nginx/conf.d/lxdpanel.conf ]]; then
        echo "前端目录: ${FRONT_DIR}"
        echo "域名: $(grep server_name /etc/nginx/conf.d/lxdpanel.conf | head -1 | awk '{print $2}')"
    fi
    if [[ -n "$DOMAIN" ]] && command -v openssl >/dev/null 2>&1; then
        local crt="/etc/letsencrypt/live/${DOMAIN}/fullchain.pem"
        [[ -f "$crt" ]] || crt="/etc/nginx/ssl/${DOMAIN}.crt"
        if [[ -f "$crt" ]]; then
            echo "证书: $crt 到期 $(openssl x509 -enddate -noout -in "$crt" | cut -d= -f2)"
        else
            echo "证书: 未申请"
        fi
    fi
}

# ---------- 卸载 ----------
do_uninstall() {
    if [[ $FORCE -eq 0 ]]; then
        read -r -p "确认卸载？(先自动备份到 /root/backup) [y/N] " ans
        [[ "$ans" == "y" || "$ans" == "Y" ]] || { warn "已取消"; exit 0; }
    fi
    BACKUP_DIR="${BACKUP_DIR:-/root/backup}"
    do_backup
    systemctl stop lxdapi 2>/dev/null || true
    systemctl disable lxdapi 2>/dev/null || true
    rm -f /etc/systemd/system/lxdapi.service /etc/nginx/conf.d/lxdpanel.conf
    systemctl daemon-reload
    systemctl reload nginx 2>/dev/null || true
    ok "已卸载服务与 Nginx 配置（数据保留在 ${APP_DIR}，备份在 ${BACKUP_DIR}；如需彻底删除请手动执行 rm -rf）"
}

# ---------- 主流程 ----------
main() {
    parse_args "$@"
    check_env

    case "$ACTION" in
        --backend)
            mkdir -p "${APP_DIR}"
            build_backend
            gen_backend_config
            install_backend_service
            ;;
        --frontend)
            install_frontend
            gen_nginx_conf
            if [[ $DO_CERT -eq 1 && -n "$DOMAIN" ]]; then setup_cert; fi
            ;;
        --all)
            mkdir -p "${APP_DIR}"
            build_backend
            gen_backend_config
            install_backend_service
            install_frontend
            gen_nginx_conf
            if [[ $DO_CERT -eq 1 ]]; then
                if [[ -n "$DOMAIN" ]]; then setup_cert; else warn "未指定 --domain，跳过证书申请"; fi
            fi
            ;;
        --reinstall)
            info "重新安装：先备份现有配置与数据"
            BACKUP_DIR="${BACKUP_DIR:-/root/backup}"
            do_backup
            mkdir -p "${APP_DIR}"
            build_backend
            gen_backend_config
            install_backend_service
            install_frontend
            gen_nginx_conf
            if [[ $DO_CERT -eq 1 && -n "$DOMAIN" ]]; then setup_cert; fi
            ;;
        --cert)
            setup_cert
            ;;
        --backup)
            do_backup
            ;;
        --restore)
            do_restore
            ;;
        --status)
            do_status
            ;;
        --uninstall)
            do_uninstall
            ;;
    esac

    echo ""
    if [[ "$ACTION" != "--backup" && "$ACTION" != "--restore" && "$ACTION" != "--status" ]]; then
        if [[ -n "$DOMAIN" ]]; then
            ok "部署完成：https://${DOMAIN}/ （管理后台 /admin/login.html）"
        else
            ok "部署完成：http://<服务器IP>/ （管理后台 /admin/login.html）"
        fi
        echo "   - 后端目录：${APP_DIR}   前端目录：${FRONT_DIR}"
        echo "   - 查看状态：bash install.sh --status"
    fi
}

main "$@"
