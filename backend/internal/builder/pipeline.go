package builder

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"isobuilder/internal/buildlog"
	"isobuilder/internal/diskutil"
	"isobuilder/internal/model"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
)

// tool paths, resolved once at start-up by CheckTools.
var (
	binXorriso = "xorriso"
	binMkisofs = "xorriso" // invoked as `xorriso -as mkisofs`
	binMd5     = "implantisomd5"
	binHybrid  = "isohybrid"
)

// RequiredTools is the set of external programs the pipeline needs. syslinux
// tools are only needed for x86_64 images, so they are reported separately.
func RequiredTools(arch string) []string {
	common := []string{"xorriso", "implantisomd5"}
	if arch == model.ArchX86 {
		return append(common, "isohybrid")
	}
	return common
}

// MissingTools reports which required binaries are absent from PATH.
func MissingTools(arch string) []string {
	var missing []string
	for _, name := range RequiredTools(arch) {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if arch == model.ArchX86 {
		if _, err := findIsohdpx(); err != nil {
			missing = append(missing, "isohdpfx.bin (syslinux)")
		}
	}
	return missing
}

// IsohdpxPaths are the candidate locations of the isolinux MBR template.
var IsohdpxPaths = []string{
	"/usr/share/syslinux/isohdpfx.bin",
	"/usr/lib/syslinux/isohdpfx.bin",
	"/usr/lib/ISOLINUX/isohdpfx.bin",
}

func findIsohdpx() (string, error) {
	for _, p := range IsohdpxPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("isohdpfx.bin not found")
}

// run performs one pipeline invocation for a queued build.
func (m *Manager) run(ctx context.Context, b model.Build, logFile *buildlog.File) error {
	pub := &publisher{mgr: m, buildID: b.ID, log: logFile}
	if err := m.store.StartBuild(b.ID); err != nil {
		return fmt.Errorf("mark build running: %w", err)
	}

	workDir := filepath.Join(m.cfg.WorkDir, "iso-repack-"+b.ID)
	if err := m.store.SetBuildWorkDir(b.ID, workDir); err != nil {
		return fmt.Errorf("record work dir: %w", err)
	}
	defer m.cleanup(workDir)

	pub.logf("=== ISO 重打包开始 ===")
	pub.logf("构建 ID:  %s", b.ID)
	pub.logf("架构:     %s", b.Arch)
	pub.logf("基础镜像: %s", b.IsoPath)
	pub.logf("kickstart: %s", b.KSID)
	pub.logf("输出:     %s", b.OutputName)
	pub.logf("工作目录: %s", workDir)
	pub.logf("CPU 核数: %d", runtime.NumCPU())

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	treeDir := filepath.Join(workDir, "new")

	isoInfo, err := inspectISO(ctx, pub, b.IsoPath)
	if err != nil {
		return err
	}
	label := b.IsoLabel
	if label == "" {
		label = isoInfo.Label
	}
	if label == "" {
		return errors.New("无法自动识别 ISO 卷标（Volume id），请在项目高级设置里手动填写 iso_label")
	}
	if len(label) > 32 {
		return fmt.Errorf("iso_label %q 超过 32 个字符（ISO9660 限制）", label)
	}
	if label != b.IsoLabel {
		pub.logf("卷标自动识别: %s", label)
	}

	// --- stage 1: extract -------------------------------------------------
	pub.stageAt(1, "mount")
	if err := m.ensureDiskSpace(pub, b.IsoPath); err != nil {
		return err
	}
	if err := os.MkdirAll(treeDir, 0o755); err != nil {
		return fmt.Errorf("create tree dir: %w", err)
	}
	if err := extractISO(ctx, pub, b.IsoPath, treeDir); err != nil {
		return err
	}
	if err := makeWritable(treeDir); err != nil {
		return err
	}
	pub.progress(1)

	// --- stage 2: inject --------------------------------------------------
	pub.stageAt(2, "inject")
	ksRel, err := m.injectKickstart(pub, treeDir, b)
	if err != nil {
		return err
	}
	pub.progress(1)

	// --- stage 3: boot config --------------------------------------------
	pub.stageAt(3, "boot")
	if err := patchBootConfig(pub, treeDir, b.Arch, label, ksRel); err != nil {
		return err
	}
	pub.progress(1)

	// --- stage 4: repack --------------------------------------------------
	pub.stageAt(4, "build")
	outputPath := filepath.Join(workDir, b.OutputName)
	implanted, err := m.repack(ctx, pub, treeDir, outputPath, b.Arch, label)
	if err != nil {
		return err
	}
	pub.progress(1)

	// --- stage 5: checksum + publish -------------------------------------
	pub.stageAt(5, "seal")
	size, sum, err := hashFile(ctx, pub, outputPath)
	_ = implanted
	if err != nil {
		return err
	}
	artifact, err := m.publish(ctx, pub, b, outputPath, size, sum, implanted)
	if err != nil {
		return err
	}
	if err := m.store.SetBuildArtifact(b.ID, artifact.ID); err != nil {
		return fmt.Errorf("link artifact: %w", err)
	}
	pub.progress(1)

	_ = m.store.FinishBuild(b.ID, model.BuildSucceeded, "", 100)
	pub.logf("")
	pub.logf("镜像: %s（%s）", artifact.Filename, humanBytes(artifact.SizeBytes))
	pub.logf("SHA256: %s", artifact.SHA256)
	if artifact.URL != "" {
		pub.logf("下载地址: %s", artifact.URL)
	} else {
		pub.logf("下载地址: /api/artifacts/%s/download（对象存储未配置，本机提供下载）", artifact.ID)
	}
	return nil
}

// diskSlackBytes is headroom on top of the estimated peak. The estimate covers
// the extracted tree plus the rewritten image; slack absorbs filesystem
// metadata, libisofs alignment blocks and log growth.
const diskSlackBytes = 1 << 30

// neededDiskBytes is the transient space one build of a given image needs. Both
// the extracted tree and the rewritten ISO live under the data directory at the
// same time, so the peak is about twice the base image.
func neededDiskBytes(isoSize int64) int64 {
	if isoSize <= 0 {
		return diskSlackBytes
	}
	return isoSize*2 + diskSlackBytes
}

// ensureDiskSpace fails a build before it writes anything, with numbers the
// operator can act on. Container deployments with a small volume hit this
// immediately rather than halfway through an extraction.
func (m *Manager) ensureDiskSpace(pub *publisher, isoPath string) error {
	fi, err := os.Stat(isoPath)
	if err != nil {
		return fmt.Errorf("读取基础镜像大小失败: %w", err)
	}
	need := neededDiskBytes(fi.Size())
	free, err := diskutil.FreeBytes(m.cfg.WorkDir)
	if err != nil {
		pub.logf("警告: 无法读取可用空间，跳过磁盘预检: %v", err)
		return nil
	}
	pub.logf("磁盘预检: 需要约 %s（基础镜像 %s ×2），可用 %s（%s）",
		humanBytes(need), humanBytes(fi.Size()), humanBytes(free), m.cfg.WorkDir)
	return checkDiskSpace(fi.Size(), need, free, m.cfg.DataDir)
}

// checkDiskSpace is the pure decision the guard wraps, kept separate so it can
// be exercised without statfs.
func checkDiskSpace(isoSize, need, free int64, dir string) error {
	if free >= need {
		return nil
	}
	return fmt.Errorf("磁盘空间不足：基础镜像 %s，构建峰值约需 %s，当前可用 %s。"+
		"请清理 %s（工作目录与旧产物）或扩容后重试；"+
		"产物若已在对象存储，可安全删除服务器上的本地副本",
		humanBytes(isoSize), humanBytes(need), humanBytes(free), dir)
}

// injectKickstart renders the kickstart with the host inventory embedded and
// returns the in-ISO path of the rendered file.
func (m *Manager) injectKickstart(pub *publisher, treeDir string, b model.Build) (string, error) {
	ksSrc, err := m.store.GetKickstart(b.KSID)
	if err != nil {
		return "", fmt.Errorf("读取 kickstart 模板失败: %w", err)
	}
	body, err := os.ReadFile(ksSrc.Path)
	if err != nil {
		return "", fmt.Errorf("读取 kickstart 文件 %s 失败: %w", ksSrc.Path, err)
	}
	text := string(body)

	// The host inventory is injected as JSON. It is spliced by literal
	// replacement, not by shell substitution, so no character in the JSON can
	// corrupt the template.
	const placeholder = "%%HOSTS_JSON_PLACEHOLDER%%"
	if !strings.Contains(text, placeholder) {
		return "", fmt.Errorf("kickstart 模板 %s 缺少 %s 占位符，无法注入主机清单", ksSrc.Name, placeholder)
	}
	rendered := strings.ReplaceAll(text, placeholder, b.HostsJSON)

	// U 盘 payload 注入（可选）：模板带占位符时按 payload_mode 替换。
	// 与 hosts 占位符不同，这个是可选的 —— 老模板没有它就跳过；
	// 有它就一定替换（off 也会替换成一行注释，避免 %%…%% 残留在脚本里）。
	const payloadPlaceholder = "%%PAYLOAD_SNIPPET%%"
	if strings.Contains(rendered, payloadPlaceholder) {
		rendered = strings.ReplaceAll(rendered, payloadPlaceholder, renderPayloadSnippet(b.PayloadMode))
		if b.PayloadMode != "" && b.PayloadMode != "off" {
			pub.logf("已注入 U 盘 payload 拷贝逻辑（payload_mode=%s）", b.PayloadMode)
		}
	}


	// A project-level post script is appended as an independent %post section.
	// The script body is base64-encoded so no character in it (%, quotes,
	// heredocs of its own) can break the kickstart syntax. It runs after every
	// %post in the template, i.e. once network config is already written.
	if strings.TrimSpace(b.PostScript) != "" {
		rendered += renderPostScript(b.PostScript)
		pub.logf("已追加装机后脚本（%s）", humanBytes(int64(len(b.PostScript))))
	}

	ksRel := "ks/" + filepath.Base(ksSrc.Name)
	dest := filepath.Join(treeDir, filepath.FromSlash(ksRel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, []byte(rendered), 0o644); err != nil {
		return "", fmt.Errorf("写入 %s 失败: %w", ksRel, err)
	}

	var doc struct {
		Hosts []json.RawMessage `json:"hosts"`
	}
	_ = json.Unmarshal([]byte(b.HostsJSON), &doc)
	pub.logf("已注入 kickstart: /%s（%d 台主机，%s 字节）", ksRel, len(doc.Hosts), humanBytes(int64(len(rendered))))
	return ksRel, nil
}

// renderPostScript wraps an operator-supplied shell script into a chrooted
// %post section. The script is stored under /var/log/isobuilder/ on the
// installed system together with its log and exit code, so an operator can
// audit what ran long after the install console is gone. A non-zero exit is
// logged but never fails the install: the OS is already fully installed at
// this point.
func renderPostScript(script string) string {
	return "\n\n%post --interpreter=/bin/bash\n" +
		"# --- isobuilder: project post-install script ---\n" +
		"mkdir -p /var/log/isobuilder\n" +
		"base64 -d << 'ISOBUILDER_POST_EOF' > /var/log/isobuilder/post.sh\n" +
		base64.StdEncoding.EncodeToString([]byte(script)) + "\n" +
		"ISOBUILDER_POST_EOF\n" +
		"chmod +x /var/log/isobuilder/post.sh\n" +
		"bash /var/log/isobuilder/post.sh 2>&1 | tee -a /var/log/isobuilder/post.log\n" +
		"echo ${PIPESTATUS[0]} > /var/log/isobuilder/post.exit\n" +
		"%end\n"
}

// renderPayloadSnippet builds the optional "copy files from the USB stick"
// block that the template calls from its %post --nochroot section (the only
// phase where removable media is still visible to the installer).
//
// Payload must live on a *separate* USB partition: Ventoy's own data partition
// cannot be mounted after boot (ventoy/Ventoy#667), so the recommended setup is
// a dedicated partition carved from Ventoy's reserved space, labelled
// ISO_PAYLOAD with a `payload/` directory inside.
//
// Everything is best-effort by design: no partition, an unmountable filesystem
// or a failed copy must never fail the installation. The whole feature is
// opt-in (payload_mode) so a site that does not want it leaves no code behind.
func renderPayloadSnippet(mode string) string {
	switch mode {
	case "label":
		return payloadScript(false)
	case "auto":
		return payloadScript(true)
	default:
		return "# payload 注入未启用（payload_mode=off）"
	}
}

func payloadScript(scan bool) string {
	const header = `# 卷标必须是 ISO_PAYLOAD（11 字符）：FAT32/exFAT 的卷标字段固定 11 字节，
# 更长的名字在 Windows/macOS 格式化工具里会被截断或拒绝，blkid -L 就匹配不到。
# 这也是它不能叫 ISOBUILDER_PAYLOAD（18 字符）的原因。
PAYLOAD_LABEL="ISO_PAYLOAD"
PAYLOAD_DIR="payload"
PAYLOAD_DEST="/mnt/sysimage/opt/payload"
PAYLOAD_MP=/tmp/isobuilder-usb
PAYLOAD_LOG=/mnt/sysimage/var/log/pxe-install.log
mkdir -p "$PAYLOAD_MP" 2>/dev/null
# 预热文件系统模块：Rocky 9（5.14+ 原生）与 openEuler 22.03（exfat backport）
# 都编了 exfat.ko，但安装器 initrd 未必已加载；挂载前 modprobe 一次即可，
# 无需安装任何包（%packages 装的包进的是目标系统，对当前安装环境无效）。
# vfat 兜底 FAT32；万一 exfat 模块不在 initrd 里，FAT32 分区仍可挂载。
modprobe exfat 2>/dev/null
modprobe vfat 2>/dev/null

# 只读挂载，绝不在 U 盘上写任何东西；任何一步失败都只影响这一步。
payload_try() {
    mount -o ro "$1" "$PAYLOAD_MP" 2>/dev/null || return 1
    if [ -d "$PAYLOAD_MP/$PAYLOAD_DIR" ]; then
        mkdir -p "$PAYLOAD_DEST" 2>/dev/null
        cp -a "$PAYLOAD_MP/$PAYLOAD_DIR/." "$PAYLOAD_DEST/" 2>/dev/null
        umount "$PAYLOAD_MP" 2>/dev/null
        return 0
    fi
    umount "$PAYLOAD_MP" 2>/dev/null
    return 1
}

PAYLOAD_DONE=0
PAYLOAD_DEV=$(blkid -L "$PAYLOAD_LABEL" 2>/dev/null || true)
if [ -n "$PAYLOAD_DEV" ]; then
    payload_try "$PAYLOAD_DEV" && PAYLOAD_DONE=1
fi
`

	const fallback = `
# 兜底扫描：卷标未命中时遍历分区找含 payload/ 的，排除目标磁盘。
# 目标磁盘名由 %pre 写入 /tmp/ks-pre.log，避免误挂正在安装的盘。
if [ "$PAYLOAD_DONE" = "0" ]; then
    PAYLOAD_TARGET=$(sed -n 's/^disk=//p' /tmp/ks-pre.log 2>/dev/null | tail -1)
    for PAYLOAD_PART in /dev/sd*[0-9] /dev/vd*[0-9] /dev/nvme*p[0-9]; do
        [ -b "$PAYLOAD_PART" ] || continue
        case "$PAYLOAD_PART" in
            *"$PAYLOAD_TARGET"*) continue ;;
        esac
        if payload_try "$PAYLOAD_PART"; then
            PAYLOAD_DONE=1
            PAYLOAD_DEV="$PAYLOAD_PART"
            break
        fi
    done
fi
`

	const footer = `
if [ "$PAYLOAD_DONE" = "1" ]; then
    echo "payload: copied from $PAYLOAD_DEV to $PAYLOAD_DEST" >> "$PAYLOAD_LOG" 2>/dev/null
else
    echo "payload: no payload partition (label=$PAYLOAD_LABEL)" >> "$PAYLOAD_LOG" 2>/dev/null
fi
`

	if scan {
		return header + fallback + footer
	}
	return header + footer
}

// inspectISO reads the volume label of a base image with xorriso, sparing the
// image a second ISO toolkit dependency (isoinfo lives in EPEL on RHEL 9).
type isoInfo struct {
	Label string
}

var volumeIDRe = regexp.MustCompile(`(?m)^Volume Id\s*:\s*(.+?)\s*$`)

func inspectISO(ctx context.Context, pub *publisher, isoPath string) (isoInfo, error) {
	out, err := capture(ctx, pub, binXorriso, "-indev", isoPath, "-pvd_info")
	if err != nil {
		return isoInfo{}, fmt.Errorf("读取 ISO 信息失败（xorriso -pvd_info）: %w", err)
	}
	var info isoInfo
	if m := volumeIDRe.FindStringSubmatch(out); m != nil {
		info.Label = strings.TrimSpace(m[1])
	}
	return info, nil
}

// extractISO unpacks the ISO filesystem with xorriso. osirrox extraction is used
// instead of a loop mount so the pipeline runs in an unprivileged container.
func extractISO(ctx context.Context, pub *publisher, isoPath, dest string) error {
	pub.logf("解包 %s → %s", filepath.Base(isoPath), dest)
	_, err := runTool(ctx, pub, binXorriso, "-osirrox", "on", "-acl", "off",
		"-indev", isoPath, "-extract", "/", dest)
	if err != nil {
		return fmt.Errorf("解包原版 ISO 失败: %w", err)
	}
	return nil
}

// patchBootConfig rewrites grub.cfg / isolinux.cfg so the ISO boots straight
// into the kickstart install.
func patchBootConfig(pub *publisher, treeDir, arch, label, ksRel string) error {
	switch arch {
	case model.ArchARM:
		return patchGrubARMTree(pub, treeDir, label, ksRel)
	default:
		return patchX86Tree(pub, treeDir, label, ksRel)
	}
}

// patchX86Tree installs a fixed dual-mode (BIOS + EFI) boot configuration, which
// is what the validated repack-iso-x86.sh script writes.
func patchX86Tree(pub *publisher, treeDir, label, ksRel string) error {
	grubPath := filepath.Join(treeDir, "EFI/BOOT/grub.cfg")
	if _, err := os.Stat(grubPath); err != nil {
		return fmt.Errorf("原版 ISO 里没有 EFI/BOOT/grub.cfg，不是受支持的 x86_64 安装镜像")
	}

	// Rocky/CentOS use linuxefi/initrdefi, openEuler x86 uses linux/initrd.
	efiCmd, initrdCmd := "linuxefi", "initrdefi"
	if body, err := os.ReadFile(grubPath); err == nil {
		text := string(body)
		if regexp.MustCompile(`(?m)^\s*linux\s`).MatchString(text) && !strings.Contains(text, "linuxefi") {
			efiCmd, initrdCmd = "linux", "initrd"
			pub.logf("检测到 openEuler 风格 grub 命令（linux/initrd）")
		} else {
			pub.logf("检测到 Rocky/CentOS 风格 grub 命令（linuxefi/initrdefi）")
		}
	}

	grub := fmt.Sprintf(`set default="0"
set timeout=10
function load_video { insmod efi_gop; insmod efi_uga; insmod video_bochs; insmod video_cirrus; insmod all_video; }
load_video
set gfxpayload=keep
insmod gzio
insmod part_gpt
insmod ext2
search --no-floppy --set=root -l '%s'

menuentry 'Auto Install (Kickstart)' --class fedora --class gnu-linux --class gnu --class os {
    %s /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=%s inst.ks=cdrom:/%s ro inst.geoloc=0 console=tty0
    %s /images/pxeboot/initrd.img
}
menuentry 'Install (manual)' --class fedora --class gnu-linux --class gnu --class os {
    %s /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=%s ro inst.geoloc=0 console=tty0
    %s /images/pxeboot/initrd.img
}
menuentry 'Rescue' --class fedora --class gnu-linux --class gnu --class os {
    %s /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=%s inst.rescue console=tty0
    %s /images/pxeboot/initrd.img
}
`, label, efiCmd, label, ksRel, initrdCmd, efiCmd, label, initrdCmd, efiCmd, label, initrdCmd)
	if err := os.WriteFile(grubPath, []byte(grub), 0o644); err != nil {
		return fmt.Errorf("写入 EFI/BOOT/grub.cfg 失败: %w", err)
	}
	pub.logf("已改写 /EFI/BOOT/grub.cfg（Auto Install 为默认项，10 秒超时）")

	isolinuxPath := filepath.Join(treeDir, "isolinux/isolinux.cfg")
	if _, err := os.Stat(isolinuxPath); err == nil {
		isolinux := fmt.Sprintf(`default auto
timeout 100

label auto
  menu label ^Auto Install (Kickstart)
  kernel /images/pxeboot/vmlinuz
  append initrd=/images/pxeboot/initrd.img inst.stage2=hd:LABEL=%s inst.ks=cdrom:/%s ro inst.geoloc=0 console=tty0

label manual
  menu label ^Install (manual)
  kernel /images/pxeboot/vmlinuz
  append initrd=/images/pxeboot/initrd.img inst.stage2=hd:LABEL=%s ro inst.geoloc=0 console=tty0

label rescue
  menu label ^Rescue
  kernel /images/pxeboot/vmlinuz
  append initrd=/images/pxeboot/initrd.img inst.stage2=hd:LABEL=%s inst.rescue console=tty0
`, label, ksRel, label, label)
		if err := os.WriteFile(isolinuxPath, []byte(isolinux), 0o644); err != nil {
			return fmt.Errorf("写入 isolinux/isolinux.cfg 失败: %w", err)
		}
		pub.logf("已改写 /isolinux/isolinux.cfg（BIOS 段）")
	} else {
		pub.logf("原版 ISO 无 isolinux 段，跳过 BIOS 菜单改写")
	}
	return nil
}

// patchGrubARMTree inserts an auto-install entry into the existing grub.cfg,
// preserving the vendor menu (openEuler/麒麟 arm images ship a maintained one).
func patchGrubARMTree(pub *publisher, treeDir, label, ksRel string) error {
	grubPath := filepath.Join(treeDir, "EFI/BOOT/grub.cfg")
	body, err := os.ReadFile(grubPath)
	if err != nil {
		return fmt.Errorf("原版 ISO 里没有 EFI/BOOT/grub.cfg，不是受支持的 aarch64 安装镜像")
	}
	text := string(body)

	// Reuse the kernel arguments of the original first entry (inst.stage2 ...).
	stage2 := regexp.MustCompile(`linux\s+.*?inst\.stage2=[^\s]*([^\n]*)`).FindStringSubmatch(text)
	extra := ""
	if stage2 != nil {
		extra = strings.TrimRight(stage2[1], " \t\r")
	}
	labelArg := strings.ReplaceAll(label, " ", "\\x20")

	entry := fmt.Sprintf(`menuentry 'Auto Install (Kickstart)' --class red --class gnu-linux --class os {
        linux /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=%s inst.ks=hd:LABEL=%s:/%s%s
        initrd /images/pxeboot/initrd.img
}`, labelArg, labelArg, ksRel, extra)

	const anchor = "### BEGIN /etc/grub.d/10_linux ###"
	if strings.Contains(text, anchor) {
		text = strings.Replace(text, anchor, anchor+"\n"+entry, 1)
	} else {
		text = entry + "\n\n" + text
	}
	text = regexp.MustCompile(`(?m)^set timeout=.*$`).ReplaceAllString(text, "set timeout=10")
	if regexp.MustCompile(`(?m)^set default=.*$`).MatchString(text) {
		text = regexp.MustCompile(`(?m)^set default=.*$`).ReplaceAllString(text, `set default="0"`)
	} else {
		text = "set default=\"0\"\n" + text
	}
	if err := os.WriteFile(grubPath, []byte(text), 0o644); err != nil {
		return fmt.Errorf("写入 EFI/BOOT/grub.cfg 失败: %w", err)
	}
	pub.logf("已在 /EFI/BOOT/grub.cfg 插入 Auto Install 菜单项（默认项，10 秒超时），保留原厂菜单")
	if extra == "" {
		pub.logf("警告: 未从原版 grub.cfg 提取到内核参数，装机可能缺少发行版必需参数")
	}
	return nil
}

// repack rebuilds the ISO. The argument order reproduces the findings baked into
// the shell scripts, which were validated against Inspur BMC virtual media:
//
//   - no `-c` flag: it overwrites the El Torito boot directory and makes
//     isohybrid report an invalid boot catalogue.
//   - x86 needs `-eltorito-alt-boot` to switch segments, otherwise the BIOS
//     segment is swallowed.
//   - the EFI segment must be a FAT image (/images/efiboot.img), not a bare
//     grubx64.efi: BMC virtual CD loaders only understand FAT images.
//   - implantisomd5 runs last, because isohybrid erases the checksum and strict
//     BMCs refuse to boot an image without an MD5 marker.
func (m *Manager) repack(ctx context.Context, pub *publisher, treeDir, outputPath, arch, label string) (string, error) {
	efiBoot, err := findEFIBoot(treeDir)
	if err != nil {
		return "", err
	}
	pub.logf("EFI 启动镜像: /%s", efiBoot)

	args := []string{"-as", "mkisofs", "-o", outputPath, "-V", label, "-R", "-J", "-joliet-long"}
	if arch == model.ArchX86 {
		isohdpx, err := findIsohdpx()
		if err != nil {
			return "", err
		}
		pub.logf("isolinux MBR 模板: %s", isohdpx)
		args = append(args,
			"-b", "isolinux/isolinux.bin",
			"-no-emul-boot", "-boot-load-size", "4", "-boot-info-table",
			"-eltorito-alt-boot",
			"-e", "/"+efiBoot, "-no-emul-boot", "-isohybrid-gpt-basdat",
			"-isohybrid-mbr", isohdpx,
		)
	} else {
		args = append(args,
			"-e", efiBoot, "-no-emul-boot", "-isohybrid-gpt-basdat",
		)
	}
	args = append(args, ".")

	if _, err := runToolIn(ctx, pub, treeDir, binMkisofs, args...); err != nil {
		return "", fmt.Errorf("xorriso 重建 ISO 失败: %w", err)
	}
	if fi, err := os.Stat(outputPath); err != nil || fi.Size() == 0 {
		return "", errors.New("xorriso 未产出 ISO 文件")
	}
	pub.progress(0.85)

	if arch == model.ArchX86 {
		if _, err := runTool(ctx, pub, binHybrid, "-u", outputPath); err != nil {
			pub.logf("警告: isohybrid -u 失败（%v），MBR/GPT 混合头可能不完整，BMC 可能不识别", err)
		}
	}
	implantedMD5 := ""
	if path, err := exec.LookPath(binMd5); err == nil {
		out, err := runTool(ctx, pub, path, "--force", outputPath)
		if err != nil {
			pub.logf("警告: implantisomd5 失败（%v），BMC 可能因缺少 MD5 标记拒绝启动", err)
		} else {
			// implantisomd5 prints `md5 = <digest>`; capture it for the artifact record
			if m := regexp.MustCompile(`md5\s*=\s*([0-9a-f]{32})`).FindStringSubmatch(out); m != nil {
				implantedMD5 = m[1]
				pub.logf("产物 MD5: %s", implantedMD5)
			}
		}
	} else {
		pub.logf("警告: 未安装 implantisomd5（isomd5sum 包），BMC 可能拒绝启动")
	}

	return implantedMD5, nil
}

// findEFIBoot locates the FAT EFI boot image inside an extracted ISO tree.
func findEFIBoot(treeDir string) (string, error) {
	candidates := []string{"images/efiboot.img", "boot/efiboot.img", "EFI/efiboot.img", "EFI/BOOT/efiboot.img"}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(treeDir, filepath.FromSlash(c))); err == nil {
			return c, nil
		}
	}
	for _, pattern := range []string{"images/*.img", "*.img", "EFI/*.img"} {
		matches, _ := filepath.Glob(filepath.Join(treeDir, pattern))
		for _, match := range matches {
			if strings.Contains(filepath.Base(match), "efi") {
				rel, err := filepath.Rel(treeDir, match)
				if err == nil {
					return filepath.ToSlash(rel), nil
				}
			}
		}
	}
	return "", errors.New("原版 ISO 里找不到 EFI 启动镜像（efiboot.img），无法生成 EFI 引导段")
}

// hashFile computes the SHA-256 of the rebuilt image, logging progress.
func hashFile(ctx context.Context, pub *publisher, path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", fmt.Errorf("打开产物失败: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0, "", err
	}
	pub.logf("计算 SHA256（%s）…", humanBytes(fi.Size()))

	h := sha256.New()
	buf := make([]byte, 8<<20)
	var done int64
	lastLog := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return 0, "", err
		}
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			done += int64(n)
			if time.Since(lastLog) > 10*time.Second {
				lastLog = time.Now()
				pub.logf("  已校验 %s / %s", humanBytes(done), humanBytes(fi.Size()))
				pub.progress(0.15 + 0.35*float64(done)/float64(fi.Size()))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, "", fmt.Errorf("读取产物失败: %w", err)
		}
	}
	sum := hex.EncodeToString(h.Sum(nil))
	return fi.Size(), sum, nil
}

// publish registers the artifact, uploads it when object storage is configured
// and records the download URL.
func (m *Manager) publish(ctx context.Context, pub *publisher, b model.Build, outputPath string, size int64, sum, md5 string) (model.Artifact, error) {
	artifact := model.Artifact{
		ID:        store.NewID(),
		BuildID:   b.ID,
		Filename:  filepath.Base(outputPath),
		SizeBytes: size,
		SHA256:    sum,
		MD5:       md5,
		CreatedAt: store.Now(),
	}

	localPath := filepath.Join(m.cfg.ArtifactDir, artifact.ID+"-"+artifact.Filename)
	if err := moveFile(outputPath, localPath); err != nil {
		return artifact, fmt.Errorf("归档产物失败: %w", err)
	}
	artifact.LocalPath = localPath

	if m.blob.Kind() == "s3" {
		key := storage.ObjectKey(m.cfg.Storage.Prefix, b.ID, artifact.Filename)
		pub.logf("上传对象存储 s3://%s/%s …", m.blob.Bucket(), key)
		lastLog := time.Now()
		uploadErr := uploadWithRetry(ctx, pub, m.blob, localPath, key, size, &lastLog)
		if uploadErr != nil {
			pub.logf("产物保留在服务器: %s", localPath)
			return artifact, uploadErr
		}
		artifact.Storage = "s3"
		artifact.StorageKey = key
		url, expires, err := m.blob.URL(ctx, key)
		if err != nil {
			pub.logf("产物已上传，但生成下载链接失败: %v", err)
			return artifact, fmt.Errorf("生成下载链接失败: %w", err)
		}
		artifact.URL = url
		if !expires.IsZero() {
			artifact.URLExpiresAt = expires.UTC().Format(time.RFC3339)
		}
		if !m.cfg.KeepLocalArtifact {
			if err := os.Remove(localPath); err == nil {
				artifact.LocalPath = ""
			}
		}
	} else {
		artifact.Storage = "local"
		pub.logf("对象存储未配置，产物保存在服务器: %s", localPath)
	}

	if err := m.store.CreateArtifact(artifact); err != nil {
		return artifact, fmt.Errorf("登记产物失败: %w", err)
	}
	return artifact, nil
}

func uploadWithRetry(ctx context.Context, pub *publisher, blob storage.Backend, localPath, key string, size int64, lastLog *time.Time) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		err = blob.Put(ctx, localPath, key, func(sent int64) {
			if time.Since(*lastLog) < 10*time.Second {
				return
			}
			*lastLog = time.Now()
			percent := 0
			if size > 0 {
				percent = int(float64(sent) * 100 / float64(size))
			}
			pub.logf("  已上传 %s / %s（%d%%）", humanBytes(sent), humanBytes(size), percent)
			pub.progress(0.6 + 0.4*float64(sent)/float64(size))
		})
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		pub.logf("上传失败（第 %d 次）: %v", attempt, err)
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 5 * time.Second):
			}
		}
	}
	return fmt.Errorf("上传对象存储失败（已重试 3 次）: %w", err)
}

// --- process helpers ----------------------------------------------------

// runTool runs a command, streaming its output into the build log.
func runTool(ctx context.Context, pub *publisher, name string, args ...string) (string, error) {
	return runToolIn(ctx, pub, "", name, args...)
}

func runToolIn(ctx context.Context, pub *publisher, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	// Kill the whole process group so a canceled build cannot leave a child
	// mid-write on the output file.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return cmd.Process.Kill()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	pub.logf("$ %s %s", name, strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		return "", err
	}

	var (
		mu  sync.Mutex
		buf strings.Builder
		wg  sync.WaitGroup
	)
	collect := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			buf.WriteString(line)
			buf.WriteByte('\n')
			mu.Unlock()
			pub.logf("  %s", line)
		}
	}
	wg.Add(2)
	go collect(stdout)
	go collect(stderr)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var runErr error
	select {
	case runErr = <-done:
	case <-ctx.Done():
		runErr = ctx.Err()
		<-done
	}
	wg.Wait()
	_ = stdout.Close()
	_ = stderr.Close()

	mu.Lock()
	output := buf.String()
	mu.Unlock()

	if runErr != nil {
		return output, fmt.Errorf("%s 退出异常: %w", name, runErr)
	}
	return output, nil
}

// capture runs a command and returns its combined output without logging it.
func capture(ctx context.Context, pub *publisher, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		pub.logf("$ %s %s → %v", name, strings.Join(args, " "), err)
		return string(out), fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}

// makeWritable restores write permission on the extracted tree: xorriso writes
// read-only files because the source ISO was mastered read-only.
func makeWritable(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if d.IsDir() {
			mode = 0o755
		}
		return os.Chmod(path, mode)
	})
}

// moveFile renames a file, falling back to a copy across filesystems.
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

// humanBytes renders a byte count in the units an operator reads quickly.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	value := float64(n)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.2f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.2f PB", value/unit)
}
