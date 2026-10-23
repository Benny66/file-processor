# Kylin 采用 CI 近似构建 + 真机冒烟

Kylin 是本产品的一等目标，但 Wails 无法从 macOS 交叉编译 Linux。决定：在 CI 的旧 glibc 容器（初始 Ubuntu 20.04，最终基础镜像由 spike 的 `ldd` 结果决定）构建 Linux 产物并打成 .deb，再在真实麒麟 V10 SP1 机器上安装冒烟；不在麒麟机器上维护完整工具链。

**备选方案**：直接在麒麟上安装 Go/Node/wails 构建（兼容性最真，但企业环境常缺管理员权限或无法联网）；双环境互验（最稳但成本最高）。

**后果**：首个 Linux 任务必须先做兼容性 spike，确认麒麟的 glibc、WebKit/GTK 包版本；若麒麟的 glibc 低于 Ubuntu 20.04，构建基础镜像必须再往旧版本退，不能用更新环境产出的二进制代替。后续 Linux 构建以该 spike 的结果为准，不能把“Ubuntu 上能跑”当作“麒麟上能跑”。
