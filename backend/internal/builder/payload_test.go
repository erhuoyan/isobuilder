package builder

import (
	"strings"
	"testing"
)

// TestRenderPayloadSnippet pins the optional U-disk payload block:
//   - off  → 只有一行注释，不含任何挂载逻辑（可选项必须真的"什么都不做"）
//   - label→ 只按卷标查找，不含兜底扫描
//   - auto → 含兜底扫描，且扫描时排除目标磁盘
//   - 任何模式下都不能出现以 % 开头的行（kickstart 会把它当段指令）
func TestRenderPayloadSnippet(t *testing.T) {
	off := renderPayloadSnippet("off")
	if !strings.HasPrefix(strings.TrimSpace(off), "#") {
		t.Fatalf("off 应生成注释, 得到 %q", off)
	}
	if strings.Contains(off, "mount") {
		t.Fatal("off 不应包含任何挂载逻辑")
	}

	label := renderPayloadSnippet("label")
	for _, want := range []string{`PAYLOAD_LABEL="ISO_PAYLOAD"`, "mount -o ro", "payload_try", "/mnt/sysimage/opt/payload"} {
		if !strings.Contains(label, want) {
			t.Fatalf("label 模式缺少 %q", want)
		}
	}
	// FAT32/exFAT 的卷标字段固定 11 字节：超过就会被格式化工具截断或拒绝，
	// blkid -L 永远匹配不到，payload 静默不拷。这里锁死上限防回归。
	if n := len("ISO_PAYLOAD"); n > 11 {
		t.Fatalf("卷标 %q 有 %d 字节，超过 FAT32/exFAT 的 11 字节上限", "ISO_PAYLOAD", n)
	}
	// 挂载前必须预热文件系统模块：安装器 initrd 未必已加载 exfat/vfat，
	// 没有 modprobe 的话 exFAT/FAT32 分区会静默挂载失败（payload 看似没拷上）。
	for _, want := range []string{"modprobe exfat", "modprobe vfat"} {
		if !strings.Contains(label, want) {
			t.Fatalf("label 模式缺少 %q（挂载前需预热内核模块）", want)
		}
	}
	if strings.Contains(label, "PAYLOAD_TARGET") {
		t.Fatal("label 模式不应包含兜底扫描")
	}

	auto := renderPayloadSnippet("auto")
	for _, want := range []string{"PAYLOAD_TARGET", "/tmp/ks-pre.log", "/dev/nvme*p[0-9]"} {
		if !strings.Contains(auto, want) {
			t.Fatalf("auto 模式缺少 %q", want)
		}
	}

	// 未知模式一律按 off 处理，避免拼错配置就把代码注进去。
	if got := renderPayloadSnippet("bogus"); got != off {
		t.Fatalf("未知模式应等价于 off, 得到 %q", got)
	}

	for mode, body := range map[string]string{"off": off, "label": label, "auto": auto} {
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "%") {
				t.Fatalf("%s 模式生成了以 %% 开头的行（会破坏 kickstart）: %q", mode, line)
			}
		}
	}

	// 兜底扫描必须排除目标磁盘，否则会误挂正在安装的盘。
	if !strings.Contains(auto, `*"$PAYLOAD_TARGET"*`) {
		t.Fatal("兜底扫描缺少目标盘排除逻辑")
	}
}
