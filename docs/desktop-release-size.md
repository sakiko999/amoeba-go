# desktop release 体积分析

> 快照（2026-11-09，Windows 11 / electron 45.2.3 / bun 1.4.4）：实测数字会随 Electron
> 版本与页面组件增减变化，结论（运行时占绝对大头）不受影响。
> 同日已实施运行时裁剪（locales + 可选 GPU 组件，见下文）与依赖内联（见「依赖产物
> 瘦身」节），产物 **283.33 MB**。

## 打包方式与产物

```powershell
bun run pages:prepare   # 组件包（含 120KB 页面 JS 体积守卫）
bun run desktop:dist    # = electron-vite build → sync-pages → @electron/packager → pruneRuntime
```

- 产物：`apps/desktop/release/Tabs Demo-win32-x64/`，裁剪前 364.72 MB → 裁剪后
  **286.33 MB**（免安装目录，asar，win32 x64；`dist.ts` 的 `pruneRuntime` 释放 78.39 MB）
- 验收：`"Tabs Demo.exe" --smoke` → `SMOKE OK: 6 tabs loaded`，exit 0（裁剪后已实测通过）
- 同构的 media-manager 独立包：`bun run --cwd packages/media-manager dist` →
  `packages/media-manager/release/Media Manager-win32-x64/`，357.8 MB，`--smoke` 通过

## 总体构成（364.72 MB）

| 项目                   |    大小 |  占比 | 说明                                   |
| ---------------------- | ------: | ----: | -------------------------------------- |
| `Tabs Demo.exe`        | 238.26M | 65.3% | Electron 45 本体（Chromium + Node）    |
| `locales/`（55 个）    |  48.82M | 13.4% | Chromium 语言资源包                    |
| `dxcompiler.dll`       |  24.58M |  6.7% | DirectX Shader Compiler（WebGPU/D3D12）|
| `resources.pak`        |  12.75M |  3.5% | Chromium 核心资源                      |
| `icudtl.dat`           |  10.32M |  2.8% | ICU 国际化数据                         |
| `resources/app.asar`   |   8.02M |  2.2% | 应用全部代码                           |
| `LICENSES.chromium.html` |  5.44M |  1.5% | 法律要求保留                           |
| `vk_swiftshader.dll`   |   5.20M |  1.4% | 软件 Vulkan 渲染 fallback              |
| `d3dcompiler_47.dll`   |   4.53M |  1.2% | D3D 编译器                             |
| `ffmpeg.dll`           |   2.95M |  0.8% | 音视频解码                             |
| 其余（vulkan-1、*.pak、snapshots 等） | ≈3.8M | ~1% | |

基准对比：同版本裸 `electron/dist` 为 **356.8 MB**，release 只多约 **7.9 MB** —— 就是
`app.asar` 本身。打包流程没有引入额外冗余。

## app.asar 细分（8.02 MB）

- `out/pages/` **6.23 MB**（22 个 runtime 页面组件）
  - `node_modules/` **3.15 MB**：`fast-xml-parser` 1.30 + `ramda` 1.17 + 其余 0.6
  - `platform-douyin/main.js` 0.71 MB（最大单页）、`media-manager` 0.24、`platform-bili` 0.21，
    其余 16 个组件各 0.1~0.2 MB
- `out/vendor/` **1.19 MB**：`demo-ui` 0.98 + `react-dom` 0.21 + `react` 0.01
- `out/renderer` 0.33 MB、`out/main` 0.06 MB、`out/preload` ≈4 KB
- 共 932 个文件，其中 **1.27 MB 是开发文件**（`.map` 995KB + README/CHANGELOG 133KB +
  `.d.ts` 107KB，实测修正早前 1.4MB 估算），来自共享层里的 npm 包原样复制——已通过
  依赖内联根治（共享层消失，见下文「依赖产物瘦身」节），下表数字为治理前快照

## 为什么组件 npm 依赖是原样复制而不是 bundle

不是遗漏打包，而是组件运行模型的要求（详见 `packages/platform-kit/src/pack.ts`）：

- **workspace 依赖内联**，普通 npm 依赖加入 `external`（`pack.ts:18-28`、`:54`），
  然后在组件产物目录里 `bun install --production` 安装真实依赖树（`pack.ts:67-83`）。
- 宿主 `sync-pages.ts:8-13`、`:136-160` 把各组件产物的 `node_modules` 合并为
  `out/pages/node_modules` 共享层：同名同版本去重，版本冲突的组件保留局部 `node_modules`
  （隔离优先）。

理由：

1. **组件是独立安装/升级/分发的单元**。runtime 组件由宿主 `createRequire(main.js)` 以组件
   目录为基准加载，`require.cache` 失效是热重载的基础；依赖必须是可解析的真实文件。
   bundle 进 `main.js` 会把依赖冻结在构建期，破坏独立分发与升级。
2. **版本语义与去重**。external 后同一版本全宿主只有一份（当前共享层共 3 MB）；各自
   bundle 会让 `fast-xml-parser` 等在每个组件里重复 N 份，也无法做版本冲突检测/隔离。
3. **原生扩展、动态 `require`、按包目录读取数据文件的依赖**无法可靠 bundle。
4. **workspace 依赖只能内联**：宿主 asar 内没有 workspace 链接。

对照：renderer 侧采用相反的打包策略（`page-vendor` 预构建 + import map 共享层，
见 [renderer-shared-layer.md](renderer-shared-layer.md)），`scripts/pack-components.ts:64-87`
还有 120KB 的页面 JS 体积守卫。即：**渲染层适合 bundle + 共享层，主进程/服务组件按
Node 包语义保留真实依赖树。**

## 依赖产物瘦身：已实施 devDependencies 内联（2026-11-09）

诉求：不接受 dist 阶段事后过滤产物，要求打包过程本身产出干净的依赖树。
共享层真实 require 面实测只有 **483KB / 15.4%**：ramda 只走 `src/`（367 文件
399KB；`es/`+`dist/` 双份 788KB 是 CJS 宿主永不读取的拷贝），fast-xml-parser 只走
自包含的 `lib/fxp.cjs`（79KB，文件内 0 个 require——7 个传递依赖包共 601KB 永不
加载）。组件 `main.js` 只做字面量根入口 require，无子路径、无 require .json。

### 为什么不能在「安装」或「构建器插件」层面根治

- bun/npm/pnpm 的 `--production`/`--omit` 只选择装哪类**依赖**，不选包内哪些
  **文件**；npm 官方明确拒绝 runtime-files-only 安装（rfcs discussion #292），
  bun 只有 open feature request（oven-sh/bun#24512）
- tarball 内容由发布者的 `files`/.npmignore 决定，README/LICENSE **强制**进包
- 构建器插件（vite/rolldown/Bun.build/esbuild）对安装产物**没有钩子**：依赖文件集
  由独立的 `bun install` 子进程产出，不在任何构建器的模块图内；且组件 main 由
  Bun.build 构建（不经过 vite，platform-* 无 vite 配置）。插件唯一能改变的是
  external↔bundle 决策——而这个开关就是组件 package.json 的声明字段

### 落地方案：依赖声明字段即运输模式开关

把 platform-bili/douyin/youtube 的 ramda、fast-xml-parser 从 `dependencies` 移到
`devDependencies`（连同 bun.lock 的分区变更），打包代码零改动：

- `pack.ts` 只把 `dependencies`（非 workspace）放进 external 并安装真实文件树；
  `devDependencies` 由 Bun.build tree-shake 内联进 `main.js`（先例：browser-proxy
  的 ws）。版本由仓库 bun.lock 锁定，比原先产物目录内无锁的 `^` 现场解析更可复现
- 语义验证（双盲实测）：fxp 内联产物（ESM lane）与 `lib/fxp.cjs` 对同一 Atom 样本
  逐字段一致；ramda 14 个 API（47 处调用）bundle 输出与直跑逐字节一致。语义基线
  固化为 `platform-youtube/tests/{feed,ramda-semantics}.test.ts`——升级任一依赖
  必须重跑
- 守卫（`scripts/pack-components.ts`）：内联守卫（bundle 内不得出现 electron/node
  内置之外的字面量 require，防 external 漏配与动态依赖误内联）+ main.js 体积预算
  （bili<300 / douyin<800 / youtube<400KB，防 tree-shake 静默退化）
- MIT 义务：bundle 后 node_modules 的 LICENSE 不再随包分发，改由
  `apps/desktop/THIRD_PARTY_NOTICES.txt`（13 包，顺手补上既有缺口 react/react-dom/
  scheduler/ws）承担；dist 放 release 根，升级依赖后重跑
  `apps/desktop/scripts/gen-notices.ts`

实测（实施后）：共享层 `out/pages/node_modules` 3.15MB/865 文件整体消失（`bun install`
不再执行）；out/pages 6.53MB/921 文件 → **3.41MB/55 文件**；`app.asar` 8.41 →
**5.01MB（-40.4%）**；release 286.33 → **283.33MB**；构建树另省 3 份局部
node_modules 共 5.55MB。代价：三个 main.js 合计 +191KB（tree-shake 后 ramda ≈37KB/
组件、fxp ≈121KB）。验收：`exe --smoke` 全绿（6 tabs / 21 组件节点）。

### 未采用的方案

- **②运行时白名单投影**（共享层 3.15→0.48MB）：机制保留作未来重型依赖的兜底，
  但工程量大（require 探针 + 闭包投影 + 缓存 + pack/sync 两处接入），且对抗评审
  实测发现「静态 require + 动态 import() 混合」场景会产出静默坏包（投影删掉
  ramda `es/` 后 import lane 断裂）；对当前依赖集不如直接内联
- **③facade 预构建**：原否决理由（条件 exports、ESM 具名导入）经复核部分弱化
  （rss ESM 消费方已停用、fxp.cjs 自包含），但其目标已由 devDependencies 内联以
  更轻的方式达成
- `dependencies` 通道原样保留：原生扩展（.node）、运行期动态 require、从包目录读
  数据文件、需跨组件单例/版本隔离的依赖仍走真实文件树 + 共享层（运输模式约定见
  `platform-kit/src/pack.ts` 头注释）

## 运行时裁剪（已实施，`dist.ts` pruneRuntime）

packager 的 `ignore` 只作用于 app 目录，Electron 运行时文件必须打包后删：

1. **locales 白名单**（-47.7 MB）：只留 `zh-CN.pak` + `en-US.pak`，其余 53 个 `*.pak`
   删除。Chromium 的 ResourceBundle 以 en-US 为硬性兜底（`crash_on_failure=true` 仅在
   en-US 也缺失时触发），OS locale 不在白名单时回退 en-US。安全前提：全仓库无
   `app.getLocale/setLocale`、`--lang`、locale 相关 `appendSwitch`；原生菜单/dialog
   文案是硬编码中文。删后 assert `en-US.pak` 存在，防止未来打包不完整静默产出不可
   启动的包。已知行为变化：非 zh-CN/en-US 系统上 Chromium 自带 UI（错误页、右键菜单、
   DevTools 界面语言）变英文；zh-TW 未保留（要繁体加回 zh-TW.pak ≈0.6MB）。
   实测：`--lang=de-DE` / `--lang=fr-FR` / 默认三种情况 `--smoke` 全过。
2. **可选 GPU 组件**（-30.7 MB）：删 `dxcompiler.dll`(24.6M) + `vk_swiftshader.dll`(5.2M)
   + `vk_swiftshader_icd.json` + `vulkan-1.dll`(0.9M)。GPU 进程启动即预载这些文件、
   失败非致命（Chromium `gpu_init.cc`）；实际使用方是 WebGPU / Skia Graphite(D3D12)，
   本仓库无任何使用面（源码与产物 grep 零命中，PE 导入表无静态依赖）。WebGL 的软件
   回退走 ANGLE D3D11 + WARP（系统组件），不受影响。实测（Intel Arc）：删除后正常
   启动、WebGL 仍 ANGLE D3D11、`--disable-gpu` 的 WARP 路径与 GPU 进程崩溃恢复行为
   均与删除前一致。
   - 能力移除（有意）：运行时安装的第三方页面若用 `navigator.gpu`，`requestDevice`
     会优雅失败（rejection，不崩进程）；显式 `--use-angle=swiftshader*` 失去 WebGL。
   - **升级 Electron 时必须复核此清单**：Chromium 已计划在 Windows 默认开启 Graphite
     （D3D12），届时删 dxcompiler 会让渲染静默退回 D3D11；需要 WebGPU 时从
     `RUNTIME_DROP_FILES` 移除并重打包。

## 优化空间（按收益排序）

| 动作 | 收益 | 状态 |
| --- | ---: | --- |
| 裁剪 locales（zh-CN / en-US） | ≈47 MB（-13%） | ✅ 已实施（见上） |
| 删 `dxcompiler.dll` + `vk_swiftshader`/`vulkan-1` | ≈30 MB（-8%） | ✅ 已实施并验证（见上） |
| 过滤 asar 内开发文件 | 2.66MB | ✅ 已实施（devDependencies 内联，共享层消失，app.asar -40.4%，见「依赖产物瘦身」节） |
| `LICENSES.chromium.html` / `resources.pak` / `icudtl.dat` | — | 不要删（法律要求 / Chromium 核心资源） |

分发压缩：`7z a -mx=7` 实测内联后整目录 **90.44 MB**（3.14×；裁剪前 105.36、
仅运行时裁剪后 90.74 MB）。
electron-builder NSIS（LZMA）预计同量级；只做绿色版时直接附 7z/zip 即可。

## 结论

- release 的 **97%+ 仍是 Electron/Chromium 运行时**，应用代码仅 5 MB（1.8%），没有异常膨胀。
- 裁剪与依赖内联均已落地：**364.72 → 283.33 MB（-22.3%）**，下载量（7z）105 → 90.4 MB
  （共享层文件高度可压缩，内联对压缩包收益收窄）。
- 想比 ~286 MB 再显著变小，只能更换运行时（Tauri/WebView2），不在当前方案范围内。
- media-manager 独立包（`packages/media-manager/scripts/dist.ts`）结构相同但尚未加
  裁剪——独立分发方案本身待确认，落地时把同样的 prune 段复制过去即可。
