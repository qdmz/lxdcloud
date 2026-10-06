#!/bin/bash

set -e

echo "开始编译 lxdapi..."

if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go 编译器"
    exit 1
fi

echo "Go 版本: $(go version)"

echo "清理旧文件..."
rm -f lxdapi-amd64 lxdapi-arm64

echo "下载依赖..."
go mod tidy
go mod download

echo ""
echo "生成 Swagger 文档..."
if command -v swag &> /dev/null; then
    swag init -g cmd/lxdapi/main.go --output docs --parseDependency --parseInternal
    echo "✓ Swagger 文档生成完成"
elif [ -f ~/go/bin/swag ]; then
    ~/go/bin/swag init -g cmd/lxdapi/main.go --output docs --parseDependency --parseInternal
    echo "✓ Swagger 文档生成完成"
else
    echo "警告: 未找到 swag 命令，跳过 Swagger 文档生成"
    echo "可通过以下命令安装: go install github.com/swaggo/swag/cmd/swag@latest"
fi

echo ""
echo "准备嵌入式资源..."
rm -rf cmd/lxdapi/templates cmd/lxdapi/docs cmd/lxdapi/static
cp -r ../lxdweb/templates cmd/lxdapi/
cp -r ../lxdweb/static cmd/lxdapi/
cp -r docs cmd/lxdapi/
echo "✓ 资源文件已准备"

echo ""
echo "编译 linux/amd64..."
cd cmd/lxdapi
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "../../lxdapi-amd64" .
cd ../..
echo "✓ lxdapi-amd64 编译完成"

echo ""
echo "编译 linux/arm64..."
cd cmd/lxdapi
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "../../lxdapi-arm64" .
cd ../..
echo "✓ lxdapi-arm64 编译完成"

echo ""
echo "编译完成！"
ls -lh lxdapi-* | awk '{print $9 " (" $5 ")"}'

echo ""
echo "清理临时文件..."
rm -rf cmd/lxdapi/templates cmd/lxdapi/docs
echo "✓ 临时文件已清理"

echo ""
echo "开始打包..."

rm -rf release
mkdir -p release

echo "打包 amd64 版本..."
mkdir -p release/lxdapi-amd64
cp lxdapi-amd64 release/lxdapi-amd64/
cp -r configs release/lxdapi-amd64/

# 打包 OpenGFW 插件
mkdir -p release/lxdapi-amd64/plugins/opengfw/bin
mkdir -p release/lxdapi-amd64/plugins/opengfw/data
cp plugins/opengfw/bin/OpenGFW-linux-amd64 release/lxdapi-amd64/plugins/opengfw/bin/
cp plugins/opengfw/data/*.dat release/lxdapi-amd64/plugins/opengfw/data/

# 打包 Nginx 插件
mkdir -p release/lxdapi-amd64/plugins/nginx/conf/sites
mkdir -p release/lxdapi-amd64/plugins/nginx/ssl
cp plugins/nginx/*.tmpl release/lxdapi-amd64/plugins/nginx/ 2>/dev/null || true
cp plugins/nginx/*.md release/lxdapi-amd64/plugins/nginx/ 2>/dev/null || true
cp plugins/nginx/*.sh release/lxdapi-amd64/plugins/nginx/ 2>/dev/null || true
chmod +x release/lxdapi-amd64/plugins/nginx/*.sh 2>/dev/null || true

cd release
tar -czf lxdapi-linux-amd64.tar.gz lxdapi-amd64
rm -rf lxdapi-amd64
cd ..
echo "✓ lxdapi-linux-amd64.tar.gz 打包完成"

echo "打包 arm64 版本..."
mkdir -p release/lxdapi-arm64
cp lxdapi-arm64 release/lxdapi-arm64/
cp -r configs release/lxdapi-arm64/

# 打包 OpenGFW 插件（仅对应架构）
mkdir -p release/lxdapi-arm64/plugins/opengfw/bin
mkdir -p release/lxdapi-arm64/plugins/opengfw/data
cp plugins/opengfw/bin/OpenGFW-linux-arm64 release/lxdapi-arm64/plugins/opengfw/bin/
cp plugins/opengfw/data/*.dat release/lxdapi-arm64/plugins/opengfw/data/

# 打包 Nginx 插件
mkdir -p release/lxdapi-arm64/plugins/nginx/conf/sites
mkdir -p release/lxdapi-arm64/plugins/nginx/ssl
cp plugins/nginx/*.tmpl release/lxdapi-arm64/plugins/nginx/ 2>/dev/null || true
cp plugins/nginx/*.md release/lxdapi-arm64/plugins/nginx/ 2>/dev/null || true
cp plugins/nginx/*.sh release/lxdapi-arm64/plugins/nginx/ 2>/dev/null || true
chmod +x release/lxdapi-arm64/plugins/nginx/*.sh 2>/dev/null || true

cd release
tar -czf lxdapi-linux-arm64.tar.gz lxdapi-arm64
rm -rf lxdapi-arm64
cd ..
echo "✓ lxdapi-linux-arm64.tar.gz 打包完成"

echo ""
echo "所有文件已打包到 release 目录："
ls -lh release/*.tar.gz | awk '{print $9 " (" $5 ")"}'
