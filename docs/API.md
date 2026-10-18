# isobuilder HTTP API 契约 (v1)

> 冻结版本。后端实现、前端 SPA、验收脚本三方共用此文件。
> 所有请求/响应均为 `application/json`（除标注 multipart / 二进制 / SSE 的接口）。

## 通用约定

- Base path: `/api`。前端静态资源由同一进程在 `/` 提供（SPA fallback）。
- 时间戳：RFC3339 UTC 字符串（如 `2026-09-29T07:11:02Z`）。
- **信封**：所有 JSON 响应统一为 `{code, message, data, details?}`（HTTP 恒为 200，与 SoybeanAdmin
  约定一致；错误也用 HTTP 200 + 非成功 code 表示）：
  ```json
  { "code": "200", "message": "ok", "data": { …实际负载… } }                    // 成功
  { "code": "invalid_hosts", "message": "第 3 台主机缺少 sn", "data": {},
    "details": { "hosts": [...] } }                                                 // 业务错误（HTTP 200）
  ```
  成功 `code` 固定为 `"200"`（前端 `VITE_SERVICE_SUCCESS_CODE=200` 即可）。业务错误的 `code`
  取本文档里的稳定标识符（前端据此分支处理），`message` 可直接展示给用户。
  HTTP 4xx/5xx 只用于传输/边界故障（未布署接口、网关、下载不存在的文件等），不走信封。
- 认证（可选，由 `ISOBUILDER_AUTH_TOKEN` 控制）：
  - 开启时除 `GET /api/health` 外全部接口要求 `Authorization: Bearer <token>` 或 `X-Auth-Token: <token>`。
  - 未开启时任意请求均可访问。
  - `GET /api/config` 永远可访问，其 `auth_required` 字段告诉前端是否需要登录。
- ID：服务端生成的 12 位小写十六进制字符串（`[0-9a-f]{12}`）。

## 实体

### Iso（基础镜像）
```json
{
  "id": "3f9a1c0b7d21",
  "name": "Rocky-9.8-x86_64-dvd",
  "filename": "Rocky-9.8-x86_64-dvd.iso",
  "arch": "x86_64",                 // "x86_64" | "aarch64"
  "size_bytes": 16106127360,
  "sha256": "e3b0c442...",          // 校验中/失败时为空字符串
  "status": "ready",                // "uploading" | "downloading" | "hashing" | "ready" | "error"
  "source": "uploaded",             // "uploaded" | "registered" | "s3"
  "path": "/data/isos/Rocky-9.8-x86_64-dvd.iso",
  "created_at": "2026-09-29T07:11:02Z",
  "error": ""                       // status=error 时的原因
}
```

### Kickstart 模板
```json
{
  "id": "0a1b2c3d4e5f",
  "name": "rocky9.ks",
  "arch": "x86_64",                 // "x86_64" | "aarch64" | "any"
  "builtin": true,
  "placeholders": ["%%HOSTS_JSON_PLACEHOLDER%%"],
  "size_bytes": 8123,
  "created_at": "2026-09-29T07:11:02Z"
}
```

### Script（装机后脚本库）
```json
{
  "id": "7f3c1a9b2d4e",
  "name": "安装监控 agent",
  "remark": "所有机器基线",
  "content": "#!/bin/bash\n...",     // 脚本正文
  "size_bytes": 812,
  "created_at": "2026-09-29T07:11:02Z",
  "updated_at": "2026-09-29T07:11:02Z"
}
```

### Project（项目）
```json
{
  "id": "9c8d7e6f5a4b",
  "name": "京西机房 perf 批次",
  "description": "20 台高性能节点一次性装机",
  "arch": "x86_64",
  "iso_id": "3f9a1c0b7d21",
  "iso": { ...Iso... },             // 只读内联展开；iso 被删除时为 null
  "kickstart_id": "0a1b2c3d4e5f",
  "kickstart": { ...Kickstart... }, // 只读内联展开
  "script_id": "7f3c1a9b2d4e",      // 引用的脚本库条目；空表示未引用
  "script": { ...Script... },       // 只读内联展开（不含 content）；未引用或已删除时为 null
  "iso_label": "Rocky-9-8-x86_64",  // 允许为空 → 构建时从原版 ISO 自动探测
  "output_name": "Rocky-9.8-x86_64-auto.iso",  // 允许为空 → 构建时自动生成
  "post_script": "#!/bin/bash\n...", // 项目级装机后脚本，允许为空 → 不注入
  "payload_mode": "off",             // U 盘文件注入：off(默认) | label | auto
  "hosts_format": "yaml",           // "yaml" | "json"  —— hosts_raw 的原始格式
  "hosts_raw": "hosts:\n- sn: ...", // 用户上传/编辑的原文（YAML 或 JSON）
  "hosts_count": 3,                 // 校验通过的主机台数
  "hosts_summary": [                // 解析结果摘要，顺序与 hosts_raw 一致
    { "sn": "YS2B60105814", "hostname": "perf001", "ip": "172.100.7.1",
      "gateway": "172.100.7.254",   // 未配置网关时为空串
      "disk_gb": 480,               // 未配置 disk_target_size 时为 0
      "network_mode": "single",     // "single" | "bond"
      "mac_count": 1, "bmc_ip": "10.126.1.30" }
  ],
  "created_at": "2026-09-29T07:11:02Z",
  "updated_at": "2026-09-29T07:11:02Z"
}
```

### Build（构建任务）
```json
{
  "id": "112233aabbcc",
  "project_id": "9c8d7e6f5a4b",
  "project_name": "京西机房 perf 批次",
  "status": "succeeded",            // "queued" | "running" | "succeeded" | "failed" | "canceled"
  "stage": "seal",                  // 见下方 stage 枚举；未开始为空串
  "stage_index": 5,
  "stage_total": 5,
  "progress": 100,                  // 0-100，由 stage 推算
  "arch": "x86_64",
  "iso_name": "Rocky-9.8-x86_64-dvd",
  "iso_label": "Rocky-9-8-x86_64",
  "output_name": "Rocky-9.8-x86_64-auto.iso",
  "hosts_count": 3,
  "log_bytes": 48213,
  "error": "",                      // status=failed 时的原因
  "artifact": { ...Artifact... },   // 尚未产出或对象存储关闭时为 null
  "created_at": "2026-09-29T07:11:02Z",
  "started_at": "2026-09-29T07:11:03Z",
  "finished_at": "2026-09-29T07:14:20Z"
}
```
`stage` 枚举与 `stage_index`（共 5 段）：
| index | stage | 含义 |
|---|---|---|
| 1 | `mount` | 提取原版 ISO 内容 |
| 2 | `inject` | 注入 kickstart + 主机清单 JSON |
| 3 | `boot` | 改引导配置（grub.cfg / isolinux.cfg） |
| 4 | `build` | xorriso 重建 + isohybrid |
| 5 | `seal` | implantisomd5 + 上传对象存储 |

### Artifact（产物）
```json
{
  "id": "ffeeddccbbaa",
  "build_id": "112233aabbcc",
  "filename": "Rocky-9.8-x86_64-auto.iso",
  "size_bytes": 16108236800,
  "sha256": "9f86d081...",
  "storage": "s3",                  // "s3" | "local"
  "url": "https://oss.example.com/iso-builder/2026/09/Rocky-9.8-x86_64-auto.iso?X-Amz-...",
  "url_expires_at": "2026-09-30T07:14:20Z",  // s3 预签名有效期；storage=local 时为空
  "created_at": "2026-09-29T07:14:20Z"
}
```

## 接口

### 系统
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | `{"status":"ok","version":"0.1.0","time":"..."}` 无需认证 |
| GET | `/api/config` | 见下，无需认证 |
| GET | `/api/settings` | 系统配置（对象存储），密钥脱敏 |
| PUT | `/api/settings` | 更新系统配置并**立即生效** |

```json
{
  "version": "0.1.0",
  "auth_required": false,
  "archs": ["x86_64", "aarch64"],
  "max_upload_bytes": 34359738368,
  "max_concurrent_builds": 1,
  "storage": { "enabled": true, "driver": "s3", "endpoint": "https://oss.example.com",
               "bucket": "iso-builder", "prefix": "iso-builder", "presign_ttl_seconds": 86400 },
  "disk": { "data_dir": "/data", "free_bytes": 812345678901, "iso_dir": "/data/isos",
            "work_dir": "/data/work", "builds_dir": "/data/builds" }
}
```
对象存储未配置时 `storage.enabled=false, driver=""`，产物改由 `GET /api/artifacts/{id}/download` 直传。

### ISO 库
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/isos` | `{"isos":[...]}`，按创建时间倒序 |
| POST | `/api/isos/uploads` | 发起分片上传，body `{"name":"...","filename":"...","arch":"x86_64","size_bytes":16106127360}` → `{"iso":{...status:"uploading"},"upload_id":"u_...","chunk_size":67108864}` |
| PUT | `/api/isos/uploads/{upload_id}?offset=N` | body 为原始字节流（`application/octet-stream`），必须严格顺序追加；`offset` 与已接收字节数不一致 → `409 offset_mismatch` → `{"received_bytes":N}` |
| POST | `/api/isos/uploads/{upload_id}/finish` | 收尾（落盘 + 异步算 sha256）→ `{"iso":{...}}` |
| DELETE | `/api/isos/uploads/{upload_id}` | 取消上传并删除临时文件 |
| POST | `/api/isos/register` | 登记已有 ISO（服务器本地路径，或从对象存储拉取），见下 |
| DELETE | `/api/isos/{id}` | 删除记录与文件（`source=registered` 只删记录）。被项目引用 → `409 iso_in_use` |

`POST /api/isos/register` body（`source` 省略时为 `path`）：
```json
// 1) 服务器本地文件
{ "source": "path", "path": "/data/isos/x.iso", "name": "可选", "arch": "x86_64" }

// 2) 从对象存储拉取（不填 AK/SK 时用「系统配置」里的凭据）
{ "source": "s3", "arch": "aarch64", "name": "可选",
  "s3": { "url": "s3://bucket/iso/openEuler.iso",
          "endpoint": "可选，覆盖系统配置", "region": "可选",
          "bucket": "可选", "key": "可选",
          "access_key": "可选", "secret_key": "可选", "path_style": true } }
```
s3 模式先 `HeadObject` 探活（错 key / 错凭据立刻报错），通过后立即返回 `201 {"iso":{...status:"downloading"}}`，
后台流式下载到本机镜像目录并计算 SHA256（状态流转 `downloading → hashing → ready|error`）。
失败码：`invalid_s3`、`s3_object_unreachable`、`storage_not_configured`、`invalid_source`。

### 系统配置（对象存储）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/settings` | 返回当前生效配置（密钥脱敏，只报告是否已配置） |
| PUT | `/api/settings` | 局部更新并**立即生效**（无需重启） |

```json
// GET /api/settings → 200
{ "storage": {
    "endpoint": "https://oss.example.com", "region": "us-east-1",
    "bucket": "auto-iso", "prefix": "auto-iso", "path_style": true,
    "insecure_tls": false, "public_base": "", "presign_ttl_seconds": 86400,
    "access_key_set": true, "secret_key_set": true,
    "enabled": true,
    "source": "db"            // "db" = 来自配置页（已持久化）；"env" = 仅来自环境变量
  },
  "storage_raw": true }        // 配置页是否已写入过
```

`PUT /api/settings` body（字段可省略 = 不变；**密钥传空串 = 保留原值**，传值 = 覆盖）：
```json
{ "storage": { "endpoint": "...", "region": "...", "bucket": "...", "prefix": "auto-iso",
               "path_style": true, "insecure_tls": false, "public_base": "",
               "presign_ttl_seconds": 86400,
               "access_key": "可选", "secret_key": "可选" } }
```
- `bucket: ""` → 关闭对象存储，产物改存本机 `/data/artifacts`，由 `GET /api/artifacts/{id}/download` 直传。
- 保存前用 `HeadBucket` 探活；连不上 → `400 storage_unreachable`（**配置已保存**，错误信息说明原因）。
- 立即生效范围：后续构建的产物上传、产物下载链接签名。历史产物仍指向它当时的 bucket。
- 校验失败 → `400 invalid_settings`。

### Kickstart 模板
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/kickstarts` | `{"kickstarts":[...]}` |
| POST | `/api/kickstarts` | multipart：`file`（`.ks` 文件）、`name`（可选，默认文件名）、`arch`（`x86_64`/`aarch64`/`any`） |
| GET | `/api/kickstarts/{id}/content` | 模板正文 `{"content":"..."}`，前端可预览 |
| DELETE | `/api/kickstarts/{id}` | 内置模板 → `409 builtin_kickstart`；被项目引用 → `409 kickstart_in_use` |

### 装机后脚本库
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/scripts` | `{"scripts":[...]}`，含正文，按更新时间倒序 |
| POST | `/api/scripts` | JSON body `{"name":"...","remark":"可选","content":"..."}` → 201 `{"script":{...}}`；空名 → `400 invalid_name`，空内容 → `400 empty_script`，超 64KiB → `413 too_large` |
| PUT | `/api/scripts/{id}` | 同上 body（省略字段表示不变）→ `{"script":{...}}` |
| DELETE | `/api/scripts/{id}` | 被项目引用 → `409 script_in_use` |

### 项目
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/projects` | `{"projects":[...]}`（不含 `hosts_raw`，含 `hosts_count`、内联 `iso`/`kickstart`） |
| POST | `/api/projects` | body 见下 → 201 `{"project":{...}}`（含 `hosts_raw`） |
| GET | `/api/projects/{id}` | `{"project":{...}}` |
| PUT | `/api/projects/{id}` | body 同 POST（省略字段表示不变；`hosts_raw:""` 表示清空） |
| DELETE | `/api/projects/{id}` | 级联删除其构建记录；有 running/queued 构建 → `409 project_busy` |
| POST | `/api/validate/hosts` | 只校验不落库，body `{"content":"<yaml|json 原文>","format":"auto"}` → 见下 |
| POST | `/api/projects/import` | 项目包上传（见下），multipart |

项目创建/更新 body：
```json
{
  "name": "京西机房 perf 批次",
  "description": "...",
  "arch": "x86_64",
  "iso_id": "3f9a1c0b7d21",
  "iso_label": "",
  "output_name": "",
  "script_id": "",                  // 可选；引用脚本库；省略 = 不变，空串 = 解除绑定
  "post_script": "",                // 可选；省略 = 不变，空串 = 清除；>64KiB → 400 invalid_post_script
  "payload_mode": "off",            // 可选；"off" | "label" | "auto"，省略 = 不变
  "hosts_raw": "hosts:\n- sn: ...",
  "hosts_format": "auto"            // "auto" | "yaml" | "json"
}
```
校验失败 → `400 invalid_hosts`，`details.hosts = [{index, field, message}, ...]`。

`post_script` / `script_id` 说明：构建时把「脚本库脚本 + 项目追加脚本」合并为一段，以 base64 编码追加为独立的 `%post` 段（chroot 内执行），排在模板原有 `%post` 之后；**库脚本在前，项目追加脚本在后**。合并结果快照进构建记录，之后改脚本库或改项目都不影响已入队的构建。内容、输出与退出码分别落盘到装机机的 `/var/log/isobuilder/post.sh`、`post.log`、`post.exit`。脚本失败不影响安装结果。

`payload_mode` 说明（可选的「从 U 盘注入文件」，默认关闭）：安装时把 U 盘上一批文件拷进新系统，
供装机后脚本使用。开启后模板的 `%%PAYLOAD_SNIPPET%%` 占位符会被替换为对应逻辑，注入位置在
模板已有的 `%post --nochroot` 段内（只有该阶段还能看到安装介质）。

- `off` —— 不注入任何逻辑（默认）。
- `label` —— 只按卷标 `ISO_PAYLOAD` 查找 U 盘分区，找不到就跳过。
- `auto` —— 卷标优先；未命中时扫描非目标磁盘的分区，找含 `payload/` 目录的并挂载。

约定：U 盘上需有一个**独立分区**（Ventoy 用其「保留空间」创建），卷标设为 `ISO_PAYLOAD`
（**恰好 11 字符**——FAT32/exFAT 的卷标字段固定 11 字节，更长会被格式化工具截断或拒绝），
内含 `payload/` 目录；文件最终落到新系统的 `/opt/payload`。
**注意**：Ventoy 自身的数据分区启动后无法挂载（[ventoy/Ventoy#667](https://github.com/ventoy/Ventoy/issues/667)），
因此不能把文件放在放 ISO 的那个分区里。

整个过程是尽力而为：分区不存在、挂载失败或拷贝出错都只记日志，**不会导致装机失败**；
需要彻底排除风险时把 `payload_mode` 设为 `off` 即可。

`POST /api/validate/hosts` 响应：
```json
{
  "ok": false,
  "format": "yaml",
  "hosts_count": 3,
  "hosts_summary": [ ...同 Project.hosts_summary... ],
  "errors": [ { "index": 2, "field": "network.bond.slaves", "message": "bond 至少需要 2 个 slave" } ]
}
```
主机清单校验规则（后端唯一权威实现，前端不重复实现）：
1. 顶层为 `{"hosts":[...]}` 或裸数组 `[...]`；`hosts` 为空 → `empty_hosts`。
2. 每台：`sn` 必填且全局唯一（`duplicate_sn`）；`hostname` 必填、RFC1123 且唯一。
3. `network.ip` 必填且为 IPv4；`network.netmask` 为合法掩码（或 `network.prefix` 0-32）；
   `network.gateway` 若填写须与 `ip` 同网段。
4. 网络模式二选一、不可同时存在也不能都缺：`network.mac`（单口）或 `network.bond`（bond，
   需 `slaves` 为偶数且 ≥2、MAC 合法、`mode` 0-6）。同时存在 → `network_mode_conflict`。
5. 可选字段类型检查：`root_password` 字符串；`disk_target_size` 正整数；
   `partition.boot_size/efi_size` 正整数；`partition.root_fstype` ∈ {xfs, ext4, ext3, btrfs}；
   `partition.use_lvm` 布尔。
6. 未知字段不报错（兼容 redfish-info.sh 输出的额外键），但 `status`、`os` 等透传保留。

项目包上传 `POST /api/projects/import`（multipart）：
- 字段 `file`：`.zip` / `.tar.gz` / `.tgz` / `.tar`，大小 ≤ 64 MiB；其余字段 `name`、`description`、`arch`、`iso_id`、`kickstart_id` 可选。
- 服务端解开后在**任意深度**查找（忽略隐藏文件、`__MACOSX`）：
  - kickstart：扩展名 `.ks`，取字典序第一个；找不到则用请求的 `kickstart_id`。
  - 主机清单：文件名匹配 `hosts*.y(a)ml` / `hosts*.json` / `*.yaml` / `*.yml` / `*.json`，取匹配到的第一个可解析者。
  - 目录穿越/绝对路径成员 → `400 invalid_archive`。
- 响应：201 `{"project":{...}, "detected":{"kickstart":"path/in/archive.ks","hosts":"path/hosts.yaml"}}`；未能识别主机清单 → `400 archive_no_hosts`。

### 构建
| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/projects/{id}/builds` | 入队 → 202 `{"build":{...status:"queued"}}`。项目缺 iso/kickstart/hosts → `400 project_incomplete`（`details.missing=["iso_id","hosts_raw"]`） |
| GET | `/api/builds?project_id=&limit=50` | `{"builds":[...]}`，按创建时间倒序 |
| GET | `/api/builds/{id}` | `{"build":{...}}` |
| GET | `/api/builds/{id}/log?offset=0` | 默认 **SSE**（`text/event-stream`）：逐行 `data: {"offset":N,"line":"..."}`；结束发 `event: end` + `data: {"status":"succeeded","next_offset":N}`；`?format=text` 则返回 `text/plain` 全文。`offset` 为字节偏移，用于断线续读 |
| POST | `/api/builds/{id}/cancel` | 终止进程并清理工作目录，`status` → `canceled`；已结束 → `409 build_finished` |
| POST | `/api/builds/{id}/retry` | 复制配置重新入队 → 202 `{"build":{...}}` |
| GET | `/api/artifacts/{id}` | 刷新预签名 URL（s3 模式）→ `{"artifact":{...}}` |
| GET | `/api/artifacts/{id}/download` | `302 Location:` 预签名 URL；`storage=local` 时直接 `200` 流式返回文件（支持 `Range`） |

## 静态资源
- `GET /` → SPA `index.html`；未知非 `/api` 路径同样返回 `index.html`（前端路由）。
- 构建前端时 `base` 必须为 `/`，产物目录 `backend/web/dist`。
