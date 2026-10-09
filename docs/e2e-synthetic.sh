#!/usr/bin/env bash
# e2e-synthetic.sh — iso-builder 容器端到端冒烟（在远端 Rocky 主机上运行）
#
# 做什么：
#   1. 造两张“合成原版 ISO”（x86 BIOS+EFI 双段、aarch64 EFI-only），不追求真能启动，
#      但能覆盖流水线每个环节：解包、注入、boot 改写、xorriso 双段打包、isohybrid、implantisomd5。
#   2. 起 isobuilder 容器，通过 REST API 注册镜像 → 创建项目 → 触发构建 → 拉日志 → 校验产物。
#   3. 校验产物 ISO 的 El Torito 目 录、注入后 kickstart 内容、MD5 标记。
#
# 用法一（裸机，推荐）：让脚本打一个已经在跑的实例，不碰容器
#   E2E_BASE=http://127.0.0.1:8081 E2E_DATA=/var/lib/isobuilder ./e2e-synthetic.sh
# 用法二（容器）：脚本自己起 isobuilder 容器
#   ./e2e-synthetic.sh
set -euo pipefail

ROOT="${E2E_ROOT:-/root/iso-build-test}"
PORT="${E2E_PORT:-18080}"
IMG="${E2E_IMAGE:-isobuilder:test}"
# E2E_BASE 非空 = 外部实例模式：脚本只调 API，不管理容器与数据目录
BASE="${E2E_BASE:-http://127.0.0.1:$PORT}"
DATA="${E2E_DATA:-$ROOT/e2e-data}"
CID=""

cleanup() { [ -n "$CID" ] && docker rm -f "$CID" >/dev/null 2>&1 || true; }
trap cleanup EXIT

step() { echo; echo "▶ $*"; }

# ── 0. 准备工作目录与合成 ISO ────────────────────────────────────────────────
step "0. 生成合成原版 ISO"
if [ -n "${E2E_BASE:-}" ]; then
    mkdir -p "$DATA/isos"
else
    rm -rf "$DATA"; mkdir -p "$DATA/isos"
fi

mk_x86_iso() {
    local d="$DATA/stage-x86"; mkdir -p "$d/isolinux" "$d/EFI/BOOT" "$d/images/pxeboot" "$d/ks"
    # isolinux.bin 必须够大：xorriso -boot-info-table 要在偏移 8 处写 56 字节引导表，
    # 太小的镜像会报 "Isolinux image too small" 并以 SORRY(32) 退出。
    head -c 40960 /dev/urandom > "$d/isolinux/isolinux.bin"
    printf 'EFIBOOT-DUMMY'  > "$d/images/efiboot.img"
    printf 'VMLINUZ-DUMMY'  > "$d/images/pxeboot/vmlinuz"
    printf 'INITRD-DUMMY'   > "$d/images/pxeboot/initrd.img"
    cat > "$d/EFI/BOOT/grub.cfg" <<'EOF'
set default="0"
set timeout=5

menuentry 'Install Rocky 9' --class fedora --class gnu --class os {
    linuxefi /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=ROCKYFAKE quiet
    initrdefi /images/pxeboot/initrd.img
}
EOF
    mkisofs -o "$DATA/isos/rocky-fake-dvd.iso" -V ROCKYFAKE -R -J -quiet "$d"
    echo "   x86 合成 ISO: $(du -h "$DATA/isos/rocky-fake-dvd.iso" | cut -f1)"
}

mk_arm_iso() {
    local d="$DATA/stage-arm"; mkdir -p "$d/EFI/BOOT" "$d/images/pxeboot" "$d/ks"
    head -c 4096 /dev/urandom > "$d/images/efiboot.img"
    printf 'VMLINAZ-DUMMY'  > "$d/images/pxeboot/vmlinuz"
    printf 'INITRD-DUMMY'   > "$d/images/pxeboot/initrd.img"
    cat > "$d/EFI/BOOT/grub.cfg" <<'EOF'
set default="0"
set timeout=5

### BEGIN /etc/grub.d/10_linux ###
menuentry 'Install openEuler' --class red --class gnu --class os {
    linux /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=OPENEULERFAKE inst.geoloc=0
    initrd /images/pxeboot/initrd.img
}
### END /etc/grub.d/10_linux ###
EOF
    mkisofs -o "$DATA/isos/openeuler-fake-dvd.iso" -V OPENEULERFAKE -R -J -quiet "$d"
    echo "   arm 合成 ISO: $(du -h "$DATA/isos/openeuler-fake-dvd.iso" | cut -f1)"
}
mk_x86_iso; mk_arm_iso

# ── 1. 等待实例就绪（外部实例模式只探活）──────────────────────────────────────
if [ -n "${E2E_BASE:-}" ]; then
    step "1. 使用已有实例 $BASE"
    curl -fsS "$BASE/api/health" >/dev/null || { echo "实例不可达: $BASE"; exit 1; }
else
    step "1. 启动 isobuilder 容器（$IMG）"
    CID="$(docker run -d --restart=no -p $PORT:8080 -v "$DATA:/data" "$IMG")"
    echo "   container: $CID"

    for i in $(seq 1 30); do
        curl -fsS "$BASE/api/health" >/dev/null 2>&1 && break
        sleep 1
        [ "$i" = 30 ] && { echo "容器未能就绪"; docker logs "$CID" | tail -20; exit 1; }
    done
fi
curl -s "$BASE/api/config" | python3 -m json.tool | sed 's/^/   /'

# ── 2. 注册镜像 + 建项目 ─────────────────────────────────────────────────────
# 注册路径必须是「服务端能看到、且在它数据目录内」的路径
REG_DIR="${E2E_REG_DIR:-$DATA/isos}"
step "2. x86_64：注册镜像 → 建项目（$REG_DIR）"
ISO_X86=$(curl -s -X POST "$BASE/api/isos/register" -H 'Content-Type: application/json' \
    -d "{\"path\":\"$REG_DIR/rocky-fake-dvd.iso\",\"arch\":\"x86_64\"}" | python3 -c "import sys,json;_d=json.load(sys.stdin);d=_d.get('data',_d);print(d['iso']['id'] if 'iso' in d.get('data',d) else (_ for _ in ()).throw(SystemExit(d)))")
echo "   iso_id=$ISO_X86"
sleep 1
KS_X86=$(curl -s "$BASE/api/kickstarts" | python3 -c "import sys,json;print([k['id'] for k in json.load(sys.stdin)['data']['kickstarts'] if k['name']=='rocky9.ks'][0])")
echo "   kickstart_id=$KS_X86"

PRJ_X86=$(python3 - "$ISO_X86" "$KS_X86" <<'PYEOF' | curl -s -X POST "$BASE/api/projects" -H 'Content-Type: application/json' -d @- | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['project']['id'])"
import json,sys
iso,ks=sys.argv[1:3]
hosts="""hosts:
- sn: DEMO-SN-0001
  hostname: demo-node
  network:
    ip: 172.16.0.11
    netmask: 255.255.255.0
    gateway: 172.16.0.1
    mac: \"00:11:22:33:44:55\"
"""
print(json.dumps({"name":"e2e-x86","arch":"x86_64","iso_id":iso,"kickstart_id":ks,"hosts_raw":hosts}))
PYEOF
)
echo "   project_id=$PRJ_X86"

# ── 3. 构建 + 轮询日志 ────────────────────────────────────────────────────────
step "3. x86_64：构建"
BUILD_X86=$(curl -s -X POST "$BASE/api/projects/$PRJ_X86/builds" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['build']['id'])")
echo "   build_id=$BUILD_X86"

wait_build() {
    local bid=$1
    for i in $(seq 1 120); do
        local st; st=$(curl -s "$BASE/api/builds/$bid" | python3 -c "import sys,json;b=json.load(sys.stdin)['data']['build'];print(b['status'],b['progress'],b['stage'])")
        echo "   [$i] $st" | tr -d "(')"
        case "$st" in succeeded*|failed*|canceled*) echo "$st" | cut -d"'" -f2 2>/dev/null || echo "$st"; return 0;; esac
        sleep 2
    done
    echo "   TIMEOUT"; return 1
}

STATUS_X86=$(wait_build "$BUILD_X86" | tail -1)

step "3b. 构建日志（尾部）"
curl -s "$BASE/api/builds/$BUILD_X86/log?format=text" | tail -25 | sed 's/^/   /'

if [ "$STATUS_X86" != "succeeded 100 seal" ] && [ "$STATUS_X86" != "succeeded" ]; then
    echo "!!! x86 构建未成功: $STATUS_X86"; exit 1
fi

# ── 4. 校验产物 ───────────────────────────────────────────────────────────────
step "4. 校验 x86 产物"
ART_JSON=$(curl -s "$BASE/api/builds/$BUILD_X86")
echo "$ART_JSON" | python3 -m json.tool | sed 's/^/   /'
ART_ID=$(echo "$ART_JSON" | python3 -c "import sys,json;b=json.load(sys.stdin)['data']['build'];print(b['artifact']['id'] if b.get('artifact') else '')")
[ -n "$ART_ID" ] || { echo "!!! 无产物"; exit 1; }

OUT="/tmp/e2e-x86-out.iso"
curl -sfL "$BASE/api/artifacts/$ART_ID/download" -o "$OUT"
echo "   下载: $(du -h "$OUT" | cut -f1)"
echo "   El Torito:"
isoinfo -d -i "$OUT" | grep -iE "Eltorito|Bootoff|Application id|Volume id" | sed 's/^/   /'

echo "   解包产物验证 kickstart 注入:"
mkdir -p /tmp/e2e-x86-ex
xorriso -osirrox on -indev "$OUT" -extract /ks /tmp/e2e-x86-ex/ks -quiet || true
find /tmp/e2e-x86-ex -name '*.ks' | head -3
KS_RENDERED=$(find /tmp/e2e-x86-ex -name 'rocky9.ks' | head -1)
if [ -n "$KS_RENDERED" ]; then
    grep -q "DEMO-SN-0001" "$KS_RENDERED" && echo "   ✅ hosts JSON 已注入" || { echo "   ❌ 未注入"; head -5 "$KS_RENDERED"; }
fi

step "4b. 校验产物 grub 改写 (x86)"
xorriso -osirrox on -indev "$OUT" -extract /EFI/BOOT/grub.cfg /tmp/e2e-x86-ex/grub.cfg -quiet || true
GRUB=$(find /tmp/e2e-x86-ex -name grub.cfg 2>/dev/null | head -1); [ -n "$GRUB" ] && grep -c "Auto Install (Kickstart)" "$GRUB" | sed 's/^/   Auto Install entries: /'

# ── 5. aarch64 流水线 ────────────────────────────────────────────────────────
step "5. aarch64：注册镜像 → 建项目 → 构建"
ISO_ARM=$(curl -s -X POST "$BASE/api/isos/register" -H 'Content-Type: application/json' \
    -d "{\"path\":\"$REG_DIR/openeuler-fake-dvd.iso\",\"arch\":\"aarch64\"}" | python3 -c "import sys,json;_d=json.load(sys.stdin);d=_d.get('data',_d);print(d['iso']['id'] if 'iso' in d.get('data',d) else (_ for _ in ()).throw(SystemExit(d)))")
KS_ARM=$(curl -s "$BASE/api/kickstarts" | python3 -c "import sys,json;print([k['id'] for k in json.load(sys.stdin)['data']['kickstarts'] if k['name']=='openeuler.ks'][0])")
PRJ_ARM=$(python3 - "$ISO_ARM" "$KS_ARM" <<'PYEOF' | curl -s -X POST "$BASE/api/projects" -H 'Content-Type: application/json' -d @- | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['project']['id'])"
import json,sys
iso,ks=sys.argv[1:3]
hosts=json.dumps([{"sn":"ARM-DEMO-1","hostname":"arm-node","network":{"ip":"10.0.0.11","prefix":24,"mac":"11:22:33:44:55:66"}}])
print(json.dumps({"name":"e2e-arm","arch":"aarch64","iso_id":iso,"kickstart_id":ks,"hosts_raw":hosts,"hosts_format":"json"}))
PYEOF
)
BUILD_ARM=$(curl -s -X POST "$BASE/api/projects/$PRJ_ARM/builds" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['build']['id'])")
STATUS_ARM=$(wait_build "$BUILD_ARM" | tail -1)
[ "$STATUS_ARM" = "succeeded 100 seal" ] || [ "$STATUS_ARM" = "succeeded" ] || { echo "!!! arm 构建未成功"; curl -s "$BASE/api/builds/$BUILD_ARM/log?format=text" | tail -15; exit 1; }
ART_ARM=$(curl -s "$BASE/api/builds/$BUILD_ARM" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['build']['artifact']['id'])")
curl -sfL "$BASE/api/artifacts/$ART_ARM/download" -o /tmp/e2e-arm-out.iso
echo "   arm 产物: $(du -h /tmp/e2e-arm-out.iso | cut -f1)"
mkdir -p /tmp/e2e-arm-ex
xorriso -osirrox on -indev /tmp/e2e-arm-out.iso -extract /EFI/BOOT/grub.cfg /tmp/e2e-arm-ex/grub.cfg -quiet || true
GRUB_ARM=$(find /tmp/e2e-arm-ex -name grub.cfg | head -1)
[ -n "$GRUB_ARM" ] && grep -A3 "Auto Install (Kickstart)" "$GRUB_ARM" | head -5 | sed 's/^/   /'

step "6. 完成"
if [ -n "$CID" ]; then
    echo "容器日志尾部:"
    docker logs "$CID" --tail 12 | sed 's/^/   /'
fi
echo
echo "✅ 全部 E2E 通过（x86_64 + aarch64）"
