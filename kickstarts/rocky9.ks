# Rocky 9 Kickstart — Redfish 自动装机专用
# 与 auto.ks (openEuler) 差异:
#   - poweroff 保留 (用户手动重启)
#   - %post 网络配置改用 nmcli (Rocky 9 NetworkManager)
#   - 拨盘/分区/%pre 逻辑保持兼容

text
zerombr

%include /tmp/partation.ks
%include /tmp/network.ks
%include /tmp/partition_layout.ks
%include /tmp/rootpw.ks

cdrom
keyboard --xlayouts='us'
lang en_US.UTF-8
timezone Asia/Shanghai --utc
selinux --disabled
firewall --disabled
eula --agreed
poweroff
firstboot --disable
skipx
services --disabled="chronyd"

%packages
@^minimal-environment
wget
curl
tar
unzip
vim
bash-completion
%end

%pre --interpreter=/bin/bash
# Hosts config is embedded directly - no ISO mount needed
cat > /tmp/hosts.json << 'HOSTS_JSON_EOF'
%%HOSTS_JSON_PLACEHOLDER%%
HOSTS_JSON_EOF

# Match by SN
SN=$(dmidecode -s system-serial-number 2>/dev/null | tr -d ' ')
if [ -z "$SN" ] || [ "$SN" = "NotSpecified" ] || [ "$SN" = "Not Specified" ]; then
    SN=$(cat /sys/class/dmi/id/product_serial 2>/dev/null | tr -d ' ')
fi
if [ -z "$SN" ] || [ "$SN" = "NotSpecified" ] || [ "$SN" = "Not Specified" ]; then
    SN=$(cat /sys/firmware/dmi/entries/0-0/raw 2>/dev/null | strings | grep -oE 'YS2B[0-9A-Z]+|7[0-9]{8,}' | head -1)
fi
echo "SN=$SN" > /tmp/ks-pre.log

python3 -c "
import json
data = json.load(open('/tmp/hosts.json'))
sn = '${SN}'
host = next((h for h in data.get('hosts', []) if str(h.get('sn')) == sn), {})
json.dump(host, open('/tmp/provision.json', 'w'))
if host:
    print(f'Matched: {sn} -> {host.get(\"hostname\")}')
else:
    print(f'WARNING: No match for SN={sn}')
" >> /tmp/ks-pre.log 2>&1

HAVE_JSON=false
[ -s /tmp/provision.json ] && python3 -c "import json;assert json.load(open('/tmp/provision.json'))" 2>/dev/null && HAVE_JSON=true
echo "HAVE_JSON=$HAVE_JSON" >> /tmp/ks-pre.log

# --- Root password ---
if $HAVE_JSON; then
    ROOTPW=$(python3 -c "import json;print(json.load(open('/tmp/provision.json')).get('root_password','123456'))" 2>/dev/null)
    echo "rootpw --plaintext ${ROOTPW}" > /tmp/rootpw.ks
else
    echo "rootpw --plaintext 123456" > /tmp/rootpw.ks
fi

# --- Disk selection ---
TARGET_SIZE=40
if $HAVE_JSON; then
    JSON_SIZE=$(python3 -c "import json;print(json.load(open('/tmp/provision.json')).get('disk_target_size',${TARGET_SIZE}))" 2>/dev/null)
    [ -n "$JSON_SIZE" ] && TARGET_SIZE=$JSON_SIZE
fi
best_disk=""
best_diff=999999999
for disk in $(ls /sys/block | grep -E '^(sd|nvme|vd)'); do
    size=$(cat /sys/block/$disk/size)
    size=$((size * 512 / 1024 / 1024 / 1024))
    diff=$((size - TARGET_SIZE))
    [ $diff -lt 0 ] && diff=$((-diff))
    if [ $diff -lt $best_diff ]; then
        best_diff=$diff
        best_disk=$disk
    fi
done

wipefs -af /dev/$best_disk 2>/dev/null
for part in $(ls /dev/${best_disk}* 2>/dev/null); do
    wipefs -af $part 2>/dev/null
    pvremove -ff $part 2>/dev/null
done
vgs --noheadings -o vg_name 2>/dev/null | while read vg; do
    vgremove -ff $vg 2>/dev/null
done
# Force kernel to drop stale partition refs and nuke MBR/GPT for clean re-install
partprobe /dev/$best_disk 2>/dev/null || true
dd if=/dev/zero of=/dev/$best_disk bs=1M count=10 2>/dev/null || true
partprobe /dev/$best_disk 2>/dev/null || true
echo "zerombr" > /tmp/partation.ks
echo "bootloader --location=mbr --driveorder=$best_disk" >> /tmp/partation.ks
echo "ignoredisk --only-use=$best_disk" >> /tmp/partation.ks
echo "clearpart --all --initlabel" >> /tmp/partation.ks
echo "disk=$best_disk" >> /tmp/ks-pre.log

# --- Partition layout ---
if $HAVE_JSON; then
    python3 -c "
import json
d=json.load(open('/tmp/provision.json'))
p=d.get('partition',{})
boot=p.get('boot_size',1024)
efi=p.get('efi_size',1024)
fs=p.get('root_fstype','xfs')
lvm=p.get('use_lvm',True)
lines=[f'part /boot --fstype=\"xfs\" --size={boot}',f'part /boot/efi --fstype=\"vfat\" --size={efi}']
if lvm:
    lines+=['part pv.01 --fstype=\"lvmpv\" --grow','volgroup vg_root --pesize=4096 pv.01',f'logvol / --fstype=\"{fs}\" --percent=100 --name=root --vgname=vg_root']
else:
    lines+=[f'part / --fstype=\"{fs}\" --grow']
open('/tmp/partition_layout.ks','w').write('\n'.join(lines)+'\n')
" 2>/dev/null
else
    cat > /tmp/partition_layout.ks << 'PARTEOF'
part /boot --fstype="xfs" --size=1024
part /boot/efi --fstype="vfat" --size=1024
part pv.01 --fstype="lvmpv" --grow
volgroup vg_root --pesize=4096 pv.01
logvol / --fstype="xfs" --percent=100 --name=root --vgname=vg_root
PARTEOF
fi

# --- Network (hostname only, bond in %post via nmcli) ---
if $HAVE_JSON; then
    MY_HOST=$(python3 -c "import json;print(json.load(open('/tmp/provision.json')).get('hostname','localhost'))" 2>/dev/null)
    echo "network --hostname=${MY_HOST}" > /tmp/network.ks
else
    echo "network --hostname=localhost" > /tmp/network.ks
fi
%end

%post --nochroot --interpreter=/bin/bash
cp /tmp/ks-pre.log /mnt/sysimage/var/log/pxe-install.log 2>/dev/null
cp /tmp/provision.json /mnt/sysimage/var/lib/pxe-provision.json 2>/dev/null

if [ -f /tmp/provision.json ]; then
    MY_HOSTNAME=$(python3 -c "import json;print(json.load(open('/tmp/provision.json')).get('hostname','localhost'))" 2>/dev/null)
    if [ -n "$MY_HOSTNAME" ] && [ "$MY_HOSTNAME" != "localhost" ]; then
        echo "$MY_HOSTNAME" > /mnt/sysimage/etc/hostname
        echo "Hostname set: $MY_HOSTNAME" >> /mnt/sysimage/var/log/pxe-install.log
    fi
fi

# --- U 盘 payload 注入（可选，由项目 payload_mode 决定；off 时为一行注释）---
%%PAYLOAD_SNIPPET%%
%end

%post --interpreter=/bin/bash
# Write NM keyfile directly. nmcli in %post fails because NM daemon
# isn't running in the chroot; keyfile is picked up on first boot.
[ -f /var/lib/pxe-provision.json ] || exit 0

LOG=/var/log/pxe-install.log
echo "=== NM keyfile bond configuration ===" >> $LOG

python3 << 'PYEOF' >> $LOG 2>&1
import json, os, uuid

def netmask_to_cidr(nm):
    return sum(bin(int(x)).count('1') for x in nm.split('.'))

try:
    d = json.load(open('/var/lib/pxe-provision.json'))
except Exception as e:
    print(f"JSON error: {e}")
    raise SystemExit(0)

net = d.get('network')
if not net:
    print("No network config")
    raise SystemExit(0)

ip = net.get('ip', '')
netmask = net.get('netmask', '255.255.255.0')
gateway = net.get('gateway', '')
dns_list = net.get('dns', [])
bond = net.get('bond')
# VLAN tag（可选）。填了表示静态地址配在 <父接口>.<vlan> 上，
# 物理接口与 bond 只做二层承载，不再持有 IP。
vlan = net.get('vlan')
try:
    vlan_id = int(vlan) if vlan not in (None, '', 0) else 0
except (TypeError, ValueError):
    vlan_id = 0

KEYFILE_DIR = '/etc/NetworkManager/system-connections'
os.makedirs(KEYFILE_DIR, exist_ok=True)

if bond:
    mode = bond.get('mode', 4)
    raw_slaves = bond.get('slaves', [])
    miimon = bond.get('miimon', 100)
    lacp_rate = bond.get('lacp_rate', 1)
    xmit_hash_policy = bond.get('xmit_hash_policy', 'layer3+4')

    # ifcfg mode number -> keyfile mode name
    mode_map = {0: 'balance-rr', 1: 'active-backup', 2: 'balance-xor',
                3: 'broadcast', 4: '802.3ad', 5: 'balance-tlb', 6: 'balance-alb'}
    mode_name = mode_map.get(mode, '802.3ad')

    cidr = f"{ip}/{netmask_to_cidr(netmask)}"
    dns_str = ';'.join(dns_list) + ';' if dns_list else ''

    gw_line = f"gateway={gateway}\n" if gateway else ""
    # 有 VLAN 时地址移到子接口，bond0 自身只承载二层
    if vlan_id:
        bond_ipv4 = """[ipv4]
method=disabled
"""
    else:
        bond_ipv4 = f"""[ipv4]
address1={cidr}
{gw_line}dns={dns_str}
method=manual
"""

    bond_cfg = f"""[connection]
id=bond0
uuid={uuid.uuid4()}
type=bond
interface-name=bond0

{bond_ipv4}
[bond]
mode={mode_name}
miimon={miimon}
lacp_rate={lacp_rate}
xmit_hash_policy={xmit_hash_policy}
"""

    path = f'{KEYFILE_DIR}/bond0.nmconnection'
    with open(path, 'w') as f:
        f.write(bond_cfg)
    os.chmod(path, 0o600)
    print(f"Written {path}: {cidr} gw={gateway}")

    for i, raw_slave in enumerate(raw_slaves):
        mac = raw_slave.upper().replace('-', ':') if (':' in raw_slave or '-' in raw_slave) else ''
        slave_cfg = f"""[connection]
id=bond0-slave-{i}
uuid={uuid.uuid4()}
type=ethernet
master=bond0
slave-type=bond

[ethernet]
mac-address={mac}
"""
        path = f'{KEYFILE_DIR}/bond0-slave-{i}.nmconnection'
        with open(path, 'w') as f:
            f.write(slave_cfg)
        os.chmod(path, 0o600)
        print(f"Written {path}: mac={mac}")

    print(f"Bond complete: mode={mode_name} slaves={raw_slaves}")

    if vlan_id:
        vlan_cfg = f"""[connection]
id=bond0.{vlan_id}
uuid={uuid.uuid4()}
type=vlan
interface-name=bond0.{vlan_id}

[vlan]
parent=bond0
id={vlan_id}

[ipv4]
address1={cidr}
{gw_line}dns={dns_str}
method=manual
"""
        vlan_path = f'{KEYFILE_DIR}/bond0.{vlan_id}.nmconnection'
        with open(vlan_path, 'w') as f:
            f.write(vlan_cfg)
        os.chmod(vlan_path, 0o600)
        print(f"Written {vlan_path}: vlan={vlan_id} {cidr} gw={gateway}")

elif vlan_id:
    # 单口 + VLAN：按 MAC 找到物理接口，父接口不配地址，
    # 地址落在 <nic>.<vlan> 上。
    import glob as _glob
    mac = (net.get('mac') or '').strip().upper().replace('-', ':')
    nic_name = None
    for p_ in _glob.glob('/sys/class/net/*/address'):
        try:
            if open(p_).read().strip().upper() == mac:
                nic_name = p_.split('/')[4]
                break
        except Exception:
            pass
    if not nic_name:
        # 找不到 MAC 对应的真实网卡时不要瞎猜 eth0：VLAN 的 parent
        # 必须是真实接口，猜错会让整个网络配置失效，直接跳过。
        print(f"WARNING: 按 MAC {mac} 找不到真实网卡，跳过 VLAN {vlan_id} 配置")
        nic_name = ''

    parent_cfg = f"""[connection]
id={nic_name}
uuid={uuid.uuid4()}
type=ethernet
interface-name={nic_name}

[ethernet]
mac-address={mac}

[ipv4]
method=disabled
"""
    parent_path = f'{KEYFILE_DIR}/{nic_name}.nmconnection'
    with open(parent_path, 'w') as f:
        f.write(parent_cfg)
    os.chmod(parent_path, 0o600)

    if not nic_name:
        raise SystemExit(0)

    cidr = f"{ip}/{netmask_to_cidr(netmask)}"
    dns_str = ';'.join(dns_list) + ';' if dns_list else ''
    gw_line = f"gateway={gateway}\n" if gateway else ""
    vlan_cfg = f"""[connection]
id={nic_name}.{vlan_id}
uuid={uuid.uuid4()}
type=vlan
interface-name={nic_name}.{vlan_id}

[vlan]
parent={nic_name}
id={vlan_id}

[ipv4]
address1={cidr}
{gw_line}dns={dns_str}
method=manual
"""
    vlan_path = f'{KEYFILE_DIR}/{nic_name}.{vlan_id}.nmconnection'
    with open(vlan_path, 'w') as f:
        f.write(vlan_cfg)
    os.chmod(vlan_path, 0o600)
    print(f"Written {parent_path} + {vlan_path}: vlan={vlan_id} {cidr} gw={gateway} mac={mac}")

else:
    print("No bond config")
PYEOF
# Enable root SSH login (Rocky 9 default is prohibit-password, blocks password root SSH)
sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
%end
