#!/usr/bin/env bash
# ==========================================================================
# build-linux-container.sh — 在 Ubuntu 20.04 容器内构建 linux/amd64 二进制
# --------------------------------------------------------------------------
# 用途：macOS（含 Apple Silicon）上为麒麟产二进制。Wails 不能从 macOS 直接交叉
# 编译 Linux；001 确认麒麟是 glibc 2.31 + WebKitGTK 4.0 2.28.1。
#
# 容器镜像见 build/linux/Dockerfile：
#   - x86_64 宿主：原生 gcc 构建
#   - arm64 宿主：clang/lld 交叉编译 x86_64（QEMU 模拟不可用）
#
# 前置：frontend/dist（脚本会先构建）；Docker 可用。
# 产物：build/bin/ImageWorkshop（x86_64 ELF）
# ==========================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

IMAGE="imageworkshop-linux-builder:focal"
BIN="build/bin/ImageWorkshop"

command -v docker >/dev/null 2>&1 || {
  echo "错误：未找到 docker。Linux 包需要 Docker 构建 Ubuntu 20.04 环境。" >&2
  exit 1
}
docker info >/dev/null 2>&1 || {
  echo "错误：Docker 未运行。" >&2
  exit 1
}

echo "==> 构建前端（容器内跳过前端构建，直接嵌入 dist）"
if [ ! -d frontend/node_modules ]; then
  (cd frontend && npm install)
fi
(cd frontend && npm run build)

echo "==> 准备构建镜像 ${IMAGE}"
docker build -t "$IMAGE" -f build/linux/Dockerfile build/linux

echo "==> 容器内交叉/原生构建 linux/amd64"
mkdir -p build/bin
rm -f "$BIN"
docker run --rm \
  -v "$ROOT:/src:ro" \
  -v "$ROOT/build/bin:/out" \
  "$IMAGE" bash -c '
    set -euo pipefail
    arch="$(uname -m)"
    echo "容器架构: ${arch}"
    mkdir -p /work
    tar -C /src --exclude=node_modules --exclude=.git --exclude=build/bin -cf - . | tar -xf - -C /work
    cd /work
    if [ "$arch" = "aarch64" ]; then
      export CGO_ENABLED=1 GOOS=linux GOARCH=amd64
      export CC="clang --target=x86_64-linux-gnu"
      export CXX="clang++ --target=x86_64-linux-gnu"
      export CGO_LDFLAGS="-fuse-ld=lld"
      export PKG_CONFIG_LIBDIR=/usr/lib/x86_64-linux-gnu/pkgconfig:/usr/share/pkgconfig
    fi
    wails build -platform linux/amd64 -tags nodynamic -s
    cp build/bin/ImageWorkshop /out/ImageWorkshop
  '

[ -f "$BIN" ] || {
  echo "错误：容器构建未产出 $BIN" >&2
  exit 1
}
FILE_INFO="$(file -b "$BIN")"
case "$FILE_INFO" in
  *x86-64*|*x86_64*) ;;
  *)
    echo "错误：产物不是 x86-64 ELF：$FILE_INFO" >&2
    exit 1
    ;;
esac
echo "✓ 产物: $BIN"
echo "  $FILE_INFO"
