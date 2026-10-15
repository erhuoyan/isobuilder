# isobuilder HTTPS 方案（内部 CA + cert-manager + Gateway 终止）

面向公司内部使用的生产级 HTTPS：**不依赖公网域名**，用内部 CA 签发，
外部 443 纯 TCP 透传，TLS 只在 Envoy Gateway 终止一次。

```
互联网 / 内网
  │  访问 https://isobuilder.baremetal.internal（或 https://114.67.232.32）
  ▼
外部公网 LB / 机器 nginx（TCP 443 透传，不解密）
  │
  ▼  Gateway 的 443 NodePort
Envoy Gateway :443   ← 在这里 TLS 终止（证书来自 cert-manager）
  │
  ▼  HTTPRoute
isobuilder Service :8080
```

## 组件与文件

| 文件 | 作用 |
|---|---|
| `10-ca-bootstrap.yaml` | 自举内部 CA：自签 Issuer → CA 证书 → CA ClusterIssuer |
| `20-server-cert.yaml` | isobuilder 服务端证书（SAN 含域名 + IP，自动续期） |
| `30-referencegrant.yaml` | 授权跨命名空间读 Secret（Gateway 在 envy-gateway-system） |
| `40-gateway-https-listener.yaml` | 给共享 Gateway 加 443 listener（TLS 终止） |
| `50-http-redirect.yaml` | 80 → 443 强制跳转 |

## 前置：安装 cert-manager

cert-manager 不在本仓库内提供，用官方发布安装（**注意镜像可达性**）：

```bash
# 该集群的节点直连 quay.io 不通，需先把 manifest 里的镜像地址替换为可用代理
curl -sLO https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml

# 例：替换为可用的镜像代理（按你们实际的镜像源改）
sed -i 's|quay.io/jetstack|docker.m.daocloud.io/quay.io/jetstack|g' cert-manager.yaml

kubectl apply -f cert-manager.yaml
kubectl -n cert-manager wait --for=condition=Available deploy --all --timeout=180s
```

> 版本选择：cert-manager 需要匹配集群版本（本集群 k8s v1.35）。v1.16 可能偏旧，
> 若资源 CRD 报版本不兼容，改用更新的稳定版（v1.17/v1.18+）。装完确认：
> `kubectl get crd | grep cert-manager.io`

## 执行顺序

```bash
# 1. 内部 CA（先跑，等 ClusterIssuer Ready）
kubectl apply -f deploy/k8s/tls/10-ca-bootstrap.yaml
kubectl -n cert-manager get certificate isobuilder-internal-ca   # READY 应为 True
kubectl get clusterissuer                                        # internal-ca READY=True

# 2. 服务端证书
kubectl apply -f deploy/k8s/tls/20-server-cert.yaml
kubectl -n isobuilder get certificate isobuilder-tls             # READY=True

# 3. 跨命名空间授权（必须早于 40）
kubectl apply -f deploy/k8s/tls/30-referencegrant.yaml

# 4. Gateway 加 443 listener
kubectl apply -f deploy/k8s/tls/40-gateway-https-listener.yaml
kubectl -n envoy-gateway-system get gateway envoy \
  -o jsonpath='{range .status.listeners[*]}{.name}{" attached="}{.attachedRoutes}{"\n"}{end}'

# 5. 业务路由改挂 https listener（关键，见下）
#    编辑 deploy/k8s/httproute.yaml，给 parentRefs 加 sectionName: https，然后 apply
kubectl apply -f deploy/k8s/httproute.yaml

# 6. 强制跳转
kubectl apply -f deploy/k8s/tls/50-http-redirect.yaml
```

第 5 步不能漏：`deploy/k8s/httproute.yaml` 的 `parentRefs` 要加 `sectionName: https`，
否则 80 端口仍会直接返回业务内容，重定向形同虚设。

## 证书 SAN 与访问方式

`20-server-cert.yaml` 里同时声明了：

```yaml
dnsNames:    [isobuilder.baremetal.internal]
ipAddresses: [114.67.232.32]
```

两种访问方式证书都有效：

| 访问方式 | 前提 | 跳转是否生效 |
|---|---|---|
| `https://isobuilder.baremetal.internal` | 客户端能解析该域名（内部 DNS 或 hosts） | ✅ 80→443 自动跳 |
| `https://114.67.232.32` | 无 | ⚠️ 见下 |

Gateway API 的 `hostnames` 不接受 IP，所以裸 IP 不在跳转规则里。要支持 IP 访问：
- 去掉 `50-http-redirect.yaml` 的 `hostnames`（匹配该 listener 上所有 Host）→ 全局跳转
- 或直接访问 `https://114.67.232.32`（443 本身可用，只是没有 80 跳转）

**建议**：统一用域名访问（更好迁移，将来换公网域名无需改客户端习惯）。

## 让浏览器不再警告：分发根 CA

自签/内部 CA 路径下，必须把**根 CA 证书**装到客户端信任库，否则浏览器报警。
这是 PKI 的本质，cert-manager 只负责签发与续期，不绕过信任。

导出根 CA：

```bash
kubectl -n cert-manager get secret isobuilder-internal-ca \
  -o jsonpath='{.data.ca\.crt}' | base64 -d > internal-ca.crt
openssl x509 -in internal-ca.crt -noout -subject -dates   # 核对
```

分发方式：

| 平台 | 命令 / 方式 |
|---|---|
| Linux | `cp internal-ca.crt /usr/local/share/ca-certificates/ && update-ca-certificates` |
| Windows（单机） | `certutil -addstore -f "ROOT" internal-ca.crt` |
| Windows（企业） | **组策略** 计算机配置 → 策略 → Windows 设置 → 安全设置 → 受信任的根证书颁发机构 |
| macOS | `sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain internal-ca.crt` |
| 浏览器（Firefox） | 独立信任库，需在 `about:preferences#privacy` → 查看证书 → 导入 |
| Java 应用 | `keytool -importcert -cacerts -file internal-ca.crt` |

有域控/MDM 的环境，用组策略或 MDM 推送即可，终端用户零配置——这是企业内部 PKI 的标准形
态。

## 外部 443 透传

外部 LB 只做 TCP 层转发，不解密。若用本机 nginx 做（`stream` 模块已确认可用）：

```nginx
# 追加到 /joyops/infra/conf/nginx.conf（与 http{} 平级）
stream {
    upstream k8s_gateway_https {
        server 192.168.150.11:30443;   # Gateway 443 的 NodePort，先确认实际端口
    }
    server {
        listen 443;
        proxy_pass k8s_gateway_https;
        proxy_timeout 3600s;           # 构建日志 SSE / 大文件传输
    }
}
```

确认 Gateway 443 的实际 NodePort：

```bash
kubectl -n envoy-gateway-system get svc | grep envoy-envoy
# PORT(S) 会形如 80:30080/TCP,443:30443/TCP —— 取 443 对应的那个端口
```

用云 LB 的话，监听 TCP:443 转发到任一节点的该 NodePort 即可（健康检查走 TCP）。

## 续期与监控

cert-manager 自动续期（服务证书到期前 30 天、根 CA 前 1 年）。要监控：

```bash
# 证书状态（NotAfter / RenewalTime）
kubectl -n isobuilder get certificate isobuilder-tls -o wide

# 续期失败的原因
kubectl -n isobuilder describe certificate isobuilder-tls
kubectl -n isobuilder get events --field-selector involvedObject.name=isobuilder-tls
```

生产建议对 `Certificate` 的 `Ready` 条件做告警（cert-manager 会在续期失败时写事件）。

## 将来有公网域名：平滑切到 Let's Encrypt

架构不需要动，只加一个 Issuer 并改一行引用：

```yaml
# 1) 新增 ACME Issuer（DNS-01 或 HTTP-01）
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: ops@yourdomain.com
    privateKeySecretRef: { name: letsencrypt-prod-account-key }
    solvers:
      - http01: { ingress: { class: envoy } }     # 或 dns01（推荐，支持泛域名）
---
# 2) 把 20-server-cert.yaml 的 issuerRef.name 改成 letsencrypt-prod，
#    dnsNames 换成公网域名 —— 其他（Gateway、透传、跳转）全不动
```

## 排障

| 现象 | 原因 / 处理 |
|---|---|
| `Certificate` 长期不 READY | `kubectl -n isobuilder describe certificate isobuilder-tls` 看事件；CA ClusterIssuer 的 Secret 是否在 cert-manager 命名空间 |
| Gateway listener `ResolvedRefs=False` | ReferenceGrant 缺失或不匹配（30 号文件），或 Secret 名/命名空间写错 |
| 443 连不上 | Gateway 是否真的监听了 443（看 Service 端口）；外部透传目标端口是否正确 |
| 浏览器仍警告 | 根 CA 未装到该客户端（Firefox/Java 有独立信任库） |
| 证书域名不匹配 | 访问用的地址不在 SAN 里；补 `dnsNames`/`ipAddresses` 后重新签发 |
| 80 访问没跳转 | 业务 HTTPRoute 未加 `sectionName: https`，或 redirect 路由 hostnames 不含该 Host |
