# Kylin / Linux 构建镜像（001 spike 结论；ADR-0006 扩展双架构）
#
# - 基础镜像固定 Ubuntu 20.04：glibc 2.31，与麒麟 V10 SP1（2203）一致。
# - WebKitGTK 固定 2.28.1（麒麟真机版本），只提供 4.0，禁止 webkit2_41。
#   arm64（信创）一侧不钉 2.28.1：focal arm64 ports 是否保有该精确版本未核实，
#   WebKitGTK 4.0 的 API 稳定（Wails 官方支持更高版本），运行端下限由 .deb 的
#   Depends（>= 2.28.1）声明，构建端只影响头文件。
# - TARGET_ARCH build-arg 决定预装哪个架构的开发库：
#     amd64：arm64 容器时装 amd64 库 + clang/lld 交叉；x86_64 容器原生 gcc。
#     arm64：x86_64 容器时装 arm64 库 + clang/lld 交叉；arm64 容器原生 gcc。
#   （QEMU 模拟在 dpkg/Go 阶段会段错误，001 记录在案，两条交叉路线都不走模拟。）
# - 前端由宿主构建（wails build -s 使用已有 frontend/dist），镜像不装 Node。
FROM ubuntu:20.04

ARG GO_VERSION=1.26.2
# 构建目标架构（amd64 默认 / arm64 信创）：控制预装开发库与交叉方向。
ARG TARGET_ARCH=amd64
# 构建机在目标网络环境（国内）；可用 --build-arg 覆盖。
ARG APT_MIRROR=http://mirrors.aliyun.com/ubuntu
ARG PORTS_MIRROR=http://mirrors.aliyun.com/ubuntu-ports

ENV DEBIAN_FRONTEND=noninteractive

RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    if [ "$arch" = "arm64" ]; then \
      { \
        echo "deb [arch=arm64] $PORTS_MIRROR focal main restricted universe multiverse"; \
        echo "deb [arch=arm64] $PORTS_MIRROR focal-updates main restricted universe multiverse"; \
        echo "deb [arch=arm64] $PORTS_MIRROR focal-backports main restricted universe multiverse"; \
        echo "deb [arch=arm64] $PORTS_MIRROR focal-security main restricted universe multiverse"; \
        echo "deb [arch=amd64] $APT_MIRROR focal main restricted universe multiverse"; \
        echo "deb [arch=amd64] $APT_MIRROR focal-updates main restricted universe multiverse"; \
        echo "deb [arch=amd64] $APT_MIRROR focal-security main restricted universe multiverse"; \
      } > /etc/apt/sources.list; \
      dpkg --add-architecture amd64; \
      apt-get update; \
      apt-get install -y --no-install-recommends \
        build-essential pkg-config ca-certificates curl clang lld \
        libgtk-3-dev:amd64 libgcc-9-dev:amd64 \
        libwebkit2gtk-4.0-dev:amd64=2.28.1-1 \
        libwebkit2gtk-4.0-37:amd64=2.28.1-1 \
        gir1.2-webkit2-4.0:amd64=2.28.1-1 \
        libjavascriptcoregtk-4.0-dev:amd64=2.28.1-1 \
        libjavascriptcoregtk-4.0-18:amd64=2.28.1-1 \
        gir1.2-javascriptcoregtk-4.0:amd64=2.28.1-1; \
    elif [ "$TARGET_ARCH" = "arm64" ]; then \
      { \
        echo "deb [arch=amd64] $APT_MIRROR focal main restricted universe multiverse"; \
        echo "deb [arch=amd64] $APT_MIRROR focal-updates main restricted universe multiverse"; \
        echo "deb [arch=amd64] $APT_MIRROR focal-security main restricted universe multiverse"; \
        echo "deb [arch=arm64] $PORTS_MIRROR focal main restricted universe multiverse"; \
        echo "deb [arch=arm64] $PORTS_MIRROR focal-updates main restricted universe multiverse"; \
        echo "deb [arch=arm64] $PORTS_MIRROR focal-backports main restricted universe multiverse"; \
        echo "deb [arch=arm64] $PORTS_MIRROR focal-security main restricted universe multiverse"; \
      } > /etc/apt/sources.list; \
      dpkg --add-architecture arm64; \
      apt-get update; \
      apt-get install -y --no-install-recommends \
        build-essential pkg-config ca-certificates curl clang lld \
        libgtk-3-dev:arm64 libgcc-9-dev:arm64 \
        libwebkit2gtk-4.0-dev:arm64 \
        libwebkit2gtk-4.0-37:arm64 \
        gir1.2-webkit2-4.0:arm64 \
        libjavascriptcoregtk-4.0-dev:arm64 \
        libjavascriptcoregtk-4.0-18:arm64 \
        gir1.2-javascriptcoregtk-4.0:arm64; \
    else \
      { \
        echo "deb $APT_MIRROR focal main restricted universe multiverse"; \
        echo "deb $APT_MIRROR focal-updates main restricted universe multiverse"; \
        echo "deb $APT_MIRROR focal-backports main restricted universe multiverse"; \
        echo "deb $APT_MIRROR focal-security main restricted universe multiverse"; \
      } > /etc/apt/sources.list; \
      apt-get update; \
      apt-get install -y --no-install-recommends \
        build-essential pkg-config ca-certificates curl \
        libgtk-3-dev \
        libwebkit2gtk-4.0-dev=2.28.1-1 \
        libwebkit2gtk-4.0-37=2.28.1-1 \
        gir1.2-webkit2-4.0=2.28.1-1 \
        libjavascriptcoregtk-4.0-dev=2.28.1-1 \
        libjavascriptcoregtk-4.0-18=2.28.1-1 \
        gir1.2-javascriptcoregtk-4.0=2.28.1-1; \
    fi; \
    curl -fsSLo /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz"; \
    tar -C /usr/local -xzf /tmp/go.tgz; \
    rm -f /tmp/go.tgz; \
    export PATH=/usr/local/go/bin:/root/go/bin:$PATH; \
    export GOPATH=/root/go; \
    export GOPROXY=https://goproxy.cn,direct; \
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*

ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH
ENV GOPROXY=https://goproxy.cn,direct
ENV GOPATH=/root/go

WORKDIR /src
