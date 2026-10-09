# ============================================================
# LXD Cloud - 后端 Dockerfile（多阶段构建）
#
# 构建：docker build -t lxdcloud-backend .
# 注意：容器需要挂载宿主机的 LXD unix socket 才能管理容器，
#       并且需要 lxc 客户端（二进制内通过 exec 调用 lxc 命令）。
# ============================================================

# ---------- 构建阶段 ----------
FROM golang:1.25-bookworm AS builder

WORKDIR /src

# 先复制依赖定义，利用 Docker 缓存
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# 复制源码并编译（SQLite 使用纯 Go 驱动 modernc.org/sqlite，无需 CGO）
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /lxdapi ./cmd/lxdapi

# ---------- 运行阶段 ----------
FROM debian:bookworm-slim

# lxc 客户端（后端通过 exec 调用 lxc 命令）+ 时区 + 证书
RUN apt-get update && apt-get install -y --no-install-recommends \
        lxd-client \
        ca-certificates \
        tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /lxdapi /app/lxdapi
# 默认配置（运行时用 volume 覆盖）
COPY configs/config.yaml /app/configs/config.yaml

# 数据目录（SQLite / 证书）
VOLUME ["/app/data"]

EXPOSE 9443

# 默认用 /app/configs/config.yaml；config 中 sqlite 路径建议改为 /app/data/lxdapi.db
CMD ["/app/lxdapi"]
