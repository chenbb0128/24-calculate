package auth

import (
	"time"

	jwtplatform "github.com/example/go-service/internal/platform/jwt"
)

func issueTokenPair(manager *jwtplatform.Manager, userID uint64, accessTTL time.Duration) (TokenResponse, string, error) {
	accessToken, _, err := manager.IssueAccessToken(userID)
	if err != nil {
		return TokenResponse{}, "", err
	}
	refreshToken, refreshClaims, err := manager.IssueRefreshToken(userID)
	if err != nil {
		return TokenResponse{}, "", err
	}
	return TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTTL / time.Second),
	}, refreshClaims.ID, nil
}
