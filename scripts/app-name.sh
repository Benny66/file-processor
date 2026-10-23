#!/usr/bin/env bash
# ==========================================================================
# app-name.sh — 产物名唯一读取入口
# --------------------------------------------------------------------------
# outputfilename 的单一真相是 wails.json；打包/冒烟/发布脚本共用本脚本，
# 避免多处各写一份 JSON 解析逻辑。真正的解析在 scripts/wails-info.sh
# （本脚本曾经自带 python3 解析：CI 的 linux-deb job 在 ubuntu:20.04 容器里
# 没有 python3，会在 set -e 下 127 退出——留着这个入口是为了调用方不改）。
# ==========================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
bash "$ROOT/scripts/wails-info.sh" outputfilename
