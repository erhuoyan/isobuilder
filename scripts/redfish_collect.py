#!/usr/bin/env python3
"""
通过 Redfish API 批量采集服务器 SN 和指定速率网卡的 MAC 地址。

支持多厂商 BMC 协议（--type 选择），当前已实现: huawei (默认)。
"""

import abc
import argparse
import csv
import io
import json
import os
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime

# ── 默认凭据 ────────────────────────────────────────────────────────────────
DEFAULT_USER = "jdroot"
DEFAULT_PASS = "JCss%6!8"


# ═══════════════════════════════════════════════════════════════════════════════
#  协议抽象层
# ═══════════════════════════════════════════════════════════════════════════════

class BMCProtocol(abc.ABC):
    """BMC 采集协议基类。新增厂商只需继承并注册。"""

    name: str = ""
    description: str = ""

    @abc.abstractmethod
    def collect(self, ip, user, passwd, speed_filter):
        """
        采集一台服务器，返回 dict:
            bmc_ip, sn, bmc_mac, macs: list[str], adapter, error
        """

    # ── 公共工具 ─────────────────────────────────────────────────────────
    @staticmethod
    def redfish_get(ip, user, passwd, path, timeout=15):
        url = f"https://{ip}{path}"
        try:
            r = subprocess.run(
                ["curl", "-sk", "-u", f"{user}:{passwd}", "--connect-timeout", "10",
                 "--max-time", str(timeout), url],
                capture_output=True, text=True, timeout=timeout + 5,
            )
            if r.returncode != 0:
                return None
            return json.loads(r.stdout)
        except Exception:
            return None


# ── 协议注册表 ───────────────────────────────────────────────────────────────
_PROTOCOLS: dict[str, type[BMCProtocol]] = {}


def register_protocol(cls):
    """类装饰器：把协议注册到全局表。"""
    _PROTOCOLS[cls.name] = cls
    return cls


def get_protocol(name):
    if name not in _PROTOCOLS:
        available = ", ".join(sorted(_PROTOCOLS))
        print(f"错误: 未知协议 '{name}'，可选: {available}", file=sys.stderr)
        sys.exit(1)
    return _PROTOCOLS[name]()


def list_protocols():
    return _PROTOCOLS


# ═══════════════════════════════════════════════════════════════════════════════
#  协议实现: 华为 (JD OEM)
# ═══════════════════════════════════════════════════════════════════════════════

@register_protocol
class HuaweiProtocol(BMCProtocol):
    """
    华为 / JD OEM 服务器 Redfish 采集。

    路径:
      SN       <- /redfish/v1/Systems/1  SerialNumber
      BMC MAC  <- /redfish/v1/Managers/1/EthernetInterfaces  PermanentMACAddress
      NIC MAC  <- /redfish/v1/Chassis/1/NetworkAdapters -> NetworkPorts
                 按 Oem.JD.CardModel 匹配速率关键字
    """

    name = "huawei"
    description = "华为 / JD OEM 服务器"

    def collect(self, ip, user, passwd, speed_filter):
        result = {"bmc_ip": ip, "sn": "", "bmc_mac": "", "macs": [], "adapter": "", "error": ""}
        get = self.redfish_get

        # 1) SN
        sys_data = get(ip, user, passwd, "/redfish/v1/Systems/1")
        if not sys_data:
            result["error"] = "无法连接BMC"
            return result
        result["sn"] = sys_data.get("SerialNumber", "")

        # 2) BMC MAC
        mgr_eths = get(ip, user, passwd, "/redfish/v1/Managers/1/EthernetInterfaces")
        if mgr_eths:
            members = mgr_eths.get("Members", [])
            if members:
                eth_data = get(ip, user, passwd, members[0].get("@odata.id", ""))
                if eth_data:
                    result["bmc_mac"] = (eth_data.get("PermanentMACAddress", "")
                                         or eth_data.get("MACAddress", ""))

        # 3) 列出网络适配器，按速率过滤
        adapters = get(ip, user, passwd, "/redfish/v1/Chassis/1/NetworkAdapters")
        if not adapters:
            result["error"] = "无法获取网络适配器列表"
            return result

        speed_upper = speed_filter.upper().replace("GE", "G").replace("GB", "G")
        matched_paths = []
        for member in adapters.get("Members", []):
            path = member.get("@odata.id", "")
            if not path:
                continue
            ada = get(ip, user, passwd, path)
            if not ada:
                continue
            card_model = ada.get("Oem", {}).get("JD", {}).get("CardModel", "")
            model_norm = card_model.upper().replace("GE", "G").replace("GB", "G")
            if speed_upper in model_norm:
                matched_paths.append((path, card_model))

        if not matched_paths:
            result["error"] = f"未找到 {speed_filter} 网卡"
            return result

        # 4) 获取匹配适配器的所有端口 MAC
        for adapter_path, card_model in matched_paths:
            result["adapter"] = card_model
            ports_data = get(ip, user, passwd, f"{adapter_path}/NetworkPorts")
            if not ports_data:
                continue
            for port_member in ports_data.get("Members", []):
                port_path = port_member.get("@odata.id", "")
                if not port_path:
                    continue
                port = get(ip, user, passwd, port_path)
                if not port:
                    continue
                addrs = port.get("AssociatedNetworkAddresses", [])
                if addrs:
                    result["macs"].append(addrs[0])

        if not result["macs"]:
            result["error"] = f"找到 {speed_filter} 网卡但未获取到MAC"

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
            # 跳过表头
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
            # 先 tab，再空格
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
    # 无参数时直接打印帮助
    if len(sys.argv) == 1:
        sys.argv.append("-h")

    protocols = list_protocols()
    type_help = ", ".join(f"{n} ({p.description})" for n, p in sorted(protocols.items()))

    parser = argparse.ArgumentParser(
        description="通过 Redfish API 批量采集服务器 SN、BMC MAC 和指定速率网卡的 MAC 地址，输出 CSV",
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
            "输出 CSV 列: bmc_ip, sn, bmc_mac, mac1, mac2, ..., adapter, error",
            "  MAC 列数根据实际最大端口数自动扩展，结果按 BMC IP 排序。",
            "  未指定 -o 时自动生成: redfish_<速率>_<日期时间>.csv（与输入文件同目录）。",
            "",
           f"默认账号: {DEFAULT_USER} / {DEFAULT_PASS}",
            "  文件中有账号密码时优先使用文件中的，其次 -u/-p，最后使用默认值。",
            "",
           f"已实现协议: {type_help}",
            "",
            "示例:",
            "  redfish_collect.py -f bmcinfo.txt -s 25G",
            "  redfish_collect.py -f bmcinfo.txt -s 200G -o result.csv",
            "  redfish_collect.py -f ip_list.txt -s 25G -j 20",
            "  redfish_collect.py -f bmcinfo.csv -s 100G -t huawei",
            "",
        ]),
    )
    parser.add_argument("-f", "--file", required=True, metavar="FILE",
                        help="BMC 信息文件 (CSV / TSV / 空格分隔 / 纯 IP 列表)")
    parser.add_argument("-s", "--speed", required=True, metavar="SPEED",
                        help="网卡速率过滤，如: 25G, 100G, 200G")
    parser.add_argument("-t", "--type", default="huawei", metavar="TYPE", dest="protocol",
                        help=f"BMC 协议类型 (默认: huawei)。可选: {', '.join(sorted(protocols))}")
    parser.add_argument("-u", "--user", default=None, metavar="USER",
                        help="全局 BMC 用户名 (默认: %s)" % DEFAULT_USER)
    parser.add_argument("-p", "--password", default=None, metavar="PASS",
                        help="全局 BMC 密码 (默认: %s)" % DEFAULT_PASS.replace("%", "%%"))
    parser.add_argument("-o", "--output", default=None, metavar="CSV",
                        help="输出 CSV 路径 (默认: redfish_<速率>_<时间>.csv)")
    parser.add_argument("-j", "--jobs", type=int, default=10, metavar="N",
                        help="并发数 (默认: 10)")

    args = parser.parse_args()

    protocol = get_protocol(args.protocol)

    # 默认输出文件名: redfish_25G_20261022_153012.csv
    output = args.output
    if not output:
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        output = os.path.join(
            os.path.dirname(os.path.abspath(args.file)),
            f"redfish_{args.speed}_{ts}.csv",
        )

    hosts = load_bmcinfo(args.file)
    if not hosts:
        print("错误: BMC 列表为空", file=sys.stderr)
        sys.exit(1)

    # 凭据优先级: 文件行 > -u/-p 参数 > 内置默认值
    effective_user = args.user or DEFAULT_USER
    effective_pass = args.password or DEFAULT_PASS
    for h in hosts:
        if not h["user"]:
            h["user"] = effective_user
        if not h["pass"]:
            h["pass"] = effective_pass

    total = len(hosts)
    print(f"开始采集: {total} 台, 速率过滤: {args.speed}, 协议: {args.protocol}, 并发: {args.jobs}",
          file=sys.stderr)

    results = []
    done = 0
    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        futures = {
            pool.submit(protocol.collect, h["ip"], h["user"], h["pass"], args.speed): h
            for h in hosts
        }
        for future in as_completed(futures):
            done += 1
            r = future.result()
            results.append(r)
            status = "OK" if not r["error"] else r["error"]
            mac_str = ", ".join(r["macs"]) if r["macs"] else "-"
            print(
                f"  [{done}/{total}] {r['bmc_ip']} SN={r['sn']} "
                f"BMC_MAC={r['bmc_mac']} NIC_MAC=[{mac_str}] {status}",
                file=sys.stderr,
            )

    results.sort(key=lambda x: list(map(int, x["bmc_ip"].split("."))))

    # 确定最大 MAC 端口数
    max_ports = max((len(r["macs"]) for r in results), default=0)
    if max_ports == 0:
        max_ports = 2

    header = ["bmc_ip", "sn", "bmc_mac"] + [f"mac{i+1}" for i in range(max_ports)] + ["adapter", "error"]

    with open(output, "w", newline="", encoding="utf-8") as out_fh:
        writer = csv.writer(out_fh)
        writer.writerow(header)
        for r in results:
            macs = r["macs"] + [""] * (max_ports - len(r["macs"]))
            writer.writerow([r["bmc_ip"], r["sn"], r["bmc_mac"]] + macs + [r["adapter"], r["error"]])

    ok = sum(1 for r in results if not r["error"])
    fail = total - ok
    print(f"\n完成: 成功 {ok}, 失败 {fail}, 共 {total}", file=sys.stderr)
    print(f"结果已保存到: {output}", file=sys.stderr)


if __name__ == "__main__":
    main()
