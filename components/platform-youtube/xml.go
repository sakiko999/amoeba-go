// 上游 RSS/Atom 解析(仅输入方向,本平台专属):等价 fast-xml-parser 的
// parseTagValue:false 语义——数字形文本保持字符串、属性保留、CDATA/实体解码。
// 上游 XML 归一成 ParsedFeed/ParsedItem 供渠道映射成 Item,不做 XML 序列化。
//
// 与 fxp 的对应关系:叶子节点文本 => 字符串;有属性时等价 "@_xxx" + "#text";
// 同名子节点重复出现 => 数组。解析树按元素本地名查找(命名空间前缀不参与匹配)。

package youtube

import (
	"encoding/xml"
	"io"
	"strings"
)

type xmlAttr struct {
	name  string
	value string
}

type xmlNode struct {
	name     string
	attrs    []xmlAttr
	text     string
	children []*xmlNode
}

func parseXML(data string) (*xmlNode, error) {
	decoder := xml.NewDecoder(strings.NewReader(data))
	decoder.Strict = false
	root := &xmlNode{name: "#document"}
	stack := []*xmlNode{root}
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			node := &xmlNode{name: t.Name.Local}
			for _, attr := range t.Attr {
				node.attrs = append(node.attrs, xmlAttr{name: attr.Name.Local, value: attr.Value})
			}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, node)
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			current := stack[len(stack)-1]
			current.text += string(t)
		}
	}
	trimTree(root)
	return root, nil
}

func trimTree(node *xmlNode) {
	node.text = strings.TrimSpace(node.text)
	for _, child := range node.children {
		trimTree(child)
	}
}

func (n *xmlNode) child(name string) *xmlNode {
	if n == nil {
		return nil
	}
	for _, item := range n.children {
		if item.name == name {
			return item
		}
	}
	return nil
}

func (n *xmlNode) childrenNamed(name string) []*xmlNode {
	if n == nil {
		return nil
	}
	var out []*xmlNode
	for _, item := range n.children {
		if item.name == name {
			out = append(out, item)
		}
	}
	return out
}

func (n *xmlNode) attr(name string) (string, bool) {
	if n == nil {
		return "", false
	}
	for _, item := range n.attrs {
		if item.name == name {
			return item.value, true
		}
	}
	return "", false
}

// textValue fxp asString 语义:元素文本;有属性且无文本 → 无值(nil)。
func (n *xmlNode) textValue() *string {
	if n == nil {
		return nil
	}
	if n.text != "" || len(n.attrs) == 0 {
		text := n.text
		return &text
	}
	return nil
}

// pickLink 单节点取链接:属性 href 优先(对象),否则文本(字符串)。
func pickLink(n *xmlNode) string {
	if n == nil {
		return ""
	}
	if href, ok := n.attr("href"); ok {
		return href
	}
	return n.text
}

// asLink Atom 允许一条 entry 多个 <link>:rel="alternate" 优先,其次无 rel 的,
// 最后将就第一个有值的。
func asLink(nodes []*xmlNode) string {
	if len(nodes) == 0 {
		return ""
	}
	if len(nodes) == 1 {
		return pickLink(nodes[0])
	}
	var alternate, noRel *xmlNode
	for _, node := range nodes {
		if rel, ok := node.attr("rel"); ok {
			if rel == "alternate" && alternate == nil {
				alternate = node
			}
		} else if noRel == nil {
			noRel = node
		}
	}
	if alternate != nil {
		if link := pickLink(alternate); link != "" {
			return link
		}
	}
	if noRel != nil {
		if link := pickLink(noRel); link != "" {
			return link
		}
	}
	for _, node := range nodes {
		if link := pickLink(node); link != "" {
			return link
		}
	}
	return ""
}

func asAuthor(n *xmlNode) string {
	if n == nil {
		return ""
	}
	if name := n.child("name"); name != nil {
		if len(name.attrs) == 0 && len(name.children) == 0 {
			return name.text
		}
		return ""
	}
	if value, ok := n.attr("name"); ok {
		return value
	}
	if len(n.attrs) == 0 && len(n.children) == 0 {
		return n.text
	}
	return ""
}

type parsedFeed struct {
	Channel parsedChannel
}

type parsedChannel struct {
	Title       *string
	Link        string
	Description *string
	Items       []parsedItem
	Raw         *xmlNode
}

type parsedItem struct {
	Title       *string
	Link        string
	Description *string
	Content     *string
	PubDate     *string
	Author      string
	GUID        *string
	Raw         *xmlNode
}

func parseFeed(data string) (parsedFeed, error) {
	doc, err := parseXML(data)
	if err != nil {
		return parsedFeed{}, err
	}
	root := doc.child("rss")
	if root == nil {
		root = doc.child("feed")
	}
	if root == nil {
		root = doc
	}
	if channel := root.child("channel"); channel != nil {
		return parsedFeed{Channel: parseRSSChannel(channel)}, nil
	}
	return parsedFeed{Channel: parseAtomFeed(root)}, nil
}

func parseRSSChannel(channel *xmlNode) parsedChannel {
	out := parsedChannel{
		Title:       channel.child("title").textValue(),
		Link:        asLink(channel.childrenNamed("link")),
		Description: channel.child("description").textValue(),
		Raw:         channel,
	}
	for _, item := range channel.childrenNamed("item") {
		out.Items = append(out.Items, parsedItem{
			Title:       item.child("title").textValue(),
			Link:        asLink(item.childrenNamed("link")),
			Description: item.child("description").textValue(),
			Content:     firstNodeValue(item.child("encoded"), item.child("content")),
			PubDate:     item.child("pubDate").textValue(),
			Author:      asAuthor(item.child("author")),
			GUID:        item.child("guid").textValue(),
			Raw:         item,
		})
	}
	return out
}

func parseAtomFeed(root *xmlNode) parsedChannel {
	out := parsedChannel{
		Title:       root.child("title").textValue(),
		Link:        asLink(root.childrenNamed("link")),
		Description: root.child("subtitle").textValue(),
		Raw:         root,
	}
	for _, entry := range root.childrenNamed("entry") {
		out.Items = append(out.Items, parsedItem{
			Title:       entry.child("title").textValue(),
			Link:        asLink(entry.childrenNamed("link")),
			Description: firstNodeValue(entry.child("summary"), entry.child("content")),
			Content:     firstNodeValue(entry.child("content"), entry.child("summary")),
			PubDate:     firstNodeValue(entry.child("published"), entry.child("updated")),
			Author:      asAuthor(entry.child("author")),
			GUID:        entry.child("id").textValue(),
			Raw:         entry,
		})
	}
	return out
}

// firstNodeValue TS `a ?? b` 语义:前者存在(含空文本)即取前者。
func firstNodeValue(nodes ...*xmlNode) *string {
	for _, node := range nodes {
		if node != nil {
			return node.textValue()
		}
	}
	return nil
}
