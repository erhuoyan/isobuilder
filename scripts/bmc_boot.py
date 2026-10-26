#!/usr/bin/env python3
"""
通过 Redfish API 批量控制服务器下一次的启动设备（PXE / 虚拟光驱 / BIOS 设置），
可选在设置后立即重启。

与 bmc_bios_set.py 的区别：
  - bmc_bios_set.py  修改 BIOS 持久配置（写 pending，需重启生效）
  - bmc_boot.py      设置一次性启动重定向（BootSourceOverride），重启即生效

用法:
  bmc_boot.py -f bmcinfo.txt -t pxe             # 下次启动走 PXE（不重启）
  bmc_boot.py -f bmcinfo.txt -t pxe --reset     # 设置并重启
  bmc_boot.py -f bmcinfo.txt -t cd --iso https://x/y.iso --reset
  bmc_boot.py -f bmcinfo.txt -t bios --reset    # 重启进 BIOS 设置界面
  bmc_boot.py -f bmcinfo.txt --status           # 只查电源状态和当前引导源
  bmc_boot.py -f bmcinfo.txt --clear            # 清除引导重定向
  bmc_boot.py -f bmcinfo.txt --vnc-ssl off      # 关掉 VNC 的 TLS（改明文 RFB）
"""

import argparse
import csv
import io
import json
import os
import subprocess
import sys
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime

# ── 默认凭据 ────────────────────────────────────────────────────────────────
DEFAULT_USER = "jdroot"
DEFAULT_PASS = "JCss%6!8"

# ── 路径 ────────────────────────────────────────────────────────────────────
SYSTEM_PATH = "/redfish/v1/Systems/1"
RESET_PATH = "/redfish/v1/Systems/1/Actions/ComputerSystem.Reset"
VM_CD_PATH = "/redfish/v1/Managers/1/VirtualMedia/CD"
VM_INSERT_PATH = "/redfish/v1/Managers/1/VirtualMedia/CD/Actions/VirtualMedia.InsertMedia"
VM_EJECT_PATH = "/redfish/v1/Managers/1/VirtualMedia/CD/Actions/VirtualMedia.EjectMedia"

# VNC 开关在 NetworkProtocol 的 OEM 段（标准 RFB 字段只读）；
# 密码与 TLS 开关（SSLEncryptionEnabled）都在 VncService，
# 密码最长 8 位，超长报 StringValueTooLong。
NETPROTO_PATH = "/redfish/v1/Managers/1/NetworkProtocol"
VNC_SERVICE_PATH = "/redfish/v1/Managers/1/VncService"
VNC_PASSWORD_MAX = 8

# 用户输入 → Redfish BootSourceOverrideTarget
TARGET_MAP = {
    "none": "None",
    "pxe": "Pxe",
    "cd": "Cd",
    "iso": "Cd",
    "hdd": "Hdd",
    "bios": "BiosSetup",
    "floppy": "Floppy",
}


# ═══════════════════════════════════════════════════════════════════════════════
#  Redfish 基础操作
# ═══════════════════════════════════════════════════════════════════════════════

# 本线程最后一次请求的失败原因。并发用 ThreadPoolExecutor，不能用模块级全局变量。
# 只影响报错文案，函数返回值保持原样。
_err = threading.local()


def _reason_suffix():
    return ": " + _err.reason if getattr(_err, "reason", "") else ""


def _curl(args, timeout):
    """执行 curl；返回 stdout，失败返回 None 并把原因记到本线程错误槽。"""
    _err.reason = ""
    try:
        r = subprocess.run(
            ["curl", "-sk", "--connect-timeout", "10", "--max-time", str(timeout)] + args,
            # 用 3.6 也能跑的写法：capture_output/text 是 3.7 新增参数，
            # 在 3.6 上会抛 TypeError 且被下面的 except 吞掉，误报“无法连接BMC”
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
    return r.stdout


def redfish_get(ip, user, passwd, path, timeout=15):
    """GET 返回 (body_dict, etag)。etag 来自响应头。"""
    out = _curl(["-u", f"{user}:{passwd}", "-D", "-", f"https://{ip}{path}"], timeout)
    if not out:
        if not _err.reason:
            _err.reason = "curl 无输出"
        return None, None

    sep = "\r\n\r\n" if "\r\n\r\n" in out else "\n\n"
    parts = out.split(sep, 1)
    if len(parts) != 2:
        _err.reason = "响应缺少 HTTP 头/体分隔"
        return None, None
    headers, body = parts

    etag = None
    for line in headers.splitlines():
        if line.lower().startswith("etag:"):
            etag = line.split(":", 1)[1].strip()
            break

    try:
        return json.loads(body), etag
    except Exception:
        _err.reason = "响应不是合法 JSON: %s" % body.strip()[:120]
        return None, None


def redfish_patch(ip, user, passwd, path, payload, etag=None, timeout=20):
    """PATCH，返回 (http_code, body)。412 时自动带 ETag 重试一次。"""
    def run(with_etag):
        args = ["-u", f"{user}:{passwd}", "-X", "PATCH",
                "-H", "Content-Type: application/json",
                "-w", "\n__CODE__%{http_code}"]
        if with_etag:
            args += ["-H", f"If-Match: {with_etag}"]
        args += ["-d", json.dumps(payload), f"https://{ip}{path}"]

        out = _curl(args, timeout)
        if out is None:
            return 0, None

        code = 0
        if "__CODE__" in out:
            body_str, code_str = out.rsplit("__CODE__", 1)
            out = body_str
            try:
                code = int(code_str.strip())
            except ValueError:
                pass
        try:
            return code, json.loads(out)
        except Exception:
            return code, None

    code, body = run(etag)
    # 华为 BMC 强制 If-Match：没带 ETag 会返回 412
    if code == 412 and etag is None:
        _, fresh_etag = redfish_get(ip, user, passwd, path)
        if fresh_etag:
            code, body = run(fresh_etag)
    return code, body


def redfish_post(ip, user, passwd, path, payload=None, timeout=30):
    """POST 动作，返回 (http_code, body)。412 时带 ETag 重试一次。"""
    def run(with_etag):
        args = ["-u", f"{user}:{passwd}", "-X", "POST",
                "-H", "Content-Type: application/json",
                "-w", "\n__CODE__%{http_code}"]
        if with_etag:
            args += ["-H", f"If-Match: {with_etag}"]
        args += ["-d", json.dumps(payload or {}), f"https://{ip}{path}"]

        out = _curl(args, timeout)
        if out is None:
            return 0, None

        code = 0
        if "__CODE__" in out:
            body_str, code_str = out.rsplit("__CODE__", 1)
            out = body_str
            try:
                code = int(code_str.strip())
            except ValueError:
                pass
        try:
            return code, json.loads(out)
        except Exception:
            return code, None

    code, body = run(None)
    if code == 412:
        _, etag = redfish_get(ip, user, passwd, SYSTEM_PATH)
        if etag:
            code, body = run(etag)
    return code, body


# ═══════════════════════════════════════════════════════════════════════════════
#  单台处理
# ═══════════════════════════════════════════════════════════════════════════════

def _wait_vm_ready(ip, user, passwd, expect_inserted, timeout=90):
    """
    轮询 VirtualMedia，直到 Inserted 与预期一致。

    InsertMedia/EjectMedia 是异步动作（HTTP 202 + Task），返回 202 只代表
    "请求已受理"，镜像可能因网络/路径/权限问题挂载失败。必须回读实际状态，
    否则失败会被当成成功。
    """
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        data, _ = redfish_get(ip, user, passwd, VM_CD_PATH)
        if data:
            last = data
            if bool(data.get("Inserted")) == expect_inserted:
                return data
        time.sleep(2)
    return last


def _check_iso_url(url):
    """校验镜像 URL 形式，返回错误消息或 None。"""
    if url.lower().startswith("nfs://"):
        rest = url[6:]
        if "/" not in rest:
            return "NFS URL 缺少路径，应为 nfs://<host>/<服务器绝对路径>"
        host, path = rest.split("/", 1)
        if not host:
            return "NFS URL 缺少主机名"
        # 常见错误：填了本机挂载点而不是服务器导出路径
        if path.startswith(("mnt/", "media/", "mount/")):
            return (f"NFS 路径 '{path}' 看起来是本机挂载点；"
                    "应填 BMC 直接访问的服务器绝对路径，如 /export/iso/x.iso")
        return None
    if url.lower().startswith(("https://", "cifs://", "smb://")):
        return None
    return f"不支持的协议: {url}（仅支持 nfs:// / https:// / cifs://）"


def process_one(ip, user, passwd, opts):
    """
    对一台服务器执行：可选插入 ISO → 设置引导重定向 → 可选重启。

    返回 dict: bmc_ip, sn, power_before, target_before, target_after, status, error
    """
    r = {"bmc_ip": ip, "sn": "", "power_before": "", "target_before": "",
         "target_after": "", "iso_before": "", "vnc": "", "vnc_ssl": "",
         "status": "", "error": ""}

    sysdata, etag = redfish_get(ip, user, passwd, SYSTEM_PATH)
    if not sysdata:
        r["status"] = "FAIL"
        r["error"] = "无法连接BMC" + _reason_suffix()
        return r

    r["sn"] = sysdata.get("SerialNumber", "")
    boot = sysdata.get("Boot", {}) or {}
    r["power_before"] = sysdata.get("PowerState", "")
    cur_target = boot.get("BootSourceOverrideTarget", "None")
    cur_enabled = boot.get("BootSourceOverrideEnabled", "Disabled")
    r["target_before"] = f"{cur_target}/{cur_enabled}"

    # 虚拟光驱现状（决定插入前是否需要先弹旧盘）
    vm_cur, _ = redfish_get(ip, user, passwd, VM_CD_PATH)
    cur_inserted = bool((vm_cur or {}).get("Inserted"))
    r["iso_before"] = (vm_cur or {}).get("ImageName") or ""

    # ── 只查询 ───────────────────────────────────────────────────────────
    if opts["status"]:
        r["target_after"] = r["target_before"]
        net, _ = redfish_get(ip, user, passwd, NETPROTO_PATH)
        vnc = (net or {}).get("Oem", {}).get("JD", {}).get("VNC", {})
        r["vnc"] = "on" if vnc.get("ProtocolEnabled") else "off"
        svc, _ = redfish_get(ip, user, passwd, VNC_SERVICE_PATH)
        if svc is not None:
            r["vnc_ssl"] = "on" if svc.get("SSLEncryptionEnabled") else "off"
        r["status"] = "OK"
        return r

    vm_note = ""

    # ── VNC 开关 / SSL / 改密码 ──────────────────────────────────────────
    if opts["vnc"] or opts["vnc_ssl"]:
        want_on = opts["vnc"] == "on"
        want_ssl = opts["vnc_ssl"] == "on"
        if opts["dry_run"]:
            if opts["vnc"]:
                r["vnc"] = "on" if want_on else "off"
            if opts["vnc_ssl"]:
                r["vnc_ssl"] = "on" if want_ssl else "off"
            r["status"] = "DRY_RUN"
            r["target_after"] = r["target_before"]
            return r

        notes = []

        # 开关：标准 RFB 字段只读，必须写 OEM 段
        if opts["vnc"]:
            net, net_etag = redfish_get(ip, user, passwd, NETPROTO_PATH)
            if net is None:
                r["status"] = "FAIL"
                r["error"] = "无法读取 NetworkProtocol"
                return r
            code, body = redfish_patch(
                ip, user, passwd, NETPROTO_PATH,
                {"Oem": {"JD": {"VNC": {"ProtocolEnabled": want_on}}}},
                etag=net_etag,
            )
            if code != 200:
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = f"VNC 开关 HTTP {code}: {_brief(body)}"
                return r
            notes.append("VNC开" if want_on else "VNC关")

        # SSL 与密码同属 VncService，合并成一次 PATCH
        svc_payload = {}
        if opts["vnc_ssl"]:
            svc_payload["SSLEncryptionEnabled"] = want_ssl
        if opts["vnc_pass"]:
            if len(opts["vnc_pass"]) > VNC_PASSWORD_MAX:
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = (f"VNC 密码最多 {VNC_PASSWORD_MAX} 位"
                              f"（当前 {len(opts['vnc_pass'])} 位）")
                return r
            svc_payload["Password"] = opts["vnc_pass"]

        if svc_payload:
            _, vnc_etag = redfish_get(ip, user, passwd, VNC_SERVICE_PATH)
            code, body = redfish_patch(
                ip, user, passwd, VNC_SERVICE_PATH, svc_payload, etag=vnc_etag,
            )
            if code != 200:
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = f"VNC 配置 HTTP {code}: {_brief(body)}"
                return r
            if opts["vnc_ssl"]:
                notes.append("SSL开" if want_ssl else "SSL关")
            if opts["vnc_pass"]:
                notes.append("密码已设")

        # 回读确认，避免 200 假成功
        if opts["vnc"]:
            net, _ = redfish_get(ip, user, passwd, NETPROTO_PATH)
            actual = bool((net or {}).get("Oem", {}).get("JD", {}).get("VNC", {})
                          .get("ProtocolEnabled"))
            r["vnc"] = "on" if actual else "off"
            if actual != want_on:
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = (f"VNC 写入后状态不符"
                              f"（期望 {'on' if want_on else 'off'}，实际 {r['vnc']}）")
                return r

        if opts["vnc_ssl"]:
            svc, _ = redfish_get(ip, user, passwd, VNC_SERVICE_PATH)
            if svc is None:
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = "VNC SSL 写入后无法回读 VncService"
                return r
            actual_ssl = bool(svc.get("SSLEncryptionEnabled"))
            r["vnc_ssl"] = "on" if actual_ssl else "off"
            if actual_ssl != want_ssl:
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = (f"VNC SSL 写入后状态不符"
                              f"（期望 {'on' if want_ssl else 'off'}，实际 {r['vnc_ssl']}）")
                return r

        r["status"] = "OK(" + "+".join(notes) + ")"
        r["target_after"] = r["target_before"]
        return r

    # 同一个镜像已挂载时无需反复弹/挂（批量重跑时省掉几十秒）
    same_iso = (opts["iso"] and cur_inserted
                and (vm_cur or {}).get("Image") == opts["iso"])
    if same_iso:
        vm_note = "ISO已挂载+"

    # ── 弹出 ISO（--eject，或插入新 ISO 前先弹旧盘）───────────────────────
    should_eject = opts["eject"] or (opts["iso"] and cur_inserted and not same_iso)
    if should_eject:
        if opts["dry_run"]:
            if not opts["target"] and not opts["iso"]:
                r["target_after"] = r["target_before"]
                r["status"] = "DRY_RUN"
                return r
        else:
            code, body = redfish_post(ip, user, passwd, VM_EJECT_PATH)
            if code not in (200, 202, 204):
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = f"EjectMedia HTTP {code}: {_brief(body)}"
                return r
            vm = _wait_vm_ready(ip, user, passwd, False, opts["wait"])
            if vm and vm.get("Inserted"):
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = "EjectMedia 未生效（Inserted 仍为 true）"
                return r
            if not opts["target"] and not opts["iso"]:
                r["target_after"] = r["target_before"]
                r["status"] = "OK(已弹出)"
                return r
            vm_note = "弹旧盘+"

    # ── 插入 ISO ─────────────────────────────────────────────────────────
    if opts["iso"] and not same_iso:
        if opts["dry_run"]:
            r["target_after"] = r["target_before"]
            r["status"] = "DRY_RUN"
            return r

        # 注意：华为 BMC 不接受 Inserted 字段（Base.1.0.ActionParameterUnknown），
        # 只认 Image / TransferProtocolType / UserName / Password。
        payload = {"Image": opts["iso"]}
        proto = _proto_of(opts["iso"])
        if proto:
            payload["TransferProtocolType"] = proto
        if opts["vm_user"]:
            payload["UserName"] = opts["vm_user"]
        if opts["vm_pass"]:
            payload["Password"] = opts["vm_pass"]

        code, body = redfish_post(ip, user, passwd, VM_INSERT_PATH, payload)
        if code not in (200, 202, 204):
            r["target_after"] = r["target_before"]
            r["status"] = "FAIL"
            r["error"] = f"InsertMedia HTTP {code}: {_brief(body)}"
            return r

        # 202 只是受理，回读确认真的挂上了
        vm = _wait_vm_ready(ip, user, passwd, True, opts["wait"])
        if not vm or not vm.get("Inserted"):
            r["target_after"] = r["target_before"]
            r["status"] = "FAIL"
            detail = ""
            if vm:
                detail = f" (Image={vm.get('Image')}, Via={vm.get('ConnectedVia')})"
            r["error"] = (f"InsertMedia 未生效{detail}；"
                          "检查 BMC 能否访问该主机、路径是否为服务器绝对路径")
            return r
        vm_note += "已挂载ISO+"

    # ── 设置引导重定向 ───────────────────────────────────────────────────
    want_target = opts["target"]          # 已映射为 Redfish 值，可能为 None
    reset_type = opts["reset_type"]

    # 电源关闭时 ForceRestart / PowerCycle 无效，改用 On 开机
    if (opts["reset"] and r["power_before"] == "Off"
            and reset_type in ("ForceRestart", "GracefulRestart", "PowerCycle")):
        reset_type = "On"

    if want_target is not None:
        same = (cur_target == want_target and cur_enabled == opts["mode"])
        if same:
            r["target_after"] = r["target_before"]
            changed = False
        elif opts["dry_run"]:
            r["target_after"] = f"{want_target}/{opts['mode']}"
            r["status"] = "DRY_RUN"
            return r
        else:
            code, body = redfish_patch(
                ip, user, passwd, SYSTEM_PATH,
                {"Boot": {"BootSourceOverrideTarget": want_target,
                          "BootSourceOverrideEnabled": opts["mode"]}},
                etag=etag,
            )
            if code != 200:
                r["target_after"] = r["target_before"]
                r["status"] = "FAIL"
                r["error"] = f"PATCH Boot HTTP {code}: {_brief(body)}"
                return r
            r["target_after"] = f"{want_target}/{opts['mode']}"
            changed = True
    else:
        r["target_after"] = r["target_before"]
        changed = False

    # ── 重启 ─────────────────────────────────────────────────────────────
    if not opts["reset"]:
        if changed or vm_note:
            r["status"] = f"OK({vm_note}已设置)"
        else:
            r["status"] = "SKIP(状态一致)"
        return r

    if opts["dry_run"]:
        r["status"] = "DRY_RUN"
        return r

    code, body = redfish_post(ip, user, passwd, RESET_PATH, {"ResetType": reset_type})
    if code in (200, 202, 204):
        tail = "已设置并重启" if changed else "已重启"
        r["status"] = f"OK({vm_note}{tail}:{reset_type})"
    else:
        r["status"] = "FAIL"
        r["error"] = f"Reset HTTP {code}: {_brief(body)}"
    return r


def _proto_of(url):
    low = url.lower()
    if low.startswith("https://"):
        return "HTTPS"
    if low.startswith("cifs://") or low.startswith("smb://"):
        return "CIFS"
    if low.startswith("nfs://"):
        return "NFS"
    return None


def _brief(body, limit=160):
    if not body:
        return ""
    return json.dumps(body, ensure_ascii=False)[:limit]


# ═══════════════════════════════════════════════════════════════════════════════
#  输入解析
# ═══════════════════════════════════════════════════════════════════════════════

def load_bmcinfo(filepath):
    hosts = []
    with open(filepath, encoding="utf-8") as f:
        raw = f.read()

    lines = raw.splitlines()
    if not lines:
        return hosts

    is_csv = sum(1 for ln in lines if "," in ln) > len(lines) * 0.5

    if is_csv:
        for row in csv.reader(io.StringIO(raw)):
            row = [c.strip() for c in row if c.strip()]
            if not row or "bmc_ip" in row[0].lower():
                continue
            hosts.append({
                "ip": row[0],
                "user": row[1] if len(row) >= 3 else None,
                "pass": row[2] if len(row) >= 3 else None,
            })
    else:
        for line in lines:
            line = line.strip()
            if not line or "bmc_ip" in line.lower():
                continue
            parts = line.split("\t")
            if len(parts) < 2:
                parts = line.split()
            hosts.append({
                "ip": parts[0],
                "user": parts[1] if len(parts) >= 3 else None,
                "pass": parts[2] if len(parts) >= 3 else None,
            })

    return hosts


# ═══════════════════════════════════════════════════════════════════════════════
#  主流程
# ═══════════════════════════════════════════════════════════════════════════════

def main():
    if len(sys.argv) == 1:
        sys.argv.append("-h")

    parser = argparse.ArgumentParser(
        description="通过 Redfish API 批量设置服务器下次启动设备（PXE/虚拟光驱/BIOS），可立即重启",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="\n".join([
            "",
            "启动目标 (-t):",
            "  pxe     PXE 网络启动",
            "  cd/iso  虚拟光驱（配合 --iso 插入镜像）",
            "  hdd     硬盘",
            "  bios    重启进入 BIOS 设置界面",
            "  none    清除重定向，恢复正常启动顺序",
            "",
            "模式 (-m):",
            "  once        只生效一次（默认）",
            "  continuous  持续生效，直到再次清除",
            "",
            "电源操作:",
            "  --reset        设置后立即重启（默认 ForceRestart）",
            "                 单独使用 --reset = 只重启，不改引导",
            "                 （BIOS pending 生效、重装系统等场景）",
            "  --reset-type   指定重启方式，可选:",
            "                 On / ForceOff / GracefulShutdown / ForceRestart /",
            "                 ForcePowerCycle / PowerCycle / Nmi",
            "  电源为 Off 时，ForceRestart 会自动改用 On 开机。",
            "",
            "虚拟光驱:",
            "  --iso URL      插入 ISO（HTTPS/CIFS/NFS），自动应用 -t cd",
            "  --eject        弹出已挂载的镜像",
            "  --vm-user/--vm-pass  访问镜像所在的认证信息（可选）",
            "",
            "VNC (独立动作，不能与引导/ISO/重启组合):",
            "  --vnc on|off       开关远程画面",
            "  --vnc-ssl on|off   通道是否 TLS 加密（off = 明文 RFB，默认 on）",
            "  --vnc-pass PASS    VNC 密码，最多 8 位",
            "",
           f"默认账号: {DEFAULT_USER} / {DEFAULT_PASS}",
            "",
            "示例:",
            "  bmc_boot.py -f bmcinfo.txt --status                    # 查状态",
            "  bmc_boot.py -f bmcinfo.txt -t pxe                      # 下次走 PXE",
            "  bmc_boot.py -f bmcinfo.txt -t pxe --reset              # 设置并重启",
            "  bmc_boot.py -f bmcinfo.txt -t pxe --reset --dry-run     # 预演",
            "  bmc_boot.py -f bmcinfo.txt -t bios --reset             # 进 BIOS",
            "  bmc_boot.py -f bmcinfo.txt -t cd --iso https://h/a.iso --reset",
            "  bmc_boot.py -f bmcinfo.txt --eject                     # 弹盘",
            "  bmc_boot.py -f bmcinfo.txt --clear                     # 清除重定向",
            "  bmc_boot.py -f bmcinfo.txt --reset                     # 仅重启",
            "  bmc_boot.py -f bmcinfo.txt --reset --dry-run           # 预演重启",
            "  bmc_boot.py -f bmcinfo.txt --vnc on --vnc-pass 'pw123456'",
            "  bmc_boot.py -f bmcinfo.txt --vnc-ssl off               # 关 TLS",
            "",
            "注意: --reset 会真实重启服务器，先在单台验证再用 -f 批量。",
            "",
        ]),
    )
    parser.add_argument("-f", "--file", required=True, metavar="FILE",
                        help="BMC 信息文件 (CSV / TSV / 空格分隔 / 纯 IP 列表)")
    parser.add_argument("-t", "--target", metavar="TARGET",
                        choices=sorted(TARGET_MAP), help="启动目标: pxe/cd/iso/hdd/bios/none")
    parser.add_argument("-m", "--mode", default="Once", choices=["Once", "Continuous"],
                        help="重定向模式 (默认 Once)")
    parser.add_argument("--reset", action="store_true", help="设置后立即重启")
    parser.add_argument("--reset-type", default="ForceRestart", metavar="TYPE",
                        help="重启方式 (默认 ForceRestart)")
    parser.add_argument("--iso", metavar="URL", help="插入虚拟光驱 ISO，自动应用 -t cd")
    parser.add_argument("--eject", action="store_true", help="弹出虚拟光驱镜像")
    parser.add_argument("--vm-user", metavar="USER", help="镜像服务器用户名 (可选)")
    parser.add_argument("--vm-pass", metavar="PASS", help="镜像服务器密码 (可选)")
    parser.add_argument("--vnc", choices=["on", "off"], metavar="{on,off}",
                        help="开启/关闭 VNC（远程画面，配合 --vnc-pass 设密码）")
    parser.add_argument("--vnc-ssl", choices=["on", "off"], metavar="{on,off}",
                        help="VNC 通道是否走 TLS（off = 明文 RFB，可嗅探）")
    parser.add_argument("--vnc-pass", metavar="PASS",
                        help=f"VNC 密码，最多 {VNC_PASSWORD_MAX} 位")
    parser.add_argument("--status", action="store_true", help="只查询，不修改")
    parser.add_argument("--clear", action="store_true", help="清除引导重定向 (= -t none)")
    parser.add_argument("--dry-run", action="store_true", help="只读不写")
    parser.add_argument("--wait", type=int, default=90, metavar="SEC",
                        help="等待 ISO 挂载/弹出生效的超时秒数 (默认: 90)")
    parser.add_argument("-u", "--user", default=None, metavar="USER",
                        help="全局 BMC 用户名 (默认: %s)" % DEFAULT_USER)
    parser.add_argument("-p", "--password", default=None, metavar="PASS",
                        help="全局 BMC 密码 (默认: %s)" % DEFAULT_PASS.replace("%", "%%"))
    parser.add_argument("-o", "--output", default=None, metavar="CSV",
                        help="输出 CSV 路径 (默认: bmc_boot_<时间>.csv)")
    parser.add_argument("-j", "--jobs", type=int, default=10, metavar="N",
                        help="并发数 (默认: 10)")

    args = parser.parse_args()

    # ── 动作校验 ─────────────────────────────────────────────────────────
    if args.clear and args.target:
        print("错误: --clear 与 -t 互斥", file=sys.stderr)
        sys.exit(1)
    if args.status and (args.target or args.reset or args.iso or args.eject
                        or args.vnc or args.vnc_ssl):
        print("错误: --status 只查询，不能与其他动作组合", file=sys.stderr)
        sys.exit(1)
    if args.vnc_pass and not args.vnc:
        print("错误: --vnc-pass 需要配合 --vnc on", file=sys.stderr)
        sys.exit(1)
    if args.vnc_pass and len(args.vnc_pass) > VNC_PASSWORD_MAX:
        print(f"错误: VNC 密码最多 {VNC_PASSWORD_MAX} 位（当前 {len(args.vnc_pass)} 位）",
              file=sys.stderr)
        sys.exit(1)
    if (args.vnc or args.vnc_ssl) and (args.target or args.iso or args.eject
                                       or args.reset):
        print("错误: VNC 配置是独立动作，不能与引导/ISO/重启组合", file=sys.stderr)
        sys.exit(1)

    target = None
    if args.clear:
        target = "None"
    elif args.target:
        target = TARGET_MAP[args.target]
    elif args.iso:
        target = "Cd"          # --iso 隐含光驱启动

    # 清除重定向时 Enabled 必须是 Disabled，否则会留下 None/Once 这种脏状态
    mode = "Disabled" if target == "None" else args.mode

    # --reset 单独使用 = 只重启，不改引导（例：让 BIOS pending 生效）
    reset_only = args.reset and not any([args.iso, args.eject, target])
    if not any([args.status, args.eject, args.iso, target, args.reset,
                args.vnc, args.vnc_ssl]):
        print("错误: 需要指定动作：-t TARGET / --status / --eject / --clear / "
              "--iso URL / --reset / --vnc / --vnc-ssl", file=sys.stderr)
        sys.exit(1)

    # 提前校验 ISO URL，避免批量跑完才发现统统格式错误
    if args.iso:
        err = _check_iso_url(args.iso)
        if err:
            print(f"错误: {err}", file=sys.stderr)
            sys.exit(1)

    if args.reset:
        allowed = {"On", "ForceOff", "GracefulShutdown", "ForceRestart",
                   "ForcePowerCycle", "PowerCycle", "Nmi"}
        if args.reset_type not in allowed:
            print(f"错误: --reset-type 不支持 '{args.reset_type}'，可选: "
                  f"{', '.join(sorted(allowed))}", file=sys.stderr)
            sys.exit(1)

    opts = {
        "target": target,
        "mode": mode,
        "reset": args.reset,
        "reset_type": args.reset_type,
        "iso": args.iso,
        "eject": args.eject,
        "vm_user": args.vm_user,
        "vm_pass": args.vm_pass,
        "status": args.status,
        "dry_run": args.dry_run,
        "wait": args.wait,
        "reset_only": reset_only,
        "vnc": args.vnc,
        "vnc_ssl": args.vnc_ssl,
        "vnc_pass": args.vnc_pass,
    }

    output = args.output
    if not output:
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        output = os.path.join(os.path.dirname(os.path.abspath(args.file)),
                              f"bmc_boot_{ts}.csv")

    hosts = load_bmcinfo(args.file)
    if not hosts:
        print("错误: BMC 列表为空", file=sys.stderr)
        sys.exit(1)

    eff_user = args.user or DEFAULT_USER
    eff_pass = args.password or DEFAULT_PASS
    for h in hosts:
        if not h["user"]:
            h["user"] = eff_user
        if not h["pass"]:
            h["pass"] = eff_pass

    # ── 概要 ─────────────────────────────────────────────────────────────
    if args.status:
        action = "查询状态"
    else:
        bits = []
        if args.iso:
            bits.append(f"插入ISO({args.iso})")
        if args.eject:
            bits.append("弹出ISO")
        if target:
            bits.append(f"引导={target}/{mode}")
        if args.reset:
            bits.append(f"重启({args.reset_type})")
        if args.vnc or args.vnc_ssl:
            vnc_bits = []
            if args.vnc:
                vnc_bits.append(args.vnc)
            if args.vnc_ssl:
                vnc_bits.append(f"SSL={args.vnc_ssl}")
            if args.vnc_pass:
                vnc_bits.append("设密码")
            bits.append("VNC=" + "+".join(vnc_bits))
        action = " + ".join(bits) + (" [DRY RUN]" if args.dry_run else "")

    total = len(hosts)
    print(f"动作: {action}", file=sys.stderr)
    print(f"共 {total} 台, 并发: {args.jobs}", file=sys.stderr)
    if args.vnc_ssl == "off":
        print("  ⚠ 关闭 TLS 后 VNC 为明文 RFB，密码可被嗅探；"
              "只在内网或 SSH 隧道内使用", file=sys.stderr)
    print(f"{'─' * 62}", file=sys.stderr)

    results = []
    done = 0
    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        futures = {
            pool.submit(process_one, h["ip"], h["user"], h["pass"], opts): h
            for h in hosts
        }
        for future in as_completed(futures):
            done += 1
            r = future.result()
            results.append(r)

            if r["target_before"] != r["target_after"]:
                tgt = f"{r['target_before']} → {r['target_after']}"
            else:
                tgt = r["target_after"]
            iso = f" ISO={r['iso_before']}" if r["iso_before"] else ""
            vnc = ""
            if r["vnc"]:
                vnc += f" VNC={r['vnc']}"
            if r["vnc_ssl"]:
                vnc += f" SSL={r['vnc_ssl']}"
            print(
                f"  [{done}/{total}] {r['bmc_ip']} SN={r['sn']} "
                f"电源={r['power_before']}{iso} 引导={tgt}{vnc} [{r['status']}]"
                + (f" ERROR: {r['error']}" if r["error"] else ""),
                file=sys.stderr,
            )

    results.sort(key=lambda x: list(map(int, x["bmc_ip"].split("."))))

    with open(output, "w", newline="", encoding="utf-8") as fh:
        w = csv.writer(fh)
        w.writerow(["bmc_ip", "sn", "power_before", "iso_before",
                    "target_before", "target_after", "vnc", "vnc_ssl",
                    "status", "error"])
        for r in results:
            w.writerow([r["bmc_ip"], r["sn"], r["power_before"], r["iso_before"],
                        r["target_before"], r["target_after"], r["vnc"],
                        r["vnc_ssl"], r["status"], r["error"]])

    ok = sum(1 for r in results if r["status"].startswith("OK"))
    skip = sum(1 for r in results if r["status"].startswith("SKIP"))
    dry = sum(1 for r in results if r["status"] == "DRY_RUN")
    fail = sum(1 for r in results if r["status"] == "FAIL")

    print(f"{'─' * 62}", file=sys.stderr)
    print(f"完成: 成功 {ok}, 跳过 {skip}, 失败 {fail}, 共 {total}", file=sys.stderr)
    if dry:
        print(f"  DRY RUN: {dry} 台未执行", file=sys.stderr)
    if args.reset and not args.dry_run and ok:
        print(f"  ⚠ {ok} 台已下发重启，机器会立即重启", file=sys.stderr)
    print(f"结果已保存到: {output}", file=sys.stderr)


if __name__ == "__main__":
    main()
