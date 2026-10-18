# isobuilder — 自动装机 ISO 构建平台

把「原版 Linux ISO → 自动装机 ISO → 对象存储下载链接」做成一个可交付的 Web 平台。
工程师浏览器上传基础镜像 + 项目包（machine info + kickstart），几分钟后拿到一个
URL，交付工程师直接 `curl` 下载写盘或挂 BMC 虚拟光驱装机。

```
┌──────────┐   浏览器上传     ┌───────────────────────────┐      ┌──────────────┐
│ 交付工程师 │ ──────────────→ │  isobuilder（单容器）      │ ───→ │ 对象存储 S3   │
└──────────┘                  │  Vue3 SPA + Go API +       │      └──────┬───────┘
                              │  xorriso / isohybrid /     │             │
                              │  implantisomd5 流水线       │       预签名 URL
                              └───────────────────────────┘             ↓
                                                                下载自动装机 ISO
```

## 快速开始

### 裸机部署（推荐）

> 完整操作手册（部署 / 首次配置 / 项目流程 / 运维 / 排障）见 [`deploy/README.md`](deploy/README.md)。

```bash
# 构建机：打发布包（交叉编译 linux/amd64，自动重新构建前端）
./deploy/release.sh 0.1.0
#   → release/isobuilder-0.1.0-linux-amd64.tar.gz

# 目标机：装依赖 + 二进制 + systemd
scp release/isobuilder-0.1.0-linux-amd64.tar.gz root@<host>:/root/
ssh root@<host> 'mkdir -p /root/rel && tar xzf /root/isobuilder-0.1.0-linux-amd64.tar.gz -C /root/rel --strip-components=1 \
                 && /root/rel/install.sh --from-release /root/isobuilder-0.1.0-linux-amd64.tar.gz'
```

安装脚本会：装 `xorriso/syslinux/isomd5sum` → 建 `isobuilder` 用户 → 装到 `/opt/isobuilder` →
数据放 `/var/lib/isobuilder` → 注册 systemd 单元。默认监听 `:8080`；被占用时自动挑一个空闲端口
（写进 `/etc/isobuilder/env` 的 `ISOBUILDER_ADDR`）。升级时沿用已配置端口。

```
systemctl status isobuilder
journalctl -u isobuilder -f
```

裸机不需要 Docker、不需要特权：解包用 `xorriso -osirrox`，不挂 loop 设备。
单次构建需要约 **2× 基础 ISO** 的可用空间（解包一份 + 产物一份）。

### 1. 本地开发

```bash
# 后端（go ≥ 1.24，依赖 xorriso、isoinfo、isohybrid、implantisomd5）
cd backend
go run ./cmd/isobuilder
# 配置用环境变量，开发目录自动在当前仓库根创建 data/

# 前端（node ≥ 22 + pnpm）
cd frontend
pnpm install
pnpm dev        # → http://127.0.0.1:9527，API 自动代理到 127.0.0.1:8080
```

RHEL/Rocky/CentOS 上安装构建依赖：

```bash
sudo dnf install -y xorriso genisoimage syslinux isomd5sum
```

### 2. 单镜像交付（docker compose）

```bash
# 在仓库根目录
docker build -f isobuilder/Dockerfile -t isobuilder:latest .
cd isobuilder && docker compose up -d
# → http://<host>:8080
```

只配一个 MinIO 就能跑通完整链路：

```bash
export ISOBUILDER_S3_ENDPOINT=http://minio:9000
export ISOBUILDER_S3_BUCKET=auto-iso
export ISOBUILDER_S3_ACCESS_KEY=minioadmin
export ISOBUILDER_S3_SECRET_KEY=minioadmin
docker compose up -d --build
```

### 3. Kubernetes 部署

单副本 Pod + 云 PostgreSQL + NFS 动态供给卷；完整手册见 [`deploy/k8s/README.md`](deploy/k8s/README.md)。

```bash
# 交叉构建 amd64 镜像（Apple Silicon 亦可），tag = <版本号>-<commit>-<时间>
REGISTRY=hub-pub.jdcloud.com/iaas-jf ./deploy/build-image.sh 0.1.10 --push

kubectl create namespace isobuilder
# 按 deploy/k8s/secret.example.yaml 填好 RDS 连接串
kubectl -n isobuilder apply -f deploy/k8s/secret.yaml
kubectl -n isobuilder apply -f deploy/k8s/pvc.yaml
kubectl -n isobuilder apply -f deploy/k8s/deployment.yaml
kubectl -n isobuilder apply -f deploy/k8s/service.yaml
```

数据库通过 `ISOBUILDER_DB_URL` 切到云 RDS；`/data`（基础镜像、构建工作目录、日志）
由 StorageClass 动态供给到 NFS 独立子目录。

## 使用流程

```
 ISO 库 ──→ 项目 ──→ 构建 ──→ 下载链接
  ↑         ↑
 三种入库   上传项目包（zip/tar.gz，含 ks + hosts.yaml）
```

1. **ISO 库**：原版镜像入库。三种方式：
   - **从对象存储拉取**（推荐）：填 `s3://bucket/path/base.iso`，服务端拉回本地并校验
   - 服务器已有大文件时**就地登记**（`POST /api/isos/register`，免上传；文件须在数据目录内）
   - 浏览器分片上传（`POST /api/isos/uploads` → 分块 `PUT` → `finish`，14 GB 可断点续）
2. **项目**：名称 + 架构 + 基础镜像 + kickstart 模板 + 主机清单（hosts.yaml/JSON）。
   项目包（zip/tar.gz，随便什么目录结构）可以直接导入：平台按内容识别，自动找到 `*.ks`
   与 `hosts.*`。
3. **构建**：`mounted → inject → boot → build → seal` 五段流水线，SSE 实时日志。
   同一路径对应原脚本的保守实现：
   - 解包（loop 挂载）
   - 注入 kickstart（%%HOSTS_JSON_PLACEHOLDER%% → 主机 JSON）
   - 改写引导菜单（grub.cfg / isolinux.cfg，自动识别 litunefi / linuxefi 风格）
   - xorriso 重建 ISO（BIOS+EFI 双段、Inspur BMC 验证过的参数序）
   - implantisomd5 + 上传对象存储
4. **交付**：产物带 SHA256 与预签名 URL，可用 `GET /api/artifacts/{id}` 刷新
   过期链接，用 `GET /api/artifacts/{id}/download` 直接下载。

## 配置

### 系统配置页（对象存储，改完即生效）

打开 **系统配置**，填 endpoint / bucket / AK / SK（密钥只写不读，保存后页面只显示「已配置」）。
保存时用 `HeadBucket` 探活：连不上会明确提示「配置已保存，但连接失败」。
`bucket` 留空 = 关闭对象存储，产物留在服务器 `/opt/isobuilder/data/artifacts`，由后端直传下载。

配置存在 SQLite 的 `settings` 表里，**重启不丢**；环境变量只是首次启动的默认值，
两者关系见 `GET /api/settings` 的 `source` 字段（`db` / `env`）。

```bash
# 也可以用 API 直接配置（等价于配置页保存）
curl -X PUT http://<host>:8080/api/settings -H 'Content-Type: application/json' -d '{
  "storage": {"endpoint":"http://s3-internal.cn-north-1.jdcloud-oss.com","region":"cn-north-1",
              "bucket":"0-yunhai-poc","prefix":"iso-builder","path_style":true,
              "presign_ttl_seconds":86400,"access_key":"...","secret_key":"..."}}'
```

### 环境变量（首次默认值 / 服务参数）

见 `/etc/isobuilder/env` 与 `docker-compose.yml`。最常用的：

| 变量 | 默认 | 说明 |
|---|---|---|
| `ISOBUILDER_ADDR` | `:8080` | 监听地址 |
| `ISOBUILDER_DATA_DIR` | `/data` | 镜像/日志/产物/DB 根目录，生产要挂载卷 |
| `ISOBUILDER_DB_URL` | 空 | PostgreSQL 连接串；**留空则用内置 SQLite**（`$DATA_DIR/isobuilder.db`） |
| `ISOBUILDER_AUTH_TOKEN` | 空 | 设置后 API 需要 `Authorization: Bearer` |
| `ISOBUILDER_S3_*` | 空 | 对象存储的首次默认值；**推荐改在 Web「系统配置」里维护** |
| `ISOBUILDER_S3_PUBLIC_BASE` | 空 | 有公共基地址则跳过预签名，直接拼 URL |
| `ISOBUILDER_S3_PRESIGN_TTL_SECONDS` | `86400` | 预签名 URL 有效期 |
| `ISOBUILDER_MAX_CONCURRENT_BUILDS` | `1` | 并发构建数（每个构建都占 2×ISO 空间 + IO） |
| `ISOBUILDER_MAX_UPLOAD_BYTES` | `40GiB` | 单文件上传上限 |
| `ISOBUILDER_BUILD_TIMEOUT_MINUTES` | `180` | 单个构建超时 |

## 目录结构

```
isobuilder/
├── deploy/                 交付
│   ├── install.sh          裸机：装依赖/二进制/systemd，端口占用自动避让
│   ├── release.sh          打裸机发布包（交叉编译 + 前端 + 模板 + 脚本）
│   ├── build-image.sh      打容器镜像，tag = <版本号>-<commit>-<时间>
│   ├── k8s/                Kubernetes 清单（共享 NFS PV/PVC + Deployment + Service）
│   ├── isobuilder.service  systemd 单元
│   └── env.example         /etc/isobuilder/env 模板
├── containerlize/          容器构建路径（三种，详见 JDOS.md）
│   ├── standalone/         通用运行时镜像（宿主机出产物，容器只装工具+拷贝）
│   ├── base/               编译基础镜像（jdos base_image）
│   └── app/                jdos 应用镜像（三阶段）
├── docker-compose.yml
├── backend/                Go（标准库 mux + SQLite/PostgreSQL + S3 SDK）
│   ├── cmd/isobuilder/     main
│   └── internal/
│       ├── builder/        五段 ISO 流水线（原 repack-iso-*.sh 的原语移植）
│       ├── buildlog/       按字节偏移断点续读的 SSE 日志流
│       ├── config/         环境变量解析
│       ├── hosts/          hosts.yaml 校验（SN 唯一性/MAC/网关同网段等）
│       ├── httpapi/        REST API + SPA 静态文件
│       ├── model/          领域模型
│       ├── settings/       系统配置（对象存储），DB 持久化 + 运行时热切换
│       ├── storage/        S3 兼容对象存储 / 本地文件 / 可热替换的 Switcher
│       └── store/          SQLite / PostgreSQL 双方言（ISOBUILDER_DB_URL 切换）
└── frontend/               SoybeanAdmin（Vue3 + Vite + Naive UI + UnoCSS）
```

本目录的 ISO 流水线不是复制版的脚本逻辑——它是 `repack-iso-*.sh` 在容器里
不可 loop 挂载之外的统一到 Go 的实现：参数序、grub 命令自动识别、Inspur BMC
的四个坑位（`-eltorito-alt-boot`、`-isohybrid-mbr`、`isohybrid` 顺序、
`implantisomd5 --force`）与脚本保持一致，验收通过后才能上生产。

原脚本仍然保留在仓库根目录作为单机应急工具，与镜像内 `/opt/isobuilder/scripts` 同源。

## 已知边界

- 解包使用 `xorriso -osirrox`（无 loop 挂载、无特权），代价是每构建需要 ~2×ISO 的工作空间，`/opt/isobuilder/data/work` 留足（预检失败会先报错）。
- 产物上传对象存储后**自动删除服务器本地副本**（`ISOBUILDER_KEEP_LOCAL_ARTIFACT=true` 可保留），容器仅有基础镜像与当前正在构建的一份占用。
- 日志断点续读按字节 offset，客户端重连不丢行；服务进程重启会中断流（进程重启时未完成的构建会被标记为失败，不会假死）。
- 预签名 URL 过期后 `GET /api/artifacts/{id}`（界面上「刷新链接」）自动续签。
- 改「系统配置」只影响**之后**的构建与链接签名；历史产物仍指向它当时所在的 bucket。
- 单机单进程：构建并发由 `ISOBUILDER_MAX_CONCURRENT_BUILDS` 控制，SQLite 单连接串行写。

## 已验证

在裸机 Rocky 9（x86_64，systemd 部署）上跑通，S3 用京东云 OSS：

| 链路 | 结果 |
|---|---|
| 从 OSS 拉取基础镜像 → `downloading→hashing→ready` + SHA256 | ✅ |
| x86_64 构建（BIOS+EFI 双段） | ✅ El Torito 含 BIOS/UEFI 两段，卷标正确 |
| aarch64 构建（EFI-only，插入式改 grub 保留原厂菜单） | ✅ 单 UEFI 段 |
| 主机 JSON 注入 kickstart（SN/hostname/IP/bond/MAC/密码） | ✅ 占位符已被替换 |
| 产物上传 OSS + 预签名 URL 下载 | ✅ 下载文件 sha256 与 API 一致 |
| 系统配置页改 OSS 配置并立即生效（无需重启） | ✅ 探活失败有明确提示 |
| SSE 实时日志（含断线按 offset 续读、终态事件） | ✅ 73 行 / 4.3 KB 完整展示 |
| 36 台主机清单实时校验 + 载荷表 | ✅ |

后端 `go test ./...` 15 项通过（含构建终态、超时文案、卷标解析、主机校验、项目持久化回归）。
