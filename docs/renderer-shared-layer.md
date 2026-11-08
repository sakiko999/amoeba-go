# renderer 共享层（vendor:// + import maps）

> 目的：页面组件与宿主 UI 不再各自打包一份 `react / react-dom / @demo/ui`。
> 共享层是**项目级基建**，不是组件；它不经过组件注册表。

## 结构

```
packages/page-vendor/
├── entries/          jsx-runtime.ts、demo-ui.ts（CJS 包的命名导出由脚本生成）
├── scripts/build.ts  每个 entry 单独 Bun.build → out/dist（内容 hash + manifest.json）
├── scripts/import-map.ts  manifest → import map / CSP hash 注入工具
└── out/dist/         react-<hash>.js、react-dom-<hash>.js、demo-ui-<hash>.js、manifest.json
```

`manifest.json` 记录 `baseUrl: "vendor://app/"`、各包解析版本（`versions`）与
entry → 文件名映射（`entries`）。

## 构建要点

- **每个 entry 单独构建**：捆绑自己的包，external 其余共享裸名。绝不能把 entry 自己的包
  也 external——会产出空 facade，形成 import map 自引用死循环。
- `react-dom` 的根与 `react-dom/client` 是同一个 entry（同一实例，scheduler 内联）；
  `react` / `react-dom` 是 CJS 包，`export *` 只会剩 default，命名导出必须按运行时实际键面
  生成到 `out/gen/`。
- 必须用 **Bun.build**：rolldown-vite 对 external 的 CJS 依赖会留运行时 `require()` shim，
  浏览器不可用；构建脚本末尾有 `require()` 泄漏断言。
- 输出内容 hash 命名，消费方只依赖 manifest，不写死文件名。

## 消费方式

- 页面组件与宿主 UI 的 `electron.vite.config.ts`：manifest 存在时把共享裸名放进
  `rolldownOptions.external`（`SHARED_SPECIFIERS`），并在 `transformIndexHtml` 里注入
  import map。
- **注入位置**：构建插件匹配 CSP meta，把 `<script type="importmap">…</script>` 插在它
  之后——必须早于所有 module script，同时受该 CSP 策略管辖。源 HTML 里**没有**占位脚本。
- 内联 import map 受 CSP `script-src` 管辖：注入时同步追加其 `'sha256-…'` hash source
  （锚点 `script-src 'self' vendor:`），不需要 `'unsafe-inline'`。
- dev / 独立应用构建不 external、不注入：Vite 自己解析裸导入，页面全内联。
- 宿主侧：`vendor://` scheme（standard/secure/supportFetchAPI/corsEnabled）映射
  `apps/desktop/out/vendor`（由 sync-pages 从 `page-vendor/out/dist` 投影）；
  响应必须带 `Access-Control-Allow-Origin: *`——跨 scheme 的 ESM import 是 CORS 受限的。
- `pages:prepare` 先构建 page-vendor，再打包页面组件；生产构建顺序由此保证。

## 效果（2026-11 实测）

| 产物 | 接入前 | 接入后 |
| --- | --- | --- |
| home | ~774KB | ~19KB |
| component-inspector | ~782KB | ~26KB |
| media-manager | ~867KB | ~57KB |
| 宿主 UI | ~724KB | ~168KB |
| vendor | 无 | ~1.2MB（全宿主一份） |

页面越多收益越大。注意这是**磁盘/网络去重**：每个 renderer 进程仍各自实例化一份模块，
import map 的价值是同一 URL 单实例、不重复打包。

## 设计取舍

- **为什么需要独立子包**：共享产物必须在所有消费者构建之前产出。页面的 vite config 只能
  消费 manifest、external 裸名，不能负责生产它；把构建逻辑塞进任一页面 config 会造成
  隐式构建顺序与跨包写文件。子包还归属共享集清单、版本、CJS 命名导出与 hash/manifest 格式。
- **为什么不用 `rolldownOptions.output.paths` 直接写 vendor URL**：消费者的 bundle 可以，
  但 vendor 自身的 `demo-ui.js` 也 import `react`（裸名），仍需要 import map 或二次 URL
  重写；加上 rolldown 对 CJS external 的 require shim 问题，import map 是唯一同时覆盖
  “消费者 + vendor 内部”两层解析的机制（规范要求 import map 内联）。
- **CSS 暂不共享**：Tailwind 产物是页面级生成的（~170KB/页），共享需要跨页统一扫描，
  留作后续优化。
- **lucide-react / dnd-kit 不进共享集**：lucide 全量入口过大，逐页 tree-shake 更小；
  dnd-kit 只有宿主使用，没有跨页重复。
- **版本一致性**：`pages:prepare` 每次先重建 vendor；workspace 内解析版本一致，
  因此不再做额外的版本校验。

## 回归守卫

- `scripts/pack-components.ts` 在打包后打印 home / component-inspector / media-manager 的
  JS 体积；任一超过 120KB 直接失败。防的是 external 静默失效——React 被重新打进页面包时，
  功能 smoke 仍可能全绿。
- 改动共享集时必须同步：`SHARED_SPECIFIERS`、`page-vendor` 的 `entries` 与
  `importMapOf`、各消费方 vite config 的 `SHARED_EXTERNAL`。

## 常见坑

- import map 必须早于 module script；Vite 构建会把 script 移到 head 末尾，所以注入点选
  CSP meta 之后而不是 `</head>` 之前。
- CSP hash 覆盖 script 的**精确文本内容**；注入的 JSON 必须与实际写入字节一致。
- 宿主 renderer 构建是单 html 入口；页面组件各自构建，不再进宿主 MPA。
- `vendor://` 是独立 scheme，不占用 `page://` 的 host 命名空间。
