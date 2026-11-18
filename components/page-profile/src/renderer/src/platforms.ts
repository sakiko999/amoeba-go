/**
 * 平台视觉元数据：namespace → 主题色 / 显示名。
 * 与 page-search 的同名表刻意重复——两页独立分发，抽公共包等第三个消费方出现再说。
 */

const PLATFORM_COLORS: Record<string, string> = {
  bili: "#fb7299",
  douyu: "#ff5e23",
  huya: "#ff8d28",
  youtube: "#ff0033",
  twitch: "#9146ff",
};

const PLATFORM_NAMES: Record<string, string> = {
  bili: "B站",
  douyu: "斗鱼",
  huya: "虎牙",
  youtube: "YouTube",
  twitch: "Twitch",
};

const FALLBACK_COLOR = "#565f89";

export function platformColor(namespace: string): string {
  return PLATFORM_COLORS[namespace] ?? FALLBACK_COLOR;
}

export function platformName(namespace: string): string {
  return PLATFORM_NAMES[namespace] ?? namespace;
}
