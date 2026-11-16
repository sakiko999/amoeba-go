// 豆瓣 rexxar v2 上游响应结构(/movie/{id} 请求由 301 落到 /v2/tv/{id},两者共用);
// 字段全部可选,映射层兜底。

package douban

type doubanPerson struct {
	Name *string `json:"name"`
}

type doubanRating struct {
	Value *float64 `json:"value"`
	Count *int     `json:"count"`
}

type doubanPic struct {
	Large *string `json:"large"`
}

type doubanSubject struct {
	ID            *string        `json:"id"`
	Title         *string        `json:"title"`
	OriginalTitle *string        `json:"original_title"`
	Intro         *string        `json:"intro"`
	ReleaseDate   *string        `json:"release_date"`
	Rating        *doubanRating  `json:"rating"`
	Genres        []string       `json:"genres"`
	CoverURL      *string        `json:"cover_url"`
	Pic           *doubanPic     `json:"pic"`
	Actors        []doubanPerson `json:"actors"`
	Directors     []doubanPerson `json:"directors"`
	EpisodesCount *int           `json:"episodes_count"`
	IsTv          *bool          `json:"is_tv"`
}
