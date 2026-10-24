# P0 发布就绪方案（v2）—— 面向个人与朋友分发的发布链收口

- 状态：已实施（2026-10-24；决策点按 §6 推荐默认执行，SHA256SUMS 已随 Release 生成）
- 依据：`docs/竞品对比与迭代建议.md` P0-2；**2026-10-24 范围修订**：不采购任何证书（Apple Developer Program、Windows 代码签名证书均不办），分发对象为个人自用与普通朋友，接受无签名的首次打开摩擦，用文档与校验和消除不确定性
- 范围说明：麒麟真机验收继续挂起（无硬件）；签名/公证/代码签名证书**明确不做**（未来若需正式分发再立项）
- 编号续接：US-54 起、D-47 起

**关键决策记录（本次修订确立）**：

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-47 | 不办证书、不做签名与公证：macOS 保持 ad-hoc 自签，Windows 保持未签名 | 个人+朋友分发，¥1000+/年的证书成本不值；Gatekeeper/SmartScreen 的首次放行摩擦改由安装指南覆盖（变更二） |
| D-48 | 身份收口降级：bundle id 保持占位、module 名不改，只更正「发布前必须替换为公司域名」类表述 | 没有公司域名可换；占位值对 ad-hoc 分发无实际影响；25 文件 module 机械替换属纯化妆，风险大于收益。未来若做公证再连本方案一起重启 |

---

## 变更一：`.deb` 接入 tag Release + 发布链修偏

朋友从 GitHub Release 直接下载三个平台的包，是最现实的分发路径；现在 tag Release 缺 `.deb`（只能从 CI artifact 拿，朋友拿不到）。

### 现状（代码事实）

- 机制：release.yml 的 package matrix 只有 macos/windows 两腿（:55-59），tag run 内没有 linux job；收集步骤只 cp dmg/exe（:126-143，无 .deb 分支）；`linux-deb` artifact 只存在于 build.yml 的 push/PR run，跨 workflow run 不可见。README:149 如实标注「排期中」。
- 现成先例：build.yml `linux-deb` job（:47-94）——ubuntu-latest + `container: ubuntu:20.04` + 装 git → 钉版 `libwebkit2gtk-4.0-*=2.28.1-1` → `wails@v2.15.0` → `package.sh linux` → upload-artifact，可整体平移。
- 资产命名先例：release.sh:40 `ImageWorkshop_<V>_amd64.deb → ImageWorkshop-<V>-linux-amd64.deb`，CI 收集照抄。
- 漂移隐患（顺带修）：release.yml:72-73 用 `wails@latest`，build.yml:82 钉 `v2.15.0`——发布链与验证链工具链版本不一致。

### 设计

1. release.yml 新增独立 job `package-linux`（`needs: verify`）：平移 build.yml linux-deb job 步骤，加 `set-version.sh "${GITHUB_REF_NAME#v}"`（对齐其他腿 :102-104）→ `package.sh linux` → 收集重命名 `dist/${APP}-${V}-linux-amd64.deb` → upload-artifact `installers-linux`；`release` job 已是 merge-multiple 下载（:161-165），**自动带上 deb，零改动**。
2. deb 元数据校验步骤：`dpkg-deb --info` 断言 `Architecture: amd64`、`Package: imageworkshop`、版本 = tag 版本（现状只查文件存在）。
3. 修偏：release.yml 的 wails 安装统一钉 `@v2.15.0`。
4. 规格同步：`openspec/specs/package-release`「多平台产物汇入同一 Release」由滞后表述转为已交付；README 已知限制删「tag 发布暂不含 .deb」条、产物表补 Release 资产名。

**US-54 tag 发布含 .deb**：打 tag 的 Release 自动含 `ImageWorkshop-<版本>-linux-amd64.deb`，元数据经 CI 断言。

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-49 | release.yml 自建 linux job，不跨 workflow 取 build.yml 的 artifact | tag run 自包含、可复跑；GitHub 无跨 run 取物原生机制 |
| D-50 | 发布工具链统一钉 v2.15.0 | 消除 latest 漂移：发布链 = 验证链 |

工作量：**S**（无凭据依赖，最先做）。

---

## 变更二：无签名场景的朋友分发体验（安装指南 + 校验和）

替代原「签名公证」两个变更：证书不办，但「朋友拿到包打不开 / 不知道文件是否传坏了」这两个真实痛点要用零证书手段解决。

### 现状

- README 没有任何平台的「首次打开」指引；macOS 产物 ad-hoc 自签，朋友下载 dmg 后会被 Gatekeeper 拦（「无法打开，因为无法验证开发者」），Windows 未签名装器会触发 SmartScreen 蓝框（「Windows 已保护你的电脑」）——普通朋友不知道怎么放行。
- 校验和现状：仅 `package-deb.sh:116` **打印** sha256，不上传不落文件；dmg/exe 完全没有校验和。经网盘/IM 转发的文件损坏无从自查。
- release.sh 的 RELEASE_NOTES 模板（L46-63）有「校验建议」占位但没有实际校验物。
- release.yml 用 `--generate-notes` 自动生成说明（:180 附近），无安装指引。

### 设计

1. **README 新增「安装与首次打开」小节**（三平台各自三五行）：
   - macOS：dmg 拖入 Applications 后首次打开被拦 → 右键 App →「打开」；或在 系统设置 → 隐私与安全性 → 点「仍要打开」；或命令行 `xattr -dr com.apple.quarantine /Applications/ImageWorkshop.app`。
   - Windows：运行安装器出现蓝色 SmartScreen → 「更多信息」→「仍要运行」。
   - 麒麟/Linux：`sudo dpkg -i ImageWorkshop_<版本>_amd64.deb`（依赖由 apt 补齐）；README 已有的平台表保持一致。
   - 明确一句「个人分发的未签名包，首次打开需手动放行，属正常现象」。
2. **Release 资产附 `SHA256SUMS.txt`**：
   - CI：release job 在 download-artifact 汇齐 dist/ 后 `shasum -a 256 * > SHA256SUMS.txt` 一并 upload。
   - 本地：release.sh 在 `ls -la dist/` 前生成同文件。
   - README 安装小节附一句验证方法（`shasum -a 256 -c SHA256SUMS.txt` / `certutil -hashfile`）。
3. **（可选）release 说明尾注**：release.yml 的 `--generate-notes` 保留，Release 正文末尾用 `gh release edit` 追加安装指引短段+指向 README 小节的链接。不作为必做项。

**US-55 朋友可自行完成安装**：任一平台的普通用户按 README/Release 说明可独立完成下载、校验、放行、安装四步；文件传输损坏可用 SHA256SUMS 自查。

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-51 | 无签名摩擦用「右键打开/仍要运行」官方放行路径解决，不引导关闭安全机制（如全局禁 Gatekeeper） | 官方路径每机一次性、够用且不降低朋友机器的安全性 |
| D-52 | 校验和以 `SHA256SUMS.txt` 资产为准，每 tag 全量生成 | 网盘/IM 转发场景的完整性自查；deb 现有的「仅打印」升级为可下载物 |

工作量：**S**（纯文档 + 两处脚本/CI 尾部追加）。

---

## 变更三：身份表述收口（轻量）

原「bundle id + module 改名」降级为表述更正（D-48）。

### 现状

- README:150-151「应用标识」条与规格 D-32/§7.4 承诺「发布前必须替换为公司域名」——不存在公司域名，承诺悬空。
- 规格 §7.4 还有一处事实偏差：「Kylin `.deb` 包名使用小写 `image-workshop`」，实际 `package-deb.sh:83` 是 `imageworkshop`。
- 护栏 `TestBundleIDIsProjectControlled`（wiring_test.go:271-297）的错误提示文本引用 `com.example` 默认值说明，语义仍准确，可不改。

### 设计

1. README「应用标识」条改写为：「个人分发：bundle id 为占位值 `com.example.imageworkshop`、Go module 名为 `file-processor`，对 ad-hoc 分发无影响；若未来需要签名公证正式分发，再统一收口」。
2. 规格 D-32 与 §7.4 同步改口径；§7.4 的 deb 包名笔误一并更正为 `imageworkshop`。
3. 不改任何代码、模板与护栏。

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-53 | 身份值不动，只把「必须替换」的悬空承诺改为「个人分发现状 + 未来触发条件」 | 文档与事实一致；避免无收益的大范围机械替换引入风险 |

工作量：**S**（半小时级，纯文档）。

---

## 4. 实施顺序

```
变更三 表述收口 (S) ─── 纯文档，先做，立即消除悬空承诺
变更一 .deb 进 Release (S) ── CI 改动，下一个 tag 即生效
变更二 安装指南+校验和 (S) ── 随变更一一起交付也行（Release 资产同次成型）
```

三项合计约一天工作量，均无凭据依赖、无前置采购。

## 5. 护栏影响

- 变更一：新增 CI job 只调既有脚本（`make` 验证入口、`package.sh`、`set-version.sh`）；`TestWailsBuildHasSingleEntryPoint` 与禁 python3 两条脚本护栏自动覆盖且不冲突。
- 变更二/三：纯文档与 CI 尾部追加，零护栏影响。

## 6. 决策点（均有默认推荐，确认即执行）

| # | 问题 | 推荐 |
|---|---|---|
| 1 | Release 说明尾追安装指引（变更二第 3 项） | 做可选不做——README 小节 + SHA256SUMS 已覆盖主路径；默认**不做**，保持 CI 简单 |
| 2 | README 安装小节是否也写给「仓库为私有」的朋友下载路径（需 GitHub 账号邀协或网盘转发） | 默认**不写**，按公开 Release 假设；若仓库将保持私有再补 |

## 7. 既有文档修订清单（随各切片交付）

| 条款 | 修订 |
|---|---|
| README 已知限制 | 删「tag 发布暂不含 .deb」；「应用标识」条按变更三改写；删「正式发布需补开发者签名与公证」表述改为个人分发现状说明 |
| README 正文 | 新增「安装与首次打开」小节（变更二） |
| 规格 D-32 / §7.4 | 口径改写 + deb 包名笔误更正（变更三） |
| `openspec/specs/package-release` | 「多平台产物汇入同一 Release」转为已交付（变更一） |
| `docs/竞品对比与迭代建议.md` | P0-2 状态注明：签名公证按个人分发决策裁撤，`.deb` 接入完成 |
