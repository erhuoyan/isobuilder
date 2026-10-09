#!/bin/bash
# x86_64 自动装机 ISO 重打包 (BIOS + EFI 双模式)
# 用法: ./repack-iso-x86.sh <ORIG_ISO> <OUTPUT_ISO> <KS_FILE> <HOSTS_FILE> [LABEL]
# 例:   ./repack-iso-x86.sh /data/iso/Rocky-9.8-x86_64-dvd.iso \
#                          /data/iso/Rocky-9.8-x86_64-auto.iso \
#                          ./rocky9.ks ./hosts.yaml
#
# 通用: Rocky / CentOS / openEuler x86_64 均可
# 自动检测 grub 命令风格:
#   - Rocky/CentOS 用 linuxefi/initrdefi
#   - openEuler 用 linux/initrd
#
# 关键坑 (Inspur BMC 兼容性, 实测):
#   1. xorriso -as mkisofs 不能用 -c (覆盖 El Torito 启动目录, 导致 isohybrid 报 invalid boot catalogue)
#   2. 必须加 -eltorito-alt-boot 否则 BIOS 启动段被吞
#   3. EFI 段必须用 /images/efiboot.img (FAT 镜像), 不能用单文件 grubx64.efi
#      Inspur BMC 虚拟 CD 启动器只把 EFI 段当 FAT 镜像处理
#   4. implantisomd5 必须最后跑 (isohybrid 会擦掉 MD5; BMC 严格查 MD5 标记, 无标记拒启动)
#   5. bootx64.efi 副本对 BMC 无用 (EFI 入口在 El Torito 段, 不在 fallback 路径)

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
        LABEL=$(blkid -o value -s LABEL "$ORIG_ISO" 2>/dev/null)
    fi
    if [ -z "$LABEL" ]; then
        echo "ERROR: cannot auto-detect LABEL, please pass as 5th arg"; exit 1
    fi
fi

WORK_DIR="/data/pxe/tmp/iso-repack-$$"

# Auto-detect ISOHDPFX (RH first, Ubuntu fallback)
ISOHDPFX=""
for p in /usr/share/syslinux/isohdpfx.bin /usr/lib/syslinux/isohdpfx.bin /usr/lib/ISOLINUX/isohdpfx.bin; do
    [ -f "$p" ] && { ISOHDPFX="$p"; break; }
done

KS_REL="ks/$(basename "$KS_FILE")"

echo "=== ISO Repack (x86_64 BIOS+EFI) ==="
echo "Source:  $ORIG_ISO"
echo "Output:  $OUTPUT_ISO"
echo "KS:      $KS_FILE  (rel: $KS_REL)"
echo "Hosts:   $HOSTS_FILE"
echo "LABEL:   $LABEL"
echo "ISOHDPFX: $ISOHDPFX"
echo

[ -f "$ORIG_ISO" ]   || { echo "ERROR: source ISO not found"; exit 1; }
[ -f "$KS_FILE" ]    || { echo "ERROR: KS file not found: $KS_FILE"; exit 1; }
[ -f "$HOSTS_FILE" ] || { echo "ERROR: hosts file not found: $HOSTS_FILE"; exit 1; }
[ -f "$ISOHDPFX" ]   || { echo "ERROR: isohdpfx.bin not found (install syslinux)"; exit 1; }
command -v xorriso >/dev/null 2>&1        || { echo "ERROR: xorriso not installed (dnf install xorriso)"; exit 1; }
command -v isoinfo >/dev/null 2>&1        || { echo "ERROR: isoinfo not installed (dnf install genisoimage)"; exit 1; }
command -v isohybrid >/dev/null 2>&1      || echo "WARN: isohybrid 未安装 (dnf install syslinux), 跳过 GPT 二次处理, BMC 可能不认"
command -v implantisomd5 >/dev/null 2>&1  || echo "WARN: implantisomd5 未安装 (dnf install isomd5sum), BMC 可能拒启动"
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

# Detect grub command style from original grub.cfg
SRC_GRUB="$WORK_DIR/iso/EFI/BOOT/grub.cfg"
EFI_CMD="linuxefi"
INITRD_CMD="initrdefi"
if [ -f "$SRC_GRUB" ]; then
    if grep -q '^\s*linux\s' "$SRC_GRUB" && ! grep -q 'linuxefi' "$SRC_GRUB"; then
        EFI_CMD="linux"
        INITRD_CMD="initrd"
        echo "   Detected openEuler-style grub (linux/initrd)"
    else
        echo "   Detected Rocky/CentOS-style grub (linuxefi/initrdefi)"
    fi
fi

echo "[4/5] Patching EFI/BOOT/grub.cfg + isolinux/isolinux.cfg..."
# UEFI grub.cfg
cat > "$WORK_DIR/new/EFI/BOOT/grub.cfg" << GRUBEOF
set default="0"
set timeout=10
function load_video { insmod efi_gop; insmod efi_uga; insmod video_bochs; insmod video_cirrus; insmod all_video; }
load_video
set gfxpayload=keep
insmod gzio
insmod part_gpt
insmod ext2
search --no-floppy --set=root -l '${LABEL}'

menuentry 'Auto Install (Kickstart)' --class fedora --class gnu-linux --class gnu --class os {
    ${EFI_CMD} /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=${LABEL} inst.ks=cdrom:/${KS_REL} ro inst.geoloc=0 console=tty0
    ${INITRD_CMD} /images/pxeboot/initrd.img
}
menuentry 'Install (manual)' --class fedora --class gnu-linux --class gnu --class os {
    ${EFI_CMD} /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=${LABEL} ro inst.geoloc=0 console=tty0
    ${INITRD_CMD} /images/pxeboot/initrd.img
}
menuentry 'Rescue' --class fedora --class gnu-linux --class gnu --class os {
    ${EFI_CMD} /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=${LABEL} inst.rescue console=tty0
    ${INITRD_CMD} /images/pxeboot/initrd.img
}
GRUBEOF

# BIOS isolinux.cfg
cat > "$WORK_DIR/new/isolinux/isolinux.cfg" << ISOEOF
default auto
timeout 100

label auto
  menu label ^Auto Install (Kickstart)
  kernel /images/pxeboot/vmlinuz
  append initrd=/images/pxeboot/initrd.img inst.stage2=hd:LABEL=${LABEL} inst.ks=cdrom:/${KS_REL} ro inst.geoloc=0 console=tty0

label manual
  menu label ^Install (manual)
  kernel /images/pxeboot/vmlinuz
  append initrd=/images/pxeboot/initrd.img inst.stage2=hd:LABEL=${LABEL} ro inst.geoloc=0 console=tty0

label rescue
  menu label ^Rescue
  kernel /images/pxeboot/vmlinuz
  append initrd=/images/pxeboot/initrd.img inst.stage2=hd:LABEL=${LABEL} inst.rescue console=tty0
ISOEOF

echo "[5/5] Building ISO with xorriso (BIOS+EFI)..."
cd "$WORK_DIR/new"
# 关键点:
#   - 不带 -c (会覆盖 El Torito 启动目录)
#   -eltorito-alt-boot 切段 (否则 BIOS 段被吞)
#   -e 用 efiboot.img FAT 镜像 (BMC 启动器只认 FAT 段)
xorriso -as mkisofs -o "$OUTPUT_ISO" \
    -V "$LABEL" \
    -R -J -joliet-long \
    -b isolinux/isolinux.bin \
    -no-emul-boot -boot-load-size 4 -boot-info-table \
    -eltorito-alt-boot \
    -e /images/efiboot.img -no-emul-boot -isohybrid-gpt-basdat \
    -isohybrid-mbr "$ISOHDPFX" \
    . 2>&1 | tail -3

# isohybrid 二次处理 (确保 MBR+GPT hybrid 头完整, 标准化 isohybrid 表)
if command -v isohybrid >/dev/null 2>&1; then
    isohybrid -u "$OUTPUT_ISO" 2>&1 | head -3
fi

# implantisomd5 重新注入 MD5 (isohybrid 会擦掉; BMC 严格查, 无标记拒启动)
if command -v implantisomd5 >/dev/null 2>&1; then
    implantisomd5 --force "$OUTPUT_ISO" 2>&1
fi

umount "$WORK_DIR/iso" 2>/dev/null || true
rm -rf "$WORK_DIR"

echo
echo "=== Done: $OUTPUT_ISO ($(du -h "$OUTPUT_ISO" | awk '{print $1}')) ==="
echo "    LABEL:    $LABEL"
echo "    KS:       /${KS_REL} (embedded hosts.json)"
echo "    EFI 段:   /images/efiboot.img (FAT 镜像)"
echo "    BIOS 段:  /isolinux/isolinux.bin"
echo "    GRUB:     ${EFI_CMD}/${INITRD_CMD}"
echo
echo "验证 MD5 (可选, 重算 15G ISO 需几分钟):"
echo "    checkisomd5 $OUTPUT_ISO"
