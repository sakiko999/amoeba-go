# Renderer 共享层

> 目的：页面组件与宿主 chrome 不重复打包 `react` / `react-dom` / `@demo/ui` /
> `platform-icons`。
> 共享层是项目级构建基建，不进组件注册表，也没有运行时生命周期。

## 结构

```text
web/page-vendor/
├── entries/            jsx-runtime、demo-ui、platform-icons 等共享入口
├── scripts/build.ts    逐 entry Bun.build，输出内容 hash 文件与 manifest
├── scripts/import-map.ts
└── out/dist/           react-*.js、react-dom-*.js、demo-ui-*.js、platform-icons-*.js、manifest.json
```

`web/platform-icons/` 是对应的源码包（simple-icons SVG + 虎牙/斗鱼/抖音 favicon），
入口在 `entries/platform-icons.ts`。

`manifest.json` 记录 `vendor://app/`、共享包版本与 entry → 文件名映射。构建脚本
由根 `scripts/build.ts` 在页面组件打包前执行；宿主运行时经 `vendor://` 直接
服务 `web/page-vendor/out/dist`。

## 构建与消费

- 每个 entry 单独构建，并 external 其余共享裸名；entry 自己的包必须内联，否则会
  产出空 facade 与 import map 自引用。
- `react-dom` 与 `react-dom/client` 是同一 entry，保证 scheduler 单实例；CJS
  命名导出按运行时键面生成。
- 必须用 Bun.build：当前 rolldown-vite 对 external CJS 依赖会留下浏览器不可用的
  `require()` shim。
- 页面 `vite.config.ts` 读取 manifest，把共享裸名加入 external，并把 import map
  注入到 CSP meta 之后、首个 module script 之前；内联 JSON 同步追加 CSP sha256。
- 宿主 `vendor://` 响应带 `Access-Control-Allow-Origin: *`。跨自定义 scheme 的
  ESM import 受 CORS 约束，缺这头 import map 方案不成立。

## 设计边界

- Tailwind CSS 不共享：产物由每页输入生成，跨页统一扫描会引入全局样式耦合。
- 共享包的组件不依赖 Tailwind class：external 后页面扫描不到其源码；图标用固定
  `width/height` 与内联样式控制尺寸。
- `lucide-react` 不共享：全量入口大，逐页 tree-shake 更小。
- 共享集变更需同步 `web/page-vendor/scripts/build.ts` 的 entry 清单、import map
  生成逻辑与各消费方 vite 配置。
- 这只做磁盘/网络去重；每个 renderer 进程仍实例化自己的模块对象。
