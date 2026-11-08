# page-vendor

项目级 renderer 共享层（不是组件）：把 `react / react-dom / @demo/ui` 预构建成
ESM，由宿主 `vendor://` 协议提供，页面组件与宿主 UI 经 **import map** 解析裸模块名
到 `vendor://app/<file>` —— 全宿主同一 URL = 同一实例，消除每页各带一份 React 的重复。

## 构建（Bun.build，不是 vite）

```bash
bun run --cwd packages/page-vendor build   # out/dist + manifest.json
```

- 每个 entry **单独一次构建**：捆绑自己的包、external 其余共享裸名。
  ⚠️ 绝不能把 entry 自己的包也 external——会产出空 facade，import map 自引用死循环。
- `react-dom` 的根与 `/client` 是同一个 entry（同一实例、scheduler 内联）；
  `react / react-dom` 是 CJS 包，命名导出按运行时实际键面**生成**到 `out/gen/`
  （`export *` 会只剩 default）。
- 必须用 Bun.build：rolldown-vite 对 CJS 依赖的 external require 会留运行时
  `require()` shim，浏览器不可用（脚本里也有 `require()` 泄漏断言）。
- 产物内容 hash 命名；`manifest.json` 记录 `baseUrl / versions / entries`。

## 消费方式

- 页面 pack（home / component-inspector / media-manager）：manifest 存在时以
  `PAGE_VENDOR_EXTERNAL=1` 触发 external 构建（vite `rolldownOptions.external` =
  `SHARED_SPECIFIERS`），各页 electron-vite 插件在 `transformIndexHtml` 时用
  `injectIntoHtmlText` 把 import map 写进 HTML，并给 CSP 追加该内联 map 的
  `'sha256-…'` hash source（无需 `'unsafe-inline'`）。pack 脚本只做组装、不再改 HTML。
- 宿主 UI（apps/desktop）：electron-vite build 时读 manifest，同样 external +
  `transformIndexHtml` 注入（见 `apps/desktop/electron.vite.config.ts`）。
- **dev / 独立应用不 external**：仍走 node_modules 全内联，manifest 缺失时 pack
  自动回退内联。HTML 源里放的是合法空 map `{"imports":{}}`，其 sha256 已预置在
  源 HTML 的 CSP 中，dev 下内联占位不会被拦截（模块仍走 alias 解析）。
- `pages:prepare` 会打印各页 JS 体积；超过 120KB 直接失败，防止 external 静默失效
  （React 被重新打进页面包时功能 smoke 仍可能通过）。
- 宿主侧：`vendor://` scheme 注册（standard/secure/supportFetchAPI/corsEnabled），
  `out/vendor` 由 sync-pages 投影，响应带 `Access-Control-Allow-Origin: *`
  （跨 scheme 的 ESM import 是 CORS 受限的）。

## 改共享集时

同步三处：`SHARED_SPECIFIERS` / `entries` / `importMapOf`（本包），以及各消费方
vite config 的 `SHARED_EXTERNAL`。CSS 不共享（Tailwind utilities 是页面级生成的）。
