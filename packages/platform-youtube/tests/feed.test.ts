/**
 * parseFeed fixture：锁住 fast-xml-parser 内联（devDependencies 构建）后的解析语义。
 * 关键回归点：parseTagValue:false 下数字形字符串（guid "0123"）必须保持字符串；
 * 实体/CDATA/命名空间属性的处理来自 fxp 及其 ESM 传递依赖（strnum 等）——
 * 升级任一依赖后此文件必须重跑（见 bun.lock）。
 */
import { describe, expect, test } from "bun:test";
import { parseFeed } from "../src/lib/feed.ts";

const RSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/" xmlns:yt="http://www.youtube.com/xml/schemas/2015">
  <channel>
    <title>Channel &amp; Title</title>
    <link>https://example.com/feed</link>
    <description><![CDATA[Desc with <b>html</b>]]></description>
    <item>
      <title>Item 1</title>
      <link>https://example.com/w/1</link>
      <guid isPermaLink="false">0123</guid>
      <pubDate>Mon, 02 Jan 2026 03:04:05 GMT</pubDate>
      <media:group><yt:videoId>vid1</yt:videoId></media:group>
    </item>
    <item>
      <title>007 &lt;keep&gt;</title>
      <link>https://example.com/w/2</link>
      <guid>007</guid>
    </item>
  </channel>
</rss>`;

const ATOM = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Feed</title>
  <link rel="self" href="https://example.com/atom"/>
  <link rel="alternate" href="https://example.com/"/>
  <entry>
    <id>tag:youtube,2019:video:abc</id>
    <title>Entry 1</title>
    <link rel="alternate" href="https://example.com/e1"/>
    <link rel="enclosure" href="https://example.com/e1.mp4"/>
    <published>2026-01-02T03:04:05Z</published>
    <author><name>Author A</name></author>
    <summary>Sum &amp; summary</summary>
  </entry>
  <entry>
    <id>tag:youtube,2019:video:def</id>
    <title>Entry 2</title>
    <link href="https://example.com/e2"/>
    <updated>2026-02-03T04:05:06Z</updated>
  </entry>
</feed>`;

describe("parseFeed RSS", () => {
  const feed = parseFeed(RSS);

  test("实体与 CDATA 正确解码", () => {
    expect(feed.channel.title).toBe("Channel & Title");
    expect(feed.channel.description).toBe("Desc with <b>html</b>");
  });

  test("数字形 guid 保持字符串（parseTagValue:false）", () => {
    expect(feed.channel.item[0]?.guid).toBe("0123");
    expect(feed.channel.item[1]?.guid).toBe("007");
    expect(feed.channel.item[1]?.title).toBe("007 <keep>");
  });

  test("命名空间扩展节点进 raw", () => {
    const group = (feed.channel.item[0]?.raw as Record<string, unknown>)[
      "media:group"
    ] as Record<string, unknown>;
    expect(group["yt:videoId"]).toBe("vid1");
  });

  test("属性保留 @_ 前缀", () => {
    const guid = feed.channel.item[0]?.raw as Record<string, unknown>;
    expect(guid.guid).toMatchObject({ "@_isPermaLink": "false", "#text": "0123" });
  });
});

describe("parseFeed Atom", () => {
  const feed = parseFeed(ATOM);

  test("channel.link 取 rel=alternate 优先", () => {
    expect(feed.channel.link).toBe("https://example.com/");
  });

  test("entry link 同样 alternate 优先，无 rel 兜底", () => {
    expect(feed.channel.item[0]?.link).toBe("https://example.com/e1");
    expect(feed.channel.item[1]?.link).toBe("https://example.com/e2");
  });

  test("id→guid、author 对象取 name、published 缺省回退 updated", () => {
    expect(feed.channel.item[0]?.guid).toBe("tag:youtube,2019:video:abc");
    expect(feed.channel.item[0]?.author).toBe("Author A");
    expect(feed.channel.item[0]?.description).toBe("Sum & summary");
    expect(feed.channel.item[1]?.pubDate).toBe("2026-02-03T04:05:06Z");
  });

  test("多 entry 展开为数组", () => {
    expect(feed.channel.item).toHaveLength(2);
  });
});
