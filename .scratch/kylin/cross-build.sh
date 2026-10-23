#!/usr/bin/env bash
# 001 spike：原生 arm64 容器 + clang/lld 交叉编译 linux/amd64。
# QEMU 模拟 amd64 在 apt postinst 与 Go 编译阶段都会段错误，故改为交叉编译。
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

arch="$(uname -m)"
if [ "$arch" != "aarch64" ]; then
  echo "错误：本脚本需在 arm64 容器运行，当前 $arch" >&2
  exit 1
fi

echo "[1/5] 配置 apt（arm64 走 ubuntu-ports 国内镜像，amd64 走普通镜像）"
# 每次重写为固定内容：重复执行不会叠加 [arch=] 前缀。
cat > /etc/apt/sources.list <<'EOF'
deb [arch=arm64] http://mirrors.aliyun.com/ubuntu-ports/ focal main restricted universe multiverse
deb [arch=arm64] http://mirrors.aliyun.com/ubuntu-ports/ focal-updates main restricted universe multiverse
deb [arch=arm64] http://mirrors.aliyun.com/ubuntu-ports/ focal-backports main restricted universe multiverse
deb [arch=arm64] http://mirrors.aliyun.com/ubuntu-ports/ focal-security main restricted universe multiverse
deb [arch=amd64] http://mirrors.aliyun.com/ubuntu/ focal main restricted universe multiverse
deb [arch=amd64] http://mirrors.aliyun.com/ubuntu/ focal-updates main restricted universe multiverse
deb [arch=amd64] http://mirrors.aliyun.com/ubuntu/ focal-security main restricted universe multiverse
EOF
dpkg --add-architecture amd64
apt-get update -qq

echo "[2/5] 安装交叉编译工具与 amd64 开发库"
# 麒麟真机是 WebKitGTK 2.28.1；按该版本链接，避免误用 focal-updates 的 2.38 ABI。
apt-get install -y -qq --no-install-recommends --allow-downgrades \
  build-essential pkg-config ca-certificates curl \
  clang lld \
  libgtk-3-dev:amd64 libgcc-9-dev:amd64 \
  libwebkit2gtk-4.0-dev:amd64=2.28.1-1 \
  libwebkit2gtk-4.0-37:amd64=2.28.1-1 \
  gir1.2-webkit2-4.0:amd64=2.28.1-1 \
  libjavascriptcoregtk-4.0-dev:amd64=2.28.1-1 \
  libjavascriptcoregtk-4.0-18:amd64=2.28.1-1 \
  gir1.2-javascriptcoregtk-4.0:amd64=2.28.1-1 >/dev/null

echo "[3/5] 安装原生 arm64 Go 1.26.2"
if [ ! -x /usr/local/go/bin/go ]; then
  curl -fsSLo /tmp/go.tgz https://go.dev/dl/go1.26.2.linux-arm64.tar.gz
  tar -C /usr/local -xzf /tmp/go.tgz
fi
export PATH=/usr/local/go/bin:/root/go/bin:$PATH
export GOPATH=/root/go
export GOPROXY=https://goproxy.cn,direct
go version

echo "[4/5] 复制源码并安装 Wails CLI（原生 arm64）"
rm -rf /work
mkdir -p /work
tar -C /src --exclude=node_modules --exclude=.git --exclude=build/bin -cf - . | tar -xf - -C /work
cd /work
if [ ! -x /root/go/bin/wails ]; then
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
fi

echo "[5/5] 交叉编译 linux/amd64（默认 webkit2gtk-4.0）"
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64
# -fuse-ld 只能出现在链接阶段；放在 CC 里会被编译阶段的 -Werror 判为未使用参数。
export CC="clang --target=x86_64-linux-gnu"
export CXX="clang++ --target=x86_64-linux-gnu"
export CGO_LDFLAGS="-fuse-ld=lld"
export PKG_CONFIG_LIBDIR=/usr/lib/x86_64-linux-gnu/pkgconfig:/usr/share/pkgconfig
pkg-config --modversion gtk+-3.0 webkit2gtk-4.0
wails build -platform linux/amd64 -tags nodynamic -s

cp build/bin/ImageWorkshop /out/ImageWorkshop-linux-amd64
readelf -h build/bin/ImageWorkshop | grep -E 'Class:|Machine:' | tee /out/elf.txt
readelf -d build/bin/ImageWorkshop | grep NEEDED | tee /out/needed.txt
readelf --version-info build/bin/ImageWorkshop | grep -o 'GLIBC_[0-9.]*' | sort -Vu | tail -5 | tee /out/glibc.txt
pkg-config --modversion gtk+-3.0 webkit2gtk-4.0 | tee /out/pkgconfig.txt
sha256sum build/bin/ImageWorkshop | tee /out/sha256.txt
