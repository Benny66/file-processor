#!/usr/bin/env bash
# ==========================================================================
# build-linux-container.sh — 在 Ubuntu 20.04 容器内构建 linux 二进制（amd64/arm64）
# --------------------------------------------------------------------------
# 用途：macOS（含 Apple Silicon）与 CI 上产麒麟二进制。Wails 不能从 macOS 直接交叉
# 编译 Linux；001 确认麒麟是 glibc 2.31 + WebKitGTK 4.0 2.28.1。
#
# 用法：bash scripts/build-linux-container.sh [amd64|arm64]（默认 amd64）
#   arm64 = 信创飞腾/鲲鹏机型，路线见 ADR-0006：与 amd64 同族的容器内 clang/lld
#   交叉，不用 QEMU（001 记录过 QEMU 在 apt/Go 阶段段错误）。
#
# 容器镜像见 build/linux/Dockerfile（TARGET_ARCH build-arg 控制预装的库架构）：
#   - 目标 amd64：x86_64 容器原生 / arm64 容器交叉 x86_64
#   - 目标 arm64：x86_64 容器交叉 aarch64 / arm64 容器原生
#
# 前置：frontend/dist（脚本会先构建）；Docker 可用。
# 产物：build/bin/ImageWorkshop（对应架构 ELF）
# ==========================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ARCH="${1:-amd64}"
[ -z "$ARCH" ] && ARCH="amd64"
case "$ARCH" in
  amd64|arm64) ;;
  *)
    echo "用法: $0 [amd64|arm64]（默认 amd64）" >&2
    exit 1
    ;;
esac

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

echo "==> 准备构建镜像 ${IMAGE}（目标 linux/${ARCH}）"
docker build -t "$IMAGE" --build-arg TARGET_ARCH="$ARCH" -f build/linux/Dockerfile build/linux

echo "==> 容器内构建 linux/${ARCH}"
mkdir -p build/bin
rm -f "$BIN"
# 提交号在宿主取（tar 排除了 .git，容器内取不到），交给 build.sh 注入。
BUILD_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || true)"
docker run --rm \
  -e TARGET_ARCH="$ARCH" \
  -e BUILD_COMMIT="$BUILD_COMMIT" \
  -v "$ROOT:/src:ro" \
  -v "$ROOT/build/bin:/out" \
  "$IMAGE" bash -c '
    set -euo pipefail
    target="${TARGET_ARCH:-amd64}"
    host="$(uname -m)"
    echo "容器架构: ${host}，目标: linux/${target}"
    mkdir -p /work
    tar -C /src --exclude=node_modules --exclude=.git --exclude=build/bin -cf - . | tar -xf - -C /work
    cd /work
    # 交叉仅在「容器架构 ≠ 目标架构」时需要：对应架构的开发库已由镜像按 TARGET_ARCH 预装。
    if [ "$target" = "arm64" ] && [ "$host" = "x86_64" ]; then
      export CGO_ENABLED=1 GOOS=linux GOARCH=arm64
      export CC="clang --target=aarch64-linux-gnu"
      export CXX="clang++ --target=aarch64-linux-gnu"
      export CGO_LDFLAGS="-fuse-ld=lld"
      export PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig:/usr/share/pkgconfig
    elif [ "$target" = "amd64" ] && [ "$host" = "aarch64" ]; then
      export CGO_ENABLED=1 GOOS=linux GOARCH=amd64
      export CC="clang --target=x86_64-linux-gnu"
      export CXX="clang++ --target=x86_64-linux-gnu"
      export CGO_LDFLAGS="-fuse-ld=lld"
      export PKG_CONFIG_LIBDIR=/usr/lib/x86_64-linux-gnu/pkgconfig:/usr/share/pkgconfig
    fi
    # 必须经 scripts/build.sh：它是版本与提交号注入的【唯一入口】。
    # 此前这里裸调 wails build，产出的二进制 appinfo.Resolve() 恒为 "dev"，
    # 而 .deb 的 control 写着 0.1.0 —— 同一版本两个真相且没有任何症状。
    bash scripts/build.sh -platform "linux/${target}" -tags nodynamic -s
    cp build/bin/ImageWorkshop /out/ImageWorkshop
  '

[ -f "$BIN" ] || {
  echo "错误：容器构建未产出 $BIN" >&2
  exit 1
}
FILE_INFO="$(file -b "$BIN")"
elf_arch=""
case "$FILE_INFO" in
  *x86-64*|*x86_64*) elf_arch="amd64" ;;
  *aarch64*|*arm64*) elf_arch="arm64" ;;
esac
if [ "$elf_arch" != "$ARCH" ]; then
  echo "错误：产物不是 ${ARCH} ELF：$FILE_INFO" >&2
  exit 1
fi
echo "✓ 产物: $BIN"
echo "  $FILE_INFO"
