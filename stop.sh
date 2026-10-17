#!/bin/bash
# jdos 平台停止入口（照平台调度器范式）。真实停止脚本在 /home/export/App/bin/stop.sh。

source /etc/profile

if [ -f "/home/admin/stop_before.sh" ]; then
    cd /home/admin
    chmod 777 stop_before.sh
    su -m admin -c ./stop_before.sh
fi

# 停止是临终操作，不许失败：找不到 /export/App/bin/stop.sh（如首次部署尚未
# 复制真实脚本，或服务从未启动）不视为错误，直接以未运行处理退出 0。
# isobuilder 进程不存在时真实脚本自身也会优雅退出（pidof 空 → exit 0）。
if [ -f "/export/App/bin/stop.sh" ]; then
    cd /export/App/bin/
    chmod +x stop.sh
    su -m admin -c ./stop.sh
else
    echo "[stop.sh] /export/App/bin/stop.sh 不存在，视为服务未运行"
fi

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
