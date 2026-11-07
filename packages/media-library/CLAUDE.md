# media-library

媒体库后端服务（`@tabs/media-library`，主仓库普通子包，非 submodule）：扫描 → 刮削
→ NFO 读写 → 目录整理 → sqlite 元数据缓存。元数据渠道（Bangumi/TMDB/豆瓣）是
**扩展组件**（`packages/media-provider-*`），经 `media.providers` 扩展点注册；本包只
保留注册表与 provider 公共基座。

## ⚠️ 安全铁律（继承自 media-manager，搬代码不搬闸门等于没搬）

**绝不把破坏性操作指向真实数据。** 前身曾因「junction 当隔离副本 + organize --apply
+ rm -rf」链丢失 15GB 视频。

- 整理强制 plan（dry-run）→ 用户确认 → apply；目标已存在一律跳过，绝不覆盖
- 源目录是符号链接/junction 时 `planOrganize` 直接拒绝 —— **不要移除该闸门**
- 写测试只对可丢弃目录（`tmp/`）运行，绝不在真实媒体库上跑 apply
- 刮削/NFO 写回只由用户在当前作品页显式触发（service 不自触发）
- 凭证只走环境变量（`TMDB_TOKEN`），不经前端传递、不做命令行参数

## 结构与消费方

```
src/
├── index.ts         公共入口（createMediaService / bindMediaIpc / 类型）
├── main/
│   ├── service.ts        纯能力对象（无 IPC；三种宿主形态共用的核心）
│   ├── ipcBinding.ts     channel → 服务方法绑定表（sender origin 鉴权）
│   ├── serviceEntry.ts   宿主服务组件 main（CJS）：ctx.provide("media.library")
│   │                     + ctx.provide("media.providers")（渠道注册表扩展点）
│   ├── library.ts / naming.ts / nfo*.ts / organize.ts / scrape.ts / cache.ts
│   ├── providers.ts      ProviderRegistry + MetadataProvider 接口（scrape 经此取渠道）
│   └── provider/http.ts  net.fetch + 缓存包装 + ProviderError/NotFoundError（公共基座）
└── shared/           model.ts + types.ts（三方契约，renderer/preload 以 type-only 引用）
```

消费方：

- **宿主形态**：`bun run --cwd packages/media-library pack` 产出 `out/component/`
  （kind=service 组件，id=media-library），`sync-pages.ts` 同步进宿主 `out/pages/`；
  页面组件 media-manager 经 `ctx.require("media.library")` 消费。元数据渠道是
  独立服务组件（`media-provider-{tmdb,douban,bangumi}`，inject `media.providers`），
  activate 时 `register(createXxxProvider())`；`scrape.ts` 只经 `ProviderRegistry`
  取渠道，未安装时抛「未安装 X 元数据渠道组件」。`sync-pages.ts` 对同名组件 id 会
  告警跳过（out/ 旧产物残留不得静默覆盖新组件）。
- **独立形态**：media-manager（submodule）的 `src/main/backend.ts` 用
  `createMediaService + bindMediaIpc` 直接组装到 ipcMain，不经宿主服务表；provider
  在同一处静态注册（`providers.register(createXxxProvider())`）。
- channel 契约（renderer ↔ 服务）唯一来源是 `src/main/ipcBinding.ts` 的 BINDINGS。
- provider 单元测试靠 `tests/setup.ts`（bunfig preload）把 electron 换成桩；三个
  `media-provider-*` 包各有一份同样的桩（测试只跑 fixture 映射，不发网络）。

## 命令（monorepo 根）

```bash
bun run --cwd packages/media-library test    # 单元测试（bun:test，fixtures 驱动）
bun run --cwd packages/media-library pack    # 产出宿主服务组件包 out/component/
bun run --cwd packages/media-manager start --probe-bangumi 8   # 真实网络只读探针
```

探针实现已随渠道迁移：`packages/media-provider-bangumi/src/probe.ts`（`--probe-bangumi`
/ `--probe-bangumi-apply`，写盘探针只对可丢弃目录使用），media-manager 仍负责命令行入口。

领域约定（库根一级子目录 = 作品、Season NN 优先、poster/folder、每集 NFO 候选
S01E01.nfo → 视频同名 .nfo）、两套集号解析（library 展示三模式 / naming 整理 6 模式）、
Bangumi 陷阱（自定义 UA、POST JSON 搜索、书籍/音乐拒绝）见各模块注释与黄金测试。
