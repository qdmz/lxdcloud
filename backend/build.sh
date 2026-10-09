#!/usr/bin/env bash
# ============================================================
# lxdapi 编译打包脚本
#   用法：bash build.sh            # 编译 amd64 + arm64 并打包到 release/
#         ARCHS="amd64" bash build.sh
# 说明：
#   - 模板/静态资源/文档直接从 cmd/lxdapi/{templates,static,docs} 嵌入，
#     不再从不存在的 ../lxdweb 复制，也不会在编译后删除这些目录（v1.1.0 修复）。
#   - SQLite 使用纯 Go 驱动（modernc.org/sqlite），CGO_ENABLED=0 即可交叉编译。
#   - 版本号取仓库根目录 VERSION 文件，注入 main.Version。
# ============================================================
set -euo pipefail
cd "$(dirname "$0")"

ARCHS="${ARCHS:-amd64 arm64}"
VERSION="$(cat ../VERSION 2>/dev/null || echo dev)"

command -v go >/dev/null 2>&1 || { echo "错误: 未找到 Go 编译器"; exit 1; }
echo "Go 版本: $(go version)"
echo "构建版本: v${VERSION}"

for d in templates static docs; do
  [ -d "cmd/lxdapi/$d" ] || { echo "错误: 缺少嵌入资源目录 cmd/lxdapi/$d"; exit 1; }
done

echo "下载依赖..."
go mod download

# 可选：重新生成 Swagger 文档（存在 swag 时）
SWAG="$(command -v swag || true)"; [ -z "$SWAG" ] && [ -x "$HOME/go/bin/swag" ] && SWAG="$HOME/go/bin/swag"
if [ -n "$SWAG" ]; then
  "$SWAG" init -g cmd/lxdapi/main.go --output docs --parseDependency --parseInternal >/dev/null && \
    cp docs/* cmd/lxdapi/docs/ && echo "✓ Swagger 文档已更新"
else
  echo "提示: 未找到 swag，沿用现有 Swagger 文档"
fi

rm -rf release && mkdir -p release
for arch in $ARCHS; do
  echo ""
  echo "编译 linux/${arch}..."
  out="lxdapi-${arch}"
  GOOS=linux GOARCH="$arch" CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" -o "$out" ./cmd/lxdapi
  echo "✓ ${out} ($(du -h "$out" | cut -f1))"

  pkg="release/lxdapi-${arch}"
  mkdir -p "$pkg/plugins/nginx/conf/sites" "$pkg/plugins/nginx/ssl" "$pkg/plugins/opengfw/bin" "$pkg/plugins/opengfw/data"
  cp "$out" "$pkg/lxdapi"
  cp -r configs "$pkg/"
  cp plugins/nginx/*.tmpl "$pkg/plugins/nginx/" 2>/dev/null || true
  cp plugins/opengfw/bin/OpenGFW-linux-"$arch" "$pkg/plugins/opengfw/bin/" 2>/dev/null || true
  cp plugins/opengfw/data/*.dat "$pkg/plugins/opengfw/data/" 2>/dev/null || true
  [ -d ../frontend ] && cp -r ../frontend "$pkg/frontend"
  tar -C release -czf "release/lxdapi-v${VERSION}-linux-${arch}.tar.gz" "lxdapi-${arch}"
  rm -rf "$pkg"
  echo "✓ release/lxdapi-v${VERSION}-linux-${arch}.tar.gz"
done

echo ""
echo "编译完成："
ls -lh release/*.tar.gz | awk '{print $9 " (" $5 ")"}'
