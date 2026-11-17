/** 平台视觉元数据：namespace → 主题色；显示名优先用宿主 Source.Label。 */

const PLATFORM_COLORS: Record<string, string> = {
  bili: "#fb7299",
  douyu: "#ff5e23",
  huya: "#ff8d28",
  youtube: "#ff0033",
  twitch: "#9146ff",
};

const FALLBACK_COLOR = "#565f89";

export function platformColor(namespace: string): string {
  return PLATFORM_COLORS[namespace] ?? FALLBACK_COLOR;
}

export function platformName(namespace: string, label?: string): string {
  return label !== undefined && label !== "" ? label : namespace;
}
