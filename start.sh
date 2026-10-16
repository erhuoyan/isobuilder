#!/bin/bash
# isobuilder 启动脚本（jdos 控制台「启动」按钮 / start_container.sh 调用，以 admin 身份执行）
set -uo pipefail

APP_DIR="/opt/isobuilder"
LOG_DIR="/export/log/isobuilder"

export ISOBUILDER_ADDR="${ISOBUILDER_ADDR:-:8080}"
export ISOBUILDER_DATA_DIR="${ISOBUILDER_DATA_DIR:-/data}"
export ISOBUILDER_WEB_DIR="${ISOBUILDER_WEB_DIR:-$APP_DIR/web}"
export ISOBUILDER_BUILTIN_DIR="${ISOBUILDER_BUILTIN_DIR:-$APP_DIR/kickstarts}"

if pgrep -x isobuilder >/dev/null; then
  echo "isobuilder is already running"
  exit 0
fi

cd "$APP_DIR"
nohup ./isobuilder >> "$LOG_DIR/stdout.log" 2>&1 &
echo "isobuilder start success (pid $!)"
