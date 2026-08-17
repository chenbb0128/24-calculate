package player

type LeaderboardEntry struct {
	Rank     int    `json:"rank"`
	UserID   uint64 `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Score    int    `json:"score"`
	IsMe     bool   `json:"is_me"`
	Anomaly  bool   `json:"anomaly"`
}

type LeaderboardResponse struct {
	Mode     string             `json:"mode"`
	Scope    string             `json:"scope"`
	DateKey  string             `json:"date_key,omitempty"`
	SeasonID string             `json:"season_id,omitempty"`
	MyUserID uint64             `json:"my_user_id"`
	Entries  []LeaderboardEntry `json:"entries"`
	MyRank   int                `json:"my_rank"`
	MyScore  int                `json:"my_score"`
	Period   string             `json:"period"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int                `json:"total"`
}

type LeaderboardQuery struct {
	Scope    string
	Period   string
	SeasonID string
	Page     int
	PageSize int
}
