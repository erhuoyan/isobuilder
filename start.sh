#!/bin/bash
# jdos 平台启动入口（照平台调度器范式）。真实启动脚本在 /export/App/bin/start.sh
# ——运行时可改，改完点「停止/启动」即生效，无需重打镜像。

source /etc/profile

if [ -f "/home/admin/start_before.sh" ]; then
    cd /home/admin
    chmod 777 start_before.sh
    su -m admin -c ./start_before.sh
fi

cd /export/App/bin/
su -m admin -c ./start.sh

MONITOR_SHELL=/export/Shell/pid_monitor/task.sh
if [[ -f $MONITOR_SHELL ]]; then
  if [ "$JDOS_PID_KEY" ]; then
      setsid bash $MONITOR_SHELL $APP_ID 1 "$JDOS_PID_KEY" 1 &
  else
      setsid bash $MONITOR_SHELL $APP_ID 1 "server1" 1 &
  fi
fi

if [ -f "/home/admin/start_after.sh" ]; then
    cd /home/admin
    chmod 777 start_after.sh
    su -m admin -c ./start_after.sh
fi
