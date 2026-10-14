#!/usr/bin/env bash
# 输出本机作为「节点」被其它主控添加时所需的信息：节点地址 + api_hash，并自测
# 用法：bash scripts/node-info.sh [config.yaml 路径]
CFG="${1:-${CFG:-/opt/lxdapi/configs/config.yaml}}"
[ -f "$CFG" ] || CFG=/opt/lxdcloud/configs/config.yaml
[ -f "$CFG" ] || { echo "找不到 config.yaml，请把路径作为参数传入"; exit 1; }

HASH=$(grep -E '^\s*api_hash:' "$CFG" | head -1 | sed -E 's/.*api_hash:\s*"?([^"]*)"?.*/\1/')
PORT=$(grep -E '^\s*port:' "$CFG" | head -1 | awk '{print $2}')
IP=$(curl -s -m 5 https://api.ipify.org || hostname -I | awk '{print $1}')

echo "配置文件 : $CFG"
echo "节点地址 : https://${IP}:${PORT}   (有域名就用 https://域名:${PORT})"
case "$HASH" in
  ""|__*__) echo "api_hash : 未配置（系统级 API 已禁用），请在 system.security.api_hash 设置后重启" ; exit 1 ;;
  *) echo "api_hash : ${HASH}" ;;
esac
echo -n "自测     : "
curl -sk -m 10 "https://127.0.0.1:${PORT}/api/admin/storage-pools" -H "X-API-Hash: ${HASH}" | head -c 160; echo
