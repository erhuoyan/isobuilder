#!/usr/bin/env bash
# 打发布包：isobuilder-<version>-linux-<arch>.tar.gz
# 在仓库根目录执行：./deploy/release.sh [版本号]
#
# 产物结构：
#   isobuilder-0.1.0-linux-amd64.tar.gz  （含 config.baremetal.yaml / env.example）
#   ├── isobuilder          静态二进制
#   ├── web/                前端 SPA（后端同源提供）
#   ├── kickstarts/         内置 kickstart 模板
#   ├── scripts/            单机应急用的 repack-iso-*.sh
#   └── README.md
set -euo pipefail

VERSION="${1:-0.1.0}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/release"
GOOS_TARGET="${GOOS_TARGET:-linux}"
GOARCH_TARGET="${GOARCH_TARGET:-amd64}"
STAGE="$OUT/isobuilder-$VERSION-$GOOS_TARGET-$GOARCH_TARGET"

# 默认总是重新构建前端：dist/ 里可能是不含最新改动的旧产物，而发布包一旦用旧产物
# 就无法从包本身看出问题（曾经因此把「系统配置」页缺失的包发出去）。
# 确认产物是最新时可加 --skip-web 跳过，节省约 1 分钟。
if [ "${SKIP_WEB:-0}" = "1" ]; then
    echo "▶ 跳过前端构建（SKIP_WEB=1），使用现有 dist/"
    [ -f "$ROOT/backend/web/dist/index.html" ] || { echo "dist/ 不存在，无法跳过前端构建"; exit 1; }
else
    echo "▶ 构建前端"
    (cd "$ROOT/frontend" && pnpm install && pnpm build)
fi

echo "▶ 交叉编译后端 $GOOS_TARGET/$GOARCH_TARGET"
mkdir -p "$OUT"
rm -rf "$STAGE"; mkdir -p "$STAGE"
(cd "$ROOT/backend" && CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" \
    go build -trimpath -ldflags="-s -w" -o "$STAGE/isobuilder" ./cmd/isobuilder)

echo "▶ 收集前端、模板与脚本"
# 排除 macOS AppleDouble 旁文件（._foo.ks）：它们没有任何占位符，会让模板库里
# 出现一堆不可用的 163 字节「模板」。
tar -cf - --exclude='._*' --exclude='.DS_Store' -C "$ROOT/backend/web/dist" . \
    | (mkdir -p "$STAGE/web" && tar -xf - -C "$STAGE/web")
mkdir -p "$STAGE/kickstarts" "$STAGE/scripts"
cp "$ROOT"/kickstarts/*.ks "$STAGE/kickstarts/"
cp "$ROOT"/scripts/*.sh "$STAGE/scripts/"
install -m 0644 "$ROOT/README.md" "$STAGE/README.md"
install -m 0755 "$ROOT/deploy/install.sh" "$STAGE/install.sh"
install -m 0644 "$ROOT/deploy/isobuilder.service" "$STAGE/isobuilder.service"
install -m 0644 "$ROOT/deploy/config.baremetal.yaml" "$STAGE/config.baremetal.yaml"
install -m 0600 "$ROOT/deploy/env.example" "$STAGE/env.example"

tar -czf "$STAGE.tar.gz" -C "$OUT" "$(basename "$STAGE")"
echo "✓ $STAGE.tar.gz ($(du -h "$STAGE.tar.gz" | cut -f1))"
echo "  在目标机执行：./install.sh --from-release <包路径>"
