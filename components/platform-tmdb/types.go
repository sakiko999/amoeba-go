// TMDB 上游响应结构(字段全可选:缺字段时映射层兜底)。
// 搜索响应 tv/movie 共用一份结构(name 系 tv 字段、title 系 movie 字段)。

package tmdb

type person struct {
	Name        string  `json:"name"`
	Character   string  `json:"character"`
	Order       *int    `json:"order"`
	ProfilePath *string `json:"profile_path"`
}

type tvDetail struct {
	ID               *int         `json:"id"`
	Name             *string      `json:"name"`
	OriginalName     *string      `json:"original_name"`
	Overview         *string      `json:"overview"`
	FirstAirDate     *string      `json:"first_air_date"`
	Genres           []nameOnly   `json:"genres"`
	ProductionCos    []nameOnly   `json:"production_companies"`
	Status           *string      `json:"status"`
	VoteAverage      *float64     `json:"vote_average"`
	VoteCount        *int         `json:"vote_count"`
	PosterPath       *string      `json:"poster_path"`
	NumberOfSeasons  *int         `json:"number_of_seasons"`
	NumberOfEpisodes *int         `json:"number_of_episodes"`
	Seasons          []season     `json:"seasons"`
	Credits          *credits     `json:"credits"`
	ExternalIDs      *externalIDs `json:"external_ids"`
}

type movieDetail struct {
	ID            *int         `json:"id"`
	Title         *string      `json:"title"`
	OriginalTitle *string      `json:"original_title"`
	Overview      *string      `json:"overview"`
	ReleaseDate   *string      `json:"release_date"`
	Genres        []nameOnly   `json:"genres"`
	ProductionCos []nameOnly   `json:"production_companies"`
	Status        *string      `json:"status"`
	VoteAverage   *float64     `json:"vote_average"`
	VoteCount     *int         `json:"vote_count"`
	PosterPath    *string      `json:"poster_path"`
	Runtime       *int         `json:"runtime"`
	Credits       *credits     `json:"credits"`
	ExternalIDs   *externalIDs `json:"external_ids"`
}

type searchResponse struct {
	Results []searchItem `json:"results"`
}

// searchItem tv(name/first_air_date)与 movie(title/release_date)字段并存。
type searchItem struct {
	ID            *int     `json:"id"`
	Name          *string  `json:"name"`
	OriginalName  *string  `json:"original_name"`
	FirstAirDate  *string  `json:"first_air_date"`
	Title         *string  `json:"title"`
	OriginalTitle *string  `json:"original_title"`
	ReleaseDate   *string  `json:"release_date"`
	Overview      *string  `json:"overview"`
	PosterPath    *string  `json:"poster_path"`
	VoteAverage   *float64 `json:"vote_average"`
	VoteCount     *int     `json:"vote_count"`
}

type seasonDetail struct {
	Episodes []seasonEpisode `json:"episodes"`
}

type seasonEpisode struct {
	EpisodeNumber *int     `json:"episode_number"`
	Name          *string  `json:"name"`
	Overview      *string  `json:"overview"`
	AirDate       *string  `json:"air_date"`
	Runtime       *int     `json:"runtime"`
	StillPath     *string  `json:"still_path"`
	VoteAverage   *float64 `json:"vote_average"`
}

type nameOnly struct {
	Name string `json:"name"`
}

type credits struct {
	Cast []person `json:"cast"`
}

type externalIDs struct {
	ImdbID *string `json:"imdb_id"`
}

type season struct {
	SeasonNumber *int `json:"season_number"`
	EpisodeCount *int `json:"episode_count"`
}
