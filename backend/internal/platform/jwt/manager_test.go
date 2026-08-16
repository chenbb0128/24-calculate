package jwt

import (
	"errors"
	"testing"
	"time"

	"github.com/example/go-service/internal/config"
)

func TestManagerIssuesAndParsesAccessToken(t *testing.T) {
	manager, err := NewManager(config.JWTConfig{
		Secret:     "01234567890123456789012345678901",
		Algorithm:  "HS256",
		Issuer:     "go-service",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	token, issued, err := manager.IssueAccessToken(42)
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}
	parsed, err := manager.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if parsed.UserID != 42 || parsed.ID != issued.ID {
		t.Fatalf("claims = %+v, want user 42 and matching jti", parsed)
	}
}

func TestManagerRejectsWrongTokenType(t *testing.T) {
	manager, err := NewManager(config.JWTConfig{
		Secret:     "01234567890123456789012345678901",
		Algorithm:  "HS256",
		Issuer:     "go-service",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	token, _, err := manager.IssueRefreshToken(42)
	if err != nil {
		t.Fatalf("IssueRefreshToken() error = %v", err)
	}
	if _, err := manager.ParseAccessToken(token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("error = %v, want ErrTokenInvalid", err)
	}
}
