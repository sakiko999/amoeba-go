/** 把 assets 下的平台 favicon 内联为 base64，供共享层打包。 */

import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const root = join(import.meta.dir, "..");
const assets = join(root, "assets");
const out = join(root, "src", "favicons.ts");
const names = ["huya", "douyu", "douyin"] as const;

const lines = [
  "// 由 scripts/gen-favicons.ts 生成：各站 favicon 的 base64，勿手改。",
  "",
];
for (const name of names) {
  const b64 = readFileSync(join(assets, `${name}.png`)).toString("base64");
  lines.push(`export const ${name.toUpperCase()}_PNG =`);
  lines.push(`  "data:image/png;base64,${b64}";`);
  lines.push("");
}
writeFileSync(out, `${lines.join("\n")}\n`, "utf8");
console.log(`generated ${out}`);
