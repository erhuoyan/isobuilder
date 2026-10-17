#!/bin/bash
# isobuilder 真实停止脚本（位于镜像层 /home/export/App/bin/stop.sh，运行时可改）。
#
# 与参考范式（kill -9 直接强杀）的一处刻意偏离：先 SIGTERM 让后端优雅退出
#   CancelAll（取消运行中构建，连带清理 xorriso 子进程）→ server.Shutdown(≤20s)
#   → manager.Wait（等构建 goroutine 收尾）
# 直接 kill -9 会留下孤儿 xorriso 进程与半截工作目录。超时才 KILL 兜底。
# ***** USER CODE START（唯一工程项：BIN 改成你的二进制名，到 END 为止）*****
# 进程名，改这里
BIN=isobuilder
# ***** USER CODE END *****

LOG_DIR=/export/log/isobuilder
BOOT_LOG=$LOG_DIR/boot.log
log() { echo "[$(date '+%F %T')] [bin/stop.sh] $*" | tee -a "$BOOT_LOG" 2>/dev/null || true; }

log "===== 真实 stop.sh begin ====="
log "user=$(id -un) uid=$(id -u) BIN=$BIN"

if ! PIDPROC=$(pidof "$BIN"); then
  log "$BIN 未在运行，无需停止"
  exit 0
fi

log "PIDPROC: $PIDPROC，发送 SIGTERM"
kill $PIDPROC
log "SIGTERM 已发，等优雅退出（最长 25s）"

for ((i = 0; i < 25; i++)); do
  pidof "$BIN" > /dev/null || break
  sleep 1
done

if pidof "$BIN" > /dev/null; then
  log "25s 后仍存活，SIGKILL 强杀: $(pidof "$BIN")"
  kill -9 $(pidof "$BIN")
fi
log "停止完成 at $(date)"
log "===== 真实 stop.sh end ====="
