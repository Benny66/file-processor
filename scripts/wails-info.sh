#!/usr/bin/env bash
# ==========================================================================
# wails-info.sh — wails.json 字段的唯一读取入口（无 python3 依赖）
# --------------------------------------------------------------------------
# 为什么存在：
#   1. 单一真相。wails.json 的字段此前被 4 处各自解析（build.sh 用 sed、
#      app-name.sh 与 package-deb.sh 用 python3），同一份配置多套解析逻辑，
#      改一个字段名就会有一处静默读到空值。
#   2. 依赖不得靠巧合。CI 的 linux-deb job 跑在 `container: ubuntu:20.04`，
#      该镜像【本身没有 python3】——它能跑起来只是因为 job 里那条
#      `apt-get install libgtk-3-dev …` 把 python3 作为**传递依赖**顺带装了
#      （2026-10-23 实测：基础镜像 `command -v python3` 为空，模拟安装同一份清单
#      出现 `Inst python3`）。也就是说：一个没人声明、也随时会随 apt 清单变化而
#      消失的解释器，被当成了打包链的前提。
#
# 用法：bash scripts/wails-info.sh <key>
#   key ∈ outputfilename | info.productVersion | info.productName |
#         info.comments | author.name | author.email
#
# 铁律 5（护栏感知自己瞎了）：字段缺失或值为空一律报错退出，绝不返回空串让
# 下游拼出 "Package: " 这种半成品。
# ==========================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FILE="$ROOT/wails.json"

KEY="${1:-}"
if [ -z "$KEY" ]; then
  echo "用法: $0 <key>   （如 info.productVersion / outputfilename / author.name）" >&2
  exit 1
fi
[ -f "$FILE" ] || {
  echo "错误：找不到 $FILE" >&2
  exit 1
}

# 取键名（去掉 info. / author. 前缀）。
# 注意不能一律按叶子键全文件匹配："name" 既是顶层产物名也是 author.name，
# 按叶子键取 author.name 会读到顶层那个。故 author.* 先切出 author 对象再取值。
LEAF="${KEY##*.}"

case "$KEY" in
  author.*)
    BLOCK="$(sed -n '/"author"[[:space:]]*:[[:space:]]*{/,/}/p' "$FILE")"
    [ -n "$BLOCK" ] || {
      echo "错误：$FILE 中未找到 author 对象 —— 写法可能已变更，请同步更新 scripts/wails-info.sh" >&2
      exit 1
    }
    VALUE="$(printf '%s\n' "$BLOCK" \
      | sed -n 's/.*"'"$LEAF"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
    ;;
  *)
    # 其余叶子键（outputfilename / productVersion / productName / comments）在本文件内唯一。
    VALUE="$(sed -n 's/.*"'"$LEAF"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$FILE" | head -1)"
    ;;
esac

if [ -z "$VALUE" ]; then
  echo "错误：$FILE 中未读到 $KEY —— 写法可能已变更，请同步更新 scripts/wails-info.sh 的解析规则" >&2
  exit 1
fi

printf '%s\n' "$VALUE"
