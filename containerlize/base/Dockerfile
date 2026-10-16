# isobuilder 编译基础镜像：Rocky Linux 9 minimal + Go 1.26.9 + Node 22.12.0 + pnpm
#
# jdos 原生构建（制品类型 base_image）：.jdos/jdos_build_base.yaml 指向本文件。
#
# 实验：FROM 用外网公共镜像 docker.io/rockylinux:9-minimal，验证 jdos 构建机
# 是否有外网出口（能拉 docker.io + 访问外网 dnf 源）。失败则回退内网
# jdt-jdos-base/jdt-base-os-openeuler22.03lts（git 历史里那份）。
#
# 本地调试：
#   docker build -t isobuilder-build-base:latest -f containerlize/base/Dockerfile .

FROM docker.io/rockylinux/rockylinux:9-minimal AS builder

# 编译期依赖：
#   microdnf 是 minimal 变体的包管理器（无完整 dnf）
#   curl 下载 SDK（minimal 不自带）
#   xz 解压 Node 的 .tar.xz；git/ca-certificates 供 go mod 与 https 使用
RUN microdnf install -y --setopt=install_weak_deps=False \
        curl xz git ca-certificates && \
    microdnf clean all && rm -rf /var/cache/yum /var/cache/dnf

# Go 1.26.9（SDK 托管京云内网对象存储）
# GOTOOLCHAIN=local：禁止运行时去 proxy.golang.org 拉 toolchain（内网会卡死）。
ENV GOTOOLCHAIN=local
RUN curl -fsSL https://s3-internal.cn-north-1.jdcloud-oss.com/0-yunhai-poc/iso-builder/go1.26.9.linux-amd64.tar.gz \
        -o /tmp/go.tgz && \
    rm -rf /usr/local/go && \
    tar -xzf /tmp/go.tgz -C /usr/local && rm -f /tmp/go.tgz
ENV GOROOT=/usr/local/go \
    PATH=/usr/local/go/bin:$PATH

# Go module 代理走京东内网 artifactory
ARG GOPROXY=http://artifactory.jd.com/api/go/go_virtual
ENV GOPROXY=${GOPROXY} GOSUMDB=off

# Node 22.12.0（托管在内网对象存储；linux-x64 → 仅 x86 架构构建）
RUN curl -fsSL https://s3-internal.cn-north-1.jdcloud-oss.com/0-yunhai-poc/iso-builder/node-v22.12.0-linux-x64.tar.xz \
        -o /tmp/node.tar.xz && \
    rm -rf /usr/local/node-v22.12.0-linux-x64 && \
    tar -xJf /tmp/node.tar.xz -C /usr/local && rm -f /tmp/node.tar.xz
ENV PATH=/usr/local/node-v22.12.0-linux-x64/bin:$PATH

# pnpm：直接 npm 全局安装，绕开 corepack（签名验证与镜像源不兼容）
RUN npm install -g pnpm@12.6.0 --registry=https://registry.npmmirror.com
