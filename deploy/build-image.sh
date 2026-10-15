#!/usr/bin/env bash
# 构建 isobuilder 容器镜像：宿主机交叉编译产物 → 最小运行时镜像。
#
# 镜像 tag 形如 <版本号>-<commit>-<时间>：
#   0.1.0-7619ddc-20261015-155030
#
# 编译全部在宿主机完成（Go 原生交叉编译、前端 Node 原生构建），容器内只做
# microdnf 安装，因此不需要 QEMU，Apple Silicon 上也能直接出 amd64 镜像。
#
# 用法（仓库根目录）:
#   ./deploy/build-image.sh                    # 版本取自 git tag，否则 0.0.0
#   ./deploy/build-image.sh 0.2.0              # 指定版本号
#   ./deploy/build-image.sh 0.2.0 --push       # 构建后推送到仓库
#   REGISTRY=registry.example.com/team ./deploy/build-image.sh 0.2.0 --push
#   ARCH=arm64 ./deploy/build-image.sh 0.2.0   # 构建其它架构
#   SKIP_WEB=1 ./deploy/build-image.sh 0.2.0   # 复用现有前端 dist
#
# 环境变量:
#   REGISTRY    镜像仓库前缀（默认空，仅本地）
#   IMAGE_NAME  镜像名（默认 isobuilder）
#   ARCH        目标架构（默认 amd64）
#   SKIP_WEB    =1 时跳过前端构建
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REGISTRY="${REGISTRY:-}"
IMAGE_NAME="${IMAGE_NAME:-isobuilder}"
ARCH="${ARCH:-amd64}"
SKIP_WEB="${SKIP_WEB:-0}"

VERSION=""
PUSH=0
for arg in "$@"; do
    case "$arg" in
        --push) PUSH=1 ;;
        -*) echo "未知参数: $arg" >&2; exit 2 ;;
        *) VERSION="$arg" ;;
    esac
done

# 版本号：显式参数 > 最近的 git tag（去掉 v 前缀）> 0.0.0
if [ -z "$VERSION" ]; then
    VERSION="$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
    [ -n "$VERSION" ] || VERSION="0.0.0"
fi

COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo nogit)"
STAMP="$(date +%Y%m%d-%H%M%S)"
TAG="${VERSION}-${COMMIT}-${STAMP}"
IMAGE="${REGISTRY:+${REGISTRY}/}${IMAGE_NAME}:${TAG}"

echo "▶ 版本: $VERSION   commit: $COMMIT   时间: $STAMP"
echo "  镜像: $IMAGE"
echo "  架构: linux/$ARCH"

# ── 1. 前端产物（Node 原生，与镜像架构无关）────────────────────────────────
if [ "$SKIP_WEB" = "1" ]; then
    echo "▶ 跳过前端构建（SKIP_WEB=1），复用 backend/web/dist"
    [ -f "$ROOT/backend/web/dist/index.html" ] || { echo "dist 不存在，无法跳过前端构建"; exit 1; }
else
    echo "▶ 构建前端产物"
    (cd "$ROOT/frontend" && pnpm build)
fi

# ── 2. Go 交叉编译（原生交叉，不经过 QEMU）─────────────────────────────────
echo "▶ 交叉编译后端 linux/$ARCH"
mkdir -p "$ROOT/backend/dist"
(cd "$ROOT/backend" && CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
    go build -trimpath -ldflags="-s -w" -o dist/isobuilder ./cmd/isobuilder)

# ── 3. 打运行时镜像（唯一进入容器的步骤，且不含编译）───────────────────────
echo "▶ 构建镜像"
TARGET="linux/$ARCH"
# --provenance=false --sbom=false：不加 buildx 默认的 attestation manifest。
# 否则多架构索引里会多出一个 unknown/unknown 平台条目（几 KB 的非镜像层），
# 镜像仓库 UI、扫描器、同步工具都会把它当成异常条目。
BUILDX_ARGS=(buildx build --platform "$TARGET" -f Dockerfile.prebuilt
             -t "$IMAGE" --provenance=false --sbom=false)
if [ "$PUSH" = "1" ]; then
    (cd "$ROOT" && docker "${BUILDX_ARGS[@]}" --push .)
else
    (cd "$ROOT" && docker "${BUILDX_ARGS[@]}" --load .)
fi

echo "✓ $IMAGE"
if [ "$PUSH" = "1" ]; then
    echo "  部署:"
    echo "    kubectl -n isobuilder set image deployment/isobuilder isobuilder=$IMAGE"
else
    echo "  本地已加载。推送: REGISTRY=<你的仓库> $0 $VERSION --push"
fi
