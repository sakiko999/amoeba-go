// parseFeed fixture 测试(移植 tests/feed.test.ts):锁住 RSS/Atom 解析语义——
// 实体/CDATA 解码、数字形字符串保持、命名空间扩展节点与属性可读。

package youtube

import "testing"

const rssFixture = `<?xml version="1.0" encoding="UTF-8"?>
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
</rss>`

const atomFixture = `<?xml version="1.0" encoding="utf-8"?>
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
</feed>`

func derefPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestParseFeedRSSEntitiesCDATA(t *testing.T) {
	feed, err := parseFeed(rssFixture)
	if err != nil {
		t.Fatal(err)
	}
	if derefPtr(feed.Channel.Title) != "Channel & Title" {
		t.Errorf("title = %q", derefPtr(feed.Channel.Title))
	}
	if derefPtr(feed.Channel.Description) != "Desc with <b>html</b>" {
		t.Errorf("description = %q", derefPtr(feed.Channel.Description))
	}
}

func TestParseFeedRSSNumericGUIDStaysString(t *testing.T) {
	feed, err := parseFeed(rssFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Channel.Items) != 2 {
		t.Fatalf("items = %d", len(feed.Channel.Items))
	}
	if derefPtr(feed.Channel.Items[0].GUID) != "0123" {
		t.Errorf("guid[0] = %q", derefPtr(feed.Channel.Items[0].GUID))
	}
	if derefPtr(feed.Channel.Items[1].GUID) != "007" {
		t.Errorf("guid[1] = %q", derefPtr(feed.Channel.Items[1].GUID))
	}
	if derefPtr(feed.Channel.Items[1].Title) != "007 <keep>" {
		t.Errorf("title[1] = %q", derefPtr(feed.Channel.Items[1].Title))
	}
}

func TestParseFeedRSSNamespaceRaw(t *testing.T) {
	feed, err := parseFeed(rssFixture)
	if err != nil {
		t.Fatal(err)
	}
	group := feed.Channel.Items[0].Raw.child("group")
	if group == nil {
		t.Fatal("media:group 未进入 raw")
	}
	if text := derefPtr(group.child("videoId").textValue()); text != "vid1" {
		t.Errorf("yt:videoId = %q", text)
	}
}

func TestParseFeedRSSAttributes(t *testing.T) {
	feed, err := parseFeed(rssFixture)
	if err != nil {
		t.Fatal(err)
	}
	guid := feed.Channel.Items[0].Raw.child("guid")
	if guid == nil {
		t.Fatal("guid 未进入 raw")
	}
	// fxp 的 guid 对象等价:{"@_isPermaLink":"false","#text":"0123"}
	if value, ok := guid.attr("isPermaLink"); !ok || value != "false" {
		t.Errorf("isPermaLink = %q ok=%v", value, ok)
	}
	if derefPtr(guid.textValue()) != "0123" {
		t.Errorf("guid #text = %q", derefPtr(guid.textValue()))
	}
}

func TestParseFeedAtomChannelLinkAlternateFirst(t *testing.T) {
	feed, err := parseFeed(atomFixture)
	if err != nil {
		t.Fatal(err)
	}
	if feed.Channel.Link != "https://example.com/" {
		t.Errorf("channel.link = %q", feed.Channel.Link)
	}
}

func TestParseFeedAtomEntryLinks(t *testing.T) {
	feed, err := parseFeed(atomFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Channel.Items) != 2 {
		t.Fatalf("items = %d", len(feed.Channel.Items))
	}
	if feed.Channel.Items[0].Link != "https://example.com/e1" {
		t.Errorf("entry[0].link = %q", feed.Channel.Items[0].Link)
	}
	if feed.Channel.Items[1].Link != "https://example.com/e2" {
		t.Errorf("entry[1].link = %q", feed.Channel.Items[1].Link)
	}
}

func TestParseFeedAtomFields(t *testing.T) {
	feed, err := parseFeed(atomFixture)
	if err != nil {
		t.Fatal(err)
	}
	first := feed.Channel.Items[0]
	if derefPtr(first.GUID) != "tag:youtube,2019:video:abc" {
		t.Errorf("entry[0].guid = %q", derefPtr(first.GUID))
	}
	if first.Author != "Author A" {
		t.Errorf("entry[0].author = %q", first.Author)
	}
	if derefPtr(first.Description) != "Sum & summary" {
		t.Errorf("entry[0].description = %q", derefPtr(first.Description))
	}
	if derefPtr(feed.Channel.Items[1].PubDate) != "2026-02-03T04:05:06Z" {
		t.Errorf("entry[1].pubDate = %q", derefPtr(feed.Channel.Items[1].PubDate))
	}
}

func TestParseFeedAtomMultipleEntries(t *testing.T) {
	feed, err := parseFeed(atomFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Channel.Items) != 2 {
		t.Errorf("entries = %d", len(feed.Channel.Items))
	}
}
