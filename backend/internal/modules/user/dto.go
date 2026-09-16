package user

type ProfileResponse struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Status    uint8  `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	NicknameModerationStatus string `json:"-"`
	AvatarModerationStatus   string `json:"-"`
}

type UpdateProfileInput struct {
	Nickname *string `json:"nickname"`
	Avatar   *string `json:"avatar"`
}

type AvatarUploadResponse struct {
	AvatarURL        string          `json:"avatar_url"`
	AvatarKey        string          `json:"avatar_key"`
	Width            int             `json:"width"`
	Height           int             `json:"height"`
	Format           string          `json:"format"`
	ModerationStatus string          `json:"moderation_status,omitempty"`
	Profile          ProfileResponse `json:"profile"`
}
