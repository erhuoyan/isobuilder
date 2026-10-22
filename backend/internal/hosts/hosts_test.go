package hosts

import "testing"

// VLAN 是可选字段：不配时必须完全等同于打了标之前的配置，配了才落到子接口。
// 这里锁住四条可观测契约：留空不算错、合法值原样进 summary、越界报错、
// 数字字符串（CSV/Excel 与手写 YAML 都会给）同样接受。
func TestParseVLAN(t *testing.T) {
	base := func(vlan string) string {
		return "hosts:\n  - sn: SN1\n    hostname: h1\n    network:\n" +
			"      ip: 10.0.0.5\n      netmask: 255.255.255.0\n" + vlan +
			"      mac: aa:bb:cc:dd:ee:ff\n"
	}

	cases := []struct {
		name   string
		vlan   string
		ok     bool
		vlanID int
	}{
		{name: "omitted", vlan: "", ok: true, vlanID: 0},
		{name: "null", vlan: "      vlan:\n", ok: true, vlanID: 0},
		{name: "empty string", vlan: "      vlan: \"\"\n", ok: true, vlanID: 0},
		{name: "number", vlan: "      vlan: 61\n", ok: true, vlanID: 61},
		{name: "quoted number", vlan: "      vlan: \"61\"\n", ok: true, vlanID: 61},
		{name: "lower bound", vlan: "      vlan: 1\n", ok: true, vlanID: 1},
		{name: "upper bound", vlan: "      vlan: 4094\n", ok: true, vlanID: 4094},
		{name: "too small", vlan: "      vlan: 0\n", ok: false},
		{name: "too large", vlan: "      vlan: 4095\n", ok: false},
		{name: "not a number", vlan: "      vlan: trunk\n", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Parse(base(tc.vlan), "yaml")
			if res.OK != tc.ok {
				t.Fatalf("OK = %v, want %v (errors: %+v)", res.OK, tc.ok, res.Errors)
			}
			if !tc.ok {
				if len(res.Errors) != 1 || res.Errors[0].Field != "network.vlan" {
					t.Fatalf("errors = %+v, want one network.vlan error", res.Errors)
				}
				return
			}
			if len(res.Summary) != 1 {
				t.Fatalf("summary length = %d, want 1", len(res.Summary))
			}
			if got := res.Summary[0].VLANID; got != tc.vlanID {
				t.Errorf("vlan_id = %d, want %d", got, tc.vlanID)
			}
		})
	}
}

// VLAN 不改变网络模式的判定：bond 仍然是 bond，MAC 数照旧。
func TestParseVLANKeepsNetworkMode(t *testing.T) {
	const yaml = `hosts:
  - sn: SN1
    hostname: h1
    network:
      ip: 10.0.0.5
      netmask: 255.255.255.0
      vlan: 61
      bond:
        mode: 4
        slaves:
          - aa:bb:cc:dd:ee:01
          - aa:bb:cc:dd:ee:02
`
	res := Parse(yaml, "yaml")
	if !res.OK {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	got := res.Summary[0]
	if got.NetworkMode != "bond" || got.MACCount != 2 || got.VLANID != 61 {
		t.Errorf("summary = %+v, want bond/2 macs/vlan 61", got)
	}
}
