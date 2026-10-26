#!/usr/bin/env python3
"""
通过 Redfish API 批量采集服务器硬件信息：SN、BMC MAC、所有网卡及端口 MAC。

纯标准 Redfish 字段，不依赖厂商 OEM 扩展。每台服务器输出一行 CSV，
所有网卡信息（名称、型号、厂商、各端口 MAC）平铺为列。
"""

import argparse
import csv
import io
import json
import os
import subprocess
import sys
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime

# ── 默认凭据 ────────────────────────────────────────────────────────────────
DEFAULT_USER = "jdroot"
DEFAULT_PASS = "JCss%6!8"


# ═══════════════════════════════════════════════════════════════════════════════
#  Redfish 采集
# ═══════════════════════════════════════════════════════════════════════════════

# 本线程最后一次请求的失败原因。并发用 ThreadPoolExecutor，不能用模块级全局变量。
# 只影响报错文案，函数返回值保持原样。
_err = threading.local()


def _reason_suffix():
    return ": " + _err.reason if getattr(_err, "reason", "") else ""


def redfish_get(ip, user, passwd, path, timeout=15):
    """GET 并解析 JSON；失败返回 None，原因记到本线程错误槽。"""
    url = f"https://{ip}{path}"
    _err.reason = ""
    try:
        r = subprocess.run(
            ["curl", "-sk", "-u", f"{user}:{passwd}", "--connect-timeout", "10",
             "--max-time", str(timeout), url],
            # capture_output/text 是 Python 3.7 才有的参数：3.6 上会抛 TypeError，
            # 被下面的 except 吞掉后误报成“连不上 BMC”。用 3.6 也认的写法。
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            universal_newlines=True, timeout=timeout + 5,
        )
    except Exception as e:
        _err.reason = "%s: %s" % (type(e).__name__, e)
        return None
    if r.returncode != 0:
        err = (r.stderr or "").strip()[:200]
        _err.reason = "curl exit %d: %s" % (r.returncode, err) if err else \
            "curl exit %d" % r.returncode
        return None
    try:
        return json.loads(r.stdout)
    except Exception:
        _err.reason = "响应不是合法 JSON: %s" % r.stdout.strip()[:120]
        return None


def collect_one(ip, user, passwd):
    """
    采集一台服务器，返回:
      bmc_ip, sn, bmc_mac,
      nics: [{name, model, manufacturer, macs: [str]}],
      error
    """
    result = {"bmc_ip": ip, "sn": "", "bmc_mac": "", "nics": [], "error": ""}

    # 1) SN
    sys_data = redfish_get(ip, user, passwd, "/redfish/v1/Systems/1")
    if not sys_data:
        result["error"] = "无法连接BMC" + _reason_suffix()
        return result
    result["sn"] = sys_data.get("SerialNumber", "")

    # 2) BMC MAC
    mgr_eths = redfish_get(ip, user, passwd, "/redfish/v1/Managers/1/EthernetInterfaces")
    if mgr_eths:
        members = mgr_eths.get("Members", [])
        if members:
            eth_data = redfish_get(ip, user, passwd, members[0].get("@odata.id", ""))
            if eth_data:
                result["bmc_mac"] = (eth_data.get("PermanentMACAddress", "")
                                     or eth_data.get("MACAddress", ""))

    # 3) 所有网络适配器
    adapters = redfish_get(ip, user, passwd, "/redfish/v1/Chassis/1/NetworkAdapters")
    if not adapters:
        result["error"] = "无法获取网络适配器列表" + _reason_suffix()
        return result

    for member in adapters.get("Members", []):
        adapter_path = member.get("@odata.id", "")
        if not adapter_path:
            continue
        ada = redfish_get(ip, user, passwd, adapter_path)
        if not ada:
            continue

        # 适配器名取路径末段，如 PCIeCard2(HP382)
        adapter_name = adapter_path.rstrip("/").rsplit("/", 1)[-1]

        nic = {
            "name": adapter_name,
            "model": ada.get("Model", ""),
            "manufacturer": ada.get("Manufacturer", ""),
            "macs": [],
        }

        # 4) 该适配器的所有端口 MAC
        ports_data = redfish_get(ip, user, passwd, f"{adapter_path}/NetworkPorts")
        if ports_data:
            for port_member in ports_data.get("Members", []):
                port_path = port_member.get("@odata.id", "")
                if not port_path:
                    continue
                port = redfish_get(ip, user, passwd, port_path)
                if not port:
                    continue
                addrs = port.get("AssociatedNetworkAddresses", [])
                if addrs:
                    nic["macs"].append(addrs[0])

        result["nics"].append(nic)

    if not result["nics"]:
        result["error"] = "未发现网络适配器"

    return result


# ═══════════════════════════════════════════════════════════════════════════════
#  输入解析
# ═══════════════════════════════════════════════════════════════════════════════

def load_bmcinfo(filepath):
    """
    解析 BMC 信息文件。支持:
      - CSV 格式 (逗号分隔)
      - TSV 格式 (tab 分隔)
      - 空格分隔
      - 只有 IP 的纯列表

    首行含 bmc_ip 关键字时自动跳过表头。
    """
    hosts = []
    with open(filepath, encoding="utf-8") as f:
        raw = f.read()

    lines = raw.splitlines()
    if not lines:
        return hosts

    # 检测是否为 CSV (含逗号的行 > 50% 即判定)
    comma_lines = sum(1 for ln in lines if "," in ln)
    is_csv = comma_lines > len(lines) * 0.5

    if is_csv:
        reader = csv.reader(io.StringIO(raw))
        for row in reader:
            row = [c.strip() for c in row if c.strip()]
            if not row:
                continue
            if "bmc_ip" in row[0].lower():
                continue
            if len(row) >= 3:
                hosts.append({"ip": row[0], "user": row[1], "pass": row[2]})
            elif len(row) >= 1:
                hosts.append({"ip": row[0], "user": None, "pass": None})
    else:
        for line in lines:
            line = line.strip()
            if not line:
                continue
            if "bmc_ip" in line.lower():
                continue
            parts = line.split("\t")
            if len(parts) < 2:
                parts = line.split()
            if len(parts) >= 3:
                hosts.append({"ip": parts[0], "user": parts[1], "pass": parts[2]})
            elif len(parts) >= 1:
                hosts.append({"ip": parts[0], "user": None, "pass": None})

    return hosts


# ═══════════════════════════════════════════════════════════════════════════════
#  主流程
# ═══════════════════════════════════════════════════════════════════════════════

def main():
    if len(sys.argv) == 1:
        sys.argv.append("-h")

    parser = argparse.ArgumentParser(
        description="通过 Redfish API 批量采集服务器 SN、BMC MAC 及所有网卡端口 MAC，输出 CSV",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="\n".join([
            "",
            "输入文件格式 (自动检测 CSV / TSV / 空格分隔，首行表头自动跳过):",
            "",
            "  CSV:    bmc_ip,bmc_user,bmc_pass",
           f"          100.88.20.1,{DEFAULT_USER},{DEFAULT_PASS}",
            "",
            "  TSV:    bmc_ip          bmc_user    bmc_pass",
           f"          100.88.20.1     {DEFAULT_USER:<12}{DEFAULT_PASS}",
            "",
            "  纯 IP:  100.88.20.1      (使用默认账号或 -u/-p)",
            "          100.88.20.2",
            "",
            "采集内容 (纯标准 Redfish 字段，不依赖厂商 OEM):",
            "  SN       <- /redfish/v1/Systems/1  SerialNumber",
            "  BMC MAC  <- /redfish/v1/Managers/1/EthernetInterfaces",
            "  网卡     <- /redfish/v1/Chassis/1/NetworkAdapters (全部)",
            "  端口 MAC <- .../NetworkPorts  AssociatedNetworkAddresses",
            "",
            "输出 CSV 一行一台服务器，所有网卡平铺为列:",
            "  bmc_ip, sn, bmc_mac,",
            "  nic1_name, nic1_model, nic1_mfr, nic1_mac1, nic1_mac2,",
            "  nic2_name, nic2_model, nic2_mfr, nic2_mac1, nic2_mac2,",
            "  ..., error",
            "  列数根据实际最大网卡数和端口数自动扩展，结果按 BMC IP 排序。",
            "",
           f"默认账号: {DEFAULT_USER} / {DEFAULT_PASS}",
            "  优先级: 文件中的 > -u/-p > 内置默认值",
            "",
            "示例:",
            "  bmc_inventory.py -f bmcinfo.txt",
            "  bmc_inventory.py -f bmcinfo.txt -o result.csv",
            "  bmc_inventory.py -f ip_list.txt -u admin -p pass123 -j 20",
            "",
        ]),
    )
    parser.add_argument("-f", "--file", required=True, metavar="FILE",
                        help="BMC 信息文件 (CSV / TSV / 空格分隔 / 纯 IP 列表)")
    parser.add_argument("-u", "--user", default=None, metavar="USER",
                        help="全局 BMC 用户名 (默认: %s)" % DEFAULT_USER)
    parser.add_argument("-p", "--password", default=None, metavar="PASS",
                        help="全局 BMC 密码 (默认: %s)" % DEFAULT_PASS.replace("%", "%%"))
    parser.add_argument("-o", "--output", default=None, metavar="CSV",
                        help="输出 CSV 路径 (默认: bmc_inventory_<日期时间>.csv)")
    parser.add_argument("-j", "--jobs", type=int, default=10, metavar="N",
                        help="并发数 (默认: 10)")

    args = parser.parse_args()

    output = args.output
    if not output:
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        output = os.path.join(
            os.path.dirname(os.path.abspath(args.file)),
            f"bmc_inventory_{ts}.csv",
        )

    hosts = load_bmcinfo(args.file)
    if not hosts:
        print("错误: BMC 列表为空", file=sys.stderr)
        sys.exit(1)

    effective_user = args.user or DEFAULT_USER
    effective_pass = args.password or DEFAULT_PASS
    for h in hosts:
        if not h["user"]:
            h["user"] = effective_user
        if not h["pass"]:
            h["pass"] = effective_pass

    total = len(hosts)
    print(f"开始采集: {total} 台, 并发: {args.jobs}", file=sys.stderr)

    results = []
    done = 0
    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        futures = {
            pool.submit(collect_one, h["ip"], h["user"], h["pass"]): h
            for h in hosts
        }
        for future in as_completed(futures):
            done += 1
            r = future.result()
            results.append(r)
            status = "OK" if not r["error"] else r["error"]
            nic_summary = "; ".join(
                f"{n['name']}({n['model']})×{len(n['macs'])}" for n in r["nics"]
            ) or "-"
            print(
                f"  [{done}/{total}] {r['bmc_ip']} SN={r['sn']} "
                f"BMC_MAC={r['bmc_mac']} [{nic_summary}] {status}",
                file=sys.stderr,
            )

    results.sort(key=lambda x: list(map(int, x["bmc_ip"].split("."))))

    # ── 动态列：找出最大网卡数和每张卡最大端口数 ─────────────────────────
    max_nics = max((len(r["nics"]) for r in results), default=0)
    max_ports = 0
    for r in results:
        for nic in r["nics"]:
            max_ports = max(max_ports, len(nic["macs"]))
    if max_nics == 0:
        max_nics = 1
    if max_ports == 0:
        max_ports = 2

    # 表头: bmc_ip, sn, bmc_mac, [nic1_name, nic1_model, nic1_mfr, nic1_mac1, ...], ..., error
    header = ["bmc_ip", "sn", "bmc_mac"]
    for i in range(max_nics):
        n = i + 1
        header += [f"nic{n}_name", f"nic{n}_model", f"nic{n}_mfr"]
        header += [f"nic{n}_mac{j+1}" for j in range(max_ports)]
    header.append("error")

    with open(output, "w", newline="", encoding="utf-8") as out_fh:
        writer = csv.writer(out_fh)
        writer.writerow(header)
        for r in results:
            row = [r["bmc_ip"], r["sn"], r["bmc_mac"]]
            for i in range(max_nics):
                if i < len(r["nics"]):
                    nic = r["nics"][i]
                    row += [nic["name"], nic["model"], nic["manufacturer"]]
                    macs = nic["macs"] + [""] * (max_ports - len(nic["macs"]))
                    row += macs
                else:
                    row += [""] * (3 + max_ports)
            row.append(r["error"])
            writer.writerow(row)

    ok = sum(1 for r in results if not r["error"])
    fail = total - ok
    print(f"\n完成: 成功 {ok}, 失败 {fail}, 共 {total}", file=sys.stderr)
    print(f"结果已保存到: {output}", file=sys.stderr)


if __name__ == "__main__":
    main()
