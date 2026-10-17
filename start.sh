#!/bin/bash
# jdos 平台启动入口。真实启动脚本在镜像层 /home/export/App/bin/start.sh。
set -uo pipefail

LOG_DIR=/export/log/isobuilder
BOOT_LOG=$LOG_DIR/boot.log
log() { echo "[$(date '+%F %T')] [start.sh] $*" | tee -a "$BOOT_LOG" 2>/dev/null || true; }

log "===== 平台 start.sh begin ====="
log "user=$(id -un) uid=$(id -u)"

source /etc/profile 2>&1 | while read -r l; do log "profile: $l"; done || true
log "sourced /etc/profile"

if [ -f "/home/admin/start_before.sh" ]; then
    cd /home/admin
    chmod 777 start_before.sh
    log "执行 start_before.sh"
    su -m admin -c ./start_before.sh 2>&1 | tee -a "$BOOT_LOG"
fi

REAL_START=/home/export/App/bin/start.sh
if [ ! -f "$REAL_START" ]; then
    log "ERROR: $REAL_START 不存在——请从仓库 bin/start.sh 复制到 /home/export/App/bin/ 并 chmod +x"
    exit 1
fi
log "委托到 $REAL_START"
cd /home/export/App/bin/
chmod +x start.sh
su -m admin -c ./start.sh 2>&1 | tee -a "$BOOT_LOG"
REAL_RC=${PIPESTATUS[0]}
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
    su -m admin -c ./start_after.sh 2>&1 | tee -a "$BOOT_LOG"
fi
log "===== 平台 start.sh end ====="
exit ${REAL_RC:-0}
