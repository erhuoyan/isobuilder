// Package hosts parses and validates host inventories (YAML or JSON) that are
// embedded into the kickstart file, and produces the canonical JSON the build
// pipeline injects.
package hosts

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"isobuilder/internal/model"
)

// Error is a single validation problem. Index is the 0-based position of the
// host in the inventory, or -1 for whole-document problems.
type Error struct {
	Index   int    `json:"index"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Result is the outcome of parsing and validating an inventory.
type Result struct {
	OK            bool                `json:"ok"`
	Format        string              `json:"format"`
	Count         int                 `json:"hosts_count"`
	Summary       []model.HostSummary `json:"hosts_summary"`
	Errors        []Error             `json:"errors"`
	CanonicalJSON string              `json:"-"`
}

const maxErrors = 200

var (
	hostnameRe  = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)
	macRe       = regexp.MustCompile(`^([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}$`)
	validFSType = map[string]bool{"xfs": true, "ext4": true, "ext3": true, "ext2": true, "btrfs": true}
	validXmit   = map[string]bool{"layer2": true, "layer2+3": true, "layer3+4": true, "encap2+3": true, "encap3+4": true}
)

// Parse validates content (YAML or JSON as indicated by format: auto, yaml, json)
// and returns the canonical `{"hosts": [...]}` JSON on success. It never returns
// a hard failure: malformed input is reported through Result.Errors.
func Parse(content, format string) Result {
	content = strings.TrimSpace(content)
	if content == "" {
		return Result{Errors: []Error{{Index: -1, Field: "", Message: "主机清单为空，至少需要 1 台"}}}
	}
	f := format
	if f == "" || f == "auto" {
		if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
			f = "json"
		} else {
			f = "yaml"
		}
	}

	var doc any
	var err error
	switch f {
	case "json":
		err = json.Unmarshal([]byte(content), &doc)
	default:
		f = "yaml"
		err = yaml.Unmarshal([]byte(content), &doc)
	}
	if err != nil {
		return Result{Format: f, Errors: []Error{{Index: -1, Field: "", Message: "解析失败：" + err.Error()}}}
	}

	raw, ok := extractHosts(doc)
	if !ok {
		return Result{Format: f, Errors: []Error{{Index: -1, Field: "hosts", Message: "顶层必须是主机数组，或包含 hosts 数组的对象"}}}
	}
	if len(raw) == 0 {
		return Result{Format: f, Errors: []Error{{Index: -1, Field: "hosts", Message: "主机清单为空，至少需要 1 台"}}}
	}

	hosts := make([]map[string]any, 0, len(raw))
	for i, item := range raw {
		m, isMap := item.(map[string]any)
		if !isMap {
			return Result{Format: f, Errors: []Error{{Index: i, Field: "", Message: fmt.Sprintf("第 %d 台主机必须是对象", i+1)}}}
		}
		hosts = append(hosts, m)
	}

	res := Result{Format: f, Count: len(hosts)}
	seenSN := map[string]int{}
	seenHostname := map[string]int{}
	summaries := make([]model.HostSummary, 0, len(hosts))
	for i, h := range hosts {
		v := &validator{errors: &res.Errors, index: i}
		s := validateHost(h, v, seenSN, seenHostname)
		if !v.failed {
			summaries = append(summaries, s)
		}
	}

	res.OK = len(res.Errors) == 0
	if !res.OK {
		return res
	}

	canonical, err := json.Marshal(map[string]any{"hosts": hosts})
	if err != nil {
		res.OK = false
		res.Errors = []Error{{Index: -1, Field: "", Message: "生成标准 JSON 失败：" + err.Error()}}
		return res
	}
	res.CanonicalJSON = string(canonical)
	res.Summary = summaries
	return res
}

// extractHosts accepts either `{"hosts": [...]}` or a bare `[...]`.
func extractHosts(doc any) ([]any, bool) {
	switch d := doc.(type) {
	case []any:
		return d, true
	case map[string]any:
		list, ok := d["hosts"]
		if !ok {
			return nil, false
		}
		items, ok := list.([]any)
		if !ok {
			return nil, false
		}
		return items, true
	default:
		return nil, false
	}
}

type validator struct {
	errors *[]Error
	index  int
	failed bool
}

func (v *validator) add(field, format string, args ...any) {
	v.failed = true
	if len(*v.errors) >= maxErrors {
		return
	}
	*v.errors = append(*v.errors, Error{Index: v.index, Field: field, Message: fmt.Sprintf(format, args...)})
}

// validateHost checks one host and returns its summary row.
func validateHost(h map[string]any, v *validator, seenSN, seenHostname map[string]int) model.HostSummary {
	var s model.HostSummary

	sn := strings.TrimSpace(text(h, "sn"))
	switch {
	case sn == "":
		v.add("sn", "缺少 sn（序列号），装机时靠它识别本机")
	default:
		if prev, dup := seenSN[sn]; dup {
			v.add("sn", "sn %s 与第 %d 台重复", sn, prev+1)
		} else {
			seenSN[sn] = v.index
		}
	}
	s.SN = sn
	// The kickstart compares SN as a string; normalise numeric YAML scalars.
	if sn != "" {
		h["sn"] = sn
	}

	hostname := strings.TrimSpace(text(h, "hostname"))
	switch {
	case hostname == "":
		v.add("hostname", "缺少 hostname")
	case !hostnameRe.MatchString(hostname):
		v.add("hostname", "hostname %q 不合法（小写字母/数字/连字符，长度 1-63，不能以连字符开头或结尾）", hostname)
	default:
		if prev, dup := seenHostname[hostname]; dup {
			v.add("hostname", "hostname %s 与第 %d 台重复", hostname, prev+1)
		} else {
			seenHostname[hostname] = v.index
		}
	}
	s.Hostname = hostname

	if pw, ok := h["root_password"]; ok {
		if _, isStr := pw.(string); !isStr {
			v.add("root_password", "root_password 必须是字符串（建议加引号）")
		}
	}
	if size, ok := h["disk_target_size"]; ok {
		if n, isInt := integer(size); !isInt || n <= 0 {
			v.add("disk_target_size", "disk_target_size 必须是正整数（GB）")
		} else {
			s.DiskGB = n
		}
	}
	if bmc := strings.TrimSpace(text(h, "bmc_ip")); bmc != "" && bmc != "<nil>" {
		if ip := net.ParseIP(bmc); ip == nil || ip.To4() == nil {
			v.add("bmc_ip", "bmc_ip %q 不是合法 IPv4 地址", bmc)
		}
		s.BMCIP = bmc
	}

	validatePartition(h, v)
	validateNetwork(h, v, &s)
	return s
}

func validatePartition(h map[string]any, v *validator) {
	raw, ok := h["partition"]
	if !ok {
		return
	}
	p, ok := raw.(map[string]any)
	if !ok {
		v.add("partition", "partition 必须是对象")
		return
	}
	for _, key := range []string{"boot_size", "efi_size"} {
		if val, ok := p[key]; ok {
			if n, isInt := integer(val); !isInt || n <= 0 {
				v.add("partition."+key, "partition.%s 必须是正整数（MB）", key)
			}
		}
	}
	if fs := strings.ToLower(strings.TrimSpace(text(p, "root_fstype"))); fs != "" && !validFSType[fs] {
		v.add("partition.root_fstype", "root_fstype %q 不支持（可用：xfs、ext4、ext3、ext2、btrfs）", fs)
	}
	if lvm, ok := p["use_lvm"]; ok {
		if _, isBool := lvm.(bool); !isBool {
			v.add("partition.use_lvm", "use_lvm 必须是布尔值")
		}
	}
}

func validateNetwork(h map[string]any, v *validator, s *model.HostSummary) {
	raw, ok := h["network"]
	if !ok {
		v.add("network", "缺少 network 配置")
		return
	}
	n, ok := raw.(map[string]any)
	if !ok {
		v.add("network", "network 必须是对象")
		return
	}

	ipStr := strings.TrimSpace(text(n, "ip"))
	var ip net.IP
	switch {
	case ipStr == "":
		v.add("network.ip", "缺少 network.ip")
	default:
		ip = net.ParseIP(ipStr)
		if ip == nil || ip.To4() == nil {
			v.add("network.ip", "network.ip %q 不是合法 IPv4 地址", ipStr)
			ip = nil
		}
	}
	s.IP = ipStr

	// netmask or prefix; prefix is canonicalised into netmask because the
	// kickstart templates only read network.netmask.
	var mask net.IPMask
	if prefixRaw, ok := n["prefix"]; ok {
		nbits, isInt := integer(prefixRaw)
		switch {
		case !isInt || nbits < 0 || nbits > 32:
			v.add("network.prefix", "network.prefix 必须是 0-32 的整数")
		default:
			mask = net.CIDRMask(nbits, 32)
			n["netmask"] = net.IP(mask).String()
		}
	}
	if mask == nil {
		maskStr := strings.TrimSpace(text(n, "netmask"))
		switch {
		case maskStr == "":
			v.add("network.netmask", "缺少 network.netmask（或改用 network.prefix）")
		default:
			m := net.ParseIP(maskStr)
			if m == nil || m.To4() == nil {
				v.add("network.netmask", "network.netmask %q 不是合法掩码", maskStr)
				break
			}
			ones, bits := net.IPMask(m.To4()).Size()
			if bits != 32 {
				v.add("network.netmask", "network.netmask %q 不是连续掩码（应为 255.255.255.0 这类值）", maskStr)
				break
			}
			mask = net.CIDRMask(ones, 32)
		}
	}

	if dnsRaw, ok := n["dns"]; ok {
		for i, item := range toList(dnsRaw) {
			text, isStr := item.(string)
			if !isStr || net.ParseIP(strings.TrimSpace(text)) == nil {
				v.add("network.dns", "network.dns[%d] 不是合法 IP 地址", i)
			}
		}
	}

	if gwStr := strings.TrimSpace(text(n, "gateway")); gwStr != "" && gwStr != "<nil>" {
		gw := net.ParseIP(gwStr)
		if gw != nil && gw.To4() != nil {
			s.Gateway = gwStr
		}
		switch {
		case gw == nil || gw.To4() == nil:
			v.add("network.gateway", "network.gateway %q 不是合法 IPv4 地址", gwStr)
		case ip != nil && mask != nil && !ip.Mask(mask).Equal(gw.Mask(mask)):
			v.add("network.gateway", "network.gateway %s 与 network.ip %s（掩码 %s）不在同一网段", gwStr, ipStr, net.IP(mask).String())
		}
	}

	// VLAN tag：可选。填了表示 IP 配在该 VLAN 子接口上，
	// 物理接口（或 bond）只做二层承载，不再持有地址。
	// vlan 留空（null / 空字符串）即表示不打标，不算错误：前端清空单元格、
	// YAML 里写成 `vlan:` 都属于这种情况。数字字符串也接受，因为 CSV/Excel
	// 与手写 YAML 都可能给出 "61" 而不是 61。
	if vlanRaw, ok := n["vlan"]; ok && vlanRaw != nil {
		raw := strings.TrimSpace(text(n, "vlan"))
		if raw != "" {
			nbits, err := strconv.Atoi(raw)
			switch {
			case err != nil || nbits < 1 || nbits > 4094:
				v.add("network.vlan", "network.vlan 必须是 1-4094 的整数，留空表示不打标")
			default:
				s.VLANID = nbits
			}
		}
	}

	mac := strings.TrimSpace(text(n, "mac"))
	bondRaw, hasBond := n["bond"]
	switch {
	case mac != "" && hasBond:
		v.add("network", "network.mac 与 network.bond 不能同时存在，二选一")
	case mac == "" && !hasBond:
		v.add("network", "缺少网络绑定方式：单口填 network.mac，双口 bond 填 network.bond.slaves")
	case mac != "":
		s.NetworkMode = "single"
		s.MACCount = 1
		if !macRe.MatchString(mac) {
			v.add("network.mac", "network.mac %q 不是合法 MAC（形如 8C:91:3A:C9:90:FE）", mac)
		}
	default:
		validateBond(bondRaw, v, s)
	}
}

func validateBond(bondRaw any, v *validator, s *model.HostSummary) {
	s.NetworkMode = "bond"
	bond, ok := bondRaw.(map[string]any)
	if !ok {
		v.add("network.bond", "network.bond 必须是对象")
		return
	}
	slaves := toList(bond["slaves"])
	switch {
	case len(slaves) == 0:
		v.add("network.bond.slaves", "bond 缺少 slaves 列表")
	case len(slaves) < 2 || len(slaves)%2 != 0:
		v.add("network.bond.slaves", "bond 需要偶数个且至少 2 个 slave，当前 %d 个", len(slaves))
	}
	s.MACCount = len(slaves)
	seen := map[string]int{}
	for i, item := range slaves {
		entry, isStr := item.(string)
		if !isStr || !macRe.MatchString(strings.TrimSpace(entry)) {
			v.add("network.bond.slaves", "network.bond.slaves[%d] 不是合法 MAC", i)
			continue
		}
		key := strings.ToLower(strings.TrimSpace(entry))
		if prev, dup := seen[key]; dup {
			v.add("network.bond.slaves", "network.bond.slaves[%d] 与 [%d] 是同一个 MAC", i, prev)
		}
		seen[key] = i
	}
	if modeRaw, ok := bond["mode"]; ok {
		if n, isInt := integer(modeRaw); !isInt || n < 0 || n > 6 {
			v.add("network.bond.mode", "bond.mode 必须是 0-6（LACP 用 4）")
		}
	}
	if miimonRaw, ok := bond["miimon"]; ok {
		if n, isInt := integer(miimonRaw); !isInt || n <= 0 {
			v.add("network.bond.miimon", "bond.miimon 必须是正整数（毫秒）")
		}
	}
	if xmit := strings.TrimSpace(text(bond, "xmit_hash_policy")); xmit != "" && !validXmit[xmit] {
		v.add("network.bond.xmit_hash_policy", "bond.xmit_hash_policy %q 不支持（可用：layer2、layer2+3、layer3+4、encap2+3、encap3+4）", xmit)
	}
}

// --- generic map helpers -------------------------------------------------

// text returns a scalar as a string, coercing integers and floats that YAML or
// JSON may produce for unquoted scalars such as serial numbers.
func text(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return ""
	}
}

func integer(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint64:
		return int(n), true
	case float64:
		if n != float64(int64(n)) {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func toList(v any) []any {
	switch l := v.(type) {
	case []any:
		return l
	case nil:
		return nil
	case string:
		if strings.TrimSpace(l) == "" {
			return nil
		}
		return []any{l}
	default:
		return nil
	}
}
