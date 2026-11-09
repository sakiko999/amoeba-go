# desktop release 体积分析

> 快照（2026-11-09，Windows 11 / electron 45.2.3 / bun 1.4.4）：实测数字会随 Electron
> 版本与页面组件增减变化，结论（运行时占绝对大头）不受影响。
> 同日已实施运行时裁剪（locales + 可选 GPU 组件，见下文），产物 **286.33 MB**。

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
  `.d.ts` 107KB，实测修正早前 1.4MB 估算），来自共享层里的 npm 包原样复制——干预
  方案见下文「依赖产物瘦身」节

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

## 依赖产物瘦身：干预点研究（2026-11-09，已完成未实施）

诉求：不接受 dist 阶段事后过滤产物，要求打包过程本身产出干净的依赖树。
共享层真实 require 面实测只有 **483KB / 15.4%**：ramda 只走 `src/`（367 文件
399KB；`es/`+`dist/` 双份 788KB 是 CJS 宿主永不读取的拷贝），fast-xml-parser 只走
自包含的 `lib/fxp.cjs`（79KB，文件内 0 个 require——7 个传递依赖包共 601KB 永不
加载）。组件 `main.js` 只做字面量根入口 require，无子路径、无 require .json。

### 为什么不能在「安装」层面根治

- bun/npm/pnpm 的 `--production`/`--omit` 只选择装哪类**依赖**，不选包内哪些
  **文件**；npm 官方明确拒绝 runtime-files-only 安装（rfcs discussion #292），
  bun 只有 open feature request（oven-sh/bun#24512）
- tarball 内容由发布者的 `files`/.npmignore 决定，README/LICENSE **强制**进包，
  「让上游少发」不可行
- 生态工具全有误伤先例：node-prune 删 `docs/` 弄坏过 googleapis、ModClean 的
  `license*` 删掉 aws-sdk 的 `licensemanager.js`、electron-builder 曾硬排除
  `test.js` 弄坏 Playwright。electron-builder 默认也只敢删 node_modules 内
  README/CHANGELOG/.d.ts/test 且保留 LICENSE、不删 .map
- 结论：只能干预自己的 pack 管线——在 `pack.ts` 与 `sync-pages.ts` 的
  `bun install --production` 之后处理，产出即干净（独立组件包与 dev out/ 同步
  受益；构建树里另有 3 份局部副本共 5.55MB 可一并清理）。这与被否决的 dist
  阶段删产物不同：文件从未进入任何分发物

### 候选方案（数字为 bun + node 双端实测，含真实组件集成验证）

| 方案 | 共享层 | 风险 / 备注 |
| --- | ---: | --- |
| ① 窄黑名单：只删 `*.map`（995KB，占可回收量 83%，可加 `.d.ts` 107KB），保留 README/CHANGELOG/LICENSE | 3.15→1.92MB | ≈0；只用扩展名精确匹配，避开目录名/文件名子串通配（全部生态误伤类） |
| ② **运行时白名单投影（推荐）**：require 探针实测每个包的加载闭包，只保留 `require.cache` 触碰的文件 + package.json + LICENSE | **3.15→0.48MB**（app.asar 8.0→5.4MB，-33%） | 仍是真实包文件树（ramda 子路径 exports、createRequire、热重载不受影响）；懒加载分支可能丢文件 → 真实组件集成测试（已验证可离线跑 activate + 网关注册）+ 宿主 smoke 兜底；探针失败的包回退完整安装 |
| ③ facade 预构建（bun build 单文件 CJS） | 3.15→0.16MB | **否决**：本仓库闭包内已有实锤——xml-naming 仅有 import 条件 exports、is-unsafe 的 require 条件是抛错 Proxy 垫片、CJS facade 会断 native ESM 具名导入（rss 数据服务进程正是 ESM 链路） |

任何方案的安全底线：保留 package.json（模块解析必需，且部分包运行时读自己的
version）、`LICENSE*`/`NOTICE*`/`COPYING*`；投影后断言每个包 main/exports/bin
指向的文件仍存在；安装加 `--frozen-lockfile` 固定闭包。实验数据与验证脚本在
`tmp/size-research/`（gitignore，不入库）。

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
| 过滤 asar 内开发文件 | 1.27MB（①）/ **2.66MB（②）** | ⏸ 研究完成未实施（见「依赖产物瘦身」节，推荐方案②运行时白名单投影） |
| `LICENSES.chromium.html` / `resources.pak` / `icudtl.dat` | — | 不要删（法律要求 / Chromium 核心资源） |

分发压缩：`7z a -mx=7` 实测裁剪后整目录 **90.74 MB**（3.16×；裁剪前 105.36 MB）。
electron-builder NSIS（LZMA）预计同量级；只做绿色版时直接附 7z/zip 即可。

## 结论

- release 的 **97%+ 仍是 Electron/Chromium 运行时**，应用代码仅 8 MB（2.2%），没有异常膨胀。
- 裁剪已落地：**364.72 → 286.33 MB（-21.5%）**，下载量（7z）105 → 91 MB。
- 想比 ~286 MB 再显著变小，只能更换运行时（Tauri/WebView2），不在当前方案范围内。
- media-manager 独立包（`packages/media-manager/scripts/dist.ts`）结构相同但尚未加
  裁剪——独立分发方案本身待确认，落地时把同样的 prune 段复制过去即可。
