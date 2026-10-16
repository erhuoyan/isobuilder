# jdos 原生构建说明（云翼资源区）

**两个分支各司其职**，构建配置（制品类型 / target）全在 jdos 控制台建任务时填写，不放 `.jdos/jdos_build.yaml`：

| 分支 | 内容 | 任务 | 制品类型 | target |
|---|---|---|---|---|
| `jdos-base-image` | 仅 `Dockerfile`（无源码） | ① 编译基础镜像 | base_image | `builder` |
| `jdos-build` | 应用源码 + `Dockerfile.jdos` + 启停脚本 | ② 应用镜像 | app_image | `builder` |

先在 `jdos-base-image` 分支跑①，产物在「制品管理 → 基础镜像」；再在 `jdos-build` 分支跑②。部署后挂 **NFS 共享存储** 做持久化。

## 资源区与镜像

云翼资源区镜像前缀 `harbor-pro.jd.local`（测试区 `harbor-test.jd.local`）；jdos 构建产物实际推送到 `hub.is.jd.com/<项目名>/`。

---

## ① 基础镜像（分支 jdos-base-image）

装入：Go 1.26.9 / Node 22.12.0 / pnpm 12.6.0。SDK 从京云内网对象存储拉取；GOPROXY 走 `artifactory.jd.com`，pnpm 走 `registry.npmmirror.com`。

控制台：制品类型 base_image，Dockerfile `Dockerfile`，target `builder`，基础镜像选 `jdt-jdos-base/jdt-base-os-openeuler22.03lts:latest`。

## ② 应用镜像（分支 jdos-build）

### jdos 对多阶段 Dockerfile 的改写行为（实测）

1. **注入 git 阶段**：自动插入 `FROM ... as git` 拉代码，所有相对 COPY 改写为 `COPY --from=git ...`；
2. **只替换最后一个 FROM**：运行时阶段的 FROM 被替换为控制台「基础镜像」，阶段名强制 `builder`，`--target builder`；前面的 FROM 原样保留；
3. **注入平台脚本**：时区、安全基线、`clean_export.sh`，ENTRYPOINT 被包一层。

官方「不需要写 FROM」规则只适用于单阶段 Dockerfile。多阶段时编译阶段必须显式 FROM，因此 `Dockerfile.jdos` 的设计：

- web/api 编译阶段**硬编码编译基础镜像 tag**（jdos 不替换非末位 FROM；控制台 build_args 不透传进 FROM）；
- 运行时阶段的 FROM 写 openEuler（本地构建用），**jdos 构建时被控制台「基础镜像」覆盖**——任务②控制台必须选运行时底座。

### 任务②控制台配置

| 项 | 值 |
|---|---|
| 代码库 / 分支 | 本仓库 / `jdos-build` |
| 制品类型 | app_image |
| Dockerfile | `Dockerfile.jdos` |
| target | `builder`（jdos 强制） |
| **基础镜像** | **`jdt-jdos-base/jdt-base-os-openeuler22.03lts:latest`（运行时底座）** |

⚠️ **不要选任务①的编译镜像**当任务②的基础镜像——那会把运行时阶段换成带 Go/Node 数百 MB 的肥镜像。编译镜像由 Dockerfile.jdos 的 web/api 阶段硬编码引用。

### 编译镜像 tag 更新

任务①每次构建产新 tag。工具链更新后：把 `Dockerfile.jdos` 里两处 `FROM hub.is.jd.com/technical-service/yhpoc-ui-base:...` 改为新 tag 提交。

当前编译镜像：`hub.is.jd.com/technical-service/yhpoc-ui-base:v20261016.215657-1a8f8ec5-Q9jAzg`

### 运行时内容

openEuler 22.03 + `xorriso syslinux syslinux-nonlinux isomd5sum openssh-server crontabs iputils curl wget`。四个 ISO 工具包与 Go 代码 `internal/builder/pipeline.go` 的 `exec.LookPath` / `isohdpfx.bin` 路径表完全匹配（部署机实测）。

### jdos 宿管约定（必须）

根目录三脚本拷进 `/home/admin/`（**勿拷 /export**，jdos 拉起时挂本地盘覆盖）：

| 文件 | 职责 |
|---|---|
| `start_container.sh` | 前 28 行平台固定初始化（从 `mirrors.jdfin.local` 注入行云部署公钥，否则控制台目录/启停/网络全报 Permission denied）→ 数据目录赋权 → 切 admin 启动应用 |
| `start.sh` | 控制台「启动」钩子；pgrep 幂等保活，nohup 拉起 isobuilder |
| `stop.sh` | 控制台「停止」钩子；SIGTERM 触发后端优雅退出（CancelAll 构建 → Shutdown 20s → manager.Wait），25s 超时 SIGKILL |

ENTRYPOINT 固定不可改：

```
/home/admin/tini -- bash -c "/usr/sbin/sshd && /usr/sbin/crond && bash /home/admin/start_container.sh && sleep 9999999d && sleep 9999999d"
```

容器常驻（双 sleep 保活），服务由启停钩子管理，不是 docker stop 语义。

---

## 持久化存储：NFS 共享存储

isobuilder 的 `/data` 需要持久化（基础镜像库、构建工作目录、产物、SQLite DB）。jdos 云翼区通过 **NFS 挂载** 实现：

1. **申请**：http://online.cbpmgt.com/ 新建工单 → 搜索「NFS 共享储存申请」→ 按要求填写（挂载点容量按需，单构建占 2× 基础 ISO 大小，多个基础镜像建议 ≥200G）；
2. **审批通过后**：行云部署平台 → 实例列表 → 配置管理 → 新增配置文件 `jdos_fstab`，把工单返回的 fstab 内容原样粘入；
3. **更新上线**：对容器执行更新上线，配置生效后 NFS 挂载到容器内——确保最终落在 `/data`；若工单挂载路径不是 `/data`，在 `jdos_fstab` 里把挂载点写为 `/data`，或改 `ISOBUILDER_DATA_DIR` 环境变量指向实际挂载点。

> NFS 服务端需 `no_root_squash`。旧 K8s 部署文档（`deploy/k8s/`）的 NFS subPath 策略不适用于 jdos 实例部署。

---

## 环境变量与端口

- 服务端口 `8080`，jdos 实例按平台规则配置端口映射/访问
- 环境变量同根 `README.md` 配置表；`ISOBUILDER_DATA_DIR` 默认 `/data`（NFS 挂载点）
- 密钥（`ISOBUILDER_AUTH_TOKEN`、`ISOBUILDER_DB_URL` 等）走 jdos 配置管理注入，不进镜像

## 本地调试

```bash
# 应用镜像（需能拉 hub.is.jd.com 的编译基础镜像）
docker build -f Dockerfile.jdos -t isobuilder:jdos .

# 本地跑（无 jdos 宿管，手工启动）
docker run -d -p 8080:8080 -v /data/isobuilder:/data isobuilder:jdos \
     /opt/isobuilder/isobuilder
```
