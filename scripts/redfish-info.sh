#!/bin/bash
# 通过 Redfish 获取服务器 SN 和网卡 MAC 地址
# 用法: ./redfish-info.sh <BMC_IP> <BMC_USER> <BMC_PASS>
#
# 输出可直接用于填写 hosts.yaml

BMC_IP="${1:?用法: $0 <BMC_IP> <BMC_USER> <BMC_PASS>}"
BMC_USER="${2:?缺少 BMC 用户名}"
BMC_PASS="${3:?缺少 BMC 密码}"

CURL="curl -sk -u ${BMC_USER}:${BMC_PASS}"
RF="https://${BMC_IP}/redfish/v1"

echo "=========================================="
echo " Redfish 硬件信息  BMC: ${BMC_IP}"
echo "=========================================="

# Get system info
SYS=$($CURL ${RF}/Systems/1 2>/dev/null)
SN=$(echo "$SYS" | python3 -c "import sys,json;print(json.load(sys.stdin).get('SerialNumber','N/A').strip())" 2>/dev/null)
MODEL=$(echo "$SYS" | python3 -c "import sys,json;print(json.load(sys.stdin).get('Model','N/A'))" 2>/dev/null)
POWER=$(echo "$SYS" | python3 -c "import sys,json;print(json.load(sys.stdin).get('PowerState','N/A'))" 2>/dev/null)

echo ""
echo "序列号(SN): ${SN}"
echo "型号:       ${MODEL}"
echo "电源状态:   ${POWER}"
echo ""
echo "=========================================="
echo " 网卡列表"
echo "=========================================="

# Get NICs
$CURL "${RF}/Systems/1/EthernetInterfaces" 2>/dev/null | python3 -c "
import sys, json

data = json.load(sys.stdin)
members = data.get('Members', [])

print(f'')
print(f'共 {len(members)} 个网卡:')
print(f'')
print(f'{\"序号\":<4} {\"名称\":<45} {\"MAC\":<20}')
print('-' * 70)
" 2>/dev/null

NICS=$($CURL "${RF}/Systems/1/EthernetInterfaces" 2>/dev/null | python3 -c "import sys,json;[print(m['@odata.id']) for m in json.load(sys.stdin).get('Members',[])]" 2>/dev/null)

IDX=1
declare -a MACS
for NIC in $NICS; do
  INFO=$($CURL "https://${BMC_IP}${NIC}" 2>/dev/null)
  NAME=$(echo "$INFO" | python3 -c "import sys,json;print(json.load(sys.stdin)['@odata.id'].split('/')[-1])" 2>/dev/null)
  MAC=$(echo "$INFO" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('MACAddress',d.get('PermanentMACAddress','N/A')).rstrip(':'))" 2>/dev/null)
  printf "%-4s %-45s %-20s\n" "${IDX}" "${NAME}" "${MAC}"
  MACS+=("$MAC")
  IDX=$((IDX+1))
done

echo ""
echo "=========================================="
echo " hosts.yaml 配置参考"
echo "=========================================="
echo ""
echo "- sn: ${SN}"
echo "  ip: 169.254.0.1"
echo "  hostname: TODO"
echo "  os: openeuler2203_arm64"
echo "  disk_target_size: 480"
echo "  root_password: \"123456\""
echo "  network:"
echo "    ip: TODO"
echo "    netmask: 255.255.255.0"
echo "    gateway: TODO"
echo "    bond:"
echo "      mode: 4"
echo "      slaves:"

# Print last 2 MACs as bond slaves (typically the paired ports)
for MAC in "${MACS[@]: -2}"; do
  echo "      - \"${MAC}\""
done

echo "      miimon: 100"
echo "      lacp_rate: 1"
echo "      xmit_hash_policy: layer3+4"
echo "  status: pending"
