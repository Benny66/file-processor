#!/usr/bin/env bash
# ==========================================================================
# app-name.sh — 产物名唯一读取入口
# --------------------------------------------------------------------------
# outputfilename 的单一真相是 wails.json；打包/冒烟/发布脚本共用本脚本，
# 避免多处各写一份 JSON 解析逻辑。
# ==========================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP_NAME="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["outputfilename"])' "$ROOT/wails.json" 2>/dev/null || true)"
if [ -z "$APP_NAME" ]; then
  echo "错误：无法从 wails.json 读取 outputfilename" >&2
  exit 1
fi
printf '%s\n' "$APP_NAME"
