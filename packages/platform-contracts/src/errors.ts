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
