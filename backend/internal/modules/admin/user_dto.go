package admin

import "time"

const (
	defaultAdminUsersPageSize = 20
	maxAdminUsersPageSize     = 100
)

type ListAdminUsersInput struct {
	Query    string
	Status   string
	Platform string
	Page     int
	PageSize int
}

type AdminUserRecord struct {
	ID                       uint64
	Username                 string
	Nickname                 string
	Avatar                   string
	Platform                 string
	Status                   uint8
	NicknameModerationStatus string
	AvatarModerationStatus   string
	ModerationUpdatedAt      *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type AdminUserStats struct {
	Total    int64 `json:"total"`
	NewToday int64 `json:"new_today"`
	Active   int64 `json:"active"`
	Disabled int64 `json:"disabled"`
}

type AdminUserListItem struct {
	ID                       uint64 `json:"id"`
	Username                 string `json:"username"`
	Nickname                 string `json:"nickname"`
	Avatar                   string `json:"avatar"`
	Platform                 string `json:"platform"`
	Status                   uint8  `json:"status"`
	NicknameModerationStatus string `json:"nickname_moderation_status"`
	AvatarModerationStatus   string `json:"avatar_moderation_status"`
	CreatedAt                string `json:"created_at"`
	UpdatedAt                string `json:"updated_at"`
}

type AdminUserListResponse struct {
	Items    []AdminUserListItem `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Total    int64               `json:"total"`
	Stats    AdminUserStats      `json:"stats"`
}

type AdminUserDetailResponse struct {
	ID                       uint64  `json:"id"`
	Username                 string  `json:"username"`
	Nickname                 string  `json:"nickname"`
	Avatar                   string  `json:"avatar"`
	Platform                 string  `json:"platform"`
	Status                   uint8   `json:"status"`
	NicknameModerationStatus string  `json:"nickname_moderation_status"`
	AvatarModerationStatus   string  `json:"avatar_moderation_status"`
	ModerationUpdatedAt      *string `json:"moderation_updated_at"`
	CreatedAt                string  `json:"created_at"`
	UpdatedAt                string  `json:"updated_at"`
}

type AdminUserStatusResponse struct {
	ID        uint64 `json:"id"`
	Status    uint8  `json:"status"`
	UpdatedAt string `json:"updated_at"`
}
