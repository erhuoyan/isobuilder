# bmc_boot.py — 批量控制启动设备

通过 Redfish API 批量设置服务器**下一次**的启动设备（PXE / 虚拟光驱 / BIOS 设置界面），
可选设置后立即重启。

## 与 bmc_bios_set.py 的区别

| | bmc_bios_set.py | bmc_boot.py |
|---|---|---|
| 操作对象 | BIOS 持久配置（`Bios/Settings`） | 一次性启动重定向（`Systems/1 Boot`） |
| 生效时机 | **必须重启**后才生效 | 设置后**下次启动**即生效 |
| 典型用途 | 关闭超线程、改启动顺序 | 装机引导、进 BIOS、挂 ISO |
| 是否改变硬件配置 | 是（持久） | 否（`Once` 模式下次启动后自动清除） |

装完机想恢复硬盘启动，用 `--clear` 即可，不会残留配置。

## 参数

| 参数 | 说明 |
|---|---|
| `-f, --file` | 必填。BMC 信息文件路径 |
| `-t, --target` | 启动目标：`pxe` / `cd` / `iso` / `hdd` / `bios` / `none` |
| `-m, --mode` | `Once`（默认，只生效一次）/ `Continuous`（持续生效） |
| `--reset` | 设置后立即重启 |
| `--reset-type` | 重启方式，默认 `ForceRestart` |
| `--iso URL` | 插入虚拟光驱 ISO，自动应用 `-t cd` |
| `--eject` | 弹出已挂载的镜像 |
| `--vm-user` / `--vm-pass` | 镜像服务器认证（可选） |
| `--vnc on\|off` | 开启/关闭 VNC 远程画面 |
| `--vnc-ssl on\|off` | VNC 通道是否走 TLS（`off` = 明文 RFB） |
| `--vnc-pass PASS` | VNC 密码（**最多 8 位**），配合 `--vnc on` |
| `--status` | 只查询电源状态、当前引导源、VNC 与 SSL 状态 |
| `--clear` | 清除引导重定向（= `-t none`，同时置 `Disabled`） |
| `--dry-run` | 只读不写 |
| `--wait SEC` | 等待 ISO 挂载/弹出生效的超时（默认 90 秒） |
| `-u, --user` | BMC 用户名（默认 `jdroot`） |
| `-p, --password` | BMC 密码（默认 `JCss%6!8`） |
| `-o, --output` | 输出 CSV 路径（默认 `bmc_boot_<时间戳>.csv`） |
| `-j, --jobs` | 并发数（默认 10） |

## VNC 远程画面

装机需要看界面时开 VNC，比 KVM 网页好用在可以多标签、可脚本化。

```bash
# 开启并设密码
python3 scripts/bmc_boot.py -f bmcinfo.txt --vnc on --vnc-pass 'JCss%6!8'

# 查状态（--status 输出里带 VNC=on/off/unknown SSL=on/off）
python3 scripts/bmc_boot.py -f bmcinfo.txt --status

# 关闭
python3 scripts/bmc_boot.py -f bmcinfo.txt --vnc off
```

`--vnc-ssl` 可单独用，也可与 `--vnc` / `--vnc-pass` 组合（同一条 PATCH 下发）：

```bash
# 关掉 TLS，改成明文 RFB
python3 scripts/bmc_boot.py -f bmcinfo.txt --vnc-ssl off

# 一次配齐：开 VNC + 关 TLS + 设密码
python3 scripts/bmc_boot.py -f bmcinfo.txt --vnc on --vnc-ssl off --vnc-pass 'pw123456'
```

连接方式：

```bash
brew install tiger-vnc            # 或任意 VNC 客户端
vncviewer 100.88.20.2             # 端口默认 5900
```

### 两个坑

**① 开关不在 `VncService`**

华为把 VNC 开关放在 `NetworkProtocol` 的 OEM 段，标准 `RFB` 字段是只读的：

```bash
# ✗ 报 PropertyNotWritable
PATCH /redfish/v1/Managers/1/NetworkProtocol  {"RFB":{"ProtocolEnabled":true}}

# ✓ 正确（厂商键按机器实际暴露的为准）
PATCH /redfish/v1/Managers/1/NetworkProtocol
{"Oem":{"Huawei":{"VNC":{"ProtocolEnabled":true}}}}   # 或 {"Oem":{"JD":{...}}}
```

**厂商键是自动识别的**，顺序为 `Oem.Huawei` → `Oem.JD`：

- 优先选真正带 `VNC` 段的那个命名空间（JD 定制版 iBMC 上两个键可能同时存在）；
  都没带 VNC 段时，退化为"厂商键存在"；
- 两个命名空间都带 VNC 时按上面的顺序取第一个——**批量操作前先用 `--status`
  在一台机器上确认**（`VNC=on/off` 与 Web 界面一致即可，`VNC=unknown` 说明没识别出来）；
- 两个键都识别不到时**不猜**：写操作直接 FAIL 并打印实际看到的 `Oem=` 键名，
  `--status` 报 `VNC=unknown`。盲写一个不存在的命名空间会被 BMC 忽略或 400，
  回读又对不上，只会得到误导性的"状态不符"。

**② 密码最长 8 位**

超过报 `iBMC.1.0.StringValueTooLong`。默认 BMC 密码 `JCss%6!8` 正好 8 位，可直接复用。
上限由各厂商的 `password_max` 决定，批量跑时逐台按该厂商上限校验，
且在校验通过前不写入任何东西。

### 新增 OEM 适配

厂商差异全部收在 `VncOem` 类里（`scripts/bmc_boot.py` 顶部「OEM 适配」段），
`process_one` 只跟适配器打交道，不认具体厂商。

| 成员 | 作用 |
|---|---|
| `name` / `namespace` | 厂商名（报错文案）/ `NetworkProtocol.Oem` 下的键 |
| `netproto_path` / `service_path` | 开关资源路径 / 密码与 TLS 的资源路径 |
| `password_max` | 该厂商的 VNC 密码长度上限 |
| `matches(netproto, strict)` | 是否属于本厂商（`strict` = 是否要求已有 `VNC` 段） |
| `read_enabled(netproto)` | 读回开关，字段缺失返回 `None`（不是 `False`） |
| `patch_body(enabled)` | 开关的 PATCH body |

接入分两种：

```python
# ① 形态相同（iBMC 家族，只是命名空间不同）——两行
class InspurVncOem(VncOem):
    name = "Inspur"
    namespace = "Inspur"

# ② 资源路径 / 字段名都不同——覆盖三个方法
class DellVncOem(VncOem):
    name = "Dell"
    namespace = "Dell"
    service_path = "/redfish/v1/Managers/1/OemVnc"
    password_max = 16

    def read_enabled(self, netproto):
        oem = (netproto or {}).get("Oem") or {}
        return ((oem.get("Dell") or {}).get("VNC") or {}).get("Enabled")

    def patch_body(self, enabled):
        return {"Oem": {"Dell": {"VNC": {"Enabled": enabled}}}}
```

然后登记到 `VNC_OEMS`：

```python
VNC_OEMS = (HuaweiVncOem(), JDVncOem(), InspurVncOem(), DellVncOem())
```

`VNC_PASSWORD_MAX_ANY`（CLI 层上限）和 `--help` 里的厂商列表都由注册表推导，
不用另外改。命中顺序 = 注册顺序，且"已有 VNC 段"优先于"命名空间存在"。

> 报错文案、`--status` 的 `unknown`、写入前的密码长度校验都走适配器，
> 新厂商不需要动 `process_one`。

### 关闭 TLS（`--vnc-ssl off`）

`SSLEncryptionEnabled` 位于 `VncService`，**可写**，且必须带 `If-Match`（不带报 `412`）：

```bash
PATCH /redfish/v1/Managers/1/VncService
{"SSLEncryptionEnabled": false}
```

- 立即生效，无需重启 BMC：5900 端口从"等 TLS ClientHello"变成直接吐 `RFB 003.008`。
- 只去掉传输层加密，认证仍是 VNC Auth（安全类型 `[2]`），不是无密码开放。
- 明文 RFB 的密码挑战是 DES，可被嗅探/离线爆破。仅在内网，或先 `ssh -L 5900:<BMC>:5900 user@跳板` 再连 `localhost:5900`。
- `--vnc-ssl` 与引导/ISO/重启互斥（VNC 配置是独立动作），`--status` 会一并显示 `SSL=on/off`。

### 实测

| 检查项 | 结果 |
|---|---|
| 开启 VNC | ✅ `Oem.JD.VNC.ProtocolEnabled=true` |
| TLS 握手 | ✅ TLSv1.3（`SSLEncryptionEnabled: true`） |
| 认证类型 | ✅ `[2]` = VNC Auth（需密码） |
| 密码登录 | ✅ 完整 RFB 握手验证通过 |
| 统一密码 | ✅ `JCss%6!8`（8 位）可用 |
| 关闭 TLS | ✅ `SSLEncryptionEnabled=false` 后明文 `RFB 003.008`，认证仍为 `[2]` |
| 改回 TLS | ✅ 明文连接重新超时（需 TLS 握手） |

## 启动目标

| `-t` | Redfish 值 | 效果 |
|---|---|---|
| `pxe` | `Pxe` | 从网卡 PXE 启动（装机常用） |
| `cd` / `iso` | `Cd` | 从虚拟光驱启动（配合 `--iso`） |
| `hdd` | `Hdd` | 从硬盘启动 |
| `bios` | `BiosSetup` | 重启进入 BIOS 设置界面 |
| `none` | `None` | 清除重定向，恢复正常启动顺序 |

## 重启方式

`--reset-type` 可选值（华为 iBMC3 实测支持）：

| 值 | 说明 |
|---|---|
| `ForceRestart` | 强制重启（默认，最快） |
| `GracefulRestart` | 优雅重启（通知 OS 关机） |
| `PowerCycle` | 断电重启 |
| `ForcePowerCycle` | 强制断电重启 |
| `On` | 开机 |
| `ForceOff` | 强制关机 |
| `GracefulShutdown` | 优雅关机 |
| `Nmi` | 触发 NMI |

**电源为 `Off` 时**，`ForceRestart` / `PowerCycle` 会无效，脚本自动改用 `On` 开机，
不报错。

## 用法示例

```bash
# 0. 先看当前状态（永远从这步开始）
python3 scripts/bmc_boot.py -f bmcinfo.txt --status

# 1. 下次启动走 PXE（不重启，最安全）
python3 scripts/bmc_boot.py -f bmcinfo.txt -t pxe

# 2. 设置并立即重启
python3 scripts/bmc_boot.py -f bmcinfo.txt -t pxe --reset

# 3. 挂 ISO 并从光驱启动
python3 scripts/bmc_boot.py -f bmcinfo.txt \
    --iso https://files.example.com/rocky9.iso --reset

# 4. 重启进入 BIOS 设置界面
python3 scripts/bmc_boot.py -f bmcinfo.txt -t bios --reset

# 5. 装机完成后恢复硬盘启动
python3 scripts/bmc_boot.py -f bmcinfo.txt --clear
python3 scripts/bmc_boot.py -f bmcinfo.txt --eject      # 顺带弹出 ISO

# 6. 预演（不写任何东西）
python3 scripts/bmc_boot.py -f bmcinfo.txt -t pxe --reset --dry-run

# 7. 持续从 PXE 启动（调试网络装机时用，避免每次重启都要重设）
python3 scripts/bmc_boot.py -f bmcinfo.txt -t pxe -m Continuous --reset
```

## 典型装机流程

```bash
# ① 确认当前状态
bmc_boot.py -f bmcinfo.txt --status

# ② 挂载装机 ISO + 设为下次光驱启动 + 重启
bmc_boot.py -f bmcinfo.txt --iso https://.../installer.iso --reset

# ③ 等装机完成（略）

# ④ 装完后清理，恢复硬盘启动
bmc_boot.py -f bmcinfo.txt --clear --eject
```

`Once` 模式下，服务器从光驱启动一次后会自动回到正常启动顺序，
所以 ④ 的 `--clear` 主要是为了**弹出 ISO**，防止下次误引导。

## 输出

```
动作: 引导=Pxe/Once + 重启(ForceRestart)
共 3 台, 并发: 10
──────────────────────────────────────────────────────────────
  [1/3] 100.88.20.1 SN=2102315UENN0S5100200 电源=On 引导=None/Disabled → Pxe/Once [OK(已设置并重启:ForceRestart)]
  [2/3] 100.88.20.2 SN=2102315UENN0S5100203 电源=On 引导=Pxe/Once [SKIP(状态一致)]
  [3/3] 100.88.20.3 SN=2102315UENN0S5100191 电源=Off 引导=None/Disabled [OK] [FAIL]
──────────────────────────────────────────────────────────────
完成: 成功 1, 跳过 1, 失败 1, 共 3
  ⚠ 1 台已下发重启，机器会立即重启
```

### 状态含义

| 状态 | 含义 |
|---|---|
| `OK` | 查询成功（`--status` 模式） |
| `OK(已设置)` | 引导重定向已设置，**未重启** |
| `OK(已设置并重启:ForceRestart)` | 已设置并下发重启 |
| `OK(已重启:ForceRestart)` | 重定向状态本来就一致，只执行了重启 |
| `OK(已弹出)` | 虚拟光驱已弹出 |
| `SKIP(状态一致)` | 目标状态与当前一致，未做修改 |
| `DRY_RUN` | 预演，未执行 |
| `FAIL` | 连接失败或 API 拒绝 |

### CSV 列

```csv
bmc_ip,sn,power_before,iso_before,target_before,target_after,vnc,vnc_ssl,status,error
100.88.20.1,2102315UENN0S5100200,On,,None/Disabled,Pxe/Once,on,on,OK(已设置并重启:ForceRestart),
```

`target` 格式为 `<目标>/<模式>`，如 `Pxe/Once`、`None/Disabled`。
`vnc` / `vnc_ssl` 只在 `--status` 或对应动作（`--vnc` / `--vnc-ssl`）时填值（`on`/`off`），其他动作为空。

## 安全须知

**`--reset` 会真实重启服务器。** 先在单台验证，再对整批执行：

```bash
# 单台验证
echo 100.88.20.1 > /tmp/one.txt
python3 scripts/bmc_boot.py -f /tmp/one.txt -t pxe --reset

# 确认无误再批量
python3 scripts/bmc_boot.py -f bmcinfo.txt -t pxe --reset -j 20
```

不带 `--reset` 时**只设置下次启动设备，不会重启**，可以放心在生产环境预置。

## 华为 BMC 的特殊处理

与 `bmc_bios_set.py` 相同：`PATCH /redfish/v1/Systems/1` 强制要求 `If-Match`，
不带会返回 `HTTP 412 Precondition Failed`。脚本自动取 ETag 并在 412 时重试。

## 虚拟光驱

`--iso` 支持三种协议，从 URL 前缀自动推断：

| URL 前缀 | TransferProtocolType | 需要证书 |
|---|---|---|
| `nfs://` | `NFS` | ❌ |
| `https://` | `HTTPS` | ✅ |
| `cifs://` 或 `smb://` | `CIFS` | ❌ |

**推荐 NFS**：华为 BMC 的 `EncryptionEnabled` 恒为 `True` 且不可配置
（`EncryptionConfigurable: false`），HTTPS 必须用 BMC 信任的证书才可能成功；
NFS 无此限制。实测 NFS 一次通过。

### NFS 路径写法（关键）

`nfs://` 后面是**服务器上的绝对路径**，不是客户端挂载点：

```
nfs://100.88.20.253/export/nfs-server-data/iso/foo.iso
       ────────── ─────────────────────────────
       服务器 IP    服务器上的绝对路径
```

脚本会校验并拦截常见错误（如误填 `/mnt/iso/x.iso`）。

### 镜像服务器搭建（以 Ubuntu 为例）

BMC 只在自己的网段内找服务器，所以镜像服务器必须和 BMC 同网段可达：

```bash
apt-get install -y nfs-kernel-server

# 导出目录，只允许 BMC 网段只读访问
echo '/export/iso 100.88.0.0/16(ro,sync,no_subtree_check,no_root_squash,insecure)' \
  > /etc/exports

systemctl enable --now rpcbind nfs-server
exportfs -rv
ss -tln | grep -E ':2049|:111'    # 确认监听
```

本地先自测（用真实 IP，不能用 127.0.0.1，会被 exports 拒绝）：

```bash
mkdir -p /mnt/t && mount -t nfs -o ro,nolock 100.88.20.253:/export/iso /mnt/t
ls /mnt/t && umount /mnt/t
```

### 无公网环境的包安装

内网机器装不了 `apt install` 时，在能上网的机器下载 deb 再传过去：

```bash
# 联网机器上（jammy）
apt-get download nfs-kernel-server nfs-common rpcbind libnfsidmap1 keyutils
# 或直接下：http://archive.ubuntu.com/ubuntu/pool/main/n/nfs-utils/

scp *.deb root@内网机:/tmp/nfsdeb/
ssh root@内网机 'dpkg -i /tmp/nfsdeb/*.deb'
```

依赖基本都已预装（libc6、libtirpc3、libevent-core、libkeyutils1 等），
通常只缺这 5 个包。

### 挂载是异步的

`InsertMedia` / `EjectMedia` 返回 `HTTP 202` **只代表请求已受理**，
实际挂载可能因网络、路径、权限失败。脚本会轮询 `VirtualMedia` 的
`Inserted` 字段直到状态符合预期（`--wait` 控制超时，默认 90 秒），
失败时明确报错而不是假报成功。

> 镜像 URL 需能被 BMC 网段访问到。走 NAT/跳板机的路径不适用于这一步。

## 验证记录

已在华为 iBMC3（Kunpeng 920 / J353 G3）实测：

| 场景 | 结果 |
|---|---|
| `--status` 查询 | ✅ |
| `-t pxe` 设置（不重启） | ✅ 独立 curl 验证 `Pxe/Once` |
| 重复设置同一目标 | ✅ `SKIP(状态一致)` |
| `--clear` | ✅ 置为 `None/Disabled` |
| `--eject` | ✅ 回读确认 `Inserted=false` |
| `--dry-run` | ✅ 不写任何东西 |
| **NFS 挂载 3.5G ISO** | ✅ `Inserted=true, ConnectedVia=URI, Protocol=NFS` |
| 重复挂载同一 ISO | ✅ 跳过重挂，1.4s（首次 8.5s） |
| 挂载不存在的文件 | ✅ 正确 `FAIL` 并报错（旧版会假报成功） |
| `--reset` 真实重启 + 从 ISO 引导 | ✅ 进入安装程序 |
| **ISO 挂着 + `Boot=None/Disabled` 开机** | ✅ 从硬盘启动，进新系统，未误引导 |

### 实测踩到的坑

| 问题 | 现象 | 修复 |
|---|---|---|
| `Inserted` 字段 | `Base.1.0.ActionParameterUnknown`，HTTP 400 | 华为不接受该字段，只传 `Image`/`TransferProtocolType` |
| 202 假成功 | Task 卡 Running，实际没挂上 | 轮询回读 `Inserted` 确认 |
| NFS 路径 | 填了挂载点 `/mnt/iso/x.iso` | 必须是服务器绝对路径 |
| BMC 到跳板机不通 | 抓包零 SYN | 镜像服务器必须与 BMC 同网段 |

### 装机实测流程（100.88.20.20）

```bash
# 1. 挂 ISO + 设光驱启动 + 重启
bmc_boot.py -f bmc20.txt \
  --iso nfs://100.88.20.253/export/nfs-server-data/iso/xxx.iso --reset

# 2. 约 60 秒后进入安装界面（BMC 侧 PowerState 保持 On）
# 3. 装完后清理
bmc_boot.py -f bmc20.txt --clear --eject
```

`Once` 模式启动一次后自动失效，`Boot` 会变回 `None/Disabled`。

## 常见疑问：ISO 没弹会不会又从光盘启动

**不会。** 实测验证（100.88.20.20）：

```
条件:  Boot=None/Disabled + Inserted=True（ISO 仍挂载）
动作:  开机
结果:  PowerState=On，SSH 22 可连，正常进入新装系统
```

原因：

| 机制 | 说明 |
|---|---|
| `BootSourceOverride` | 一次性重定向，用掉后 BMC 自动清空为 `None/Disabled` |
| BIOS 启动顺序 | `BootTypeOrder0=HardDiskDrive`，硬盘优先于 `DVDROMDrive` |
| 虚拟光驱 | 只是"盘在驱动器里"，不等于"从盘启动" |

**仍然建议装机后 `--eject`**，原因只有两个：

1. NFS 服务器不可达时，UEFI 探测虚拟光驱会引入超时（可能几十秒）
2. 留着盘，以后谁误设 `-t cd` 就会又进安装程序

## 相关脚本

| 脚本 | 用途 |
|---|---|
| `bmc_inventory.py` | 采集硬件信息（SN/MAC/网卡），只读 |
| `bmc_bios_set.py` | 查询/修改 BIOS 持久配置 |
| `bmc_boot.py` | 控制启动设备 + 电源操作 |
| `test_redfish_fields.sh` | 对比标准字段与 OEM 字段是否有值 |
