#!/bin/bash
# isobuilder 真实启动脚本（位于镜像层 /home/export/App/bin/start.sh，运行时可改：
# 改完在控制台点「停止/启动」即生效，无需重打镜像）。
#
# ***** USER CODE START（isobuilder 专属：APP_HOME/BIN/LOG_DIR/env。换项目改此段，到 END 为止）*****

# 应用主目录与可执行文件（二进制烤在镜像里，不能放 /export——拉起时被挂载覆盖）
APP_HOME=/opt/isobuilder
BIN=isobuilder
LOG_DIR=/export/log/isobuilder

export ISOBUILDER_ADDR="${ISOBUILDER_ADDR:-:8080}"
export ISOBUILDER_DATA_DIR="${ISOBUILDER_DATA_DIR:-/data}"
export ISOBUILDER_WEB_DIR="${ISOBUILDER_WEB_DIR:-$APP_HOME/web}"
export ISOBUILDER_BUILTIN_DIR="${ISOBUILDER_BUILTIN_DIR:-$APP_HOME/kickstarts}"

# ***** USER CODE END *****

PIDPROC=$(pidof "$BIN" || true)
if [[ -z "$PIDPROC" ]]; then
    # isobuilder 是前台服务，不自我 daemonize，故后台拉起（&）
    echo "应用启动命令："
    echo "$APP_HOME/$BIN >> $LOG_DIR/$BIN.log 2>&1 &"
    cd "$APP_HOME" && "$APP_HOME/$BIN" >> "$LOG_DIR/$BIN.log" 2>&1 &
    echo "started pid: $(pidof "$BIN")"
else
    echo "(pid:$PIDPROC) is running"
fi
