# jdos 原生构建说明（云翼资源区）

isobuilder 在京东 jdos 平台分两步构建：先构建**编译基础镜像**（Go + Node + pnpm），再用它构建**应用镜像**（三阶段：前端 → 后端 → 运行时），部署后挂 **NFS 共享存储** 做持久化。

## 资源区与镜像前缀

云翼资源区镜像前缀 `harbor-pro.jd.local`（测试区 `harbor-test.jd.local`）。控制台选基础镜像时必须选已关联到本应用的镜像。

---

## 第一步：编译基础镜像（制品类型 base_image）

| 项 | 值 |
|---|---|
| Dockerfile | `containerlize/base/Dockerfile` |
| 配置 | `containerlize/base/jdos_build.yaml` |
| 基础镜像（控制台选） | `jdt-jdos-base/jdt-base-os-openeuler22.03lts:latest` |
| build_target | `builder` |

装入：Go 1.26.9 / Node 22.12.0 / pnpm 12.6.0。SDK 与 tini 从京云内网对象存储拉取；GOPROXY 走 `artifactory.jd.com`，npm/pnpm 走 `registry.npmmirror.com` 国内源。

> 若控制台只识别 `.jdos/jdos_build.yaml`，在一个独立分支（如 `jdos-base-image`）把 `containerlize/base/jdos_build.yaml` 复制为 `.jdos/jdos_build.yaml`，指向 `containerlize/base/Dockerfile`。

构建产物在「制品管理 → 基础镜像」，记下仓库地址，下一步用。

## 第二步：应用镜像（制品类型 app_image）

| 项 | 值 |
|---|---|---|
| Dockerfile | `Dockerfile.jdos`（根目录） |
| 配置 | `.jdos/jdos_build.yaml` |
| build_target | `runtime` |
| build_args.BUILD_BASE | 第一步产物的仓库地址（**构建前必改**） |

运行时镜像：openEuler 22.03 + `xorriso syslinux syslinux-nonlinux isomd5sum openssh-server crontabs iputils curl wget`。四个 ISO 工具包与 Go 代码 `internal/builder/pipeline.go` 的 `exec.LookPath` / `isohdpfx.bin` 路径表完全匹配（已在部署机容器内实测）。

### jdos 宿管约定（必须，缺一构建/启停就挂）

仓库根目录三个脚本，Dockerfile ADD 进 `/home/admin/`（**勿拷到 /export**，jdos 拉起时挂载本地盘覆盖它）：

| 文件 | 职责 |
|---|---|
| `start_container.sh` | 前 28 行平台固定初始化（从 `mirrors.jdfin.local` 注入行云部署公钥，否则控制台目录/启停/网络全报 Permission denied）→ 数据目录赋权 → 切 admin 启动应用 |
| `start.sh` | 控制台「启动」钩子；pgrep 幂等保活，nohup 拉起 isobuilder |
| `stop.sh` | 控制台「停止」钩子；SIGTERM 触发后端优雅退出（CancelAll 构建 → Shutdown 20s → manager.Wait），25s 超时 SIGKILL |

ENTRYPOINT 固定不可改：

```
/home/admin/tini -- sh -c "/usr/sbin/sshd && /usr/sbin/crond && bash /home/admin/start_container.sh && sleep 9999999d && sleep 9999999d"
```

容器常驻（双 sleep 保活），服务由启停钩子管理，不是 docker stop 语义。

---

## 持久化存储：NFS 共享存储

isobuilder 的 `/data` 需要持久化（基础镜像库、构建工作目录、产物、SQLite DB）。jdos 云翼区通过 **NFS 挂载** 实现：

1. **申请**：http://online.cbpmgt.com/ 新建工单 → 搜索「NFS 共享储存申请」→ 按要求填写（挂载点容量按需，单构建占 2× 基础 ISO 大小，多个基础镜像建议 ≥200G）；
2. **审批通过后**：行云部署平台 → 实例列表 → 配置管理 → 新增配置文件 `jdos_fstab`，把工单返回的 fstab 内容原样粘入；
3. **更新上线**：对容器执行更新上线，配置生效后 NFS 挂载到容器内（默认挂载点即根 `/` 下对应路径——以工单返回为准，确保最终落在 `/data`；若工单指定的挂载路径不是 `/data`，在 `jdos_fstab` 里把挂载点写为 `/data`，或改 `ISOBUILDER_DATA_DIR` 环境变量指向实际挂载点）。

> NFS 服务端需 `no_root_squash`（容器内 admin/root 写入）——工单里说明即可。旧 K8s 部署文档（`deploy/k8s/`）的 NFS subPath 策略不适用于 jdos 实例部署。

---

## 环境变量与端口

- 服务端口 `8080`，jdos 实例按平台规则配置端口映射/访问
- 环境变量同根 `README.md` 配置表；`ISOBUILDER_DATA_DIR` 默认 `/data`（NFS 挂载点）
- 密钥（`ISOBUILDER_AUTH_TOKEN`、`ISOBUILDER_DB_URL` 等）走 jdos 配置管理注入，不进镜像

## 本地调试

```bash
# 基础镜像（需内网对象存储可达，或手工把 SDK URL 换成等价地址）
docker build -t isobuilder-build-base:local -f containerlize/base/Dockerfile .

# 应用镜像
docker build --build-arg BUILD_BASE=isobuilder-build-base:local \
     -f Dockerfile.jdos -t isobuilder:jdos .

# 本地跑（无 jdos 宿管，手工启动）
docker run -d -p 8080:8080 -v /data/isobuilder:/data isobuilder:jdos \
     /opt/isobuilder/isobuilder
```
