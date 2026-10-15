# openEuler 22.03 LTS aarch64 Kickstart — 自动装机专用
# 网络模式:
#   - perf (高性能): 单口 400Gb 静态 IP, 按 MAC 绑定, 无 DEVICE= 硬编码 (ifcfg)
#   - storage (容量型): 25Gb*2 LACP bond, 按 MAC 绑定 slave (ifcfg)
#
# 纯 EFI (aarch64), ifcfg 网络配置 (/etc/sysconfig/network-scripts/)

text
zerombr

%include /tmp/partation.ks
%include /tmp/network.ks
%include /tmp/partition_layout.ks
%include /tmp/rootpw.ks

%include /tmp/install_source.ks
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


# Detect install source: Ventoy (dm/hd) vs cdrom
# When booted from Ventoy, there's no real cdrom device — repo is pre-mounted
# at /run/install/repo via device-mapper. Specifying "cdrom" would make anaconda
# try to resolve LABEL= which fails on dm devices.
if [ -d /run/install/repo/Packages ] || [ -e /dev/mapper/ventoy ]; then
    echo "# Ventoy/hd mode: repo already mounted, no source directive needed" > /tmp/install_source.ks
else
    echo "cdrom" > /tmp/install_source.ks
fi
# Match by SN (华为设备 SN 以 2102 开头)
SN=$(dmidecode -s system-serial-number 2>/dev/null | tr -d ' ')
if [ -z "$SN" ] || [ "$SN" = "NotSpecified" ] || [ "$SN" = "Not Specified" ]; then
    SN=$(cat /sys/class/dmi/id/product_serial 2>/dev/null | tr -d ' ')
fi
if [ -z "$SN" ] || [ "$SN" = "NotSpecified" ] || [ "$SN" = "Not Specified" ]; then
    SN=$(cat /sys/firmware/dmi/entries/0-0/raw 2>/dev/null | strings | grep -oE '2102[0-9A-Z]+' | head -1)
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
TARGET_SIZE=480
if $HAVE_JSON; then
    JSON_SIZE=$(python3 -c "import json;print(json.load(open('/tmp/provision.json')).get('disk_target_size',${TARGET_SIZE}))" 2>/dev/null)
    [ -n "$JSON_SIZE" ] && TARGET_SIZE=$JSON_SIZE
fi
best_disk=""
best_diff=999999999
for disk in $(ls /sys/block | grep -E '^(sd|nvme|vd)'); do
    # Skip USB/removable devices (Ventoy U盘)
    removable=$(cat /sys/block/$disk/removable 2>/dev/null)
    [ "$removable" = "1" ] && continue
    # Skip devices flagged as USB transport
    readlink -f /sys/block/$disk/device 2>/dev/null | grep -qi usb && continue
    # Skip very small disks (< 50GB, likely USB stick)
    size=$(cat /sys/block/$disk/size)
    size_gb=$((size * 512 / 1024 / 1024 / 1024))
    [ $size_gb -lt 50 ] && continue
    diff=$((size_gb - TARGET_SIZE))
    [ $diff -lt 0 ] && diff=$((-diff))
    if [ $diff -lt $best_diff ]; then
        best_diff=$diff
        best_disk=$disk
    fi
done
if [ -z "$best_disk" ]; then
    # Fallback: pick the largest non-USB disk
    best_disk=$(lsblk -d -o NAME,SIZE,TYPE,RM --noheadings --bytes | awk '$3=="disk" && $4==0 {print $1, $2}' | sort -k2 -rn | head -1 | awk '{print $1}')
fi
echo "target_disk=$best_disk (target_size=${TARGET_SIZE}GB)" >> /tmp/ks-pre.log

# Clean old LVM/RAID signatures on target disk ONLY
if [ -n "$best_disk" ]; then
    # Remove VGs that have PVs on the target disk only
    for part in $(ls /dev/${best_disk}* 2>/dev/null | grep -E '/dev/[a-z]+[0-9p]+'); do
        vg_on_part=$(pvs --noheadings -o vg_name "$part" 2>/dev/null | tr -d ' ')
        [ -n "$vg_on_part" ] && vgremove -ff "$vg_on_part" 2>/dev/null
        pvremove -ff "$part" 2>/dev/null
    done
    wipefs -af /dev/$best_disk 2>/dev/null
    dd if=/dev/zero of=/dev/$best_disk bs=1M count=10 2>/dev/null || true
    partprobe /dev/$best_disk 2>/dev/null || true
fi
echo "zerombr" > /tmp/partation.ks
echo "bootloader --location=mbr --driveorder=$best_disk" >> /tmp/partation.ks
echo "ignoredisk --only-use=$best_disk" >> /tmp/partation.ks
echo "clearpart --all --initlabel --drives=$best_disk" >> /tmp/partation.ks
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
part / --fstype="xfs" --grow
PARTEOF
fi

# --- Network (hostname only, ifcfg in %post) ---
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
# In chroot - write ifcfg files (/etc/sysconfig/network-scripts/)
# 两种模式:
#   1. bond (storage): network.bond.slaves 指定 MAC, LACP bond + 静态 IP
#   2. 单口 (perf): network.mac 指定 MAC, 静态 IP
[ -f /var/lib/pxe-provision.json ] || exit 0

LOG=/var/log/pxe-install.log
echo "=== ifcfg network configuration ===" >> $LOG

python3 << 'PYEOF' >> $LOG 2>&1
import json, os

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
mac = net.get('mac', '')
bond = net.get('bond')

IFCFG_DIR = '/etc/sysconfig/network-scripts'
os.makedirs(IFCFG_DIR, exist_ok=True)

if bond:
    # ===== Bond mode (storage: 25Gb*2 LACP) =====
    mode = bond.get('mode', 4)
    raw_slaves = bond.get('slaves', [])
    miimon = bond.get('miimon', 100)
    lacp_rate = bond.get('lacp_rate', 1)
    xmit_hash_policy = bond.get('xmit_hash_policy', 'layer3+4')

    bond_cfg = f"""DEVICE=bond0
NAME=bond0
TYPE=Bond
BONDING_MASTER=yes
BONDING_OPTS="mode={mode} miimon={miimon} lacp_rate={lacp_rate} xmit_hash_policy={xmit_hash_policy}"
BOOTPROTO=static
IPADDR={ip}
NETMASK={netmask}
ONBOOT=yes
"""
    if gateway:
        bond_cfg += f"GATEWAY={gateway}\n"
    if dns_list:
        for i, dns in enumerate(dns_list, 1):
            bond_cfg += f"DNS{i}={dns}\n"

    with open(f'{IFCFG_DIR}/ifcfg-bond0', 'w') as f:
        f.write(bond_cfg)
    print(f"Written ifcfg-bond0: {ip}/{netmask} gw={gateway}")

    for i, raw_slave in enumerate(raw_slaves):
        slave_mac = raw_slave.upper().replace('-', ':')
        slave_cfg = f"""HWADDR={slave_mac}
NAME=bond0-slave-{i}
TYPE=Ethernet
BOOTPROTO=none
ONBOOT=yes
MASTER=bond0
SLAVE=yes
"""
        with open(f'{IFCFG_DIR}/ifcfg-bond0-slave-{i}', 'w') as f:
            f.write(slave_cfg)
        print(f"Written ifcfg-bond0-slave-{i}: HWADDR={slave_mac}")

    print(f"Bond complete: mode={mode} slaves={raw_slaves}")

elif mac:
    # ===== Single port mode (perf: 400Gb*1) =====
    # 按 HWADDR 匹配, 同时探测物理接口名用于 NAME 和文件名
    mac_upper = mac.upper().replace('-', ':')

    # 在 /sys 中按 MAC 反查实际接口名
    import glob as _glob
    nic_name = None
    for p in _glob.glob('/sys/class/net/*/address'):
        try:
            addr = open(p).read().strip().upper()
            if addr == mac_upper:
                nic_name = p.split('/')[4]
                break
        except:
            pass
    if not nic_name:
        nic_name = 'mgmt'  # 极端情况: 找不到则用逻辑名

    cfg = f"""HWADDR={mac_upper}
NAME={nic_name}
TYPE=Ethernet
BOOTPROTO=static
IPADDR={ip}
NETMASK={netmask}
ONBOOT=yes
"""
    if gateway:
        cfg += f"GATEWAY={gateway}\n"
    if dns_list:
        for i, dns in enumerate(dns_list, 1):
            cfg += f"DNS{i}={dns}\n"

    with open(f'{IFCFG_DIR}/ifcfg-{nic_name}', 'w') as f:
        f.write(cfg)
    print(f"Written ifcfg-{nic_name}: {ip}/{netmask} gw={gateway} HWADDR={mac_upper}")

else:
    # Fallback: no MAC binding — 运行时探测第一个物理网卡名
    import glob
    nic_name = 'eth0'
    for p in sorted(glob.glob('/sys/class/net/*/device')):
        candidate = p.split('/')[4]
        if candidate != 'lo':
            nic_name = candidate
            break
    cfg = f"""DEVICE={nic_name}
NAME=mgmt
TYPE=Ethernet
BOOTPROTO=static
IPADDR={ip}
NETMASK={netmask}
ONBOOT=yes
"""
    if gateway:
        cfg += f"GATEWAY={gateway}\n"
    if dns_list:
        for i, dns in enumerate(dns_list, 1):
            cfg += f"DNS{i}={dns}\n"

    with open(f'{IFCFG_DIR}/ifcfg-mgmt', 'w') as f:
        f.write(cfg)
    print(f"Written ifcfg-mgmt: DEVICE={nic_name} {ip}/{netmask} gw={gateway} (no MAC, auto-detected NIC)")

PYEOF

# Enable root SSH login
sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
%end
