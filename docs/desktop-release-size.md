# desktop release 体积分析

> 快照（2026-11-09，Windows 11 / electron 45.2.3 / bun 1.4.4）：实测数字会随 Electron
> 版本与页面组件增减变化，结论（运行时占绝对大头）不受影响。

## 打包方式与产物

```powershell
bun run desktop:dist        # = pages:prepare → electron-vite build → sync-pages → @electron/packager
```

- 产物：`apps/desktop/release/Tabs Demo-win32-x64/`，**364.72 MB**（免安装目录，asar，win32 x64）
- 验收：`"Tabs Demo.exe" --smoke` → `SMOKE OK: 6 tabs loaded`，exit 0
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
  - `node_modules/` **3.07 MB**：`fast-xml-parser` 1.30 + `ramda` 1.17 + 其余 0.6
  - `platform-douyin/main.js` 0.71 MB（最大单页）、`media-manager` 0.24、`platform-bili` 0.21，
    其余 16 个组件各 0.1~0.2 MB
- `out/vendor/` **1.19 MB**：`demo-ui` 0.98 + `react-dom` 0.21 + `react` 0.01
- `out/renderer` 0.33 MB、`out/main` 0.06 MB、`out/preload` ≈4 KB
- 共 932 个文件，其中约 **1.4 MB 是开发文件**（`.map`、`README.md`、`CHANGELOG`、`.d.ts`），
  来自共享层里的 npm 包原样复制

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

## 优化空间（按收益排序）

| 动作 | 收益 | 做法 / 风险 |
| --- | ---: | --- |
| 裁剪 locales（只留 zh-CN / en-US，可选 zh-TW） | **≈47 MB（-13%）** | `dist.ts` packager 完成后删白名单外 `locales/*.pak`。packager 的 `ignore` 只作用于 app 目录，管不到 Electron 运行时文件，必须打包后处理 |
| 删 `dxcompiler.dll` + `vk_swiftshader`/`vulkan-1` | ≈30 MB（-8%） | 仅当不使用 WebGPU 且不需要软件渲染 fallback；属 Chromium 可选组件，删后必须真机 smoke + 实际使用验证 |
| 过滤 asar 内开发文件（`.map`/README/CHANGELOG/.d.ts） | ≈1.4 MB（-0.4%） | 在 sync-pages 安装共享层后按运行时文件白名单过滤，不影响模块解析 |
| `LICENSES.chromium.html` / `resources.pak` / `icudtl.dat` | — | 不要删（法律要求 / Chromium 核心资源） |

分发压缩：`7z a -mx=7` 实测整目录 **105.36 MB**（3.46×）。electron-builder NSIS（LZMA）
预计同量级；只做绿色版时直接附 7z/zip 即可把下载量降到 1/3。

## 结论

- release 的 **97.8% 是 Electron/Chromium 运行时**，应用代码仅 8 MB（2.2%），没有异常膨胀。
- 唯一高性价比动作是 **裁剪 locales（-13%）**；其次是在验证后删 DXC/软件 Vulkan（-8%）。
- 想比 ~317 MB（裁 locales 后）再显著变小，只能更换运行时（Tauri/WebView2），不在当前方案范围内。
