# 组件模型 vs cordis（dsh）对比

对照对象：deepseek-harness 的 cordis 教程（deepseek-harness.github.io / dshx.dev）与
本项目 `apps/desktop/src/main/pages/registry.ts` 的实现。

## 机制对照

| 维度 | 本项目（组件模型 v1） | cordis / dsh |
| --- | --- | --- |
| 依赖声明 | manifest `inject: []`（component.json，静态） | `export const inject = ['greeter']`（插件代码内声明） |
| 解析时机 | **boot 时一次**：宿主按 provides/inject 建 Kahn 拓扑、排序后顺序 activate；inject 未满足时 `ctx.require` 抛「服务未就绪」 | **运行时持续**：fiber 状态机，服务未就绪停 `PENDING`，依赖驱动启动（加载顺序无关）；服务消失时依赖者自动卸载、回来时重载 |
| 服务注册 | `ctx.provide(name, impl, { expose })`；宿主按 manifest 的 expose **白名单**生成代理 | Service 类 `super(ctx, 'name')`；`ctx.provide` 本身是 effect，卸载自动撤销 |
| 服务消费 | `ctx.require<T>(name)`，同步；必须 manifest 声明过（防隐式依赖） | `ctx.greeter` 访问器 + TS declaration merging（编译期类型、运行时属性） |
| 生命周期 | activate/deactivate 两相；deactivate 手写清理；8s 超时 | `PENDING→LOADING→ACTIVE→UNLOADING→DISPOSED/FAILED`；`ctx.effect(()=>disposer)` 逆序清理，`ctx.on/ctx.plugin/服务注册` 本身都是 effect |
| 嵌套/组合 | 无（扁平组件表） | `ctx.plugin(child)` 插件树，fiber.dispose() 递归卸载子插件 |
| 调用形态 | 同进程直接对象方法（expose 白名单内）；跨进程（rss 数据服务）自定义 HTTP/WS RPC + owner/client token；平台 API 经 api-gateway 注册表 | 全部同进程访问器调用（无跨进程域） |
| 依赖分发 | 组件可自带 npm 依赖（pack external + production 装进产物 node_modules；宿主 createRequire(main.js) 以组件目录解析）；workspace 依赖内联 | 插件即 npm 包，宿主 node_modules 完整 |
| 热重载 | CJS main + `require.cache` 失效 + 服务 reload 级联停用/重激活 | 配置驱动 HMR，effect 栈自动回滚 |

## 我们有而 cordis 没有的

- **expose 白名单**：服务方法面在 manifest 里显式声明，宿主生成代理收敛能力——
  这是渲染进程/组件边界的安全层（owner token 不出 rss-client 这类约束的基座）。
- **跨进程组件域**：rss 数据服务进程 + 动态加载的平台 bundle（channels.js）+
  「注册动作发生在消费方进程」的注册表契约——cordis 是单进程模型，没有对应物。

## cordis 有而我们缺的（= v2 roadmap 的出处）

1. **pending 态**：服务未就绪时组件等待而非失败（现在是拓扑排序保证顺序，
   运行中服务下线只能靠级联停用，无「等待回来」）。
2. **effect 栈**：注册即 effect、逆序自动清理（现在 deactivate 手写，泄漏风险）。
3. **运行时依赖追踪**：服务消失/回归时依赖者自动卸载重载（现在是 reload 级联，
   覆盖面窄）。
4. **插件树**：ctx.plugin 嵌套组合与 fiber 句柄（现在扁平）。

## 结论

v1 用「静态 manifest 拓扑 + 同步 require + expose 白名单」换来了可审计的组件边界
与零框架依赖；cordis 用「fiber 状态机 + effect 栈 + 运行时服务事件」换来更强的
动态性。两者不冲突：v2 的 pending/effect/级联项就是把 cordis 的这三件语义
在现有 manifest 模型上补齐，而 expose 白名单与跨进程注册表契约是本项目特有、
应当保留的。
