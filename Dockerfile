# isobuilder — 单镜像交付：前端 SPA + Go 后端 + ISO 重打包工具链
#
# 构建（仓库根目录）:
#   docker build -f isobuilder/Dockerfile -t isobuilder:latest .
# 运行:
#   docker run -d -p 8080:8080 -v isobuilder-data:/data \
#       -e ISOBUILDER_S3_ENDPOINT=http://minio:9000 \
#       -e ISOBUILDER_S3_BUCKET=auto-iso \
#       -e ISOBUILDER_S3_ACCESS_KEY=... -e ISOBUILDER_S3_SECRET_KEY=... \
#       isobuilder:latest
#
# 解包原版 ISO 使用 xorriso -osirrox（无需 loop 挂载），容器不需要任何特权。
# 代价是解包一次 ISO 的全部内容落盘（≈ 1 倍原 ISO 体积 + 输出同等大小），所以
# 工作目录 /data/work 要有足够空间。

# ── Stage 1: 前端构建 ──────────────────────────────────────────────────────
FROM docker.io/library/node:22-alpine AS web
WORKDIR /app/frontend
RUN corepack enable && corepack prepare pnpm@12.6.0 --activate
COPY frontend/package.json frontend/pnpm-workspace.yaml frontend/pnpm-lock.yaml ./
COPY frontend/packages ./packages
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

# ── Stage 2: 后端构建 ─────────────────────────────────────────────────────
FROM docker.io/library/golang:1.26-alpine AS api
WORKDIR /app/backend
RUN apk add --no-cache git ca-certificates
# 公司内网默认访问不到 proxy.golang.org；海外部署时用
#   --build-arg GOPROXY=https://proxy.golang.org,direct
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} GOSUMDB=off
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/isobuilder ./cmd/isobuilder

# ── Stage 3: 运行时 ───────────────────────────────────────────────────────
FROM docker.io/rockylinux/rockylinux:9

# xorriso / syslinux / isomd5sum 覆盖整条 Go 流水线。
# 不装 genisoimage：isoinfo 在 RHEL9 只在 EPEL 里，卷标用 xorriso -pvd_info 读。
# python3 仅作为参考脚本的手动运维兜底，Go 流水线本身不依赖 Python。
RUN dnf install -y --setopt=install_weak_deps=False \
        xorriso syslinux syslinux-nonlinux isomd5sum \
        python3 ca-certificates && \
    dnf clean all && rm -rf /var/cache/dnf

WORKDIR /opt/isobuilder
COPY --from=api /out/isobuilder ./isobuilder
COPY --from=web /app/isobuilder/backend/web/dist ./web/
COPY kickstarts/openeuler.ks kickstarts/rocky9.ks ./kickstarts/
COPY scripts/repack-iso-x86.sh scripts/repack-iso-arm.sh ./scripts/

ENV ISOBUILDER_ADDR=:8080 \
    ISOBUILDER_DATA_DIR=/data \
    ISOBUILDER_WEB_DIR=/opt/isobuilder/web \
    ISOBUILDER_BUILTIN_DIR=/opt/isobuilder/kickstarts

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/opt/isobuilder/isobuilder"]
