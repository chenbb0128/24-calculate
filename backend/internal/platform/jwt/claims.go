package jwt

import "github.com/golang-jwt/jwt/v5"

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	UserID    uint64 `json:"user_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}
