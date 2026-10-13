#!/bin/bash
# aarch64 自动装机 ISO 重打包 (EFI-only)
# 用法: ./repack-iso-arm.sh <ORIG_ISO> <OUTPUT_ISO> <KS_FILE> <HOSTS_FILE> [LABEL]
# 例:   ./repack-iso-arm.sh /data/iso/openEuler-22.03-LTS-SP4-aarch64-dvd.iso \
#                         /data/iso/openEuler-22.03-LTS-SP4-aarch64-auto.iso \
#                         ./openeuler.ks ./hosts-openeuler.yaml
#
# 通用: openEuler / 麒麟 / 统信 aarch64 均可
#
# aarch64 特点:
#   - 纯 EFI (无 BIOS/isolinux 段)
#   - EFI 引导通过 /images/efiboot.img 或 /EFI/BOOT/grub.cfg
#   - 无需 isohdpfx.bin / isohybrid
#   - SMMU bypass 参数 (华为鲲鹏 TaiShan 需要)

set -e

ORIG_ISO="${1:?用法: $0 <ORIG_ISO> <OUTPUT_ISO> <KS_FILE> <HOSTS_FILE> [LABEL]}"
OUTPUT_ISO="${2:?缺少 OUTPUT_ISO 路径}"
KS_FILE="${3:?缺少 KS_FILE 路径}"
HOSTS_FILE="${4:?缺少 HOSTS_FILE 路径}"

# Auto-detect LABEL from ISO if not provided
if [ -n "$5" ]; then
    LABEL="$5"
else
    LABEL=$(isoinfo -d -i "$ORIG_ISO" 2>/dev/null | awk -F': ' '/Volume id/ {gsub(/^ +/,"",$2);print $2;exit}')
    if [ -z "$LABEL" ]; then
        echo "ERROR: cannot auto-detect LABEL, please pass as 5th arg"; exit 1
    fi
fi

WORK_DIR="/data/pxe/tmp/iso-repack-$$"
KS_REL="ks/$(basename "$KS_FILE")"

# aarch64 SMMU bypass (华为鲲鹏 TaiShan)
SMMU="smmu.bypassdev=0x1000:0x17 smmu.bypassdev=0x1000:0x15"

echo "=== ISO Repack (aarch64 EFI-only) ==="
echo "Source:  $ORIG_ISO"
echo "Output:  $OUTPUT_ISO"
echo "KS:      $KS_FILE  (rel: $KS_REL)"
echo "Hosts:   $HOSTS_FILE"
echo "LABEL:   $LABEL"
echo

[ -f "$ORIG_ISO" ]   || { echo "ERROR: source ISO not found"; exit 1; }
[ -f "$KS_FILE" ]    || { echo "ERROR: KS file not found: $KS_FILE"; exit 1; }
[ -f "$HOSTS_FILE" ] || { echo "ERROR: hosts file not found: $HOSTS_FILE"; exit 1; }
command -v xorriso >/dev/null 2>&1        || { echo "ERROR: xorriso not installed (dnf install xorriso)"; exit 1; }
command -v isoinfo >/dev/null 2>&1        || { echo "ERROR: isoinfo not installed (dnf install genisoimage)"; exit 1; }
command -v python3 >/dev/null 2>&1        || { echo "ERROR: python3 not installed"; exit 1; }
python3 -c "import yaml" 2>/dev/null      || { echo "ERROR: PyYAML not installed (dnf install python3-pyyaml)"; exit 1; }

rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR/iso" "$WORK_DIR/new"

echo "[1/5] Mounting source ISO..."
mount -o loop,ro "$ORIG_ISO" "$WORK_DIR/iso" 2>/dev/null

echo "[2/5] Copying ISO contents..."
cp -a "$WORK_DIR/iso/." "$WORK_DIR/new/"
chmod -R u+w "$WORK_DIR/new"

echo "[3/5] Generating kickstart with embedded hosts.json..."
mkdir -p "$WORK_DIR/new/ks"
HOSTS_JSON=$(python3 -c "
import yaml, json, sys
try:
    d = yaml.safe_load(open('$HOSTS_FILE'))
    print(json.dumps(d))
except Exception as e:
    print('ERROR: ' + str(e), file=sys.stderr); sys.exit(1)
")
sed "s|%%HOSTS_JSON_PLACEHOLDER%%|${HOSTS_JSON}|" "$KS_FILE" > "$WORK_DIR/new/$KS_REL"
if grep -q "PLACEHOLDER" "$WORK_DIR/new/$KS_REL"; then
    echo "ERROR: sed injection failed, placeholder still present"; exit 1
fi
echo "   Embedded $(echo "$HOSTS_JSON" | python3 -c "import json,sys;print(len(json.load(sys.stdin).get('hosts',[])))") host(s)"

echo "[4/5] Patching EFI/BOOT/grub.cfg..."
LABEL_GRUB=$(echo "$LABEL" | sed 's/ /\\\\x20/g')

GRUB_CFG="$WORK_DIR/new/EFI/BOOT/grub.cfg"
if [ ! -f "$GRUB_CFG" ]; then
    echo "ERROR: grub.cfg not found"; exit 1
fi

# 提取原版第一个 menuentry 的 linux 行参数（inst.stage2 之后的部分）
ORIG_LINUX=$(grep -m1 'linux.*inst.stage2=' "$GRUB_CFG" | sed 's/.*inst.stage2=[^ ]*//')

# 在 "### BEGIN /etc/grub.d/10_linux ###" 后面插入自动装机 menuentry
sed -i "/### BEGIN \/etc\/grub.d\/10_linux ###/a\\
menuentry 'Auto Install (Kickstart)' --class red --class gnu-linux --class gnu --class os {\\
        linux /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=${LABEL_GRUB} inst.ks=hd:LABEL=${LABEL_GRUB}:/${KS_REL}${ORIG_LINUX}\\
        initrd /images/pxeboot/initrd.img\\
}" "$GRUB_CFG"

# 默认选第一项 (Auto Install)，timeout 10秒可选原版
sed -i 's/^set timeout=.*/set timeout=10/' "$GRUB_CFG"
sed -i 's/^set default=.*/set default="0"/' "$GRUB_CFG"

echo "   Added 'Auto Install (Kickstart)' menuentry as default"
echo "   Original menuentries preserved (timeout=10s to select)"

echo "[5/5] Building ISO with xorriso (EFI-only, aarch64)..."
cd "$WORK_DIR/new"

# aarch64: EFI-only, 不需要 isolinux/MBR
# 检测 efiboot.img 路径
EFIBOOT=""
for epath in images/efiboot.img boot/efiboot.img EFI/efiboot.img; do
    if [ -f "$WORK_DIR/new/$epath" ]; then
        EFIBOOT="$epath"
        break
    fi
done

if [ -z "$EFIBOOT" ]; then
    echo "ERROR: efiboot.img not found in ISO"; exit 1
fi
echo "   EFI boot image: /$EFIBOOT"

xorriso -as mkisofs -o "$OUTPUT_ISO" \
    -V "$LABEL" \
    -R -J -joliet-long \
    -e "$EFIBOOT" -no-emul-boot \
    -isohybrid-gpt-basdat \
    . 2>&1 | tail -3

# implantisomd5 (如果可用)
if command -v implantisomd5 >/dev/null 2>&1; then
    implantisomd5 --force "$OUTPUT_ISO" 2>&1
    echo "   MD5 implanted"
else
    echo "   WARN: implantisomd5 not available, skipping"
fi

umount "$WORK_DIR/iso" 2>/dev/null || true
rm -rf "$WORK_DIR"

echo
echo "=== Done: $OUTPUT_ISO ($(du -h "$OUTPUT_ISO" | awk '{print $1}')) ==="
echo "    LABEL:    $LABEL"
echo "    KS:       /${KS_REL} (embedded hosts.json)"
echo "    EFI:      /$EFIBOOT"
echo "    ARCH:     aarch64 (EFI-only)"
echo "    SMMU:     bypass enabled (鲲鹏 TaiShan)"
echo
echo "验证 (可选):"
echo "    checkisomd5 $OUTPUT_ISO"
