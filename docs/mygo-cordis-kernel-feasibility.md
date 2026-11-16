# 极简 Go 版 cordis 作为 mygo 宿主组件系统 —— 可行性评估

> 2026-11-16。问题:基于 cordis 理念自研一套极简 Go 实现,作为本项目组件系统,
> 改造 platform/api-gateway 与将来的 page 组件,是否可行、是否值得。

## 0. 结论先行

**技术可行,工程上现在不值得。** cordis 的价值主张(可逆副作用、依赖反应式、
动态装卸/热重载)恰好全部落在本项目已明确砍掉的需求域里(§5.2:服务编译期内置、
不再支持外部安装 kind=service)。当下两个真实缺口 —— 优雅停机与页面→服务 ACL ——
各有约百行级的更小实现路径,不需要组件内核。自研内核的正确入场时机是
「外部可安装 service 组件」重新立项之时,且届时首选引 [stc-go](https://github.com/0xdenny218/stc-go)
(范式正确 + 五条元理论定理的属性测试)而非自研。

## 1. cordis 理念拆解与需求映射

cordis(时空可组合范式)的核心机制逐条对照本项目:

| # | cordis 机制 | 本项目需求命中 | 判定 |
|---|---|---|---|
| A | Context 树 / 作用域隔离 | 单宿主单作用域;页面在 WebView 进程边界外,不是子作用域 | 无用武之地 |
| B | provide/inject 命名服务 | `gateway.Register` 已是 provide;服务依赖 = `ServiceContext` 字段,编译期注入,零运行时解析 | 已有等价物(静态版) |
| C | revertible effects(卸载 LIFO 回滚) | 内置服务无卸载;page 卸载 = 删目录 + 级联关 tab,目录级操作 | 不需要 |
| D | 依赖反应式状态机(Pending/Loading/Active/…) | 9 平台互不依赖;唯一拓扑依赖 jsrunner 已静态注入;§5.2 已注明「服务内置后拓扑排序仅约束启动顺序」= 一个固定数组 | 退化为 init 顺序 |
| E | 动态装卸 / 热重载 | §5.2 明确决策不做;服务的 reload 概念消失 | 已砍 |

cordis 的精髓是 C+D+E(时空两个维度的可组合性);A+B 只是它的服务定位语法。
本项目需要的恰只有 B 的静态子集 —— 而 gateway 已经是它的静态实现。

## 2. 现状盘点(2026-11-16,feat/mygo-host)

- `gateway/gateway.go`:纯注册表(namespace → ops 函数表 + meta),Register/Call/List,
  并发安全,~150 行,可静态审计。无生命周期、无依赖、无调用方身份 —— 后两者是真实缺口(见 §4)。
- `services/services.go:27` `Activate`:固定顺序注册 9 平台,失败即 Fatal;无 Deactivate。
- `platform/extension.go` `ServiceContext`:宿主能力(HTTP/Cache/Creds/JS/Logf)以结构体
  字段注入 —— 这就是 cordis 里「宿主提供的服务」,只是编译期静态化。
- `components/manager.go`:page 组件 v1 —— 扫描 + manifest 校验 + page:// 服务;
  v1 无状态机(装载失败即不入表)。`manifest.go:32` `Inject []string` 已解析但未消费 ——
  正是 §5.2 规划的 ACL 白名单挂点。
- `main.go`:jsrunner `Init()` 在 WhenReady,退出无统一停机;`services.Activate` 的返回
  gateway 目前甚至没被页面桥消费(桥是二阶段项)。

## 3. 逐对象评估

### 3.1 改造 platform / api-gateway

cordis 化的改动 = 注册变成 fiber load、Call 经 ctx 解析服务对象、增加状态机与 effect 栈。
收益盘点:

- 依赖拓扑:0(§1-D,固定数组已覆盖)
- 动态装卸:0(已砍)
- 生命周期:**有真实缺口** —— jsrunner 的隐藏 WebView、弹幕 WS、cache 落盘需要优雅停机。
  但这是 Lifecycle 接口问题:§5.2 早已规划
  `Service interface { Manifest(); Activate(ctx); Deactivate() }`,落地该接口 + main.go
  统一 Shutdown 即可,估 50–100 行。不需要 cordis。
- 调用方身份/ACL:**有真实缺口**(未落地) —— 但解法是 `Call` 的 ctx 携带 caller、
  网关按 manifest.Inject 过滤,估 ~100 行(见 3.2),同样不需要 cordis。

代价:把「函数表、可枚举、编译期类型检查」的简单结构换成运行时容器;所有平台包的
`New(sc) Spec` 签名重写;新增一层任何人都需要重新学习的心智模型。
**净收益为负。**

### 3.2 将来的 page 组件

page 组件需要的「组件性」= manifest 注册(tab 菜单)+ inject 权限 + install/uninstall/reload。
逐项看 cordis 是否必要:

- **inject**:关键认知 —— page↔service 是跨进程边界(WebView → window.mygo 桥 → Go),
  cordis 的进程内 inject 对象模型在此根本不适用。Electron 侧把 inject 做成对象注入
  是因为页面与服务同进程同语言;Go 侧唯一合理语义就是 §5.2 已定的「inject 升格为
  可调服务 ACL」。落地路径:`Call(ctx, ...)` 的 ctx 携带 `CallerPage`(从页面 origin
  反解组件 id)→ 网关查该组件 manifest.Inject 白名单 → 未授权 op 报错且 `List` 同步过滤。
  这是网关鉴权,~100–150 行。
- **install/uninstall/reload**:目录复制 + 校验 + 重建 WebView + 级联关 tab,
  Manager 的领域操作,与组件内核无关。
- **状态机**:v1 的「校验失败不入表 + active 单终态」已够;§5.2 保留的
  inactive/active/error 状态机服务于「停用/启用」开关 —— 用 Manager 里的一个
  enabled 标志 + 重扫即可,不是反应式状态机。

**page 组件与 cordis 的交集为空**;它需要的全部机制都挂在 gateway(ACL)与
Manager(目录)上。

### 3.3 自研极简 cordis 内核(若坚持做)

规模估算(对照 stc-go core 的模块边界,砍掉 wasm/hmr/watch/registry):
Key 泛型定位 + Context + provide/inject + effect 栈 + fiber 状态机 ≈ **600–900 行
实现 + 等量测试,1–2 周**。

真正的风险不是行数,是正确性:

- 范式的难点全在并发语义 —— 级联重载的部分失败、LIFO 回滚与在途 Call 的竞态、
  Dispose→Gone 的等待协议。stc-go 用五条元理论定理的属性测试(含 -race + 随机调度
  + fuzz)做验收;Electron 侧组件内核 v2 的遗留项(pending 态、alive 令牌、effect 栈)
  正是这些难点,至今未完成。
- Go 比 TS 的交错面更宽:任意 goroutine 可并发 Call,没有 async/await 的天然串行域。
- 「极简」陷阱:砍掉定理验证的 cordis 内核,语义没有被证明 —— 等真到动态装卸场景
  还是不敢用它,前期投入等于沉没。**要麽带验证地做(stc-go 已做),要麽不做。**

## 4. 建议路径

把「需要组件系统」这个直觉拆回三个具体需求,各走最小路径:

| 需求 | 方案 | 规模 | 依赖 cordis? |
|---|---|---|---|
| 优雅停机 | §5.2 的 Service Lifecycle 接口 + main.go Shutdown 编排 | ~50–100 行 | 否 |
| 页面→服务权限 | Call 携带 CallerPage + manifest.Inject ACL 过滤(List 同步) | ~100–150 行 | 否 |
| 服务启动顺序 | 现有固定数组(services.go specs 切片) | 0 | 否 |

自研 cordis 内核的触发条件(三条件同时成立才立项):

1. 「外部可安装 service 组件」重新立项(带代码的动态服务);
2. 届时评估 stc-go 不合格(停维护 / 范式不匹配 / wazero 体积不可接受);
3. 且接受自研后需自建属性测试验收(对齐 stc-go 的五定理思路)的持续成本。

届时先做 stc-go POC(它已给出 WASM guest 路线:宿主 ABI + 依赖门控 + 失败回滚),
POC 结论再决定引入或抄范式自研 —— 顺序不可反。

## 5. 对既有决策的影响

本评估不推翻 §5.2 任何结论,反而是它的延伸:§5.2 砍掉动态服务时,顺带砍掉了
对 cordis 类组件内核的全部需求。若未来恢复动态服务,§5.2 的「宿主进程内从此没有
第三方代码」安全模型前提也随之失效,需要与 stc-go/WASM 的沙箱语义一起重新评审。
