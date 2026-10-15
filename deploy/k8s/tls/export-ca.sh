#!/usr/bin/env bash
# 导出内部根 CA，并打印各平台导入命令，用于分发给运维客户端。
# 装上 cert-manager 并应用 10-ca-bootstrap.yaml 之后执行：
#   ./deploy/k8s/tls/export-ca.sh [输出文件]
set -euo pipefail

NS="${CA_NS:-cert-manager}"
SECRET="${CA_SECRET:-isobuilder-internal-ca}"
OUT="${1:-internal-ca.crt}"

if ! kubectl get secret "$SECRET" -n "$NS" >/dev/null 2>&1; then
    echo "找不到 Secret ${NS}/${SECRET}。" >&2
    echo "请先安装 cert-manager 并应用 deploy/k8s/tls/10-ca-bootstrap.yaml。" >&2
    exit 1
fi

kubectl -n "$NS" get secret "$SECRET" -o jsonpath='{.data.ca\.crt}' | base64 -d > "$OUT"

echo "=== 已导出：$OUT ==="
openssl x509 -in "$OUT" -noout -subject -issuer -dates
openssl x509 -in "$OUT" -noout -fingerprint -sha256

cat <<EOF

=== 分发：让每台客户端信任这张根 CA（各自执行一次）===

macOS:
  sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain $OUT

Linux (Debian/Ubuntu):
  sudo cp $OUT /usr/local/share/ca-certificates/internal-ca.crt && sudo update-ca-certificates
Linux (RHEL/Rocky/openEuler):
  sudo cp $OUT /etc/pki/ca-trust/source/anchors/internal-ca.crt && sudo update-ca-trust

Windows (管理员 CMD/PowerShell):
  certutil -addstore -f "ROOT" $OUT
Windows 企业批量: 用组策略下发到「受信任的根证书颁发机构」

Firefox（独立信任库，不读系统）:
  设置 → 隐私与安全 → 证书 → 查看证书 → 证书颁发机构 → 导入

Java 应用:
  sudo "\$JAVA_HOME/bin/keytool" -importcert -cacerts -storepass changeit -alias internal-ca -file $OUT

=== 验证（导入前后各跑一次，观察差异）===
  curl --cacert $OUT -sv https://isobuilder.baremetal.internal/ 2>&1 | grep -E "SSL certificate verify|subject:|issuer:"

注意：$OUT 是公开信息（根证书不含私钥），可以自由分发；
      根 CA 的**私钥**在 Secret $NS/$SECRET 里，绝不可导出或外传。
EOF
