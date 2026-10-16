# jdos 原生构建说明（云翼资源区）

全部在 **main 单分支** 上，结构对齐官方参考 `jdos-image-build/go-app`（该示例用 base-image / app-image / package 三个分支，因为没有自定义配置文件名；isobuilder 无包部署需求，只做前两种）。原生构建**不修改 Dockerfile**（所见即所得），支持 build_args 与多阶段。

## 文件布局

```
.jdos/
├── jdos_build.yaml        任务②：应用镜像（app_image）← 平台约定名
└── jdos_build_base.yaml   任务①：编译基础镜像（base_image）
containerlize/
├── base/Dockerfile         编译基础镜像（openEuler 22.03 + Go 1.26.9 + Node 22.12.0 + pnpm 12.6.0）
├── app/Dockerfile         三阶段应用镜像（web → api → runtime）
└── standalone/Dockerfile  通用运行时镜像（docker compose / 手工 buildx / K8s，不进 jdos）
start.sh / stop.sh / start_container.sh   jdos 宿管启停入口（随镜像烤入 /home/admin/）
bin/start.sh / bin/stop.sh                真实启停脚本正本（git 记录；不进镜像，部署时手动复制到 /export/App/bin/）
```

> ⚠️ jdos 约定的配置文件名是 **`.jdos/jdos_build.yaml`**（单个）。一个 yaml 只能定义一个 `artifact_type`，官方示例靠拆分支绕开此限制。本仓库用 `jdos_build_base.yaml` 承载基础镜像任务，**平台不识别非约定名时**在临时分支把该文件改名为 `jdos_build.yaml` 再建任务。

> ⚠️ jdos 约定的配置文件名是 **`.jdos/jdos_build.yaml`**（单个）。一个 yaml 只能定义一个 `artifact_type`，官方示例靠拆分支绕开此限制。本仓库用 `jdos_build_base.yaml` 承载基础镜像任务——**平台若不识别非约定名**，回退用已验证可用的独立分支：`jdos-base-image`（任务①）+ `jdos-build`（任务②，其 `.jdos/jdos_build.yaml` 指向容器镜像教程路径）。

## 两个构建任务

控制台只需配「代码库 / 分支」，其余从 `.jdos/` yaml 读取（代码即构建配置）。

| 任务 | 配置 | 制品类型 | target | 说明 |
|---|---|---|---|---|
| ① 编译基础镜像 | `.jdos/jdos_build_base.yaml` | base_image | `builder` | FROM `jdt-jdos-base/jdt-base-os-openeuler22.03lts:latest` |
| ② 应用镜像 | `.jdos/jdos_build.yaml` | app_image | `runtime` | `build_args.BUILD_BASE` 指向任务①产物 |

先跑①，产物在「制品管理 → 基础镜像」，记下仓库地址；把 `.jdos/jdos_build.yaml` 的 `BUILD_BASE` 改为该地址后跑②。

当前编译镜像：`hub.is.jd.com/technical-service/yhpoc-ui:v20261016.232248-fb7b51ba-h27bdb`（制品名 = 控制台应用名，不带 `-base` 后缀）
## 资源区与镜像

统一使用 `hub.is.jd.com` 仓库地址（实测办公网/部署机/本机均可解析；jdos 构建机实际拉取推送也走此入口，digest 与 harbor-pro 的同源镜像一致）。内网仓库自签名证书：本地构建需在 docker daemon 配置 `"insecure-registries": ["hub.is.jd.com", ...]`。

## 运行时内容

openEuler 22.03 + `xorriso syslinux syslinux-nonlinux isomd5sum openssh-server crontabs iputils curl wget`。四个 ISO 工具包与 Go 代码 `internal/builder/pipeline.go` 的 `exec.LookPath` / `isohdpfx.bin` 路径表完全匹配（部署机实测）。

## jdos 宿管约定（必须）

根目录三脚本随镜像烤入 `/home/admin/`（**勿拷 /export**，jdos 拉起时挂本地盘覆盖）：

| 文件 | 职责 |
|---|---|
| `start_container.sh` | 前 28 行平台固定初始化（从 `mirrors.jdfin.local` 注入行云部署公钥，否则控制台目录/启停/网络全报 Permission denied）→ 数据目录赋权 → 切 admin 调用 `/home/admin/start.sh` |
| `start.sh` | 平台调度器入口：source profile + `start_before/after` 钩子 + `su -m admin -c /export/App/bin/start.sh`。**该目录无脚本即启动失败（CrashLoopBackOff）——失败必须可见** |
| `stop.sh` | 平台调度器入口：`stop_before/after` 钩子 + `/export/App/bin/stop.sh`。**永远成功**：脚本缺失视为服务未运行，进程不在 `pidof` 亦优雅退出 0 |

**首次部署必须动作**：把仓库 `bin/{start,stop}.sh` 复制到实例的 `/export/App/bin/`（平台挂载盘）并 `chmod +x`。之后修改 `/export/App/bin/*.sh`，控制台点停止/启动即生效，无需重打镜像。本地正本更新后同步覆盖即可。

ENTRYPOINT 固定不可改：

```
/home/admin/tini -- sh -c "/usr/sbin/sshd && /usr/sbin/crond && bash /home/admin/start_container.sh && sleep 9999999d && sleep 9999999d"
```

容器常驻（双 sleep 保活），服务由启停钩子管理，不是 docker stop 语义。Noah 堡垒机依赖容器内 sshd + hufu（start_container 的固定初始化已覆盖）。

## 持久化存储：NFS 共享存储

isobuilder 的 `/data` 需要持久化（基础镜像库、构建工作目录、产物、SQLite DB）。jdos 云翼区通过 **NFS 挂载** 实现：

1. **申请**：http://online.cbpmgt.com/ 新建工单 → 搜索「NFS 共享储存申请」→ 按要求填写（挂载点容量按需，单构建占 2× 基础 ISO 大小，多个基础镜像建议 ≥200G）；
2. **审批通过后**：行云部署平台 → 实例列表 → 配置管理 → 新增配置文件 `jdos_fstab`，把工单返回的 fstab 内容原样粘入；
3. **更新上线**：对容器执行更新上线，配置生效后 NFS 挂载到容器内——确保最终落在 `/data`；若工单挂载路径不是 `/data`，在 `jdos_fstab` 里把挂载点写为 `/data`，或改 `ISOBUILDER_DATA_DIR` 环境变量指向实际挂载点。

> NFS 服务端需 `no_root_squash`。日志目录走 `/export/Logs`（jdos 本地盘），应用 stdout 重定向在 start.sh 里写 `/export/log/isobuilder/stdout.log`。

## 环境变量与端口

- 服务端口 `8080`，jdos 实例按平台规则配置端口映射/访问
- 环境变量同根 `README.md` 配置表；`ISOBUILDER_DATA_DIR` 默认 `/data`（NFS 挂载点）
- 密钥（`ISOBUILDER_AUTH_TOKEN`、`ISOBUILDER_DB_URL` 等）走 jdos 配置管理注入，不进镜像

## 本地调试

```bash
# ① 基础镜像（需内网对象存储可达）
docker build -t isobuilder-build-base:local -f containerlize/base/Dockerfile .
# 拉取 jdos 产物做编译镜像：docker pull hub.is.jd.com/technical-service/yhpoc-ui:<tag>

# ② 应用镜像
docker build --build-arg BUILD_BASE=isobuilder-build-base:local \
     -f containerlize/app/Dockerfile -t isobuilder:jdos .

# 本地跑（无 jdos 宿管，手工启动）
docker run -d -p 8080:8080 -v /data/isobuilder:/data isobuilder:jdos \
     /opt/isobuilder/isobuilder
```

## 验证与已知边界

- 任务①已验证通过：产物 `hub.is.jd.com/technical-service/yhpoc-ui:v20261016.232248-fb7b51ba-h27bdb`（含 Go 1.26.9 / Node 22.12.0 / pnpm 12.6.0，npm 全局装 pnpm 绕开 corepack 签名问题）。
- 任务②已实测通过（原生构建，`v20261016.235714-1a6458f4-zti9fx`）：build_args 真透传，ARG BUILD_BASE 三阶段正常解析。首次踩的 `jdos「普通构建」模式改写 Dockerfile 使 ARG 失效` 不存在于原生构建。
- 基础镜像基底只能用平台白名单内网镜像：实测 `docker.io/rockylinux:9-minimal` 在渲染阶段即被拒（`invalid registry: docker.io`）。
