// twitch GraphQL 上游响应结构(字段全可选:缺失/null 由映射层兜底)。

package twitch

type gqlGameName struct {
	Name *string `json:"name"`
}

type gqlGameSlug struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}

type gqlStreamInfo struct {
	ID           *string      `json:"id"`
	Title        *string      `json:"title"`
	ViewersCount *int         `json:"viewersCount"`
	CreatedAt    *string      `json:"createdAt"`
	Type         *string      `json:"type"`
	Game         *gqlGameName `json:"game"`
}

type gqlUser struct {
	Login       *string        `json:"login"`
	DisplayName *string        `json:"displayName"`
	Description *string        `json:"description"`
	Stream      *gqlStreamInfo `json:"stream"`
}

type gqlBroadcaster struct {
	Login       *string `json:"login"`
	DisplayName *string `json:"displayName"`
}

type gqlStreamNode struct {
	ID              *string         `json:"id"`
	Title           *string         `json:"title"`
	ViewersCount    *int            `json:"viewersCount"`
	CreatedAt       *string         `json:"createdAt"`
	Broadcaster     *gqlBroadcaster `json:"broadcaster"`
	Game            *gqlGameSlug    `json:"game"`
	PreviewImageURL *string         `json:"previewImageURL"`
}

type gqlStreamEdge struct {
	Cursor *string        `json:"cursor"`
	Node   *gqlStreamNode `json:"node"`
}

type gqlPageInfo struct {
	HasNextPage *bool   `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}

type gqlStreamConnection struct {
	Edges    []gqlStreamEdge `json:"edges"`
	PageInfo *gqlPageInfo    `json:"pageInfo"`
}

type gqlCategoryNode struct {
	ID           *string `json:"id"`
	Name         *string `json:"name"`
	Slug         *string `json:"slug"`
	ViewersCount *int    `json:"viewersCount"`
	BoxArtURL    *string `json:"boxArtURL"`
}

type gqlCategoryEdge struct {
	Node *gqlCategoryNode `json:"node"`
}

type gqlCategoryConnection struct {
	Edges []gqlCategoryEdge `json:"edges"`
}

type gqlChannelHit struct {
	Login       *string `json:"login"`
	DisplayName *string `json:"displayName"`
	Description *string `json:"description"`
	Followers   *struct {
		TotalCount *int `json:"totalCount"`
	} `json:"followers"`
}

type gqlPlaybackToken struct {
	Signature *string `json:"signature"`
	Value     *string `json:"value"`
}

type gqlVideoNode struct {
	ID                  *string         `json:"id"`
	Title               *string         `json:"title"`
	LengthSeconds       *int            `json:"lengthSeconds"`
	CreatedAt           *string         `json:"createdAt"`
	PublishedAt         *string         `json:"publishedAt"`
	ViewCount           *int            `json:"viewCount"`
	Status              *string         `json:"status"`
	BroadcastType       *string         `json:"broadcastType"`
	PreviewThumbnailURL *string         `json:"previewThumbnailURL"`
	Owner               *gqlBroadcaster `json:"owner"`
	Game                *gqlGameName    `json:"game"`
}

type gqlVideoEdge struct {
	Cursor *string       `json:"cursor"`
	Node   *gqlVideoNode `json:"node"`
}

type gqlVideoConnection struct {
	TotalCount *int             `json:"totalCount"`
	Edges      []gqlVideoEdge   `json:"edges"`
	PageInfo   *gqlPageInfo     `json:"pageInfo"`
}
