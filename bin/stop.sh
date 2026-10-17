#!/bin/bash
# isobuilder 真实停止脚本（位于 /export/App/bin/stop.sh，运行时可改）。
#
# 与参考范式（kill -9 直接强杀）的一处刻意偏离：先 SIGTERM 让后端优雅退出
#   CancelAll（取消运行中构建，连带清理 xorriso 子进程）→ server.Shutdown(≤20s)
#   → manager.Wait（等构建 goroutine 收尾）
# 直接 kill -9 会留下孤儿 xorriso 进程与半截工作目录。超时才 KILL 兜底。
# ***** USER CODE START（唯一工程项：BIN 改成你的二进制名，到 END 为止）*****

# ***** USER CODE END *****

if ! PIDPROC=$(pidof "$BIN"); then
  echo "$BIN is not running"
  exit 0
fi

echo "PIDPROC: $PIDPROC"
echo "sending SIGTERM ..."
kill $PIDPROC

# 最多等 25s 优雅退出（后端 Shutdown 20s + Wait 余量）
for ((i = 0; i < 25; i++)); do
  pidof "$BIN" > /dev/null || break
  sleep 1
done

if pidof "$BIN" > /dev/null; then
  echo "still alive after 25s, force kill"
  kill -9 $(pidof "$BIN")
fi
echo "stop finished at $(date)"
