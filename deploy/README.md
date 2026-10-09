# 部署与运行手册（裸机）

面向运维/交付：从零部署、跑通一个项目、日常维护。全部命令都在目标机上执行（root）。

---

## 1. 部署

### 1.1 打发布包（在构建机上，仓库根目录）

```bash
./deploy/release.sh 0.1.0
# → release/isobuilder-0.1.0-linux-amd64.tar.gz
```

包内容：静态二进制 `isobuilder`、前端 `web/`、内置模板 `kickstarts/`、单机应急脚本 `scripts/`、
`install.sh`、`isobuilder.service`、`env.example`。

前端默认**每次重新构建**（避免把旧产物打进包）；确认产物最新时用 `SKIP_WEB=1` 跳过，省约 1 分钟。

交叉编译其它架构：`GOARCH_TARGET=arm64 ./deploy/release.sh 0.1.0`。

### 1.2 安装（目标机）

```bash
mkdir -p /root/isobuilder-rel
tar xzf /root/isobuilder-0.1.0-linux-amd64.tar.gz -C /root/isobuilder-rel --strip-components=1
/root/isobuilder-rel/install.sh --from-release /root/isobuilder-0.1.0-linux-amd64.tar.gz
```

也可以直接在目标机上从源码装（需要 go，前端产物缺失时需要 pnpm）：

```bash
./deploy/install.sh              # 就地编译 + 安装
./deploy/install.sh --uninstall   # 卸载（保留数据目录）
```

### 1.3 安装脚本做了什么

| 步骤 | 说明 |
|---|---|
| 装依赖 | `dnf install xorriso syslinux syslinux-nonlinux isomd5sum`（Debian 系自动换 apt） |
| 建用户 | 系统用户 `isobuilder`（nologin），目录属主交给它 |
| 装文件 | 二进制/前端/模板 → `/opt/isobuilder` |
| 建数据目录 | `/var/lib/isobuilder`（镜像、日志、产物、SQLite） |
| 装单元 | `/etc/systemd/system/isobuilder.service`，enable + restart |
| 首次默认值 | `/etc/isobuilder/env`（0600），默认只有 `ISOBUILDER_ADDR` |
| 端口选择 | 全新安装用 `:8080`，被占用自动挑 `8081-8099`；**升级沿用已配置端口** |

部署后的目录：

```
/opt/isobuilder/{isobuilder, web/, kickstarts/, scripts/, README.md}
/var/lib/isobuilder/{isos/, kickstarts/, work/, builds/, artifacts/, uploads/, isobuilder.db}
/etc/isobuilder/env
```

### 1.4 存储与清理（容器部署必读）

平台对本地磁盘的占用分三类，只有第一类是持久的：

| 目录 | 内容 | 生命周期 | 量级 |
|---|---|---|---|
| `$DATA_DIR/isos` | 基础镜像 | 持久，删项目不受影响 | 每个一个 ISO（可达 20 GB+） |
| `$DATA_DIR/work` | 解包整棵树 + 产物 | **单个构建结束后清理** | 峰值 ≈ 2× 基础 ISO |
| `$DATA_DIR/artifacts` | 已注册的产物 | 上传对象存储后**立即删除本地副本** | 上传后 0；`storage=local` 时保留 |

所以本地落盘的只有基础镜像和正在构建的那一份。`ISOBUILDER_KEEP_LOCAL_ARTIFACT=true` 时保留本地副本便于复看。

构建**开始前**会做磁盘预检，磁盘不够**不会在写一半时失败**：

```
磁盘预检: 需要约 31.00 GB（基础镜像 15.00 GB ×2），可用 12.00 GB（/var/lib/isobuilder/work）
构建失败：磁盘空间不足：基础镜像 15.00 GB，构建峰值约需 31.00 GB，当前可用 12.00 GB。
```

**容器部署**把 `/data` 挂成卷即可：`docker run -v isobuilder-data:/data ...`。业务空间的核心公式是
`单卷 >= 基础镜像 ×(自己保留多少份) + 最大 ISO ×2 ×并发数`。容器也默认把 uploaded 产物本地副本删掉，所以**容器本身只需要控制：基础镜像数量 + 并发构建**。

### 1.5 运行身份

服务**以 root 运行**，只写 `/var/lib/isobuilder`，不做别的系统改动。这样部署最省事：
你手工 `scp`/`cp` 丢进 `isos/` 的文件不存在属主或权限问题。

要收权时改一处即可（单元文件里有两行注释指引）：

```bash
useradd --system --home-dir /var/lib/isobuilder --shell /usr/sbin/nologin isobuilder
chown -R isobuilder:isobuilder /var/lib/isobuilder
sed -i '/^Type=simple/a User=isobuilder\nGroup=isobuilder' /etc/systemd/system/isobuilder.service
systemctl daemon-reload && systemctl restart isobuilder
```

非 root 运行时，登记接口会在**登记时**同步检查文件可读性，读不到直接报
`path_unreadable` 并给出该执行的 chown/chmod，不会等你到算校验和时才发现。

### 1.5 验证

```bash
systemctl status isobuilder
journalctl -u isobuilder -n 30            # 启动日志会打印 data_dir / storage / 端口
curl -s localhost:8081/api/health         # 端口以 /etc/isobuilder/env 里的为准
curl -s localhost:8081/api/config | python3 -m json.tool   # tools 字段为 null 表示构建依赖齐全
```

`GET /api/config` 的 `tools` 会列出**缺失**的构建工具（如 `["isohybrid"]`）；为 `null`/`[]` 才能构建 x86_64 镜像。

---

## 2. 首次配置（对象存储）

打开 `http://<host>:<port>` → **系统配置**，填 endpoint / bucket / AK / SK → 保存。
保存时用 `HeadBucket` 探活：连不上会提示「配置已保存，但对象存储连接失败」（配置仍然生效，只是后面构建会在上传阶段失败）。

等价 API（可用于脚本化交付）：

```bash
curl -X PUT http://localhost:8081/api/settings -H 'Content-Type: application/json' -d '{
  "storage": {
    "endpoint": "http://s3-internal.cn-north-1.jdcloud-oss.com",
    "region": "cn-north-1",
    "bucket": "0-yunhai-poc",
    "prefix": "iso-builder",
    "path_style": true,
    "presign_ttl_seconds": 86400,
    "access_key": "…",
    "secret_key": "…"
  }
}'
```

- 配置存 SQLite 的 `settings` 表，**重启不丢**；环境变量只是首次默认值。
- `GET /api/settings` 的 `source`：`db`＝来自配置页，`env`＝仅来自环境变量。
- 密钥只写不读：接口只返回 `access_key_set` / `secret_key_set`。
- `bucket` 留空并保存 → 关闭对象存储，产物留在 `/var/lib/isobuilder/artifacts`，由后端直传下载。

服务默认**不开认证**。要开：在 `/etc/isobuilder/env` 里设 `ISOBUILDER_AUTH_TOKEN=xxx` 并
`systemctl restart isobuilder`；之后 API 需要 `Authorization: Bearer xxx`，界面会出现登录入口。

---

## 3. 走一个项目流程

### 3.1 界面

1. **镜像库 → 添加基础镜像**，三种方式任选：
   - 从对象存储拉取：填 `s3://<bucket>/path/base.iso`（凭据留空＝用系统配置）
   - 就地登记：填服务器上的绝对路径（**必须在 `/var/lib/isobuilder` 内**）
   - 分片上传：浏览器直传，14 GB 可断点续
   等待状态变为 **可用**（会异步算 SHA256，大 ISO 要几分钟）。
2. **装机项目 → 新建项目**，五步向导：
   基础信息 → 基础镜像 → kickstart 模板 → 主机清单 → 高级项
   - 主机清单直接从 CMDB 导出粘贴或读入本地文件（YAML/JSON 均可），
     边输入边校验：会指出第几台、哪个字段有问题（SN 重复、网关不同网段、bond slave 数不对…）。
   - 高级项可留空：卷标在构建时从原版 ISO 自动探测，输出文件名自动生成为 `<原名>-auto.iso`。
   - 也支持**从项目包导入**：zip/tar.gz 里只要有 `*.ks` 和 `hosts.*`，按内容自动识别。
3. 项目详情 → **开始构建**，进入构建详情页：
   - 五段推进轨：解包 → 注入 → 改引导 → 重建 → 校验上传
   - 实时日志（SSE，断线按字节偏移续读）
   - 失败会显示明确原因（缺少工具、ISO 有问题、对象存储连不上…）
4. 构建成功后**构建产物**区给出：文件名、大小、SHA256（可复制）、**下载 URL**（可复制）、过期时间。
5. URL 过期后点**刷新链接**重新签名；**下载**直接 302 到对象存储。
   把 URL 丢给交付工程师，他 `curl -O` 或浏览器下载即可写盘 / 挂 BMC 虚拟光驱。

### 3.2 等价 API（脚本化）

```bash
B=http://localhost:8081

# 1) 入库基础镜像（从 OSS；本地文件改为 {"source":"path","path":"/var/lib/isobuilder/isos/xxx.iso"}）
ISO=$(curl -s -X POST $B/api/isos/register -H 'Content-Type: application/json' \
  -d '{"source":"s3","arch":"x86_64","name":"rocky9-dvd",
       "s3":{"url":"s3://0-yunhai-poc/base/Rocky-9.8-x86_64-dvd.iso"}}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)['data']["iso"]["id"])')
curl -s $B/api/isos | python3 -m json.tool        # 等 status=ready

# 2) 选 kickstart 模板
KS=$(curl -s $B/api/kickstarts | python3 -c 'import sys,json;print([k["id"] for k in json.load(sys.stdin)['data']["kickstarts"] if k["name"]=="rocky9.ks"][0])')

# 3) 先校验主机清单（不落库）
curl -s -X POST $B/api/validate/hosts -H 'Content-Type: application/json' \
  -d "{\"content\":$(python3 -c 'import json;print(json.dumps(open("hosts.yaml").read()))'),\"format\":\"auto\"}" \
  | python3 -m json.tool

# 4) 建项目（hosts_raw 直接放原文，YAML/JSON 都行）
PRJ=$(python3 - "$ISO" "$KS" <<'PY' | curl -s -X POST $B/api/projects -H 'Content-Type: application/json' -d @- | python3 -c 'import sys,json;print(json.load(sys.stdin)['data']["project"]["id"])'
import json,sys
print(json.dumps({"name":"6.24 批次","arch":"x86_64","iso_id":sys.argv[1],
                  "kickstart_id":sys.argv[2],"hosts_raw":open("hosts.yaml").read()}))
PY
)

# 5) 构建 + 跟日志
BUILD=$(curl -s -X POST $B/api/projects/$PRJ/builds | python3 -c 'import sys,json;print(json.load(sys.stdin)['data']["build"]["id"])')
curl -sN "$B/api/builds/$BUILD/log?offset=0"            # SSE 实时日志
curl -s "$B/api/builds/$BUILD/log?format=text" | tail   # 一次性取全文

# 6) 取下载链接
curl -s $B/api/builds/$BUILD | python3 -c 'import sys,json;b=json.load(sys.stdin)['data']["build"];print(b["status"], b["artifact"]["url"] if b.get("artifact") else b["error"])'
curl -sL -o out.iso "$B/api/artifacts/<artifact_id>/download"
```

---

## 4. 日常维护

| 操作 | 命令 |
|---|---|
| 看服务 | `systemctl status isobuilder` |
| 跟随日志 | `journalctl -u isobuilder -f` |
| 重启 | `systemctl restart isobuilder`（重启时未完成的构建会被标记为失败，不会假死） |
| 升级 | 重复 1.2（端口沿用、数据保留） |
| 回滚 | 解压上一版包同样执行 `install.sh --from-release` |
| 改端口 | 改 `/etc/isobuilder/env` 的 `ISOBUILDER_ADDR` → `systemctl restart isobuilder` |
| 备份 | 停服后备份 `/var/lib/isobuilder/isobuilder.db`（配置、项目、构建历史全在里面）+ `/var/lib/isobuilder/isos` |
| 清理产物 | 界面删项目/镜像，或手工清理 `/var/lib/isobuilder/{work,artifacts,builds}` |
| 磁盘检查 | `df -h /var/lib/isobuilder`（单次构建峰值约 **2× 基础 ISO**） |

`/etc/isobuilder/env` 常用项（改完都要 restart）：

```ini
ISOBUILDER_ADDR=:8081
ISOBUILDER_AUTH_TOKEN=              # 留空＝不开认证
ISOBUILDER_MAX_CONCURRENT_BUILDS=1  # 并发构建数，每个都要 2×ISO 空间
ISOBUILDER_BUILD_TIMEOUT_MINUTES=180
ISOBUILDER_KEEP_WORK_DIR=false      # true 时保留中间目录，排障用（会占大量空间）
```

---

## 5. 排障

| 现象 | 原因 / 处理 |
|---|---|
| 启动即退出，日志 `bind: address already in use` | 端口被占。改 `ISOBUILDER_ADDR` 或释放端口 |
| 登记镜像报 `path_unreadable` | 只在改成非 root 运行时可能出现。`chown <服务用户> <文件>` 或 `chmod 0644 <文件>` |
| 构建报 `磁盘空间不足` | 工作目录能写下的空间不够 2× 基础镜像。清理 `$DATA_DIR/work` 与旧产物，或缩小并发 (`ISOBUILDER_MAX_CONCURRENT_BUILDS`) |
| 触发构建报 `missing_tools` | 缺 `xorriso`/`isohybrid`/`implantisomd5`。`dnf install xorriso syslinux syslinux-nonlinux isomd5sum` |
| 构建报「找不到 EFI 启动镜像 efiboot.img」 | 该 ISO 不是受支持的安装镜像（缺 EFI 段或结构特殊） |
| 构建报 `xorriso 退出异常: exit status 32` | 见构建日志里那条 xorriso 命令行与 `libisofs` 提示；常见是基础 ISO 结构不完整 |
| 构建卡在某段不动 | `journalctl -u isobuilder -f` 看子进程输出；日志页也会实时刷 |
| 产物上传失败 | 系统配置里点保存做一次探活；产物已保留在 `/var/lib/isobuilder/artifacts`，可手工上传 |
| 界面日志区「已断开」 | SSE 断了会自动带 offset 重连；长时间不恢复查反向代理是否禁用缓冲（本服务已发 `X-Accel-Buffering: no`） |
| 磁盘涨得快 | 正常：工作目录 = 解包后的整棵 ISO 树 + 产物。构建结束会清理；`KEEP_WORK_DIR=true` 时不清理 |

---

## 6. 实测状态（参照）

2026-10 在 `rocky 9.8 / x86_64 / systemd` 上部署，对象存储用京东云 OSS：

```
/opt/isobuilder/isobuilder      ← 静态二进制，systemd 拉起
监听 :8081（8080 被别的服务占用，装脚本自动避让）
数据 /var/lib/isobuilder
```

已验证：从 OSS 拉基础镜像、x86_64（BIOS+EFI 双段）与 aarch64（EFI-only）构建、
主机 JSON 注入 kickstart、产物上传 OSS + 预签名下载、系统配置热切换、SSE 实时日志。

**未验证**：用真实发行版 ISO（约 15 GB）完整构建并实际引导装机 —— 装机需现场进行，
构建耗时与磁盘占用也需以真 ISO 实测一次。
