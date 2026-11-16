// B站动态上游结构与纯映射：动态 → 正文/图片/点赞/标题（转发拼被转内容）。
// 与渠道装配分离，便于单测直接喂 DynItem JSON。

package bili

import (
	"regexp"
	"strings"

	"mygo-host/platform"
)

type dynLike struct {
	Count *float64 `json:"count"`
}

type dynModuleAuthor struct {
	Mid   *float64 `json:"mid"`
	Name  *string  `json:"name"`
	Face  *string  `json:"face"`
	PubTs *float64 `json:"pub_ts"`
}

type dynOpusPic struct {
	URL    *string  `json:"url"`
	Width  *float64 `json:"width"`
	Height *float64 `json:"height"`
}

type dynOpus struct {
	Summary *struct {
		Text *string `json:"text"`
	} `json:"summary"`
	Pics    []dynOpusPic `json:"pics"`
	JumpURL *string      `json:"jump_url"`
}

type dynArchive struct {
	Title        *string `json:"title"`
	Cover        *string `json:"cover"`
	Desc         *string `json:"desc"`
	DurationText *string `json:"duration_text"`
}

type dynMajor struct {
	Type    *string     `json:"type"`
	Opus    *dynOpus    `json:"opus"`
	Archive *dynArchive `json:"archive"`
}

type dynDynamic struct {
	Desc *struct {
		Text *string `json:"text"`
	} `json:"desc"`
	Major *dynMajor `json:"major"`
}

type dynModule struct {
	ModuleAuthor  *dynModuleAuthor `json:"module_author"`
	ModuleDynamic *dynDynamic      `json:"module_dynamic"`
	ModuleStat    *struct {
		Like *dynLike `json:"like"`
	} `json:"module_stat"`
	ModuleInteraction *struct {
		Like *dynLike `json:"like"`
	} `json:"module_interaction"`
}

type dynItem struct {
	IDStr   *string    `json:"id_str"`
	Type    *string    `json:"type"`
	Modules *dynModule `json:"modules"`
	Orig    *dynItem   `json:"orig"`
}

type parsedDyn struct {
	content string
	images  []platform.SocialImage
	likes   int
	title   string
}

var cvRe = regexp.MustCompile(`cv\d+`)

// parseDynModule 单个动态模块 → 可渲染字段（按 major 结构分支，major.type 只是形态标签）。
func parseDynModule(d *dynModule) parsedDyn {
	parsed := parsedDyn{}
	if d == nil || d.ModuleDynamic == nil {
		parsed.content = "(无正文)"
		return parsed
	}
	dyn := d.ModuleDynamic
	content := ""
	if dyn.Desc != nil {
		content = deref(dyn.Desc.Text)
	}
	title := ""
	var opus *dynOpus
	var archive *dynArchive
	if dyn.Major != nil {
		opus, archive = dyn.Major.Opus, dyn.Major.Archive
	}
	if opus != nil {
		for _, p := range opus.Pics {
			if p.URL == nil || *p.URL == "" {
				continue
			}
			img := platform.SocialImage{URL: toHTTPS(*p.URL)}
			if jnumPtr(p.Width) != 0 {
				img.Width = platform.Int(jintPtr(p.Width))
			}
			if jnumPtr(p.Height) != 0 {
				img.Height = platform.Int(jintPtr(p.Height))
			}
			parsed.images = append(parsed.images, img)
		}
		if opus.Summary != nil && deref(opus.Summary.Text) != "" {
			content = strings.TrimSpace(content + "\n" + deref(opus.Summary.Text))
		}
		if opus.JumpURL != nil {
			title = cvRe.FindString(*opus.JumpURL)
		}
	}
	if archive != nil {
		title = deref(archive.Title)
		if cover := deref(archive.Cover); cover != "" {
			parsed.images = append(parsed.images, platform.SocialImage{URL: toHTTPS(cover)})
		}
		if content == "" {
			content = deref(archive.Desc)
		}
	}
	parsed.likes = dynLikeCount(d)
	parsed.content = content
	if parsed.content == "" {
		parsed.content = title
	}
	if parsed.content == "" {
		parsed.content = "(无正文)"
	}
	parsed.title = title
	return parsed
}

// dynLikeCount module_interaction.like.count ?? module_stat.like.count ?? 0。
func dynLikeCount(d *dynModule) int {
	if d == nil {
		return 0
	}
	if d.ModuleInteraction != nil && d.ModuleInteraction.Like != nil && d.ModuleInteraction.Like.Count != nil {
		return int(*d.ModuleInteraction.Like.Count)
	}
	if d.ModuleStat != nil && d.ModuleStat.Like != nil && d.ModuleStat.Like.Count != nil {
		return int(*d.ModuleStat.Like.Count)
	}
	return 0
}

// parseDynItem 动态 + 被转内容 → 完整社交字段（转发来源拼进正文，图沿用被转图）。
func parseDynItem(it *dynItem) parsedDyn {
	if it == nil {
		return parsedDyn{content: "(无正文)"}
	}
	parsed := parseDynModule(it.Modules)
	if it.Orig == nil || it.Orig.Modules == nil {
		return parsed
	}
	origin := parseDynModule(it.Orig.Modules)
	originName := ""
	if it.Orig.Modules.ModuleAuthor != nil {
		originName = deref(it.Orig.Modules.ModuleAuthor.Name)
	}
	parts := make([]string, 0, 3)
	if originName != "" {
		parts = append(parts, "//转发自: @"+originName+":")
	}
	if origin.title != "" {
		parts = append(parts, origin.title)
	}
	if origin.content != "" {
		parts = append(parts, origin.content)
	}
	if len(parts) > 0 {
		parsed.content = strings.TrimSpace(parsed.content + "\n" + strings.Join(parts, "\n"))
	}
	if len(parsed.images) == 0 {
		parsed.images = origin.images
	}
	return parsed
}

// toHTTPS http:// → https://（其余原样）。
func toHTTPS(u string) string {
	if strings.HasPrefix(u, "http://") {
		return "https://" + u[7:]
	}
	return u
}

// jsSlice JS String.slice(0, n) 语义（按 UTF-16 code unit 计数）。
func jsSlice(s string, n int) string {
	units := 0
	for i, r := range s {
		if units >= n {
			return s[:i]
		}
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
	}
	return s
}
