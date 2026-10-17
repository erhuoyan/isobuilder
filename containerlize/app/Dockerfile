# isobuilder — jdos 原生构建应用镜像（云翼资源区）
#
# 原生构建特性（官方文档）：不改写 Dockerfile（所见即所得），build_args 真透传，
# 多阶段构建 buildkit 自动分析依赖并发执行。FROM 来源限于平台可用基础镜像——
# 编译基础镜像（任务①）构建后进「制品管理-基础镜像」，满足条件，用 ARG 引用。
#
# 配置：.jdos/jdos_build.yaml（app_image）→ 本文件，build_args.BUILD_BASE 传编译镜像。
#
# 本地调试（先本地构建或拉取编译镜像）：
#   docker build --build-arg BUILD_BASE=<编译镜像> -f containerlize/app/Dockerfile \
#        -t isobuilder:jdos .
#   docker run -d -p 8080:8080 -v /data/isobuilder:/data isobuilder:jdos \
#        /opt/isobuilder/isobuilder

# ── Stage 1: 前端（编译基础镜像里的 Node/pnpm，JS 产物与架构无关）──────────────
# 编译镜像 tag 直接写死（jdos 的 checkDockerfile 不展开 ARG/build_args 变量，
# 用 ${BUILD_BASE} 会报 imageURLnot match / base name blank）。
# 换 tag：改这里 + Stage 2 的 FROM，两处同一个值。
FROM hub.is.jd.com/technical-service/isobuilder:v20261017.111201-6d203eb8-r55mjk AS web
WORKDIR /src
# 先拷包清单 + .npmrc 装依赖（依赖未变时复用层缓存；build_context=. → COPY 相对仓库根）
# .npmrc 必须与包清单同批到位：它指定 registry=https://registry.npmmirror.com/，
# 晚一步（等 COPY frontend/ 那层）会让 pnpm install 走官方源 npmjs.org，慢到不可用。
COPY frontend/package.json frontend/pnpm-workspace.yaml frontend/pnpm-lock.yaml frontend/.npmrc ./frontend/
COPY frontend/packages ./frontend/packages
RUN cd frontend && pnpm install --frozen-lockfile
COPY frontend/ ./frontend/
# vite build.outDir=../backend/web/dist（相对 frontend 工作目录）→ /src/backend/web/dist
RUN cd frontend && pnpm build

# ── Stage 2: 后端（编译基础镜像里的 Go，CGO 关闭纯静态编译）────────────────────
FROM hub.is.jd.com/technical-service/isobuilder:v20261017.111201-6d203eb8-r55mjk AS api
ARG GOPROXY=http://artifactory.jd.com/api/go/go_virtual
ENV GOPROXY=${GOPROXY} GOSUMDB=off GOTOOLCHAIN=local
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/isobuilder ./cmd/isobuilder

# ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
# ┃ 【模板段】运行时层默认可用；按需改：dnf 工具包（见下行实测路径）+ ENV ┃
# ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
# 四个 ISO 工具包与 Go 代码路径发现匹配（部署机实测）：
#   xorriso / isohybrid / implantisomd5 → /usr/bin；isohdpfx.bin → /usr/share/syslinux/
FROM hub.is.jd.com/jdt-jdos-base/jdt-base-os-openeuler22.03lts:latest AS runtime

RUN dnf install -y --setopt=install_weak_deps=False \
        xorriso syslinux syslinux-nonlinux isomd5sum ca-certificates \
        openssh-server crontabs iputils curl wget && \
    dnf clean all && rm -rf /var/cache/dnf

# tini 防僵尸进程（jdos 约定放 /home/admin/tini）
RUN curl -fsSL http://s3-internal.cn-north-1.jdcloud-oss.com/jdos-build-public/tools/tini/tini-amd64-v0.19.0 \
        -o /home/admin/tini && chmod 0755 /home/admin/tini

WORKDIR /opt/isobuilder
COPY --from=api /out/isobuilder ./isobuilder
COPY --from=web /src/backend/web/dist ./web/
COPY kickstarts/openeuler.ks kickstarts/rocky9.ks ./kickstarts/
COPY scripts/repack-iso-x86.sh scripts/repack-iso-arm.sh ./scripts/

# 启停脚本：
#   根 start.sh/stop.sh/start_container.sh   平台入口，随镜像烤入 /home/admin/
#   bin/{start,stop}.sh                       版本化正本（git 记录），不进镜像；
#                                             部署时手动从仓库复制到 /export/App/bin/
COPY start.sh stop.sh start_container.sh /home/admin/
RUN chmod +x /home/admin/{start,stop,start_container}.sh && \
    chown admin:admin /home/admin/{start,stop,start_container}.sh

# sshd 首跑需生成 host key
RUN ssh-keygen -A

ENV ISOBUILDER_ADDR=:8080 \
    ISOBUILDER_DATA_DIR=/data \
    ISOBUILDER_WEB_DIR=/opt/isobuilder/web \
    ISOBUILDER_BUILTIN_DIR=/opt/isobuilder/kickstarts

VOLUME ["/data"]
EXPOSE 8080

# jdos 约定 ENTRYPOINT（双 sleep 保活；容器常驻，服务由启停钩子管理）
ENTRYPOINT /home/admin/tini -- sh -c "/usr/sbin/sshd && /usr/sbin/crond && bash /home/admin/start_container.sh && sleep 9999999d && sleep 9999999d"
