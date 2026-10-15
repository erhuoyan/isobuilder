# isobuilder — 交付镜像：前端 SPA + Go 后端 + ISO 重打包工具链
#
# 构建（仓库根目录；M3 / Apple Silicon 上交叉构建 amd64）:
#   docker buildx build --platform linux/amd64 -t isobuilder:latest --load .
#
# 前端与后端阶段跑在 BUILDPLATFORM（本机原生，不模拟），只有运行时阶段跑
# TARGETPLATFORM，因此交叉构建的唯一模拟开销是那一条 microdnf install。
#
# 解包原版 ISO 使用 xorriso -osirrox（无需 loop 挂载），容器不需要任何特权。
# 代价是解包一次 ISO 的全部内容落盘（≈ 1 倍原 ISO 体积 + 输出同等大小），
# 所以数据目录 /data 必须挂足容量的卷。
#
# 需要更小/无需 QEMU 的构建路径见 Dockerfile.prebuilt + deploy/build-image.sh。

# ── Stage 1: 前端（BUILDPLATFORM，JS 产物与架构无关）────────────────────────
FROM --platform=$BUILDPLATFORM docker.io/library/node:22-alpine AS web
WORKDIR /app/frontend
RUN corepack enable && corepack prepare pnpm@12.6.0 --activate
COPY frontend/package.json frontend/pnpm-workspace.yaml frontend/pnpm-lock.yaml ./
COPY frontend/packages ./packages
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

# ── Stage 2: 后端（BUILDPLATFORM，Go 原生交叉编译）──────────────────────────
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.26-alpine AS api
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /app/backend
RUN apk add --no-cache git ca-certificates
# 公司内网默认访问不到 proxy.golang.org；海外部署时用
#   --build-arg GOPROXY=https://proxy.golang.org,direct
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} GOSUMDB=off
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/isobuilder ./cmd/isobuilder

# ── Stage 3: 运行时（TARGETPLATFORM，约 120 MB）─────────────────────────────
FROM --platform=$TARGETPLATFORM docker.io/rockylinux/rockylinux:9-minimal

# microdnf 是 minimal 变体的包管理器。四个包覆盖整条流水线：
#   xorriso            解包(-osirrox) / 重建(-as mkisofs) / 读卷标(-pvd_info)
#   syslinux           提供 isohybrid 二进制
#   syslinux-nonlinux  提供 /usr/share/syslinux/isohdpfx.bin（x86 MBR 模板）
#   isomd5sum          提供 implantisomd5（BMC 启动前的 MD5 标记）
# 不装 genisoimage（isoinfo 在 RHEL9 只在 EPEL）；不装 python3（Go 流水线
# 不依赖它，仅 scripts/*.sh 手工应急时需要，届时在宿主机安装）。
RUN microdnf install -y \
        xorriso syslinux syslinux-nonlinux isomd5sum ca-certificates && \
    microdnf clean all && rm -rf /var/cache/yum /var/cache/dnf

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
