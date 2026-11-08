/** provider 通用错误（平台插件与服务端共用同一份类型）。 */

export class ProviderError extends Error {
  constructor(
    public readonly source: string,
    message: string,
  ) {
    super(`[${source}] ${message}`);
    this.name = "ProviderError";
  }
}

export class NotFoundError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NotFoundError";
  }
}

/**
 * 网关 op 执行失败的统一出口错误：携带 `平台.操作` 上下文与原始 cause。
 * 媒体平台与数据渠道平台的 op 失败都包成这一种形状，消费方按 instanceof 判别。
 */
export class PlatformOpError extends Error {
  constructor(
    public readonly platform: string,
    public readonly op: string,
    cause: unknown,
  ) {
    super(
      `[${platform}.${op}] ${cause instanceof Error ? cause.message : String(cause)}`,
      { cause },
    );
    this.name = "PlatformOpError";
  }
}
