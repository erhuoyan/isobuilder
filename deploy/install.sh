#!/usr/bin/env bash
# 裸机安装/升级 isobuilder。
#
# 在**构建机**上产出发布包后，把 tar.gz 拷到目标机执行本脚本即可；也可以直接在
# 目标机上用本脚本从源码目录编译安装。
#
# 用法（在仓库根目录）：
#   ./deploy/install.sh                 # 本机编译 + 安装（需要 go / pnpm 或已构建产物）
#   ./deploy/install.sh --from-release isobuilder-0.1.0-linux-amd64.tar.gz
#   ./deploy/install.sh --uninstall
#
# 安装位置：
#   /opt/isobuilder/{isobuilder,web,kickstarts,scripts,README.md}
#   /var/lib/isobuilder/{isos,kickstarts,work,builds,artifacts,uploads,isobuilder.db}
#   /etc/isobuilder/env                （可选，首次默认值）
#   /etc/systemd/system/isobuilder.service
set -euo pipefail

PREFIX="${ISOBUILDER_PREFIX:-/opt/isobuilder}"
DATA_DIR="${ISOBUILDER_DATA_DIR:-/var/lib/isobuilder}"
CONF_DIR="${ISOBUILDER_CONF_DIR:-/etc/isobuilder}"
SERVICE=/etc/systemd/system/isobuilder.service
FROM_RELEASE=""

while [ $# -gt 0 ]; do
    case "$1" in
        --from-release) FROM_RELEASE="${2:?缺少发布包路径}"; shift 2 ;;
        --prefix)       PREFIX="${2:?}"; shift 2 ;;
        --uninstall)    UNINSTALL=1; shift ;;
        -h|--help)      sed -n '2,20p' "$0"; exit 0 ;;
        *) echo "未知参数: $1" >&2; exit 2 ;;
    esac
done

log() { echo -e "\033[36m▶\033[0m $*"; }

need() { command -v "$1" >/dev/null 2>&1 || { echo "缺少命令: $1"; exit 1; }; }

if [ "${UNINSTALL:-0}" = "1" ]; then
    log "停止并卸载服务（数据目录 $DATA_DIR 保留）"
    systemctl disable --now isobuilder 2>/dev/null || true
    rm -f "$SERVICE"; systemctl daemon-reload
    rm -rf "$PREFIX"
    echo "已卸载。数据仍在 $DATA_DIR，需要时手动删除。"
    exit 0
fi

[ "$(id -u)" = "0" ] || { echo "请用 root 运行（写 /opt、/etc、systemd）"; exit 1; }

# ── 1. 运行时依赖 ────────────────────────────────────────────────────────────
log "安装运行时依赖（xorriso / syslinux / isomd5sum）"
if command -v dnf >/dev/null 2>&1; then
    dnf install -y --setopt=install_weak_deps=False xorriso syslinux syslinux-nonlinux isomd5sum
elif command -v yum >/dev/null 2>&1; then
    yum install -y xorriso syslinux syslinux-nonlinux isomd5sum
elif command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq && apt-get install -y xorriso syslinux syslinux-common isolinux isomd5sum
else
    echo "未识别的包管理器，请手动安装：xorriso syslinux isomd5sum" >&2
fi

# ── 2. 目录 ──────────────────────────────────────────────────────────────────
# 服务以 root 运行（见 isobuilder.service 的说明）：部署最简，手工放进数据
# 目录的文件不会有属主/权限问题。它只写 $DATA_DIR。
log "创建目录"
mkdir -p "$PREFIX" "$CONF_DIR" "$DATA_DIR"

# ── 3. 二进制与前端 ──────────────────────────────────────────────────────────
if [ -n "$FROM_RELEASE" ]; then
    [ -f "$FROM_RELEASE" ] || { echo "发布包不存在: $FROM_RELEASE"; exit 1; }
    log "从发布包安装: $FROM_RELEASE"
    tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
    tar xzf "$FROM_RELEASE" -C "$tmp"
    src="$tmp"
    [ -f "$src/isobuilder" ] || src="$(find "$tmp" -maxdepth 2 -name isobuilder -type f | head -1 | xargs -r dirname)"
    install -m 0755 "$src/isobuilder" "$PREFIX/isobuilder"
    rm -rf "$PREFIX/web"; cp -R "$src/web" "$PREFIX/web"
    rm -rf "$PREFIX/kickstarts"; cp -R "$src/kickstarts" "$PREFIX/kickstarts"
    rm -rf "$PREFIX/scripts"; cp -R "$src/scripts" "$PREFIX/scripts"
    [ -f "$src/README.md" ] && install -m 0644 "$src/README.md" "$PREFIX/README.md"
else
    repo_root="$(cd "$(dirname "$0")/.." && pwd)"
    need go
    log "编译后端（源码目录 $repo_root/backend）"
    (cd "$repo_root/backend" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$PREFIX/isobuilder" ./cmd/isobuilder)

    if [ -d "$repo_root/backend/web/dist" ] && [ -f "$repo_root/backend/web/dist/index.html" ]; then
        log "使用已构建的前端产物"
    else
        need pnpm
        log "构建前端（首次约 2-3 分钟）"
        (cd "$repo_root/frontend" && pnpm install && pnpm build)
    fi
    rm -rf "$PREFIX/web"; cp -R "$repo_root/backend/web/dist" "$PREFIX/web"
    rm -rf "$PREFIX/kickstarts"; mkdir -p "$PREFIX/kickstarts"
    cp "$repo_root"/kickstarts/*.ks "$PREFIX/kickstarts/" 2>/dev/null || true
    rm -rf "$PREFIX/scripts"; mkdir -p "$PREFIX/scripts"
    cp "$repo_root"/scripts/repack-iso-*.sh "$PREFIX/scripts/" 2>/dev/null || true
    install -m 0644 "$repo_root/README.md" "$PREFIX/README.md"
fi
chmod 0755 "$PREFIX/isobuilder"

# ── 4. systemd ───────────────────────────────────────────────────────────────
log "安装 systemd 单元"
# 先停服务：运行中的二进制无法被覆盖（Text file busy 会让 cp 静默失败）
systemctl stop isobuilder 2>/dev/null || true
install -m 0644 "$(dirname "$0")/isobuilder.service" "$SERVICE"

# 配置文件：全部运行时设置都在这里，由 systemd 传给 isobuilder -c。
# env 文件只留给数据库凭据（可选），因此权限收紧到 0600。
CONFIG_FILE="$CONF_DIR/config.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    # 模板里的 data_dir 只是默认值；这里替换成本次安装实际使用的目录
    # （ISOBUILDER_DATA_DIR 可能指向数据盘，例如 /data/isobuilder）。
    sed "s|^data_dir: .*|data_dir: ${DATA_DIR}|" \
        "$(dirname "$0")/config.baremetal.yaml" > "$CONFIG_FILE"
    chmod 0644 "$CONFIG_FILE"
    log "已生成 $CONFIG_FILE（data_dir=$DATA_DIR）"
fi
if [ ! -f "$CONF_DIR/env" ]; then
    install -m 0600 "$(dirname "$0")/env.example" "$CONF_DIR/env"
    log "已生成 $CONF_DIR/env（只放数据库连接串；留空则用本机 SQLite）"
fi

# 端口策略（读写的都是 config.yaml 的 addr）：
#   - 升级（addr 已存在）→ 原样沿用，绝不因为「自己在跑」而改端口
#   - 全新安装 → 默认 8080，被别的进程占用时自动挑一个空闲端口
port=""
existing_addr="$(awk -F: '/^addr:[[:space:]]*/ {gsub(/["'"'"' ]/, "", $2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)"
if [ -n "$existing_addr" ]; then
    port="$existing_addr"
    log "沿用已配置端口 $port（升级不改变监听端口）"
else
    port=8080
    if ss -lntH 2>/dev/null | awk '{print $4}' | grep -qE "[:.]8080$"; then
        for candidate in $(seq 8081 8099); do
            if ! ss -lntH 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${candidate}$"; then
                port=$candidate; break
            fi
        done
        log "8080 已被占用，改用端口 $port"
    fi
    if grep -q '^addr:' "$CONFIG_FILE" 2>/dev/null; then
        sed -i "s|^addr:.*|addr: \":${port}\"|" "$CONFIG_FILE"
    else
        printf 'addr: ":%s"\n' "$port" >> "$CONFIG_FILE"
    fi
fi
systemctl daemon-reload
systemctl enable isobuilder
systemctl restart isobuilder
sleep 2
systemctl --no-pager --lines=15 status isobuilder || true

ip=$(hostname -I 2>/dev/null | awk '{print $1}')
echo
log "完成。浏览器打开: http://${ip:-<本机IP>}:${port}"
echo "  运行身份: root（只写 $DATA_DIR；需要收权见单元文件里的两行注释）"
echo "  数据目录: $DATA_DIR"
echo "  配置:     $CONFIG_FILE（全部运行时设置）"
echo "  数据库凭据: $CONF_DIR/env（可选，只放 ISOBUILDER_DB_URL）"
echo "  对象存储: 可在 Web「系统配置」页维护（存数据库，优先级高于配置文件）"
echo "  日志:     journalctl -u isobuilder -f"
echo "  端口:     $port（改 $CONFIG_FILE 里的 addr，然后 systemctl restart isobuilder）"
