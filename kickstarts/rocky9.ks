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

    bond_cfg = f"""[connection]
id=bond0
uuid={uuid.uuid4()}
type=bond
interface-name=bond0

[ipv4]
address1={cidr}
gateway={gateway}
dns={dns_str}
method=manual

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
else:
    print("No bond config")
PYEOF
# Enable root SSH login (Rocky 9 default is prohibit-password, blocks password root SSH)
sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
%end
