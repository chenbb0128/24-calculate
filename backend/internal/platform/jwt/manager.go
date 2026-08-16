package jwt

import (
	"errors"
	"fmt"
	"time"

	goJWT "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/example/go-service/internal/config"
)

var (
	ErrTokenInvalid = errors.New("token is invalid")
	ErrTokenExpired = errors.New("token is expired")
)

type Manager struct {
	secret     []byte
	algorithm  string
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(cfg config.JWTConfig) (*Manager, error) {
	if len([]byte(cfg.Secret)) < 32 {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	if cfg.Algorithm != "HS256" {
		return nil, fmt.Errorf("unsupported jwt algorithm: %s", cfg.Algorithm)
	}
	return &Manager{
		secret:     []byte(cfg.Secret),
		algorithm:  cfg.Algorithm,
		issuer:     cfg.Issuer,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
	}, nil
}

func (m *Manager) IssueAccessToken(userID uint64) (string, *Claims, error) {
	return m.issue(userID, TokenTypeAccess, m.accessTTL)
}

func (m *Manager) IssueRefreshToken(userID uint64) (string, *Claims, error) {
	return m.issue(userID, TokenTypeRefresh, m.refreshTTL)
}

func (m *Manager) issue(userID uint64, tokenType string, ttl time.Duration) (string, *Claims, error) {
	if m == nil || len(m.secret) == 0 {
		return "", nil, errors.New("jwt manager is not initialized")
	}
	now := time.Now().UTC()
	claims := &Claims{
		UserID:    userID,
		TokenType: tokenType,
		RegisteredClaims: goJWT.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("user:%d", userID),
			ID:        uuid.NewString(),
			IssuedAt:  goJWT.NewNumericDate(now),
			NotBefore: goJWT.NewNumericDate(now),
			ExpiresAt: goJWT.NewNumericDate(now.Add(ttl)),
		},
	}
	token := goJWT.NewWithClaims(goJWT.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", nil, err
	}
	return signed, claims, nil
}

func (m *Manager) ParseAccessToken(value string) (*Claims, error) {
	return m.parse(value, TokenTypeAccess)
}

func (m *Manager) ParseRefreshToken(value string) (*Claims, error) {
	return m.parse(value, TokenTypeRefresh)
}

func (m *Manager) parse(value, expectedType string) (*Claims, error) {
	if m == nil || value == "" {
		return nil, ErrTokenInvalid
	}
	claims := new(Claims)
	token, err := goJWT.ParseWithClaims(
		value,
		claims,
		func(token *goJWT.Token) (any, error) {
			if token.Method != goJWT.SigningMethodHS256 {
				return nil, ErrTokenInvalid
			}
			return m.secret, nil
		},
		goJWT.WithValidMethods([]string{m.algorithm}),
		goJWT.WithIssuer(m.issuer),
	)
	if err != nil {
		if errors.Is(err, goJWT.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if token == nil || !token.Valid || claims.TokenType != expectedType || claims.UserID == 0 || claims.ID == "" {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
