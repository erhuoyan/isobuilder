#!/bin/bash
[ -f "/home/admin/start_before.sh" ] && bash /home/admin/start_before.sh
# 此脚本必须在/home/admin下，为应用的启动脚本

SERVER_NAME="isobuilder"
RUNCMD="/opt/isobuilder/isobuilder"
LOG_DIR="/export/log/isobuilder"

export ISOBUILDER_ADDR="${ISOBUILDER_ADDR:-:8080}"
export ISOBUILDER_DATA_DIR="${ISOBUILDER_DATA_DIR:-/data}"
export ISOBUILDER_WEB_DIR="${ISOBUILDER_WEB_DIR:-/opt/isobuilder/web}"
export ISOBUILDER_BUILTIN_DIR="${ISOBUILDER_BUILTIN_DIR:-/opt/isobuilder/kickstarts}"

# isobuilder 是前台服务、不自我 daemonize（redis 模板靠 daemonize yes 自我后台化），
# 故 nohup 后台拉起让本脚本立即返回——控制台 ssh 执行不能挂住。
pgrep -x $SERVER_NAME >/dev/null && echo "$SERVER_NAME is running" || {
  cd /opt/isobuilder && nohup $RUNCMD >> "$LOG_DIR/stdout.log" 2>&1 &
  echo "$SERVER_NAME start success (pid $!)"
}

[ -f "/home/admin/start_after.sh" ] && bash /home/admin/start_after.sh
