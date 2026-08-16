package player

type SubmitLeaderboardInput struct {
	IdempotencyKey string         `json:"idempotency_key"`
	Score          int            `json:"score"`
	Questions      int            `json:"questions"`
	ElapsedMS      int            `json:"elapsed_ms"`
	RoomID         string         `json:"room_id"`
	Outcome        string         `json:"outcome"`
	Metadata       map[string]any `json:"metadata"`
}

type SubmitLeaderboardResponse struct {
	Mode                string `json:"mode"`
	IdempotencyKey      string `json:"idempotency_key"`
	Score               int    `json:"score"`
	Questions           int    `json:"questions"`
	ElapsedMS           int    `json:"elapsed_ms"`
	IdempotencyReplayed bool   `json:"idempotency_replayed"`
}
