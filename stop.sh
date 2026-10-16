#!/bin/bash
# isobuilder 停止脚本（jdos 控制台「停止」按钮调用）
# 发 SIGTERM → 后端优雅退出（CancelAll 构建 → server.Shutdown 20s → manager.Wait）；
# 超时 25s 仍存活则 SIGKILL 兜底。
set -uo pipefail

if pgrep -x isobuilder >/dev/null; then
  echo "sending SIGTERM to isobuilder ..."
  pkill -x -TERM isobuilder
  for _ in $(seq 1 25); do
    pgrep -x isobuilder >/dev/null || break
    sleep 1
  done
  if pgrep -x isobuilder >/dev/null; then
    echo "still alive after 25s, sending SIGKILL"
    pkill -x -KILL isobuilder
  fi
  echo "isobuilder stop success"
else
  echo "isobuilder is not running"
fi
