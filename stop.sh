#!/bin/bash
[ -f "/home/admin/stop_before.sh" ] && bash /home/admin/stop_before.sh
# 此脚本必须在/home/admin下，为应用的停止脚本

SERVER_NAME="isobuilder"

# isobuilder 是前台服务，监听 SIGTERM 做优雅退出：
#   CancelAll（取消运行中构建）→ server.Shutdown（最长 20s）→ manager.Wait（等构建 goroutine）
# 与 redis 模板的 pkill 不同点：这里先 TERM 给足退出窗口，超时才 KILL 兜底。
pgrep -x $SERVER_NAME >/dev/null && {
  echo "sending SIGTERM to $SERVER_NAME ..."
  pkill -x -TERM $SERVER_NAME
  for _ in $(seq 1 25); do
    pgrep -x $SERVER_NAME >/dev/null || break
    sleep 1
  done
  if pgrep -x $SERVER_NAME >/dev/null; then
    echo "$SERVER_NAME still alive after 25s, sending SIGKILL"
    pkill -x -KILL $SERVER_NAME
  fi
  echo "$SERVER_NAME stop success"
} || echo "$SERVER_NAME is not running"

[ -f "/home/admin/stop_after.sh" ] && bash /home/admin/stop_after.sh
