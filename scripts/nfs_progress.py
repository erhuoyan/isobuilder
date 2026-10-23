#!/usr/bin/env python3
"""
实时查看各 BMC 从 NFS 服务器读取 ISO 的进度，用于判断自动装机是否在推进。

原理：BMC 挂载虚拟光驱后以 NFS 读取整个 ISO，NFS 服务器内核为每条连接
维护 bytes_sent 计数。用 bytes_sent / ISO 大小 就能算出装机读取进度，
不需要 KVM/VNC 画面。

用法:
    python3 scripts/nfs_progress.py --nfs-host root@100.88.20.253 \
        --iso-size 3.5G

    # 只监控 21-40
    python3 scripts/nfs_progress.py --nfs-host root@100.88.20.253 \
        --range 21-40
"""

import argparse
import re
import subprocess
import sys
import time


def parse_size(s):
    """'3.5G' / '3500M' / '3758096384' → bytes"""
    s = s.strip().upper()
    m = re.fullmatch(r"([\d.]+)\s*([KMGT]?)I?B?", s)
    if not m:
        raise ValueError(f"无法解析大小: {s}")
    num = float(m.group(1))
    unit = m.group(2)
    mult = {"": 1, "K": 1024, "M": 1024**2, "G": 1024**3, "T": 1024**4}[unit]
    return int(num * mult)


def human(nbytes):
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if nbytes < 1024 or unit == "TB":
            return f"{nbytes:.1f}{unit}" if unit != "B" else f"{int(nbytes)}B"
        nbytes /= 1024


def sample(nfs_host, ssh_opts):
    """在 NFS 服务器上抓一次 ss 输出，返回 {ip: bytes_sent}。"""
    cmd = ("ss -tin state established '( sport = :2049 )' 2>/dev/null")
    try:
        r = subprocess.run(
            ["ssh", *ssh_opts, nfs_host, cmd],
            capture_output=True, text=True, timeout=20,
        )
    except Exception as e:
        print(f"SSH 失败: {e}", file=sys.stderr)
        return None
    if r.returncode != 0:
        print(f"SSH 返回 {r.returncode}: {r.stderr[:200]}", file=sys.stderr)
        return None

    out = {}
    cur_ip = None
    for line in r.stdout.splitlines():
        # 连接行形如: "0  0  本地:2049  对端:端口"（前两列是 Recv-Q/Send-Q）
        peer = None
        fields = line.split()
        for i, f in enumerate(fields):
            if f.endswith(":2049") and i + 1 < len(fields):
                peer = fields[i + 1].rsplit(":", 1)[0].strip("[]")
                break

        if peer:
            cur_ip = peer
            continue

        # 紧随连接行的详细统计里有 bytes_sent。
        # 同一 IP 可能有多条并发连接，累加而不是覆盖。
        if cur_ip:
            bm = re.search(r"bytes_sent:(\d+)", line)
            if bm:
                out[cur_ip] = out.get(cur_ip, 0) + int(bm.group(1))
                cur_ip = None
    return out


def main():
    ap = argparse.ArgumentParser(
        description="通过 NFS 读取字节数查看 BMC 装机进度",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="\n".join([
            "",
            "进度 = 该 BMC 已从 NFS 读到的字节 / ISO 大小。",
            "  接近 100%   → 读完了（装机大概率已完成）",
            "  卡住不动    → 安装失败或无人操作（需人工介入）",
            "  持续增长    → 正在安装",
            "",
            "示例:",
            "  nfs_progress.py --nfs-host root@100.88.20.253 --iso-size 3.5G",
            "  nfs_progress.py --nfs-host root@100.88.20.253 --iso-size 3.5G --range 21-40",
            "  nfs_progress.py --nfs-host root@100.88.20.253 --iso-size 3.5G --watch 10",
            "",
        ]),
    )
    ap.add_argument("--nfs-host", required=True, metavar="USER@HOST",
                    help="NFS 服务器 SSH 地址")
    ap.add_argument("--iso-size", required=True, metavar="SIZE",
                    help="ISO 大小，如 3.5G")
    ap.add_argument("--range", metavar="A-B",
                    help="只显示该 IP 尾号范围，如 21-40")
    ap.add_argument("--watch", type=int, default=0, metavar="SEC",
                    help="持续刷新，间隔 SEC 秒（0=只查一次）")
    ap.add_argument("--ssh-opt", action="append", default=[],
                    help="额外 ssh 参数，如 -o ConnectTimeout=5")
    args = ap.parse_args()

    iso = parse_size(args.iso_size)
    lo = hi = None
    if args.range:
        m = re.fullmatch(r"(\d+)-(\d+)", args.range)
        if not m:
            print("错误: --range 格式应为 A-B", file=sys.stderr)
            sys.exit(1)
        lo, hi = int(m.group(1)), int(m.group(2))

    ssh_opts = ["-o", "BatchMode=yes", "-o", "ConnectTimeout=10", *args.ssh_opt]

    def tail(ip):
        return int(ip.rsplit(".", 1)[-1])

    prev = {}
    try:
        while True:
            data = sample(args.nfs_host, ssh_opts)
            if data is None:
                if not args.watch:
                    sys.exit(1)
                time.sleep(args.watch)
                continue

            ips = sorted(data, key=tail)
            if lo is not None:
                ips = [ip for ip in ips if lo <= tail(ip) <= hi]

            if args.watch:
                print("\033[2J\033[H", end="")   # 清屏

            stamp = time.strftime("%H:%M:%S")
            print(f"[{stamp}] 挂载中: {len(ips)} 台    ISO: {human(iso)}")
            print(f"{'IP':<17}{'已读':<12}{'进度':<9}{'速度'}")
            print("-" * 52)
            for ip in ips:
                b = data[ip]
                pct = b / iso * 100
                speed = ""
                if ip in prev:
                    d = (b - prev[ip]) / max(args.watch, 1)
                    speed = f"{human(d)}/s" if d > 0 else "停滞"
                bar = "#" * int(pct / 5)
                print(f"{ip:<17}{human(b):<12}{pct:6.1f}%  {bar:<20}{speed}")
            print("-" * 52)
            prev = data

            if not args.watch:
                break
            time.sleep(args.watch)
    except KeyboardInterrupt:
        print("\n已停止", file=sys.stderr)


if __name__ == "__main__":
    main()
