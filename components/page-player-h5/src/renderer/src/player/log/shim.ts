/**
 * 上游 @amoeba/log 的最小替身：createLogDomain(域, 事件模板) → 语义化日志。
 * 事件模板只用于拼接可读文案，输出统一进 console（播放器调试信息，不做落盘）。
 * 返回类型按 events 的键精确映射（根 tsconfig 开了 noUncheckedIndexedAccess，
 * 宽索引签名会让每个 log.xxx 都变成可能 undefined）。
 */

interface LogEventConfig {
  level?: "log" | "debug" | "info" | "warn" | "error"
  text?: string | ((ctx: never) => string)
}

interface LogDomainConfig<E extends Record<string, LogEventConfig>> {
  events?: E
  [key: string]: unknown
}

type Emit = (payload?: unknown) => void
type FreeMethods = Record<"log" | "debug" | "info" | "warn" | "error", Emit>

const CONSOLE_METHOD = {
  log: console.log,
  debug: console.debug,
  info: console.info,
  warn: console.warn,
  error: console.error,
} as const

export function createLogDomain<const E extends Record<string, LogEventConfig>>(
  domain: string,
  config: LogDomainConfig<E> = {},
): { [K in keyof E]: Emit } & FreeMethods {
  const events = config.events ?? ({} as E)
  const out: Record<string, Emit> = {}
  for (const [name, ev] of Object.entries(events)) {
    out[name] = (payload) => {
      const text =
        typeof ev.text === "function"
          ? ev.text(payload as never)
          : (ev.text ?? name)
      CONSOLE_METHOD[ev.level ?? "debug"](`[${domain}] ${text}`, payload ?? "")
    }
  }
  for (const level of ["log", "debug", "info", "warn", "error"] as const) {
    out[level] = (payload) => CONSOLE_METHOD[level](`[${domain}]`, payload ?? "")
  }
  return out as { [K in keyof E]: Emit } & FreeMethods
}

export function formatError(err: unknown): string {
  if (err instanceof Error) return `${err.name}: ${err.message}`
  return String(err)
}
