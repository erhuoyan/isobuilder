#!/bin/bash
# jdos 平台停止入口（照平台调度器范式）。真实停止脚本在 /export/App/bin/stop.sh。

source /etc/profile

if [ -f "/home/admin/stop_before.sh" ]; then
    cd /home/admin
    chmod 777 stop_before.sh
    su -m admin -c ./stop_before.sh
fi

if [ ! -f "/export/App/bin/stop.sh" ]; then
    echo "There is no stop.sh file, skip stop"
    exit 1
fi

cd /export/App/bin/
chmod +x stop.sh
su -m admin -c ./stop.sh

MONITOR_SHELL=/export/Shell/pid_monitor/task.sh
if [[ -f $MONITOR_SHELL ]]; then
  if [ "$JDOS_PID_KEY" ]; then
      setsid bash $MONITOR_SHELL $APP_ID 1 "$JDOS_PID_KEY" 2 &
  else
      setsid bash $MONITOR_SHELL $APP_ID 1 "server1" 2 &
  fi
fi

if [ -f "/home/admin/stop_after.sh" ]; then
    cd /home/admin
    chmod 777 stop_after.sh
    su -m admin -c ./stop_after.sh
fi
