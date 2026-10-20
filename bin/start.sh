#!/bin/bash
# isobuilder 真实启动脚本（位于镜像层 /home/export/App/bin/start.sh，运行时可改：
# 改完在控制台点「停止/启动」即生效，无需重打镜像）。
#
# ***** USER CODE START（isobuilder 专属：APP_HOME/BIN/LOG_DIR/env。换项目改此段，到 END 为止）*****

# 应用主目录与可执行文件（二进制烤在镜像里，不能放 /export——拉起时被挂载覆盖）
APP_HOME=/opt/isobuilder
BIN=isobuilder
LOG_DIR=/export/log/isobuilder

# 全部运行时设置来自配置文件（-c）。除数据库凭据外，ISOBUILDER_* 环境变量
# 已不再被程序读取；ISOBUILDER_CONFIG 仅用于改变配置文件路径。
CONFIG_FILE="${ISOBUILDER_CONFIG:-/home/export/App/cfg/config.yaml}"
CONFIG_ARG=""
if [ -f "$CONFIG_FILE" ]; then
    CONFIG_ARG="-c $CONFIG_FILE"
fi

# 日志目录也来自同一份配置（扁平键，Go 程序自行解析其余项）。
cfg_get() {
    [ -f "$CONFIG_FILE" ] || return 1
    local v
    v=$(awk -v k="$1" '$0 ~ "^[[:space:]]*" k "[[:space:]]*:" {
        sub("^[[:space:]]*" k "[[:space:]]*:[[:space:]]*", ""); print; exit
    }' "$CONFIG_FILE" 2>/dev/null)
    v=${v%$'\r'}
    case "$v" in
        \"*\") v=${v#\"}; v=${v%\"} ;;
        \'*\') v=${v#\'}; v=${v%\'} ;;
    esac
    [ -n "$v" ] || return 1
    printf '%s' "$v"
}
LOG_DIR="${ISOBUILDER_LOG_DIR:-/export/log/isobuilder}"
if v=$(cfg_get log_dir); then LOG_DIR=$v; fi

# ***** USER CODE END *****

# 自己的诊断行写 app-start.log（boot.log 由链首聚合，start.log 由上层 start.sh 写——
# 各写各的文件，互不重复；stdout 继续向上流进 boot.log）。
OWN_LOG=$LOG_DIR/app-start.log
log() { echo "[$(date '+%F %T')] [bin/start.sh] $*" | tee -a "$OWN_LOG" 2>/dev/null || echo "[$(date '+%F %T')] [bin/start.sh] $*"; }

log "===== 真实 start.sh begin ====="
log "user=$(id -un) uid=$(id -u)"
log "APP_HOME=$APP_HOME BIN=$BIN LOG_DIR=$LOG_DIR"
ADDR=$(cfg_get addr) || ADDR=":8080(默认)"
DATA_DIR=$(cfg_get data_dir) || DATA_DIR="/opt/isobuilder/data(默认)"
log "配置文件: $CONFIG_FILE（$([ -n "$CONFIG_ARG" ] && echo '已启用 -c' || echo '不存在，全部用默认值')）"
log "配置值: addr=$ADDR data_dir=$DATA_DIR"
log "env DB_URL 注入: $([ -n "$ISOBUILDER_DB_URL" ] && echo 是 || echo 否)"
DB_HOST=$(printf '%s' "${ISOBUILDER_DB_URL:-}" | sed -n 's#.*@\([^:/?:]*\).*#\1#p')
log "DB_URL host: ${DB_HOST:-见配置文件 database_url（都为空则用 SQLite）}"

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
