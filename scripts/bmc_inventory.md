# bmc_inventory.py — BMC 硬件信息批量采集

通过 Redfish API 批量采集服务器的 SN、BMC MAC 和所有网卡的端口 MAC，输出 CSV。

**纯标准 Redfish 字段，不依赖任何厂商 OEM 扩展。**

## 前置条件

- Python 3.6+
- `curl`
- BMC 网络可达（HTTPS）

## 快速开始

```bash
# 1. 准备 BMC 列表
cat > bmcinfo.txt <<'EOF'
bmc_ip	bmc_user	bmc_pass
100.88.20.1	jdroot	JCss%6!8
100.88.20.2	jdroot	JCss%6!8
EOF

# 2. 采集
python3 scripts/bmc_inventory.py -f bmcinfo.txt
```

结果自动保存到 `bmcinfo.txt` 同目录的 `bmc_inventory_<时间戳>.csv`。

## 参数

| 参数 | 必填 | 说明 |
|---|---|---|
| `-f, --file` | ✅ | BMC 信息文件路径 |
| `-u, --user` | | 全局 BMC 用户名（默认 `jdroot`） |
| `-p, --password` | | 全局 BMC 密码（默认 `JCss%6!8`） |
| `-o, --output` | | 输出 CSV 路径（默认自动生成） |
| `-j, --jobs` | | 并发数（默认 10） |
| `-h, --help` | | 显示帮助 |

## 输入文件格式

自动检测，四种都支持，首行表头自动跳过。

**CSV**

```csv
bmc_ip,bmc_user,bmc_pass
100.88.20.1,jdroot,JCss%6!8
```

**TSV**

```
bmc_ip	bmc_user	bmc_pass
100.88.20.1	jdroot	JCss%6!8
```

**空格分隔**

```
bmc_ip          bmc_user    bmc_pass
100.88.20.1     jdroot      JCss%6!8
```

**纯 IP 列表**（配合 `-u/-p` 或使用内置默认账号）

```
100.88.20.1
100.88.20.2
```

### 凭据优先级

```
文件中的账号密码  >  -u/-p 参数  >  内置默认值 (jdroot / JCss%6!8)
```

每行独立判断：某行缺账号时才回退到全局值。

## 采集内容

| 数据 | Redfish 路径 | 字段 |
|---|---|---|
| SN | `/redfish/v1/Systems/1` | `SerialNumber` |
| BMC MAC | `/redfish/v1/Managers/1/EthernetInterfaces` | `PermanentMACAddress` 或 `MACAddress` |
| 网卡列表 | `/redfish/v1/Chassis/1/NetworkAdapters` | 全部适配器 |
| 网卡型号 | 同上 | `Model` |
| 网卡厂商 | 同上 | `Manufacturer` |
| 端口 MAC | `.../NetworkAdapters/{id}/NetworkPorts` | `AssociatedNetworkAddresses[0]` |

**为什么不用 OEM 字段**：早期版本用 `Oem.JD.CardModel` 按速率过滤网卡，实测这批华为 BMC 的标准速率字段（`CurrentLinkSpeedMbps`、`MaxSpeedMbps`、`SupportedLinkCapabilities`）全为空，速率只在 OEM 里。改为**全采所有网卡**后，零 OEM 依赖，换厂商也能直接跑；筛选交给 Excel。

## 输出格式

**一行一台服务器**，所有网卡平铺为列：

```csv
bmc_ip,sn,bmc_mac,nic1_name,nic1_model,nic1_mfr,nic1_mac1,nic1_mac2,nic2_name,nic2_model,nic2_mfr,nic2_mac1,nic2_mac2,error
100.88.20.1,2102315UENN0S5100200,34:66:79:fc:b7:ee,PCIeCard2(HP382),CX5,Nivida,14:70:57:40:D3:06,14:70:57:40:D3:07,PCIeCard5(SP226D),Hi1822,Huawei,B4:61:42:70:2C:DC,B4:61:42:70:2C:DD,
```

| 列 | 含义 |
|---|---|
| `bmc_ip` | BMC 地址 |
| `sn` | 服务器序列号 |
| `bmc_mac` | BMC 管理口 MAC |
| `nic{n}_name` | 网卡标识（如 `PCIeCard2(HP382)`） |
| `nic{n}_model` | 网卡芯片型号（如 `CX5`、`Hi1822`） |
| `nic{n}_mfr` | 网卡厂商 |
| `nic{n}_mac{m}` | 第 n 张卡第 m 个端口的 MAC |
| `error` | 失败原因，成功时为空 |

列数按**实际最大网卡数和端口数**自动扩展，不同批次的服务器可以混在一起采。

结果按 BMC IP 数值排序（`100.88.20.2` 排在 `100.88.20.10` 前面）。

## 用法示例

```bash
# 基本采集
python3 scripts/bmc_inventory.py -f bmcinfo.txt

# 指定输出
python3 scripts/bmc_inventory.py -f bmcinfo.txt -o result.csv

# 纯 IP 列表 + 全局账号 + 20 并发
python3 scripts/bmc_inventory.py -f ip_list.txt -u admin -p pass123 -j 20

# 直接回车 = 显示帮助
python3 scripts/bmc_inventory.py
```

## 运行输出

进度信息走 **stderr**，方便管道分离：

```
开始采集: 3 台, 并发: 10
  [1/3] 100.88.20.1 SN=2102315UENN0S5100200 BMC_MAC=34:66:79:fc:b7:ee [PCIeCard2(HP382)(CX5)×2; PCIeCard5(SP226D)(Hi1822)×2] OK
  [2/3] 100.88.20.2 SN=2102315UENN0S5100203 BMC_MAC=34:66:79:fc:b7:00 [PCIeCard2(HP382)(CX5)×2; PCIeCard5(SP226D)(Hi1822)×2] OK
  [3/3] 100.88.20.3 SN= BMC_MAC= [-] 无法连接BMC

完成: 成功 2, 失败 1, 共 3
结果已保存到: bmc_inventory_20261023_110000.csv
```

方括号内是网卡摘要：`卡名(型号)×端口数`。

## 实测数据

40 台华为 Kunpeng 920 服务器，20 并发，**11 秒完成，40/40 成功**。

## 常见错误

| error 内容 | 原因 |
|---|---|
| `无法连接BMC` | 网络不通、账号密码错误、BMC 未启用 Redfish |
| `无法获取网络适配器列表` | BMC 不支持 `/Chassis/1/NetworkAdapters` 或返回非 JSON |
| `未发现网络适配器` | 适配器列表为空（无网卡或字段结构不同） |

## 相关脚本

| 脚本 | 用途 |
|---|---|
| `bmc_inventory.py` | 采集硬件信息（只读） |
| `bmc_bios_set.py` | 查询/修改 BIOS 属性（有写操作） |
| `test_redfish_fields.sh` | 对比标准字段与 OEM 字段是否有值 |
