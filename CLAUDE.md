<!-- cc-todo:start -->

## TODO / 重要信息（cc-todo 自动维护）

- [!] bun 1.4.4 没有内置 lint 子命令：本项目 lint 用 biome（`biome lint .`），format 用内置 `bun fmt`（默认 80 列）
- [!] 本机网关证书过期（CERT_HAS_EXPIRED）：装需下载二进制的依赖时要临时 `$env:NODE_TLS_REJECT_UNAUTHORIZED='0'`（electron 已这样装好）
- [!] electron 装在 apps/desktop（bun per-workspace 安装），根目录启动必须 `bun run --cwd apps/desktop`，直接 `electron apps/desktop` 找不到命令

<!-- cc-todo:end -->
