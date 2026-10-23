# bmc_bios_set.py — BIOS 属性批量查询与修改

通过 Redfish API 批量查询或修改服务器 BIOS 属性（如关闭超线程）。

> 修改写入 **pending settings**，必须重启服务器才生效。

## 前置条件

- Python 3.6+
- `curl`
- BMC 网络可达（HTTPS）
- BMC 支持 Redfish BIOS 接口（本脚本在华为 iBMC3 上验证通过）

## 参数

| 参数 | 说明 |
|---|---|
| `-f, --file` | 必填。BMC 信息文件路径 |
| `-s, --set` | 修改模式。`KEY=VALUE`，可多次指定 |
| `-g, --get` | 查询模式。属性名，可多次指定 |
| `--list` | 列出模式。列出全部 BIOS 属性（约 394 项） |
| `--dry-run` | 配合 `-s` 使用：只读取当前值，不写入 |
| `-u, --user` | 全局 BMC 用户名（默认 `jdroot`） |
| `-p, --password` | 全局 BMC 密码（默认 `JCss%6!8`） |
| `-o, --output` | 输出 CSV 路径（默认 `bios_set_<时间戳>.csv`） |
| `-j, --jobs` | 并发数（默认 10） |

## 三种模式（互斥）

| 模式 | 参数 | 行为 |
|---|---|---|
| **修改** | `-s KEY=VALUE` | 写入 pending settings，重启后生效 |
| **查询** | `-g KEY` | 查看指定属性的生效值 + pending 值 |
| **列出** | `--list` | 导出全部属性，适合先摸清有哪些可调项 |

## 输入文件格式

与 `bmc_inventory.py` 相同：自动检测 CSV / TSV / 空格分隔 / 纯 IP 列表，首行表头自动跳过。

```
bmc_ip	bmc_user	bmc_pass
100.88.20.1	jdroot	JCss%6!8
```

或纯 IP 列表（使用默认账号或 `-u/-p`）：

```
100.88.20.1
100.88.20.2
```

## 用法示例

```bash
# 1. 先摸清有哪些属性可调
python3 scripts/bmc_bios_set.py -f bmcinfo.txt --list
python3 scripts/bmc_bios_set.py -f bmcinfo.txt --list -o bios_all.csv
#    → 每台一行，394 个属性各占两列（生效值 + pending 值）

# 2. 查询关心的属性
python3 scripts/bmc_bios_set.py -f bmcinfo.txt -g SMT2En
python3 scripts/bmc_bios_set.py -f bmcinfo.txt -g SMT2En -g BootTypeOrder0

# 3. 预演：只看会改什么，不写入
python3 scripts/bmc_bios_set.py -f bmcinfo.txt -s SMT2En=Disabled --dry-run

# 4. 实际执行
python3 scripts/bmc_bios_set.py -f bmcinfo.txt -s SMT2En=Disabled

# 5. 改多个属性
python3 scripts/bmc_bios_set.py -f bmcinfo.txt -s SMT2En=Disabled -s SomeOther=Value

# 6. 高并发
python3 scripts/bmc_bios_set.py -f bmcinfo.txt -s SMT2En=Disabled -j 20
```

## 查询输出

```
[1/3] 100.88.20.1 SMT2En=Disabled                    ← 已生效，无 pending
[2/3] 100.88.20.2 SMT2En=Enabled(pending:Disabled)   ← 改了，等重启
[3/3] 100.88.20.3 SMT2En=Enabled                     ← 未改过
```

括号里的 `pending` 值是重启后会生效的值。没有括号 = 没有待生效的修改。

## 修改状态

| 状态 | 含义 |
|---|---|
| `OK(重启生效)` | 本次写入成功，等待重启 |
| `SKIP(已生效)` | 当前生效值已是目标值，未写入 |
| `SKIP(已pending)` | 上次已写入相同值，未重复写入（幂等） |
| `DRY_RUN` | 预演模式，未写入 |
| `FAIL` | 连接失败或写入被拒 |

退出统计：

```
完成: 写入 2, 已生效跳过 36, 已pending跳过 2, 失败 0, 共 40
⚠ 修改已写入 pending，需要重启服务器才生效
```

## 输出 CSV

**修改模式**

```csv
bmc_ip,sn,SMT2En_before,SMT2En_after,status,error
100.88.20.2,2102315UENN0S5100203,Enabled,Disabled(pending),OK(重启生效),
```

**查询模式（-g）**

```csv
bmc_ip,sn,SMT2En,SMT2En_pending,error
100.88.20.2,2102315UENN0S5100203,Enabled,Disabled,
```

**列出模式（--list）**：属性数 × 2 + 3 列（约 791 列）。

## 生效流程

1. 脚本 `PATCH /redfish/v1/Systems/1/Bios/Settings`（写入 pending）
2. 状态变为 `Ineffective`，当前运行配置不变
3. **重启服务器**
4. 重启后 pending 值变为生效值

## 验证生效

重启后再查询一次：

```bash
python3 scripts/bmc_bios_set.py -f bmcinfo.txt -g SMT2En
```

应返回 `SMT2En=Disabled`（无 pending 括号）。

> **注意**：Redfish 的 `ProcessorSummary` / `Processors` 中 `TotalCores` 和 `TotalThreads`
> 报的是 **CPU 硬件规格**，不反映 SMT 是否关闭。实测 SMT 开/关的机器该字段都是 80/160。
> 要确认超线程真的关了，必须进操作系统看：
>
> ```bash
> lscpu | grep "Thread(s) per core"   # SMT 关闭应为 1
> ```

## 华为 BMC 的特殊处理

华为 iBMC3 的 `Bios/Settings` 端点**强制要求 `If-Match` 请求头**：

- PATCH 不带 ETag → `HTTP 412 Precondition Failed`
- 脚本会先 GET Settings 拿 ETag，再带 `If-Match` PATCH

这是华为特有行为，其他厂商 BMC 一般不需要，脚本自动处理。

## 撤销 pending

华为提供 OEM 撤销接口（脚本未封装，需要时手动调）：

```bash
curl -sk -u jdroot:'JCss%6!8' -X POST \
  https://<BMC_IP>/redfish/v1/Systems/1/Bios/Settings/Actions/Oem/JD/Settings.Revoke \
  -H "Content-Type: application/json" -d '{}'
```

## 常见属性参考（华为 Kunpeng 平台）

| 属性 | 取值 | 含义 |
|---|---|---|
| `SMT2En` | `Enabled` / `Disabled` | 超线程（SMT2） |
| `BootTypeOrder0` | — | 启动顺序第 1 项 |
| `BootTypeOrder1` | — | 启动顺序第 2 项 |
| `AclsEn` | `Enabled` / `Disabled` | 高级配置与电源接口 |
| `PowerPolicy` | — | 电源策略 |

属性名各平台不同，先用 `--list` 或 `-g` 确认。

## 实测数据

- 40 台华为 Kunpeng 920，20 并发，查询约 4 秒
- 单台修改写入约 3 秒
- 幂等性验证：对已 pending 的机器重复执行返回 `SKIP(已pending)`，不重复写入

## 相关脚本

| 脚本 | 用途 |
|---|---|
| `bmc_inventory.py` | 采集硬件信息（SN/MAC/网卡），只读 |
| `bmc_bios_set.py` | 查询和修改 BIOS 属性，有写操作 |
| `test_redfish_fields.sh` | 对比标准字段与 OEM 字段是否有值 |
