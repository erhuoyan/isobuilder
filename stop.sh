#!/bin/bash
# jdos 平台停止入口。真实停止脚本在镜像层 /home/export/App/bin/stop.sh。
set -o pipefail   # 不用 -u：/etc/profile 引用未定义变量会炸

LOG_DIR=/export/log/isobuilder
BOOT_LOG=$LOG_DIR/boot.log
log() { echo "[$(date '+%F %T')] [stop.sh] $*" | tee -a "$BOOT_LOG" 2>/dev/null || echo "[$(date '+%F %T')] [stop.sh] $*"; }

exec_as_admin() {
  if [ "$(id -un)" = "admin" ]; then bash "$@"; else su -m admin -c "bash $*"; fi
}

log "===== 平台 stop.sh begin ====="
log "user=$(id -un) uid=$(id -u)"

source /etc/profile 2>/dev/null || true
log "sourced /etc/profile"

if [ -f "/home/admin/stop_before.sh" ]; then
    cd /home/admin
    chmod 777 stop_before.sh
    log "执行 stop_before.sh"
    exec_as_admin /home/admin/stop_before.sh 2>&1 | tee -a "$BOOT_LOG"
fi

REAL_STOP=/home/export/App/bin/stop.sh
if [ -f "$REAL_STOP" ]; then
    log "委托到 $REAL_STOP"
    cd /home/export/App/bin/
    chmod +x stop.sh 2>/dev/null || true
    exec_as_admin /home/export/App/bin/stop.sh 2>&1 | tee -a "$BOOT_LOG"
    REAL_RC=${PIPESTATUS[0]}
    log "真实脚本返回码=${REAL_RC}"
else
    log "$REAL_STOP 不存在，视为服务未运行"
fi

MONITOR_SHELL=/export/Shell/pid_monitor/task.sh
if [[ -f $MONITOR_SHELL ]]; then
  log "拉起 pid_monitor（stop 档）"
  if [ "$JDOS_PID_KEY" ]; then
      setsid bash $MONITOR_SHELL $APP_ID 1 "$JDOS_PID_KEY" 2 &
  else
      setsid bash $MONITOR_SHELL $APP_ID 1 "server1" 2 &
  fi
fi

if [ -f "/home/admin/stop_after.sh" ]; then
    cd /home/admin
    chmod 777 stop_after.sh
    log "执行 stop_after.sh"
    exec_as_admin /home/admin/stop_after.sh 2>&1 | tee -a "$BOOT_LOG"
fi
log "===== 平台 stop.sh end ====="
exit 0
