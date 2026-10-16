# isobuilder 多副本改造调研

> 状态：方案已调研，未实施。2026-10-16 讨论结论。
> 背景：用户要求「一个镜像、一个 Deployment（可多副本）、共享 NFS+PG」，且构建已是异步后台任务（`Manager.Enqueue` 起 goroutine 立即返回，前端 SSE 订阅进度）。

## 一、结论概览

- 真正卡多副本的不是「队列在内存」一处，而是 **7 个并发/协调卡点**，逐条列在第二节，均带代码位置。
- 好消息：**所有文件/数据状态都在共享 NFS 且按 buildID 隔离**（`work/iso-repack-<buildID>`、`builds/<id>.log`、uploads、isos、artifacts、kickstarts），副本间不会互相踩工作目录。PVC 已是 RWX（`pvc.yaml` 注释明说为多副本留的）。
- 构建输入已全量快照进 build 行（`builds.go:18-19`），`handleRetryBuild` 证明重放幂等——**重启重入队是安全的**，这是所有方案的地基。
- 推荐路线：**先做「单副本 + 重启不丢构建」（方案一），再叠加 DB 租约 + DB 队列升级到「单 Deployment 多副本」（方案二）**。方案一是方案二的子集，两步之间无返工。
- 为什么不用两个 Deployment / 专 Worker 镜像：用户约束单 Deployment。所有副本跑同一二进制，谁抢到 DB 租约谁当构建执行者，其余纯 API。

## 二、现状状态清单（多副本的每个卡点）

| # | 状态 | 代码位置 | 多副本后果 | 严重度 |
|---|---|---|---|---|
| 1 | 启动判死遗留构建在 **migrate()** 里，任何副本连库即触发 | `backend/internal/store/dialect.go:416-420` | N 副本滚动时，新 Pod 一启动把别的 Pod 正在跑的构建全标 failed | 致命 |
| 2 | 队列在进程内存（`slots/cancels/logs`）；`handleCreateBuild` 直接 `Enqueue` 在本 Pod 执行 | `backend/internal/builder/manager.go:25-36`, `httpapi/builds.go:111` | queued 记录没人认领则卡死；或被两个 Pod 各跑一遍 | 致命 |
| 3 | 取消依赖进程内 cancel 表 | `httpapi/builds.go:163` → `Manager.Cancel` | 请求落非执行 Pod → `cancels` 里没有 → 取消静默无效 | 高 |
| 4 | SSE 日志流对「非本地构建」直接发 end 事件 | `httpapi/builds.go:320-323`（`Subscribe` 返回 false 且未终态时直接 finish） | 落非执行 Pod 的日志请求刷一下就停。**日志文件本身在 NFS 共享，只是没 tail** | 中 |
| 5 | 分片上传会话表在内存（`uploadRegistry.byID`） | `httpapi/isos.go:43` | 非起始 Pod `get` miss → 404。**半成品文件其实躺在共享 NFS 上** | 中 |
| 6 | 上传完成后 hashISO goroutine 也是进程内后台任务 | `httpapi/isos.go:240` | Pod 中途死 → iso 永久卡 `hashing`。和构建是同一类「后台任务孤儿」问题 | 中 |
| 7 | 对象存储热切换只切本 Pod | `httpapi/settings.go:123` `blob.Set()` | 改配置后只有处理请求的副本生效，其余旧配置直到重启 | 低 |

补充：`registerFromS3` 拉镜像、`hashISO` 同属后台任务孤儿类，方案二的「leader 接管」机制应统一处理。

## 三、方案对比

### 方案一：单副本 + 重启不丢构建（基础款，先做）

改动（~60 行）：

1. **删掉** `migrate()` 里的启动判死（#1）——判死逻辑属「进程生命周期」，不该在每次 store open 时跑。
2. **新增** 启动重入队：`UPDATE builds SET status=queued, started_at='', error='', progress=0 WHERE status=running`。输入已快照，从头重跑正确（`handleRetryBuild` 已证明）。artifact 半成品无所谓——pipeline 会重建。

效果：Pod 死了/滚动重启，构建自动重跑，不丢。**但仍是单副本**：不扩容、API 不可多实例、Deployment 仍需 Recreate。

风险：无新并发竞争（单副本）。唯一注意：重入队后旧 workDir 里可能有半成品——`cleanup` 只在成功路径 defer 里跑，重入队时 pipeline 会先 `os.MkdirAll(workDir)` 复用旧目录。需要重入队时清掉旧 `iso-repack-<id>` 目录，避免脏文件残留。

### 方案二：DB 租约选主 + DB 队列（目标形态，后做）

N 副本跑同一逻辑，一条 DB 租约决定构建执行者：

```
Settings 表新增一行 build_lease {holder, expires_at}
每副本每 5s 尝试：
  UPDATE build_lease SET holder=?, expires_at=now()+15s
  WHERE holder=? AND expires_at>now()   -- 续租（我是 holder）
     OR expires_at<=now()               -- 或抢占（租约过期）
拿到租约的副本 = leader（构建执行者）：轮询 builds WHERE status=queued 认领
其余副本 = 纯 API
```

逐条卡点处理：

- **#1 判死/重入队**：从 migrate() 挪走，改为 **「新 leader 上任时」** 的接管动作：running→queued 重入队。非 leader 不碰。复用方案一的逻辑。
- **#2 队列**：`handleCreateBuild`/`handleRetryBuild` 只插 `queued` 行，删 `Enqueue` 调用。leader 轮询认领。认领用乐观 UPDATE（所有议者并发执行，`UPDATE builds SET status=running, started_at=now() WHERE id=? AND status=queued`，`RowsAffected==1` 即认领成功）——这个写法 SQLite 和 PG 都原子，**不用** FOR UPDATE SKIP LOCKED（那是 SELECT 挑行用的，这里 UPDATE 本身带行锁，两方言天然兼容，契合 `dialect.go` 的方言中立设计）。认领延迟 = 轮询间隔，3-5s 秒级可接受。
- **#3 取消**：builds 表加 `cancel_requested` 列。API：queued 直接终态化（无进程持有）；running 置标志位。leader 在 Manager 层 ticker 轮询自己的活跃构建，标志位为真则 cancel 本地 ctx，走现有 `finishFailed`（状态已映射 `BuildCanceled`，`manager.go:97-98`）。
- **#4 日志**：`streamLog` 对「Subscribe miss 且未终态」的构建，改为**轮询 tail 共享日志文件**（NFS 上 `BuildDir/<id>.log`，读 size 增量，现有 15s 心跳 ticker 里本就在查 DB 终态，`builds.go:346`，天然逃生通道）。替换现在「直接发 end」的错误分支。注意：重入队后日志文件会被 `buildlog.Create` truncate，SSE 客户端按 offset 来，可能错位——end 事件带 `next_offset`，前端已按此重放（`builds.go:391` 的 `next_offset` 字段），风险可控。
- **#5 上传**：**文件即真相**：`uploadRegistry.get` miss 时，用 DB 里 iso 的 `path`+`os.Stat(path)` 的 size 重建 `Received`，重建内存条目（上传会话的元数据本来就在 iso 行里，`handleStartUpload` 建过一行 status=uploading 的 iso 记录）。零代码方案升级为「重建缓存」，不需要 sessionAffinity、不需要 DB 化注册表。
- **#6 hashISO**：leader 接管时扫描：`status=uploading OR hashing` 且 `created_at` 超过阈值（如 2h）→ 标 failed。或更简单：hashing 超时标记后由用户重传。治标但足够（hash 是纯计算，重传/重注册即可）。
- **#7 设置热切换**：`Server` 加轻量 ticker（10s），比较 `settings.updated_at`，变了就 `settings.Load` + `blob.Set`。十行内。比通知机制简单可靠。

K8s 侧（deploy/k8s/deployment.yaml）：
- `replicas` 可调（默认仍 1；>1 时**必须** `ISOBUILDER_DB_URL` 非 PG 走 SQLite——租约/认领依赖 PG 行锁并发语义，SQLite `SetMaxOpenConns(1)` 单进程不适合多实例）。
- `strategy` 从 Recreate 改 **RollingUpdate**：API 无进程内构建状态（leader 挂了租约过期，幸存副本接管重入队），NFS RWX 多挂载点本就允许。
- 保留 `terminationGracePeriodSeconds: 60`：被驱逐的 holder 收 SIGTERM → `CancelAll` + `Wait` → 平滑交棒。
- Service 无需 sessionAffinity（上传重建已去掉会话粘性需求）。

量级：~450-550 行（store 租约/认领/取消标志；Manager 轮询循环；streamLog tail；upload 重建；启动改造；deployment 清单）。

风险点：
- 轮询循环的正确性（认领竞态靠 UPDATE 原子性兜底）
- 构建不可断点续传，接管 = 从头来（xorriso 本性，接受）
- 轮询间隔决定取消/认领/接管延迟，需参数化（默认 3-5s，环境变量 `ISOBUILDER_LEASE_POLL_SECONDS` / `ISOBUILDER_LEASE_TTL_SECONDS`）
- 老版本 SQLite 裸机部署零影响：SQLite 下跳过租约，直接走「本进程即 leader」的快速路径（`ISOBUILDER_DB_URL` 为空时租约仪表不建、认领逻辑退化）

### 方案三：K8s Lease + client-go leader election — 不取

机制同二，租约放 K8s Lease 对象。缺点：二进制耦合 K8s API、需 RBAC/ServiceAccount、裸机部署形态变复杂（要 env 检测降级）。方案二用已有 PG 达到同样效果，对裸机零侵入。

### 方案四：Job-per-build — 不取

API 造 K8s Job（同镜像 entrypoint 加 `build <id>` 子命令）。最优雅但最重：Job 生命周期对账、RBAC、产物/日志归属，且彻底锁死 K8s 部署。与运维工具体量不匹配。

## 四、实施顺序建议

1. **方案一**（可独立上线）：删判死 + 启动重入队 + 清旧 workDir。单副本也受益（重启不丢构建），当天完成验收。
2. **方案二**（叠加）：store 加租约/认领/取消标志 → Manager 轮询 → streamLog tail → upload 重建 → 设置 ticker → deployment 清单 + README 更新。
3. 验收基线：`cd backend && go test ./...`（18 项现有测试）全绿；新增租约认领/接管/取消/上传重建的行为测试；PG+SQLite 双方言。

## 五、待拍板决策点

1. 轮询间隔默认值（租约心跳/TTL、认领间隔、取消检查间隔）。
2. SQLite 是否允许走同一多副本路径——**建议不允许**：SQLite 保持单副本语义，租约逻辑仅 PG 下启用。
3. `hashISO` 孤儿的处理策略：leader 接管扫描超时 hashing 行标 failed，还是提供手动重传入口。
4. 重入队是否清旧 workDir——**建议清**：避免重跑时脏文件残留。
