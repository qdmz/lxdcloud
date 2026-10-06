#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
LXD Cloud 自动打包脚本（版本号自动递增）
用法:
  python3 build-package.py            # 自动检测 output 下最新版本号并 +1
  python3 build-package.py --v 1.5    # 手动指定版本号
  python3 build-package.py --dry-run  # 只显示将生成的版本号，不打包

规则:
  - 版本命名: lxdcloud-v<主>.<次>-YYYYMMDD.zip
  - 自动递增: 扫描 output 目录已有 lxdcloud-vX.Y-*.zip，取最大次版本号 Y 并 +1；
    无历史包时从 v1.0 开始。
  - 打包范围: 项目根目录 lxdcloud/（排除 .git / temp / output / __pycache__ 等）
"""
import argparse
import datetime
import fnmatch
import os
import re
import sys
import zipfile

ROOT = os.path.dirname(os.path.abspath(__file__))
PROJ = ROOT
OUT = os.path.join(os.path.dirname(ROOT), "output")
EXCLUDE_DIRS = {".git", "temp", "output", "__pycache__", ".venv", "node_modules", ".idea", ".vscode"}
EXCLUDE_FILE_PATTERNS = ("*.zip", "*.tar.gz", "*.tgz")


def find_latest_version():
    """扫描 output 目录，返回 (major, minor) 最新版本；无历史则 (1, 0)"""
    latest = (1, 0)
    if not os.path.isdir(OUT):
        return latest
    pat = re.compile(r"lxdcloud-v(\d+)\.(\d+)-\d{8}\.zip$")
    for name in os.listdir(OUT):
        m = pat.match(name)
        if m:
            major, minor = int(m.group(1)), int(m.group(2))
            if (major, minor) > latest:
                latest = (major, minor)
    return latest


def build(target_version, dry_run=False):
    os.makedirs(OUT, exist_ok=True)
    major, minor = target_version
    stamp = datetime.date.today().strftime("%Y%m%d")
    target = os.path.join(OUT, f"lxdcloud-v{major}.{minor}-{stamp}.zip")
    if os.path.exists(target):
        print(f"[WARN] 目标已存在，将被覆盖: {target}")
    if dry_run:
        print(f"[DRY-RUN] 将生成: {target}")
        return None

    count = 0
    with zipfile.ZipFile(target, "w", zipfile.ZIP_DEFLATED) as z:
        for root, dirs, files in os.walk(PROJ):
            dirs[:] = [d for d in dirs if d not in EXCLUDE_DIRS]
            for fn in files:
                if any(fnmatch.fnmatch(fn, p) for p in EXCLUDE_FILE_PATTERNS):
                    continue
                full = os.path.join(root, fn)
                rel = os.path.relpath(full, os.path.dirname(PROJ))
                z.write(full, rel)
                count += 1
    print(f"[OK] 打包完成: {target}")
    print(f"[OK] 文件数: {count}, 大小: {os.path.getsize(target)} bytes")
    return target


def main():
    ap = argparse.ArgumentParser(description="LXD Cloud 自动打包（版本号自动递增）")
    ap.add_argument("--v", dest="version", help="手动指定版本号，如 1.5")
    ap.add_argument("--dry-run", action="store_true", help="只显示版本号，不打包")
    args = ap.parse_args()

    if args.version:
        m = re.fullmatch(r"(\d+)\.(\d+)", args.version)
        if not m:
            print("[FAIL] 版本号格式应为 主.次，如 1.5")
            sys.exit(1)
        target_version = (int(m.group(1)), int(m.group(2)))
    else:
        major, minor = find_latest_version()
        target_version = (major, minor + 1)
        print(f"[INFO] 检测到最新版本 v{major}.{minor}，本次自动递增为 v{major}.{minor + 1}")

    build(target_version, dry_run=args.dry_run)


if __name__ == "__main__":
    main()
