#!/bin/bash
# jdos 平台启动入口。真实启动脚本在镜像层 /home/export/App/bin/start.sh。
set -o pipefail   # 不用 -u：/etc/profile 引用未定义变量会炸（HISTCONTROL 等）

# 日志分工：boot.log 只由链首 start_container.sh 捕获写入（聚合日志）；
# 本脚本自己的诊断行写 start.log。同一文件绝不允许内外两层都 tee
# （旧版自己 tee boot.log 又被外层 tee，每行落盘两次）。
# stdout 继续向上流，boot.log 自然聚合全部输出，每行恰好一份。
LOG_DIR=/export/log/isobuilder
OWN_LOG=$LOG_DIR/start.log
log() { echo "[$(date '+%F %T')] [start.sh] $*" | tee -a "$OWN_LOG" 2>/dev/null || echo "[$(date '+%F %T')] [start.sh] $*"; }

# 已是 admin 就直跑（start_container 已 su 过来），root 才需要 su——避免 admin→admin 要密码
exec_as_admin() {
  if [ "$(id -un)" = "admin" ]; then bash "$@"; else su -m admin -c "bash $*"; fi
}

log "===== 平台 start.sh begin ====="
log "user=$(id -un) uid=$(id -u)"

source /etc/profile 2>/dev/null || true
log "sourced /etc/profile"

if [ -f "/home/admin/start_before.sh" ]; then
    cd /home/admin
    chmod 777 start_before.sh
    log "执行 start_before.sh"
    exec_as_admin /home/admin/start_before.sh 2>&1
fi

REAL_START=/home/export/App/bin/start.sh
if [ ! -f "$REAL_START" ]; then
    log "ERROR: $REAL_START 不存在——请从仓库 bin/start.sh 复制到 /home/export/App/bin/ 并 chmod +x"
    exit 1
fi
log "委托到 $REAL_START"
cd /home/export/App/bin/
chmod +x start.sh 2>/dev/null || true
exec_as_admin /home/export/App/bin/start.sh 2>&1
REAL_RC=$?
log "真实脚本返回码=${REAL_RC}"

MONITOR_SHELL=/export/Shell/pid_monitor/task.sh
if [[ -f $MONITOR_SHELL ]]; then
  log "拉起 pid_monitor"
  if [ "$JDOS_PID_KEY" ]; then
      setsid bash $MONITOR_SHELL $APP_ID 1 "$JDOS_PID_KEY" 1 &
  else
      setsid bash $MONITOR_SHELL $APP_ID 1 "server1" 1 &
  fi
fi

if [ -f "/home/admin/start_after.sh" ]; then
    cd /home/admin
    chmod 777 start_after.sh
    log "执行 start_after.sh"
    exec_as_admin /home/admin/start_after.sh 2>&1
fi
log "===== 平台 start.sh end ====="
exit ${REAL_RC:-0}
