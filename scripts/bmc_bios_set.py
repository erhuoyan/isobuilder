#!/usr/bin/env python3
"""
通过 Redfish API 批量修改 BIOS 属性（如关闭超线程）。

写入 pending settings，需重启服务器后生效。
"""

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
#  Redfish 操作
# ═══════════════════════════════════════════════════════════════════════════════

def redfish_get(ip, user, passwd, path, timeout=15):
    url = f"https://{ip}{path}"
    try:
        r = subprocess.run(
            ["curl", "-sk", "-u", f"{user}:{passwd}", "--connect-timeout", "10",
             "--max-time", str(timeout), url],
            capture_output=True, text=True, timeout=timeout + 5,
        )
        if r.returncode != 0:
            return None, None
        # 提取 ETag（某些 BMC 强制要求 If-Match）
        return json.loads(r.stdout), None
    except Exception:
        return None, None


def redfish_get_with_etag(ip, user, passwd, path, timeout=15):
    """GET 同时拿响应头里的 ETag。"""
    url = f"https://{ip}{path}"
    try:
        r = subprocess.run(
            ["curl", "-sk", "-u", f"{user}:{passwd}", "--connect-timeout", "10",
             "--max-time", str(timeout), "-D", "-", url],
            capture_output=True, text=True, timeout=timeout + 5,
        )
        if r.returncode != 0:
            return None, None
        # 分离 header 和 body（curl -D - 把 header 也写进 stdout）
        parts = r.stdout.split("\r\n\r\n", 1)
        if len(parts) < 2:
            parts = r.stdout.split("\n\n", 1)
        headers_raw = parts[0] if len(parts) == 2 else ""
        body = parts[1] if len(parts) == 2 else r.stdout

        etag = None
        for line in headers_raw.splitlines():
            if line.lower().startswith("etag:"):
                etag = line.split(":", 1)[1].strip()
                break

        return json.loads(body), etag
    except Exception:
        return None, None


def redfish_patch(ip, user, passwd, path, payload, etag=None, timeout=15):
    """PATCH，返回 (http_status, response_body)。"""
    url = f"https://{ip}{path}"
    cmd = [
        "curl", "-sk", "-u", f"{user}:{passwd}", "--connect-timeout", "10",
        "--max-time", str(timeout), "-X", "PATCH",
        "-H", "Content-Type: application/json",
        "-w", "\n__HTTP_CODE__%{http_code}",
    ]
    if etag:
        cmd += ["-H", f"If-Match: {etag}"]
    cmd += ["-d", json.dumps(payload), url]

    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout + 5)
        output = r.stdout
        # 提取 HTTP 状态码
        http_code = 0
        if "__HTTP_CODE__" in output:
            parts = output.rsplit("__HTTP_CODE__", 1)
            output = parts[0]
            try:
                http_code = int(parts[1].strip())
            except ValueError:
                pass
        body = None
        try:
            body = json.loads(output)
        except Exception:
            pass
        return http_code, body
    except Exception as e:
        return 0, {"error": str(e)}


# ═══════════════════════════════════════════════════════════════════════════════
#  单台处理
# ═══════════════════════════════════════════════════════════════════════════════

BIOS_PATH = "/redfish/v1/Systems/1/Bios"
BIOS_SETTINGS_PATH = "/redfish/v1/Systems/1/Bios/Settings"


def process_one(ip, user, passwd, attrs, dry_run=False):
    """
    对一台服务器设置 BIOS 属性。

    返回 dict: bmc_ip, sn, 每个属性的 before/after, status, error
    """
    result = {"bmc_ip": ip, "sn": "", "status": "", "error": ""}

    # 1) 读当前 BIOS 值
    bios_data, _ = redfish_get(ip, user, passwd, BIOS_PATH)
    if not bios_data:
        result["status"] = "FAIL"
        result["error"] = "无法连接BMC"
        return result

    result["sn"] = bios_data.get("SerialNumber", "") or ""
    # SN 可能在 Systems/1 而不在 Bios
    if not result["sn"]:
        sys_data, _ = redfish_get(ip, user, passwd, "/redfish/v1/Systems/1")
        if sys_data:
            result["sn"] = sys_data.get("SerialNumber", "")

    current_attrs = bios_data.get("Attributes", {}) or {}

    # 记录每个属性的前后值
    need_change = {}
    for key, target_val in attrs.items():
        current_val = current_attrs.get(key, "<未找到>")
        result[f"{key}_before"] = str(current_val)
        result[f"{key}_after"] = ""

        if str(current_val) == str(target_val):
            result[f"{key}_after"] = str(current_val)  # 已经是目标值
        else:
            need_change[key] = target_val

    if not need_change:
        result["status"] = "SKIP"
        for key in attrs:
            result[f"{key}_after"] = result[f"{key}_before"]
        return result

    if dry_run:
        result["status"] = "DRY_RUN"
        for key, val in need_change.items():
            result[f"{key}_after"] = f"→{val}(未执行)"
        for key in attrs:
            if key not in need_change:
                result[f"{key}_after"] = result[f"{key}_before"]
        return result

    # 2) GET Settings 端点拿 ETag
    _, etag = redfish_get_with_etag(ip, user, passwd, BIOS_SETTINGS_PATH)

    # 3) PATCH 写入 pending settings
    payload = {"Attributes": need_change}
    code, body = redfish_patch(ip, user, passwd, BIOS_SETTINGS_PATH, payload, etag=etag)

    if code == 200:
        result["status"] = "OK(重启生效)"
        for key, val in need_change.items():
            result[f"{key}_after"] = f"{val}(pending)"
        for key in attrs:
            if key not in need_change:
                result[f"{key}_after"] = result[f"{key}_before"]
    else:
        result["status"] = "FAIL"
        err_msg = ""
        if body:
            err_msg = json.dumps(body, ensure_ascii=False)[:200]
        result["error"] = f"HTTP {code}: {err_msg}"
        for key in attrs:
            result[f"{key}_after"] = result[f"{key}_before"]

    return result


# ═══════════════════════════════════════════════════════════════════════════════
#  输入解析（复用 bmc_inventory.py 同款）
# ═══════════════════════════════════════════════════════════════════════════════

def load_bmcinfo(filepath):
    hosts = []
    with open(filepath, encoding="utf-8") as f:
        raw = f.read()

    lines = raw.splitlines()
    if not lines:
        return hosts

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
        description="通过 Redfish API 批量修改 BIOS 属性（写入 pending settings，重启后生效）",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="\n".join([
            "",
            "属性通过 -s KEY=VALUE 指定，可重复多次:",
            "  -s SMT2En=Disabled          关闭超线程",
            "  -s BootTypeOrder=UEFI       设置启动类型",
            "  -s SMT2En=Disabled -s XX=YY 同时设置多个",
            "",
            "查看可用属性名和当前值:",
            "  curl -sk -u user:pass https://<BMC>/redfish/v1/Systems/1/Bios \\",
            "    | python3 -c \"import sys,json; [print(f'{k}: {v}')\"\\",
            "      \"for k,v in sorted(json.load(sys.stdin).get('Attributes',{}).items())]\"",
            "",
           f"默认账号: {DEFAULT_USER} / {DEFAULT_PASS}",
            "",
            "示例:",
            "  bmc_bios_set.py -f bmcinfo.txt -s SMT2En=Disabled",
            "  bmc_bios_set.py -f bmcinfo.txt -s SMT2En=Disabled --dry-run",
            "  bmc_bios_set.py -f ip_list.txt -s SMT2En=Disabled -o result.csv -j 20",
            "",
            "注意: 修改写入 pending settings，需要重启服务器才生效。",
            "      已经是目标值的机器会 SKIP，不做任何写入。",
            "",
        ]),
    )
    parser.add_argument("-f", "--file", required=True, metavar="FILE",
                        help="BMC 信息文件 (CSV / TSV / 空格分隔 / 纯 IP 列表)")
    parser.add_argument("-s", "--set", required=True, action="append", metavar="KEY=VALUE",
                        dest="settings",
                        help="BIOS 属性 KEY=VALUE，可多次指定")
    parser.add_argument("--dry-run", action="store_true",
                        help="只读取当前值，不实际修改")
    parser.add_argument("-u", "--user", default=None, metavar="USER",
                        help="全局 BMC 用户名 (默认: %s)" % DEFAULT_USER)
    parser.add_argument("-p", "--password", default=None, metavar="PASS",
                        help="全局 BMC 密码 (默认: %s)" % DEFAULT_PASS.replace("%", "%%"))
    parser.add_argument("-o", "--output", default=None, metavar="CSV",
                        help="输出 CSV 路径 (默认: bios_set_<时间>.csv)")
    parser.add_argument("-j", "--jobs", type=int, default=10, metavar="N",
                        help="并发数 (默认: 10)")

    args = parser.parse_args()

    # 解析 KEY=VALUE
    attrs = {}
    for s in args.settings:
        if "=" not in s:
            print(f"错误: '{s}' 格式不对，应为 KEY=VALUE", file=sys.stderr)
            sys.exit(1)
        k, v = s.split("=", 1)
        attrs[k.strip()] = v.strip()

    if not attrs:
        print("错误: 至少指定一个 -s KEY=VALUE", file=sys.stderr)
        sys.exit(1)

    output = args.output
    if not output:
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        output = os.path.join(
            os.path.dirname(os.path.abspath(args.file)),
            f"bios_set_{ts}.csv",
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
    mode = "DRY RUN（不修改）" if args.dry_run else "执行修改"
    attr_str = ", ".join(f"{k}={v}" for k, v in attrs.items())
    print(f"目标: {attr_str}", file=sys.stderr)
    print(f"模式: {mode}, 共 {total} 台, 并发: {args.jobs}", file=sys.stderr)
    print(f"{'─' * 60}", file=sys.stderr)

    results = []
    done = 0
    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        futures = {
            pool.submit(process_one, h["ip"], h["user"], h["pass"], attrs, args.dry_run): h
            for h in hosts
        }
        for future in as_completed(futures):
            done += 1
            r = future.result()
            results.append(r)

            # 构建属性变化摘要
            changes = []
            for key in attrs:
                before = r.get(f"{key}_before", "?")
                after = r.get(f"{key}_after", "?")
                changes.append(f"{key}: {before} → {after}")
            change_str = ", ".join(changes)

            print(
                f"  [{done}/{total}] {r['bmc_ip']} [{r['status']}] {change_str}"
                + (f" ERROR: {r['error']}" if r["error"] else ""),
                file=sys.stderr,
            )

    results.sort(key=lambda x: list(map(int, x["bmc_ip"].split("."))))

    # 写 CSV
    attr_keys = list(attrs.keys())
    header = ["bmc_ip", "sn"]
    for key in attr_keys:
        header += [f"{key}_before", f"{key}_after"]
    header += ["status", "error"]

    with open(output, "w", newline="", encoding="utf-8") as out_fh:
        writer = csv.writer(out_fh)
        writer.writerow(header)
        for r in results:
            row = [r["bmc_ip"], r["sn"]]
            for key in attr_keys:
                row += [r.get(f"{key}_before", ""), r.get(f"{key}_after", "")]
            row += [r["status"], r["error"]]
            writer.writerow(row)

    ok = sum(1 for r in results if r["status"] in ("OK(重启生效)", "SKIP"))
    skip = sum(1 for r in results if r["status"] == "SKIP")
    fail = sum(1 for r in results if r["status"] == "FAIL")
    print(f"{'─' * 60}", file=sys.stderr)
    print(f"完成: 成功 {ok - skip}, 跳过 {skip}, 失败 {fail}, 共 {total}", file=sys.stderr)
    if skip:
        print(f"  跳过 = 已经是目标值，无需修改", file=sys.stderr)
    if ok - skip > 0 and not args.dry_run:
        print(f"  ⚠ 修改已写入 pending，需要重启服务器才生效", file=sys.stderr)
    print(f"结果已保存到: {output}", file=sys.stderr)


if __name__ == "__main__":
    main()
