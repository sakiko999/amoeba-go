/** 浏览器载具日志域（从 @amoeba/host 的 host/log.ts 提取，只保留 browser 域）。 */

import { createLogDomain } from "../../logger/index.ts";

export const browserLog = createLogDomain("host:browser", {
  color: "#34d399",
  ansi: 78,
  events: {
    evalError: {
      level: "warn",
      text: (ctx: { expression: string; detail: string }) =>
        `CDP evaluate 异常\n  expression: ${ctx.expression}\n  detail: ${ctx.detail}`,
    },
    armFailed: {
      level: "warn",
      text: (ctx: { detail: string }) =>
        `资源拦截未挂上(只是少了优化,不影响抓取): ${ctx.detail}`,
    },
    idleClose: {
      level: "info",
      text: (ctx: { minutes: number }) =>
        `浏览器空闲 ${ctx.minutes} 分钟,已关闭(下次使用自动重启)`,
    },
    tabIdleClose: {
      level: "info",
      text: (ctx: { key: string; minutes: number }) =>
        `lane tab 空闲 ${ctx.minutes} 分钟,已回收 target(${ctx.key};下次使用自动重建并重新导航)`,
    },
    uiSweep: {
      level: "info",
      text: (ctx: { count: number }) =>
        `已关闭 ${ctx.count} 个 Edge 自带界面 target(省 ~200MB private)`,
    },
  },
});
