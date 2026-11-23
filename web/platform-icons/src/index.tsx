/**
 * 平台视觉共享包：品牌图标 / 主题色 / 名称。
 *
 * 单色品牌图形来自 simple-icons；虎牙/斗鱼/抖音没有可用的开源图形，
 * 使用各站 favicon（生成到 favicons.ts）。图标均为装饰性，可读名称由相邻文字承担。
 */

import type { ReactElement, ReactNode } from "react";
import {
  siApplepodcasts,
  siBilibili,
  siDouban,
  siSinaweibo,
  siTwitch,
  siXiaohongshu,
  siYoutube,
} from "simple-icons";
import { DOUYIN_PNG, DOUYU_PNG, HUYA_PNG } from "./favicons";

export interface PlatformVisual {
  label: string;
  color: string;
  icon?: ReactNode;
  img?: string;
}

const PLATFORMS: Record<string, PlatformVisual> = {
  apple: brand("Apple Podcasts", siApplepodcasts),
  bili: brand("B站", siBilibili),
  douban: brand("豆瓣", siDouban),
  douyin: favicon("抖音", "#161823", DOUYIN_PNG),
  douyu: favicon("斗鱼", "#ff5e23", DOUYU_PNG),
  huya: favicon("虎牙", "#ff8d28", HUYA_PNG),
  twitch: brand("Twitch", siTwitch),
  weibo: brand("微博", siSinaweibo),
  xhs: brand("小红书", siXiaohongshu),
  youtube: brand("YouTube", siYoutube),
};

const FALLBACK_COLOR = "#7aa2f7";

export function PlatformIcon({
  namespace,
}: {
  namespace: string;
}): ReactElement {
  const visual = PLATFORMS[namespace];
  if (visual?.img !== undefined) {
    return (
      <img
        src={visual.img}
        alt=""
        aria-hidden="true"
        draggable={false}
        width={16}
        height={16}
        style={{ objectFit: "contain", flexShrink: 0, borderRadius: 3 }}
      />
    );
  }
  return (
    <svg
      viewBox="0 0 24 24"
      aria-hidden="true"
      focusable="false"
      width={16}
      height={16}
      style={{ flexShrink: 0 }}
    >
      {visual?.icon ?? <circle cx="12" cy="12" r="10" fill="currentColor" />}
    </svg>
  );
}

export function platformColor(namespace: string): string {
  return PLATFORMS[namespace]?.color ?? FALLBACK_COLOR;
}

export function platformName(namespace: string, label?: string): string {
  return label !== undefined && label !== ""
    ? label
    : PLATFORMS[namespace]?.label ?? namespace;
}

function brand(
  label: string,
  icon: { hex: string; path: string; title: string },
): PlatformVisual {
  const color = `#${icon.hex}`;
  return {
    label,
    color,
    icon: <path d={icon.path} fill={color} />,
  };
}

function favicon(label: string, color: string, img: string): PlatformVisual {
  return { label, color, img };
}
