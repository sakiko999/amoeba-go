/**
 * dev 启动前检查关键端口。残留的上一份 dev（VS Code 终止任务后 electron-vite 树偶发存活）
 * 会占住 5173/9229，新实例静默换端口，并与旧实例共享 node_modules/.vite 缓存与 userData，
 * 结果就是依赖反复预构建、首屏白屏十几秒。宁可启动时明确报错。
 */

import { createConnection } from "node:net";

const PORTS = [
  { port: 5173, name: "renderer dev server" },
  { port: 9229, name: "main inspector" },
];

function isBusy(port: number): Promise<boolean> {
  return new Promise((resolve) => {
    const socket = createConnection({ port, host: "127.0.0.1" });
    const finish = (busy: boolean): void => {
      socket.destroy();
      resolve(busy);
    };
    socket.once("connect", () => finish(true));
    socket.once("error", () => finish(false));
    socket.setTimeout(300, () => finish(false));
  });
}

const busy: string[] = [];
for (const { port, name } of PORTS) {
  if (await isBusy(port)) busy.push(`${port} ${name}`);
}

if (busy.length > 0) {
  console.error(
    "端口已被占用，可能还有 dev 实例在跑（先关闭它的应用窗口或终止 VS Code 任务）：",
  );
  for (const item of busy) console.error(`  ${item}`);
  process.exit(1);
}
