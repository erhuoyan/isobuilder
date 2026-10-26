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
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime

# ── 默认凭据 ────────────────────────────────────────────────────────────────
DEFAULT_USER = "jdroot"
DEFAULT_PASS = "JCss%6!8"


# ═══════════════════════════════════════════════════════════════════════════════
#  Redfish 操作
# ═══════════════════════════════════════════════════════════════════════════════

# 本线程最后一次请求的失败原因。并发用 ThreadPoolExecutor，不能用模块级全局变量。
# 只影响报错文案，函数返回值保持原样。
_err = threading.local()


def _reason_suffix():
    return ": " + _err.reason if getattr(_err, "reason", "") else ""


def redfish_get(ip, user, passwd, path, timeout=15):
    """只要 body；要 ETag 时直接用 redfish_get_with_etag。"""
    return redfish_get_with_etag(ip, user, passwd, path, timeout)


def redfish_get_with_etag(ip, user, passwd, path, timeout=15):
    """GET 同时拿响应头里的 ETag。失败返回 (None, None)，原因记到本线程错误槽。"""
    url = f"https://{ip}{path}"
    _err.reason = ""
    try:
        r = subprocess.run(
            ["curl", "-sk", "-u", f"{user}:{passwd}", "--connect-timeout", "10",
             "--max-time", str(timeout), "-D", "-", url],
            # capture_output/text 是 Python 3.7 才有的参数：3.6 上会抛 TypeError，
            # 被下面的 except 吞掉后误报成“连不上 BMC”。用 3.6 也认的写法。
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            universal_newlines=True, timeout=timeout + 5,
        )
    except Exception as e:
        _err.reason = "%s: %s" % (type(e).__name__, e)
        return None, None
    if r.returncode != 0:
        err = (r.stderr or "").strip()[:200]
        _err.reason = "curl exit %d: %s" % (r.returncode, err) if err else \
            "curl exit %d" % r.returncode
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

    try:
        return json.loads(body), etag
    except Exception:
        _err.reason = "响应不是合法 JSON: %s" % body.strip()[:120]
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
        r = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           universal_newlines=True, timeout=timeout + 5)
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
        result["error"] = "无法连接BMC" + _reason_suffix()
        return result

    result["sn"] = bios_data.get("SerialNumber", "") or ""
    # SN 可能在 Systems/1 而不在 Bios
    if not result["sn"]:
        sys_data, _ = redfish_get(ip, user, passwd, "/redfish/v1/Systems/1")
        if sys_data:
            result["sn"] = sys_data.get("SerialNumber", "")

    effective = bios_data.get("Attributes", {}) or {}

    # 2) 读 pending（同时拿 ETag，华为 BMC 强制 If-Match）。
    #    只看生效值会让重启前的重复执行反复写同一个 pending，这里一并检查。
    settings_data, etag = redfish_get_with_etag(ip, user, passwd, BIOS_SETTINGS_PATH)
    pending = (settings_data or {}).get("Attributes") or {}

    need_change = {}
    already_effective = True
    for key, target_val in attrs.items():
        eff_val = effective.get(key, "<未找到>")
        pend_val = pending.get(key)
        result[f"{key}_before"] = str(eff_val)

        if str(eff_val) == str(target_val):
            result[f"{key}_after"] = str(eff_val)
        elif pend_val is not None and str(pend_val) == str(target_val):
            result[f"{key}_after"] = f"{target_val}(已pending)"
            already_effective = False
        else:
            need_change[key] = target_val
            result[f"{key}_after"] = str(eff_val)
            already_effective = False

    if not need_change:
        result["status"] = "SKIP(已生效)" if already_effective else "SKIP(已pending)"
        return result

    if dry_run:
        result["status"] = "DRY_RUN"
        for key, val in need_change.items():
            result[f"{key}_after"] = f"→{val}(未执行)"
        return result

    # 3) PATCH 写入 pending settings
    payload = {"Attributes": need_change}
    code, body = redfish_patch(ip, user, passwd, BIOS_SETTINGS_PATH, payload, etag=etag)

    if code == 200:
        result["status"] = "OK(重启生效)"
        for key, val in need_change.items():
            result[f"{key}_after"] = f"{val}(pending)"
    else:
        result["status"] = "FAIL"
        err_msg = json.dumps(body, ensure_ascii=False)[:200] if body else ""
        result["error"] = f"HTTP {code}: {err_msg}"
        for key in need_change:
            result[f"{key}_after"] = result[f"{key}_before"]

    return result


def query_one(ip, user, passwd, keys):
    """
    查询一台服务器的 BIOS 属性。

    keys 为 None 表示列出全部。返回:
      bmc_ip, sn, status, error, attrs: {key: {"effective": v, "pending": v}}
    """
    result = {"bmc_ip": ip, "sn": "", "status": "", "error": "", "attrs": {}}

    bios_data, _ = redfish_get(ip, user, passwd, BIOS_PATH)
    if not bios_data:
        result["status"] = "FAIL"
        result["error"] = "无法连接BMC" + _reason_suffix()
        return result

    result["sn"] = bios_data.get("SerialNumber", "") or ""
    if not result["sn"]:
        sys_data, _ = redfish_get(ip, user, passwd, "/redfish/v1/Systems/1")
        if sys_data:
            result["sn"] = sys_data.get("SerialNumber", "")

    effective = bios_data.get("Attributes", {}) or {}
    settings_data, _ = redfish_get(ip, user, passwd, BIOS_SETTINGS_PATH)
    pending = (settings_data or {}).get("Attributes") or {}

    if keys:
        for k in keys:
            result["attrs"][k] = {
                "effective": effective.get(k),
                "pending": pending.get(k),
            }
    else:
        for k in sorted(effective):
            result["attrs"][k] = {
                "effective": effective[k],
                "pending": pending.get(k),
            }

    result["status"] = "OK"
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
            "三种模式:",
            "  修改  -s KEY=VALUE   写入 pending settings（重启后生效）",
            "  查询  -g KEY         查指定属性的生效值 + pending 值",
            "  列出  --list         列出全部属性（约 400 项）",
            "",
            "修改示例:",
            "  -s SMT2En=Disabled          关闭超线程",
            "  -s SMT2En=Disabled -s XX=YY 同时设置多个",
            "",
           f"默认账号: {DEFAULT_USER} / {DEFAULT_PASS}",
            "",
            "示例:",
            "  bmc_bios_set.py -f bmcinfo.txt -g SMT2En          # 查询",
            "  bmc_bios_set.py -f bmcinfo.txt --list             # 列出全部属性",
            "  bmc_bios_set.py -f bmcinfo.txt -s SMT2En=Disabled # 修改",
            "  bmc_bios_set.py -f bmcinfo.txt -s SMT2En=Disabled --dry-run",
            "  bmc_bios_set.py -f ip_list.txt -s SMT2En=Disabled -o result.csv -j 20",
            "",
            "注意: 修改写入 pending settings，需要重启服务器才生效。",
            "      已生效或已 pending 的机器会 SKIP，不重复写入。",
            "",
        ]),
    )
    parser.add_argument("-f", "--file", required=True, metavar="FILE",
                        help="BMC 信息文件 (CSV / TSV / 空格分隔 / 纯 IP 列表)")
    parser.add_argument("-s", "--set", action="append", metavar="KEY=VALUE",
                        dest="settings",
                        help="BIOS 属性 KEY=VALUE，可多次指定")
    parser.add_argument("-g", "--get", action="append", metavar="KEY", dest="get_keys",
                        help="查询指定 BIOS 属性的当前值（可多次指定，不修改）")
    parser.add_argument("--list", action="store_true",
                        help="列出全部 BIOS 属性及当前值（不修改）")
    parser.add_argument("--dry-run", action="store_true",
                        help="配合 -s 使用：只读取当前值，不实际修改")
    parser.add_argument("-u", "--user", default=None, metavar="USER",
                        help="全局 BMC 用户名 (默认: %s)" % DEFAULT_USER)
    parser.add_argument("-p", "--password", default=None, metavar="PASS",
                        help="全局 BMC 密码 (默认: %s)" % DEFAULT_PASS.replace("%", "%%"))
    parser.add_argument("-o", "--output", default=None, metavar="CSV",
                        help="输出 CSV 路径 (默认: bios_set_<时间>.csv)")
    parser.add_argument("-j", "--jobs", type=int, default=10, metavar="N",
                        help="并发数 (默认: 10)")

    args = parser.parse_args()

    # 查询模式与修改模式互斥
    query_mode = bool(args.get_keys) or args.list
    if query_mode and args.settings:
        print("错误: 查询模式（-g/--list）不能与 -s 同时使用", file=sys.stderr)
        sys.exit(1)
    if not query_mode and not args.settings:
        print("错误: 需要 -s KEY=VALUE（修改）、-g KEY（查询）或 --list（列出全部）",
              file=sys.stderr)
        sys.exit(1)

    # 解析 KEY=VALUE
    attrs = {}
    for s in (args.settings or []):
        if "=" not in s:
            print(f"错误: '{s}' 格式不对，应为 KEY=VALUE", file=sys.stderr)
            sys.exit(1)
        k, v = s.split("=", 1)
        attrs[k.strip()] = v.strip()

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

    # ── 查询模式 ─────────────────────────────────────────────────────────
    if query_mode:
        label = "全部属性" if args.list else ", ".join(args.get_keys)
        print(f"查询: {label}, 共 {total} 台, 并发: {args.jobs}", file=sys.stderr)
        print(f"{'─' * 60}", file=sys.stderr)

        qresults = []
        done = 0
        with ThreadPoolExecutor(max_workers=args.jobs) as pool:
            futures = {
                pool.submit(query_one, h["ip"], h["user"], h["pass"],
                            None if args.list else args.get_keys): h
                for h in hosts
            }
            for future in as_completed(futures):
                done += 1
                r = future.result()
                qresults.append(r)
                if r["error"]:
                    print(f"  [{done}/{total}] {r['bmc_ip']} FAIL: {r['error']}",
                          file=sys.stderr)
                else:
                    items = "; ".join(
                        f"{k}={v['effective']}"
                        + (f"(pending:{v['pending']})" if v["pending"] is not None else "")
                        for k, v in r["attrs"].items()
                    )
                    print(f"  [{done}/{total}] {r['bmc_ip']} SN={r['sn']} {items}",
                          file=sys.stderr)

        qresults.sort(key=lambda x: list(map(int, x["bmc_ip"].split("."))))

        # 列：bmc_ip, sn + 每个查询键的 effective/pending
        col_keys = args.get_keys if args.get_keys else sorted(
            {k for r in qresults for k in r["attrs"]}
        )
        header = ["bmc_ip", "sn"]
        for k in col_keys:
            header += [k, f"{k}_pending"]
        header.append("error")

        with open(output, "w", newline="", encoding="utf-8") as out_fh:
            writer = csv.writer(out_fh)
            writer.writerow(header)
            for r in qresults:
                row = [r["bmc_ip"], r["sn"]]
                for k in col_keys:
                    a = r["attrs"].get(k) or {}
                    eff = a.get("effective")
                    pend = a.get("pending")
                    row += ["" if eff is None else eff,
                            "" if pend is None else pend]
                row.append(r["error"])
                writer.writerow(row)

        fail = sum(1 for r in qresults if r["error"])
        print(f"{'─' * 60}", file=sys.stderr)
        print(f"完成: 成功 {total - fail}, 失败 {fail}, 共 {total}", file=sys.stderr)
        print(f"结果已保存到: {output}", file=sys.stderr)
        return

    # ── 修改模式 ─────────────────────────────────────────────────────────
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

    changed = sum(1 for r in results if r["status"] == "OK(重启生效)")
    skip_eff = sum(1 for r in results if r["status"] == "SKIP(已生效)")
    skip_pend = sum(1 for r in results if r["status"] == "SKIP(已pending)")
    dry = sum(1 for r in results if r["status"] == "DRY_RUN")
    fail = sum(1 for r in results if r["status"] == "FAIL")
    print(f"{'─' * 60}", file=sys.stderr)
    print(f"完成: 写入 {changed}, 已生效跳过 {skip_eff}, 已pending跳过 {skip_pend}, "
          f"失败 {fail}, 共 {total}", file=sys.stderr)
    if dry:
        print(f"  DRY RUN: {dry} 台待修改（未写入）", file=sys.stderr)
    if changed:
        print(f"  ⚠ 修改已写入 pending，需要重启服务器才生效", file=sys.stderr)
    print(f"结果已保存到: {output}", file=sys.stderr)


if __name__ == "__main__":
    main()
