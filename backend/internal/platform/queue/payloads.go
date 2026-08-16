package queue

type UserWelcomePayload struct {
	Version int    `json:"version"`
	UserID  uint64 `json:"user_id"`
}
