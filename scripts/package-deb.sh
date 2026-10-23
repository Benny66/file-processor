#!/usr/bin/env bash
# ==========================================================================
# package-deb.sh — 把已构建的 linux/amd64 二进制打成 .deb
# --------------------------------------------------------------------------
# 输入：build/bin/ImageWorkshop（由 build.sh 原生构建或 build-linux-container.sh 产出）
# 输出：build/bin/ImageWorkshop_<version>_amd64.deb
#
# 包内容（001/011 约定）：
#   /usr/bin/ImageWorkshop
#   /usr/share/applications/imageworkshop.desktop
#   /usr/share/icons/hicolor/1024x1024/apps/imageworkshop.png
#
# Depends 按 001 的真机结论：libc6 >= 2.31、libgtk-3-0、libwebkit2gtk-4.0-37 >= 2.28.1。
# 若宿主没有 dpkg-deb（macOS），自动用 Ubuntu 20.04 容器打包。
# ==========================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

APP_NAME="$(bash scripts/app-name.sh)"
BIN="build/bin/${APP_NAME}"
[ -f "$BIN" ] || {
  echo "错误：找不到 $BIN；请先构建 linux/amd64 二进制。" >&2
  exit 1
}

if command -v file >/dev/null 2>&1; then
  FILE_INFO="$(file -b "$BIN")"
  case "$FILE_INFO" in
    *x86-64*|*x86_64*) ;;
    *)
      echo "错误：$BIN 不是 x86-64 ELF（$FILE_INFO）。麒麟不能运行该架构。" >&2
      exit 1
      ;;
  esac
else
  # 精简镜像可能没有 file；readelf 随 build-essential 的 binutils 提供。
  readelf -h "$BIN" | grep -q 'X86-64' || {
    echo "错误：$BIN 不是 x86-64 ELF。麒麟不能运行该架构。" >&2
    exit 1
  }
fi

# 字段读取统一走 scripts/wails-info.sh：本脚本曾在 CI 的 ubuntu:20.04 容器里
# 用 python3 解析 wails.json，而该镜像没有 python3（set -e 下 127 退出）。
VERSION="$(bash scripts/wails-info.sh info.productVersion)"
if ! printf '%s' "$VERSION" | grep -qE '^[0-9]+(\.[0-9]+){2,3}$'; then
  echo "错误：版本 '$VERSION' 不是数字点分格式" >&2
  exit 1
fi

DISPLAY_NAME="$(bash scripts/wails-info.sh info.productName)"
COMMENT="$(bash scripts/wails-info.sh info.comments)"
MAINTAINER="$(printf '%s <%s>' \
  "$(bash scripts/wails-info.sh author.name)" \
  "$(bash scripts/wails-info.sh author.email)")"

DEB_NAME="${APP_NAME}_${VERSION}_amd64.deb"
OUT="build/bin/${DEB_NAME}"
STAGING="$(mktemp -d "$ROOT/build/.deb-staging.XXXXXX")"
# mktemp 默认 0700；包内 "./" 的模式会落到安装根目录，必须是 0755。
chmod 0755 "$STAGING"
cleanup() { rm -rf "$STAGING"; }
trap cleanup EXIT

mkdir -p \
  "$STAGING/usr/bin" \
  "$STAGING/usr/share/applications" \
  "$STAGING/usr/share/icons/hicolor/1024x1024/apps" \
  "$STAGING/DEBIAN"

cp "$BIN" "$STAGING/usr/bin/${APP_NAME}"
cp build/linux/imageworkshop.desktop "$STAGING/usr/share/applications/imageworkshop.desktop"
cp build/appicon.png "$STAGING/usr/share/icons/hicolor/1024x1024/apps/imageworkshop.png"
chmod 0755 "$STAGING/usr/bin/${APP_NAME}"
chmod 0644 \
  "$STAGING/usr/share/applications/imageworkshop.desktop" \
  "$STAGING/usr/share/icons/hicolor/1024x1024/apps/imageworkshop.png"

INSTALLED_SIZE="$(du -sk "$STAGING" | awk '{print $1}')"
cat > "$STAGING/DEBIAN/control" <<EOF
Package: imageworkshop
Version: ${VERSION}
Architecture: amd64
Maintainer: ${MAINTAINER}
Installed-Size: ${INSTALLED_SIZE}
Depends: libc6 (>= 2.31), libgtk-3-0, libwebkit2gtk-4.0-37 (>= 2.28.1)
Section: graphics
Priority: optional
Description: ${DISPLAY_NAME} — ${COMMENT}
 本地批量转换图片格式、压缩到目标体积，并按宽高上限等比缩放。
 图片只在本机处理，不上传、不联网。
EOF

rm -f "$OUT"
if command -v dpkg-deb >/dev/null 2>&1; then
  dpkg-deb --build --root-owner-group "$STAGING" "$OUT"
else
  echo "==> 宿主缺少 dpkg-deb，使用 Ubuntu 20.04 容器打包"
  IMAGE="imageworkshop-linux-builder:focal"
  if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    IMAGE="ubuntu:20.04"
  fi
  docker run --rm \
    -v "$STAGING:/pkg:ro" \
    -v "$ROOT/build/bin:/out" \
    "$IMAGE" dpkg-deb --build --root-owner-group /pkg "/out/${DEB_NAME}" >/dev/null
fi

[ -f "$OUT" ] || {
  echo "错误：.deb 未生成：$OUT" >&2
  exit 1
}
echo "✓ 已生成: $OUT"
sha256sum "$OUT" 2>/dev/null || shasum -a 256 "$OUT"
