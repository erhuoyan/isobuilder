# 项目进度与交接（2026-10-09）

> 下次继续从这里接手。所有东西都已提交在 git 里（main 分支）。

## 一、这个仓库是什么

**isobuilder** —— 自动装机 ISO 构建平台（独立仓库，不依赖 iso-tools）。

浏览器建项目（绑基础镜像 + kickstart + 主机清单 SN/MAC）→ 一键构建 →
产物自动上传对象存储（S3 兼容）→ 返回预签名下载 URL 给交付工程师。

```
浏览器 ──→ Go 控制面（HTTP API + SQLite + SSE 日志）
              └─ fork xorriso / isohybrid / implantisomd5（五段流水线）
                     └─ 产物 → S3 兼容对象存储（京东云 OSS 已验证）
```

**明确不引入 K8s**（已对齐）：单 systemd 服务 + 子进程就是终态。

## 二、已完成并验证的东西

### 部署形态（裸机，主路径）
- systemd：`/opt/isobuilder`（程序）+ `/var/lib/isobuilder`（数据）+ `/etc/isobuilder/env`
- root 运行（2026-10-09 确认），监听 `:8081`（8080 被占自动避让）
- 解包用 `xorriso -osirrox`，不需要特权/loop 设备
- 磁盘预检：不够 2×ISO 会直接报错，不会写一半炸掉

### 已验证的功能（真机 + 京东云 OSS 实测）
| 功能 | 状态 |
|---|---|
| 从 OSS 拉基础镜像（s3://bucket/key）→ downloading→hashing→ready | OK |
| x86_64 构建（BIOS+EFI 双段 El Torito，卷标正确） | OK |
| aarch64 构建（EFI-only，插 grub entry 保留原厂菜单） | OK |
| 主机 JSON 注入 kickstart（SN/MAC/IP/bond） | OK |
| 产物上传 OSS + 预签名 URL 下载（SHA256 一致） | OK |
| 系统配置页改 OSS 并热生效（HeadBucket 探活） | OK |
| SSE 实时日志（offset 断点续读） | OK |
| 36 台主机清单实时校验 | OK |
| 本地产物上传后自动清理 | OK |

### 架构与响应格式
- 后端 Go 1.24（标准库 mux + modernc.org/sqlite + aws-sdk-go-v2），18 项单测
- 前端 SoybeanAdmin v2.2.0 release tarball（官方版，非 main 分支）
- 响应统一信封 {code:"200", message:"ok", data:...}；业务错误也是 HTTP 200，
  靠 code（invalid_hosts / storage_unreachable）区分 —— 与 joyops-infra 惯例一致
- 认证 401 例外：HTTP 401（transport 边界），前端 resetStore → 登录页

### 目录速览
- backend/    Go 源码（cmd/isobuilder + internal/{builder,httpapi,hosts,storage,store,settings,...}）
- frontend/   SoybeanAdmin v2.2.0 + 业务页（views/{project-*,build-detail,iso-library,kickstart-library,settings}）
- kickstarts/ 内置模板（openeuler.ks / rocky9.ks，来自 iso-tools 原工程）
- scripts/    单机应急脚本（repack-iso-*.sh 等，服务运行时不调用）
- deploy/     release.sh 打包 → install.sh 裸机安装 → isobuilder.service
- docs/       API.md（信封契约 + 全部接口）、e2e-synthetic.sh（合成 ISO E2E）
- release/    发布包输出目录（gitignore）

## 三、现在就能用的东西

1. 远端真机已部署：root@114.67.232.32 → systemd isobuilder，端口 8081
   - 数据：/var/lib/isobuilder（目前有 1 个测试 ISO + 1 个 36 台测试项目，可清）
   - OSS 配置：bucket 0-yunhai-poc，prefix iso-builder
2. 本地构建发布包：./deploy/release.sh 0.1.0 → release/*.tar.gz
3. 本地起服务：cd backend && go run ./cmd/isobuilder（需 xorriso 等本机依赖）
4. 界面：镜像库拉镜像 → 新建项目（YAML 校验实时报错）→ 构建（SSE 日志）→ 下载 URL

## 四、还没做的事（按优先级）

### P0 — 交付验收的最后一环（只有现场能做）
- [ ] 用真实发行版 ISO（约 15GB）完整走一次构建（合成 ISO 只验证了流水线环节）
- [ ] 实际引导装机：产物挂 BMC 虚拟光驱 / 写 Ventoy，确认机器自动装完

### P1 — 上生产前建议补
- [ ] 认证：目前默认关。生产建议 /etc/isobuilder/env 设 ISOBUILDER_AUTH_TOKEN，
      前端登录入口会自动出现（已实现并验证）
- [ ] HTTPS / 反向代理：现在裸 HTTP。若走 nginx，注意 SSE 需要 proxy_buffering off
      （后端已发 X-Accel-Buffering: no，一般不用额外配）
- [ ] 备份：/var/lib/isobuilder/isobuilder.db（所有配置/项目/构建历史）+ isos/
- [ ] 产物生命周期：OSS 对象永久保留，需要过期就去对象存储侧配生命周期规则

### P2 — 可选增强（未开工）
- [ ] 项目包导入只认 *.ks + hosts.*，不含驱动/脚本资产
- [ ] 构建历史无分页（一次 50 条，够用很久）
- [ ] 多用户/审计日志（单租户工具）
- [ ] en-us locale 是模板占位未维护（界面全中文）

### 已知限制（设计决定，不是 bug）
- 单机单进程：SQLite 单连接，构建并发默认 1
- 信封错误一律 HTTP 200 + biz code（SoybeanAdmin 约定）；curl 脚本要自己判 code

## 五、常用命令速查

    # 构建 + 部署
    ./deploy/release.sh 0.1.0
    scp release/isobuilder-0.1.0-linux-amd64.tar.gz root@<host>:/root/
    ssh root@<host> 'tar xzf /root/isobuilder-0.1.0-linux-amd64.tar.gz -C /root/rel --strip-components=1 && /root/rel/install.sh --from-release /root/isobuilder-0.1.0-linux-amd64.tar.gz'

    # 服务
    systemctl status isobuilder
    journalctl -u isobuilder -f
    systemctl restart isobuilder

    # 数据
    /var/lib/isobuilder/{isos,work,builds,artifacts,uploads,isobuilder.db}
    /etc/isobuilder/env            # 首次默认值（对象存储推荐走 Web 系统配置页）

    # 测试
    cd backend && go test ./...    # 18 项
    cd frontend && pnpm typecheck && pnpm build

    # E2E（合成 ISO，验证流水线不含真装机）
    E2E_BASE=http://127.0.0.1:8081 E2E_DATA=/var/lib/isobuilder ./docs/e2e-synthetic.sh

## 六、踩过的坑（避免重复踩）

1. 信封化后前端错误处理：业务错误是 HTTP 200 + biz code（不是 HTTP 4xx），
   前端靠 error.response.data.code 分支，别用 HTTP 状态判断
2. VITE_SERVICE_BASE_URL=/ 时：手写 fetch 拼 URL 必须去尾斜杠，
   否则 //api/... 被浏览器当协议相对 URL → 打到主机 api（dev 代理看不出来）
3. SSE 日志：必须 fetch + ReadableStream（要带 Authorization 头），
   不能用 EventSource；断线按 offset 续读
4. 服务用户：若将来收权改非 root，手工放进 isos/ 的文件属主必须改，
   否则登记报 path_unreadable（接口会直接给出 chown 命令）
5. 2.2.0 特有：_builtin 的 403/404/500 会生成路由并进侧栏，router 插件已加 hideInMenu；
   删 src/views/home 后必须同步删 locale 里的 home 键
6. macOS 打包：._*.ks AppleDouble 旁文件会被当模板注册——release.sh 已排除，
   builtin 注册也跳过无占位符的
