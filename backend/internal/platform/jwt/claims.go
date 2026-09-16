package jwt

import "github.com/golang-jwt/jwt/v5"

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
	RoleUser         = "user"
	RoleAdmin        = "admin"
)

type Claims struct {
	UserID    uint64 `json:"user_id"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}
