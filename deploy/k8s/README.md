# isobuilder on Kubernetes

单副本 Pod + 云数据库 + NFS 动态供给卷的部署形态。

```
                    ┌──────────────────────────────┐
   浏览器 ──────────→│ Service isobuilder:8080      │
                    │  (ClusterIP / Ingress)       │
                    └──────────────┬───────────────┘
                                   │
                    ┌──────────────▼───────────────┐
                    │ Deployment isobuilder (×1)   │
                    │  ISOBUILDER_DB_URL ───────────────→ 云 RDS PostgreSQL
                    │  /data ← PVC isobuilder-data │
                    └──────────────┬───────────────┘
                                   │  RWX
                    ┌──────────────▼───────────────┐
                    │ StorageClass                  │
                    │   managed-nfs-storage         │
                    │   ↓ 动态供给                   │
                    │ NFS /data/nfs-provisioner/    │
                    │   <ns>-<pvc>-<pv>/            │
                    └──────────────────────────────┘
```

## 1. 前置

**NFS 动态供给**：集群已装好 nfs-subdir-external-provisioner（本集群部署在
`kube-system/nfs-client-provisioner`，StorageClass 名为 `managed-nfs-storage`），
后端 NFS 配置为：

| 项 | 值 |
|---|---|
| server | `192.168.150.1` |
| path | `/data/nfs-provisioner` |
| 导出 | `/data/nfs-provisioner 192.168.150.0/24(rw,sync,no_subtree_check,no_root_squash)` |

因此**只需声明一个 PVC**（`storageClassName: managed-nfs-storage`），provisioner 会自动
创建 PV 并在 NFS 上分配本 PVC 专属子目录（命名 `<namespace>-<pvc>-<pv>`）。
不需要手工建 PV，也不需要 `subPath` / initContainer 去切目录——每个服务声明自己的 PVC 即可。

- `no_root_squash` 必需：Pod 以 root 运行（ISO 流水线预期如此），否则 NFS 会把 root 映射成匿名用户，写不进去。
- ⚠️ **该 StorageClass 的 `reclaimPolicy` 是 `Delete`，且 provisioner 未启用 `archiveOnDelete`：删除 PVC 会真删 NFS 上对应子目录的数据。** 升级应用只换镜像 tag，绝不要删 PVC。

**数据库**：云 RDS 上建好库（本项目用 `isobuilder` 库）。应用首次启动自动在默认 `public`
schema 建 6 张表（`isos` / `kickstarts` / `projects` / `builds` / `artifacts` / `settings`）。

## 2. 部署

```bash
# 构建并推送镜像（Apple Silicon 交叉构建 amd64，无需 x86 机器）
REGISTRY=hub-pub.jdcloud.com/iaas-jf ./deploy/build-image.sh 0.1.10 --push

kubectl create namespace isobuilder

cp deploy/k8s/secret.example.yaml deploy/k8s/secret.yaml
# 编辑 secret.yaml 填入真实 DSN，然后：
kubectl -n isobuilder apply -f deploy/k8s/secret.yaml

kubectl -n isobuilder apply -f deploy/k8s/pvc.yaml
kubectl -n isobuilder apply -f deploy/k8s/deployment.yaml
kubectl -n isobuilder apply -f deploy/k8s/service.yaml

kubectl -n isobuilder get pods -w
```

`deployment.yaml` 里的镜像地址已是 `hub-pub.jdcloud.com/iaas-jf/isobuilder:0.1.10-...`。
PVC 由 StorageClass 动态供给，先 apply PVC 再 apply Deployment。

验证：

```bash
kubectl -n isobuilder port-forward svc/isobuilder 8080:8080
curl -s localhost:8080/api/health
curl -s localhost:8080/api/config | python3 -m json.tool   # tools 为 null/[] 才能构建
```

`/api/config` 的 `tools` 字段会列出缺失的构建工具；为 `null` 或 `[]` 表示镜像里工具齐全。

## 3. 为什么副本数恒为 1

不是保守取值，是当前的硬约束：

1. **构建队列是单进程内存队列**。服务启动时会把 DB 里遗留的 `queued`/`running` 构建标记为失败
   （进程随重启消失，它 fork 的 xorriso 也一样）。多副本时，B 副本启动会把 A 副本
   正在跑的构建判死。
2. **共享同一份 `/data`**。两个 Pod 同时挂同一份数据，构建工作目录会互相覆盖。

需要更多吞吐时，提高 `ISOBUILDER_MAX_CONCURRENT_BUILDS`（单 Pod 内并行流水线），
而不是加副本。容量测算见下。

## 4. 存储布局与容量

PVC 上只有三类数据，且只有基础镜像是长期占用：

| 路径（PVC 内） | 内容 | 生命周期 | 量级 |
|---|---|---|---|
| `isos/` | 基础镜像 | 持久 | 每个 ISO 可达 20 GB+ |
| `work/` | 解包树 + 产物 | **单个构建结束即清理** | 峰值 ≈ 2× 基础 ISO × 并发数 |
| `artifacts/` | 产物 | 上传对象存储后立即删本地副本 | 上传后 0 |
| `kickstarts/` | 自定义模板 | 持久 | 小 |
| `builds/` | 构建日志 | 持久 | 小 |

容量公式：

```
卷容量 >= 基础镜像总量 + (最大 ISO × 2 × 并发构建数)
```

`managed-nfs-storage` 不按 PVC 声明限额，真实容量由 NFS 服务器磁盘决定，因此
`pvc.yaml` 里的 `storage: 500Gi` 只是声明（写大些，免得以后扩容要重建 PVC）。

构建**开始前**会做磁盘预检，空间不足直接失败并给出数字，不会写到一半炸掉。

数据库已迁到 RDS，PVC 上不再有 `isobuilder.db`，SQLite 那种"锁文件在 NFS 上"的顾虑不再适用。

## 5. 升级

```bash
REGISTRY=hub-pub.jdcloud.com/iaas-jf ./deploy/build-image.sh 0.1.11 --push
kubectl -n isobuilder set image deployment/isobuilder \
    isobuilder=hub-pub.jdcloud.com/iaas-jf/isobuilder:<新 tag>
kubectl -n isobuilder rollout status deployment/isobuilder
```

`strategy: Recreate` 会先停旧 Pod 再起新 Pod（避免同时挂同一份数据）。

- **PVC 不动**：升级只换镜像，数据原样保留。
- **数据库不动**：PG 在 RDS 上，与应用生命周期完全解耦。
- **回滚**：`kubectl -n isobuilder rollout undo deployment/isobuilder`。

> ⚠️ 严禁 `kubectl delete pvc isobuilder-data`。该 SC 回收策略是 `Delete`，
> 删 PVC 会连带删掉 NFS 上的整个子目录（基础镜像全没）。升级永远不需要删 PVC。

## 6. 数据库：PostgreSQL 与 SQLite 的切换

应用启动时按 `ISOBUILDER_DB_URL` 决定后端：

| 情况 | 行为 |
|---|---|
| `ISOBUILDER_DB_URL` 非空 | 连 PostgreSQL（本部署形态） |
| 未设置 | 退回容器内 SQLite（`$ISOBUILDER_DATA_DIR/isobuilder.db`） |

所以裸机 / 单容器部署（`docker-compose.yml`、`deploy/install.sh`）**零改动**继续能用 SQLite；
K8s 里通过 Secret 注入 PG 连接串。

从旧 SQLite 迁到 PG 时，需要把 6 张表的数据导出再灌入 RDS（应用不会自动迁移存量数据）。

## 7. 排障

| 现象 | 原因 / 处理 |
|---|---|
| PVC 一直 `Pending` | provisioner 不可用或 NFS 不可达。`kubectl -n kube-system logs deploy/nfs-client-provisioner` |
| Pod `Pending`，事件 `FailedMount` | NFS 导出未放行节点网段，或 NFS 服务不可达 |
| Pod `CrashLoopBackOff`，日志 `打开数据库失败` | DSN 错误或 RDS 白名单未放行节点网段 |
| `missing_tools` | 镜像里缺 `xorriso`/`isohybrid`/`implantisomd5`。确认用的是本仓库构建的镜像 |
| 构建报 `磁盘空间不足` | PVC 剩余空间不够 2× 基础镜像，清理旧 ISO 或扩容 |
| 构建卡住不动 | `kubectl -n isobuilder logs -f deploy/isobuilder`，同时看界面里的构建实时日志 |
| 界面日志区「已断开」 | SSE 流被 Ingress 缓冲。在 Ingress 上对该路由设 `nginx.ingress.kubernetes.io/proxy-buffering: "off"` |
