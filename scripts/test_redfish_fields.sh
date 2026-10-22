#!/bin/bash
# 测试华为 BMC 的 Redfish 标准字段 vs OEM 字段
# 用法: bash scripts/test_redfish_fields.sh <BMC_IP> [USER] [PASS]

IP="${1:?用法: $0 <BMC_IP> [USER] [PASS]}"
USER="${2:-jdroot}"
PASS="${3:-JCss%6!8}"
CURL="curl -sk -u ${USER}:${PASS}"
RF="https://${IP}/redfish/v1"

echo "=== 1. NetworkAdapter 标准 Model 字段 ==="
for adapter in $($CURL $RF/Chassis/1/NetworkAdapters 2>/dev/null | python3 -c "import sys,json;[print(m['@odata.id']) for m in json.load(sys.stdin).get('Members',[])]" 2>/dev/null); do
  echo "--- $adapter ---"
  $CURL "https://${IP}${adapter}" 2>/dev/null | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(f'  Model(标准):      {d.get(\"Model\",\"<空>\")}')
print(f'  Manufacturer:     {d.get(\"Manufacturer\",\"<空>\")}')
print(f'  Oem.JD.CardModel: {d.get(\"Oem\",{}).get(\"JD\",{}).get(\"CardModel\",\"<空>\")}')
print(f'  Oem keys:         {list(d.get(\"Oem\",{}).keys())}')
"
done

echo ""
echo "=== 2. NetworkPort 标准速率字段 ==="
for adapter in $($CURL $RF/Chassis/1/NetworkAdapters 2>/dev/null | python3 -c "import sys,json;[print(m['@odata.id']) for m in json.load(sys.stdin).get('Members',[])]" 2>/dev/null); do
  for port in $($CURL "https://${IP}${adapter}/NetworkPorts" 2>/dev/null | python3 -c "import sys,json;[print(m['@odata.id']) for m in json.load(sys.stdin).get('Members',[])]" 2>/dev/null); do
    echo "--- $port ---"
    $CURL "https://${IP}${port}" 2>/dev/null | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(f'  CurrentLinkSpeedMbps(标准): {d.get(\"CurrentLinkSpeedMbps\",\"<空>\")}')
print(f'  MaxSpeedMbps:              {d.get(\"MaxSpeedMbps\",\"<空>\")}')
caps = d.get('SupportedLinkCapabilities',[])
if caps:
    for i,c in enumerate(caps):
        print(f'  SupportedLinkCapabilities[{i}]: {c}')
else:
    print(f'  SupportedLinkCapabilities:  <空>')
print(f'  Oem.JD.PortMaxSpeed:       {d.get(\"Oem\",{}).get(\"JD\",{}).get(\"PortMaxSpeed\",\"<空>\")}')
print(f'  AssociatedNetworkAddresses: {d.get(\"AssociatedNetworkAddresses\",[])}')
"
  done
done
