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
export ISOBUILDER_DATA_DIR="${ISOBUILDER_DATA_DIR:-/data}"
export ISOBUILDER_WEB_DIR="${ISOBUILDER_WEB_DIR:-$APP_HOME/web}"
export ISOBUILDER_BUILTIN_DIR="${ISOBUILDER_BUILTIN_DIR:-$APP_HOME/kickstarts}"

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

mkdir -p "$LOG_DIR" 2>&1 | while read -r l; do log "mkdir: $l"; done || log "mkdir $LOG_DIR 失败 rc=$?"

PIDPROC=$(pidof "$BIN" || true)
if [[ -z "$PIDPROC" ]]; then
    log "进程不存在，执行启动"
    log "命令: $APP_HOME/$BIN >> $LOG_DIR/$BIN.log 2>&1 &"
    cd "$APP_HOME" && "$APP_HOME/$BIN" >> "$LOG_DIR/$BIN.log" 2>&1 &
    LAUNCH_RC=$?
    sleep 2
    log "nohup 段返回 rc=${LAUNCH_RC}，2s 后进程: $(pidof "$BIN" || echo 无)"
    log "服务日志头部：$(head -3 "$LOG_DIR/$BIN.log" 2>/dev/null | tr '\n' '|')"
else
    log "(pid:$PIDPROC) 已在运行，跳过"
fi
log "===== 真实 start.sh end ====="
