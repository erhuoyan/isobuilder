#!/bin/bash
# isobuilder 真实启动脚本（位于镜像层 /home/export/App/bin/start.sh，运行时可改：
# 改完在控制台点「停止/启动」即生效，无需重打镜像）。
#
# ***** USER CODE START（isobuilder 专属：APP_HOME/BIN/LOG_DIR/env。换项目改此段，到 END 为止）*****

# 应用主目录与可执行文件（二进制烤在镜像里，不能放 /export——拉起时被挂载覆盖）
APP_HOME=/opt/isobuilder
BIN=isobuilder
LOG_DIR=/export/log/isobuilder

export ISOBUILDER_ADDR="${ISOBUILDER_ADDR:-:8080}"
export ISOBUILDER_DATA_DIR="${ISOBUILDER_DATA_DIR:-/opt/isobuilder/data}"
export ISOBUILDER_WEB_DIR="${ISOBUILDER_WEB_DIR:-$APP_HOME/web}"
export ISOBUILDER_BUILTIN_DIR="${ISOBUILDER_BUILTIN_DIR:-$APP_HOME/kickstarts}"

# 配置文件：默认 /home/export/App/cfg/config.yaml，存在即用 -c 传入（无需任何环境变量）；
# ISOBUILDER_CONFIG 可指向别处。文件里放一个键 database_url 即可。
# 文件不存在则不带 -c，完全走环境变量，行为与以前一致。
CONFIG_FILE="${ISOBUILDER_CONFIG:-/home/export/App/cfg/config.yaml}"
CONFIG_ARG=""
if [ -f "$CONFIG_FILE" ]; then
    CONFIG_ARG="-c $CONFIG_FILE"
fi

# ***** USER CODE END *****

# 自己的诊断行写 app-start.log（boot.log 由链首聚合，start.log 由上层 start.sh 写——
# 各写各的文件，互不重复；stdout 继续向上流进 boot.log）。
OWN_LOG=$LOG_DIR/app-start.log
log() { echo "[$(date '+%F %T')] [bin/start.sh] $*" | tee -a "$OWN_LOG" 2>/dev/null || echo "[$(date '+%F %T')] [bin/start.sh] $*"; }

log "===== 真实 start.sh begin ====="
log "user=$(id -un) uid=$(id -u)"
log "APP_HOME=$APP_HOME BIN=$BIN LOG_DIR=$LOG_DIR"
log "ADDR=$ISOBUILDER_ADDR DATA_DIR=$ISOBUILDER_DATA_DIR"
log "DB_URL 注入: $([ -n "$ISOBUILDER_DB_URL" ] && echo 是 || echo 否)"
DB_HOST=$(printf '%s' "${ISOBUILDER_DB_URL:-}" | sed -n 's#.*@\([^:/?:]*\).*#\1#p')
log "DB_URL host: ${DB_HOST:-未设置(用SQLite)}"
log "配置文件: $CONFIG_FILE（$([ -n "$CONFIG_ARG" ] && echo '已启用 -c' || echo '不存在，走环境变量')）"

mkdir -p "$LOG_DIR" 2>&1 | while read -r l; do log "mkdir: $l"; done || log "mkdir $LOG_DIR 失败 rc=$?"

PIDPROC=$(pidof "$BIN" || true)
if [[ -z "$PIDPROC" ]]; then
    log "进程不存在，执行启动"
    log "命令: $APP_HOME/$BIN $CONFIG_ARG >> $LOG_DIR/$BIN.log 2>&1 &"
    cd "$APP_HOME" && "$APP_HOME/$BIN" $CONFIG_ARG >> "$LOG_DIR/$BIN.log" 2>&1 &
    LAUNCH_RC=$?
    sleep 2
    log "nohup 段返回 rc=${LAUNCH_RC}，2s 后进程: $(pidof "$BIN" || echo 无)"
    log "服务日志尾部（最新5行）：$(tail -5 "$LOG_DIR/$BIN.log" 2>/dev/null | tr '\n' '|')"
else
    log "(pid:$PIDPROC) 已在运行，跳过"
fi
log "===== 真实 start.sh end ====="
