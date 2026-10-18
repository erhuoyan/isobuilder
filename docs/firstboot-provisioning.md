# 装机后脚本与 U 盘文件注入：为什么必须放在「首次启动」

本文记录 isobuilder「装机后脚本（post_script）」与「U 盘 payload 注入」的时序设计依据，
以及业界在同类场景下的成熟做法。

---

## 1. 问题从哪来

两个需求：

1. **装机后执行自定义脚本**（装 agent、注册 CMDB、改配置）
2. **执行脚本前，从 U 盘拷一批文件进新系统**（离线环境，文件不放进 ISO 以免每次重打）

第 2 条踩到一个坑：U 盘在**安装期间读不到**。用户的原话很准：

> 装机完成后可以作为普通分区挂载在系统里呀，装机后期挂载不上？

答案是：**重启之后能挂，安装期间不能**。差别就在"内核是谁的"。

---

## 2. 时序真相：三个阶段

| 阶段 | 谁在跑 | 根文件系统 | 内核 | U 盘（Ventoy）能挂吗 |
|---|---|---|---|---|
| `%pre` | anaconda | 安装环境 | anaconda 的 | ❌ |
| `%post --nochroot` | anaconda | 安装环境（目标系统在 `/mnt/sysimage`） | anaconda 的 | ❌ |
| `%post`（chroot） | anaconda，chroot 到目标系统 | 目标系统 | **anaconda 的** | ❌ |
| **首次启动** | **新系统的 systemd** | 目标系统 | **新系统的** | ✅ |

**关键点**：`%post` 看起来是"装机完成后"，但它只是 chroot —— **内核仍是 anaconda 的**，
安装环境的一切约束都还在。只有真正重启、用新系统的内核引导，才是另一个世界。

### 为什么 Ventoy 分区在安装期间挂不上

Ventoy 启动 ISO 时会把分区 1 上的 `.iso` 文件通过 **device-mapper 映射成伪光驱**。
为了这个映射，Ventoy 接管了该分区 —— 此时用原生 filesystem 去 `mount` 它会冲突失败。

Ventoy 作者在 [ventoy/Ventoy#667](https://github.com/ventoy/Ventoy/issues/667)：

> `/dev/sdc1` can **NOT** be mounted after boot due to Ventoy's internal mechanism.
> If you want to access the USB drive after boot, you can create a 3rd or 4th partition.

（"after boot" 指的是**从 Ventoy 引导起来的那个环境**，不是"系统装完重启之后"。）

重启进入新系统后，这个占用消失 —— 此时它就是一个普通可挂载分区。**用户的直觉是对的。**

---

## 3. 业界怎么做

### 3.1 Red Hat 官方：一次性脚本要放到首次启动

Red Hat 有一篇 Verified Solution，
[How to run a one-time configuration script on first boot in RHEL](https://access.redhat.com/solutions/7135087)
（覆盖 RHEL 7/8/9/10），其问题描述与我们的处境一字不差：

> System configuration tasks need to be performed that require a fully running systemd environment
> (e.g., starting services, communicating with DBus, or registering with external tooling).
> Tasks fail during the Kickstart `%post` phase because **the systemd manager (PID 1) is not active
> in the installer's chroot environment**. A method is required to **defer execution until the first
> system boot, run the script exactly once, and then automatically disable it**.

即官方结论：**`%post` 里没有 systemd（PID 1 不是它），要跑"需要完整系统"的任务必须延到首次启动，
且只跑一次、跑完自动禁用。**

这正是本文推荐的形态。

### 3.2 历史：kickstart 的 `%firstboot` 段已不可用

老版本 RHEL/CentOS 的 kickstart 有 `%firstboot` 段（配合 `firstboot --enable`），
语义就是"首次启动执行"。但它依赖 `firstboot` 包，该包在 RHEL 8 起已从发行版移除，
`%firstboot` 段随之失效。

**现代替代就是"systemd oneshot 单元"** —— 语义等价、无额外依赖、可控可观测。
这也是 Red Hat 官方现在给出的做法（见 3.1）。

### 3.3 Google Cloud：Golden image vs Silver image

[Best practices for automatically provisioning and configuring edge and bare metal systems](https://docs.cloud.google.com/architecture/connected-devices/best-practices-provisioning-configuring-bare-metal)
把镜像分成两类，恰好描述了我们的处境：

| 概念 | 含义 |
|---|---|
| **Golden image** | 把所有依赖打进镜像，设备首次启动无需再装任何东西 |
| **Silver image** | 镜像只带基础系统 + "自配置能力"，**设备在首次启动时完成配置** |

原文强调：

> During their first boot ... your devices should be able to **provision and configure themselves
> using only the software image installed inside them**.

我们的方案（镜像里带 systemd 单元 + 脚本，首次启动自我配置）就是典型的 **Silver image** 形态。
Google 推荐的工具是 cloud-init + NoCloud datasource（见下节为什么我们不用它）。

### 3.4 其它方案的适用性

| 方案 | 机制 | 适合我们吗 |
|---|---|---|
| **systemd oneshot**（本文方案） | 安装期写入单元并 enable，首次启动执行 | ✅ 零依赖、离线可用、任何发行版 |
| **cloud-init** | 首次启动读 datasource（NoCloud/网络）完成配置 | ⚠️ 需要 datasource 载体（U 盘卷标 CIDATA / 网络元数据）；RHEL 最小安装默认不带 |
| **Ignition / Butane** | CoreOS/RHCOS 专用，首次启动按 JSON 配置 | ❌ 仅限 Fedora CoreOS 系 |
| **PXE + Ansible pull** | 网络引导 + 首次启动拉配置 | ⚠️ 需要网络基础设施；纯离线场景不适用 |
| **`%post` 直接执行** | 安装期执行 | ❌ 本文要解决的问题（无 systemd、介质不可挂、网络未配） |

**cloud-init 的取舍**：它是业界标准，能力也强，但依赖一个"datasource"：
- NoCloud：需要一个卷标为 `CIDATA` 的分区（同样要单独分区，且 Ventoy 场景下又要处理分区问题）
- 网络 datasource：需要 metadata 服务（云环境有，裸机没有）

我们面向的是**离线裸机 + Ventoy U 盘**，没有 metadata 服务，
所以用最朴素的 systemd 单元反而更可靠、依赖更少。

---

## 4. 推荐方案：首次启动的 systemd oneshot

### 4.1 形态

```
安装期间（%post，anaconda chroot）—— 只"布置任务"，不执行：
  ① 写脚本        → /usr/local/bin/isobuilder-firstboot.sh
  ② 写服务单元    → /etc/systemd/system/isobuilder-firstboot.service
  ③ 登记开机执行  → systemctl enable isobuilder-firstboot.service（只建符号链接）

        ↓ 安装完成，重启

首次启动 —— 新系统的 systemd 按单元执行：
  ① 挂载 U 盘（此时 Ventoy 占用已消失，可正常挂载）
  ② 拷贝 payload → /opt/payload
  ③ 执行装机后脚本
  ④ systemctl disable 自己 → 以后开机不再跑
```

### 4.2 服务单元

```ini
# /etc/systemd/system/isobuilder-firstboot.service
[Unit]
Description=isobuilder 首次启动：注入 U 盘文件并执行装机后脚本
After=network-online.target
Wants=network-online.target
# 跑过一次就不再满足条件（双保险：脚本里也会 disable 自己）
ConditionPathExists=!/var/lib/isobuilder/firstboot.done

[Service]
Type=oneshot
ExecStart=/usr/local/bin/isobuilder-firstboot.sh
RemainAfterExit=yes
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

### 4.3 为什么 `systemctl enable` 在安装期间可行

`enable` **不需要 systemd 正在运行** —— 它只是创建一个符号链接：

```
/etc/systemd/system/multi-user.target.wants/isobuilder-firstboot.service
        └──▶ ../isobuilder-firstboot.service
```

新系统第一次真正启动 systemd 时，扫描 `multi-user.target.wants/` 即可发现并执行。

### 4.4 本方案相对"安装期间执行"的优势

| 维度 | 安装期间（`%post`） | 首次启动 |
|---|---|---|
| systemd（PID 1） | ❌ 不在（chroot） | ✅ 完整可用 |
| 启停服务 / DBus | ❌ 不可用 | ✅ |
| 网络 | 一般未配置 | ✅ `After=network-online.target` |
| Ventoy U 盘分区 | ❌ 挂不上 | ✅ 普通分区 |
| 失败后果 | 影响装机过程 | 系统已可启动，服务可手动重跑（`systemctl start isobuilder-firstboot`） |
| 只跑一次 | — | ✅ 标记文件 + 自动 disable |

### 4.5 失败可恢复

首次启动失败**不会**让你失去机器：系统已经装好、能正常启动。
排查与重跑：

```bash
systemctl status isobuilder-firstboot
journalctl -u isobuilder-firstboot          # 首次启动的完整输出
cat /var/log/isobuilder/firstboot.log       # 脚本自己的日志（若启用）
systemctl start isobuilder-firstboot        # 修好后手动重跑
```

---

## 5. 对 isobuilder 的落地含义

| 能力 | 期望的执行时机 |
|---|---|
| 装机后脚本（post_script / 脚本库） | **首次启动**（需要服务、网络、包管理器的场景） |
| U 盘 payload 拷贝 | **首次启动**（Ventoy 分区此时才可挂） |
| 纯系统内、不依赖服务/网络的调整 | 安装期间 `%post` 亦可（更快、失败早暴露） |

因此 `payload_mode` 的语义应落在**首次启动**执行：

| 模式 | 含义 |
|---|---|
| `off` | 不注入（默认） |
| `label` | 首次启动时按卷标 `ISO_PAYLOAD` 查找 U 盘分区 |
| `auto` | 首次启动时卷标优先，未命中则扫描非目标盘分区找 `payload/` |

U 盘侧约定（配合 Ventoy）：

1. 用 Ventoy 的**保留空间**建一个独立分区（**不能**用放 ISO 的那个分区）
2. 分区卷标设为 `ISO_PAYLOAD`（11 字符，FAT32/exFAT 卷标上限）
3. 分区内建 `payload/` 目录
4. 文件最终出现在新系统的 `/opt/payload`


---

## 6. 待验证：实测确认执行时机

本文第 2、3 节的结论（`%post` 期间 Ventoy 分区挂不上、首次启动可挂）来自官方文档与
Ventoy 作者说明，**尚未在真实 U 盘 + 目标机上实测**。实现改造前应先测，因为它直接决定
`payload_mode` 落在哪个阶段。

### 待验证假设

| # | 假设 | 若为假会怎样 |
|---|---|---|
| H1 | `%post --nochroot` 阶段，Ventoy 数据分区**无法**挂载 | 则现有实现（安装期拷贝）其实可行，不必改 firstboot |
| H2 | 首次启动时，同一分区**可以**正常挂载 | 若也挂不上，说明分区策略本身有问题（需重做 U 盘加独立分区） |
| H3 | 首次启动时网络已就绪（`network-online.target` 可达） | 影响脚本能否做联网动作，不影响挂载 |

### 怎么测：一次装机拿到两个阶段的对比数据

在 kickstart 的 **`%post --nochroot`** 段末尾加诊断（输出到目标系统的日志里）：

```bash
{
    echo "=== [probe] %post --nochroot ==="
    echo "--- lsblk ---";          lsblk -o NAME,SIZE,FSTYPE,LABEL,MOUNTPOINT
    echo "--- /dev/mapper ---";    ls -l /dev/mapper/ 2>/dev/null
    echo "--- /run/install/repo ---"; ls /run/install/repo 2>/dev/null | head -5
    echo "--- blkid ---";          blkid
    echo "--- 逐个尝试只读挂载非目标盘 ---"
    for p in /dev/sd*[0-9] /dev/nvme*p[0-9]; do
        [ -b "$p" ] || continue
        if mount -o ro "$p" /mnt 2>/dev/null; then
            echo "OK   $p -> $(ls /mnt | head -3 | tr '\n' ' ')"
            umount /mnt 2>/dev/null
        else
            echo "FAIL $p"
        fi
    done
} > /mnt/sysimage/var/log/probe-post.log 2>&1
```

再在**首次启动**放一份相同诊断（systemd oneshot，输出到 `/var/log/probe-firstboot.log`）。
装一次机、重启一次，就能拿到两个阶段的完整对照。

### 判定与对应实现

| 实测结果 | 结论 |
|---|---|
| H1 成立、H2 成立 | ✅ 采用**首次启动**执行（本文推荐）—— 现有实现需改造 |
| H1 不成立（`%post` 也能挂） | 保持现有实现（安装期拷贝）即可，无需改造，代价最小 |
| H1 成立、H2 不成立 | Ventoy 分区两条路都不通 → 必须**重做 U 盘**加独立分区，或改用第二个 U 盘 |

### 测试时注意

- 用**目标机 + 真实 Ventoy U 盘**，不要在虚拟机上测（虚拟机的 USB 直通与真实固件行为不同）
- U 盘上要**真的放一份 `payload/` 目录**，才能区分"分区能挂"与"分区存在但内容不对"
- 记录 U 盘的分区布局（`fdisk -l /dev/sdX`）与 Ventoy 版本，结论依赖这两者

---

## 7. 参考

- Red Hat，[How to run a one-time configuration script on first boot in RHEL](https://access.redhat.com/solutions/7135087)（Verified Solution，RHEL 7–10）
- Google Cloud，[Best practices for automatically provisioning and configuring edge and bare metal systems](https://docs.cloud.google.com/architecture/connected-devices/best-practices-provisioning-configuring-bare-metal)（Golden/Silver image 概念）
- Ventoy，[issue #667：Ventoy 分区启动后无法挂载](https://github.com/ventoy/Ventoy/issues/667)
- Ventoy，[磁盘布局与保留空间](https://www.ventoy.net/en/doc_disk_layout.html#reserve_space)
- cloud-init，[NoCloud datasource](https://cloudinit.readthedocs.io/en/latest/reference/datasources/nocloud.html)
