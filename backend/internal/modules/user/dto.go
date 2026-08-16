package user

type ProfileResponse struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Status    uint8  `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type UpdateProfileInput struct {
	Nickname *string `json:"nickname"`
	Avatar   *string `json:"avatar"`
}
