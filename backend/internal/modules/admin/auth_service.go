package admin

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/go-service/internal/modules/auth"
	"github.com/example/go-service/internal/modules/user"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
	db "github.com/example/go-service/internal/store/sqlc"
)

type AdminAuthAccountStore interface {
	GetAdminAccountByID(context.Context, uint64) (db.AdminAccount, error)
	GetAdminAccountByUsername(context.Context, string) (db.AdminAccount, error)
	TouchAdminLastLogin(context.Context, db.TouchAdminLastLoginParams) error
}

type AdminAuthTokenStore interface {
	SaveRefreshToken(context.Context, string, uint64, time.Duration) error
	ConsumeRefreshToken(context.Context, string) (uint64, error)
	RevokeRefreshToken(context.Context, string) error
	AllowLogin(context.Context, string, int64, time.Duration) (bool, error)
}

type AdminAuthService struct {
	accounts    AdminAuthAccountStore
	tokens      AdminAuthTokenStore
	jwt         *jwtplatform.Manager
	accessTTL   time.Duration
	refreshTTL  time.Duration
	loginLimit  int64
	loginWindow time.Duration
	logger      *slog.Logger
}

func NewAdminAuthService(accounts AdminAuthAccountStore, tokens AdminAuthTokenStore, manager *jwtplatform.Manager, accessTTL, refreshTTL time.Duration) *AdminAuthService {
	return &AdminAuthService{
		accounts: accounts, tokens: tokens, jwt: manager,
		accessTTL: accessTTL, refreshTTL: refreshTTL,
		loginLimit: 5, loginWindow: time.Minute,
		logger: slog.Default(),
	}
}

func (s *AdminAuthService) SetLogger(logger *slog.Logger) {
	if s != nil && logger != nil {
		s.logger = logger
	}
}

func (s *AdminAuthService) Login(ctx context.Context, input LoginInput, ip string) (auth.TokenResponse, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" || input.Password == "" {
		return auth.TokenResponse{}, auth.InvalidCredentials(nil)
	}
	allowed, err := s.tokens.AllowLogin(ctx, ip, s.loginLimit, s.loginWindow)
	if err != nil {
		return auth.TokenResponse{}, err
	}
	if !allowed {
		return auth.TokenResponse{}, auth.TooManyAttempts(nil)
	}
	account, err := s.accounts.GetAdminAccountByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.TokenResponse{}, auth.InvalidCredentials(err)
	}
	if err != nil {
		return auth.TokenResponse{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(input.Password)) != nil {
		return auth.TokenResponse{}, auth.InvalidCredentials(nil)
	}
	if account.Status == 0 {
		return auth.TokenResponse{}, user.Disabled(nil)
	}
	now := time.Now().UTC()
	if err := s.accounts.TouchAdminLastLogin(ctx, db.TouchAdminLastLoginParams{
		LastLoginAt: sql.NullTime{Time: now, Valid: true}, UpdatedAt: now, ID: account.ID,
	}); err != nil {
		return auth.TokenResponse{}, err
	}
	return s.issueAndSave(ctx, account.ID)
}

func (s *AdminAuthService) Refresh(ctx context.Context, input auth.RefreshInput) (auth.TokenResponse, error) {
	claims, err := s.jwt.ParseRefreshToken(strings.TrimSpace(input.RefreshToken))
	if errors.Is(err, jwtplatform.ErrTokenExpired) {
		return auth.TokenResponse{}, auth.ExpiredToken(err)
	}
	if err != nil || claims.Role != jwtplatform.RoleAdmin {
		return auth.TokenResponse{}, auth.InvalidToken(err)
	}
	adminID, err := s.tokens.ConsumeRefreshToken(ctx, claims.ID)
	if err != nil || adminID != claims.UserID {
		return auth.TokenResponse{}, auth.InvalidToken(err)
	}
	account, err := s.accounts.GetAdminAccountByID(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.TokenResponse{}, auth.InvalidToken(err)
	}
	if err != nil {
		return auth.TokenResponse{}, err
	}
	if account.Status == 0 {
		return auth.TokenResponse{}, user.Disabled(nil)
	}
	return s.issueAndSave(ctx, account.ID)
}

func (s *AdminAuthService) Logout(ctx context.Context, input auth.LogoutInput) error {
	claims, err := s.jwt.ParseRefreshToken(strings.TrimSpace(input.RefreshToken))
	if errors.Is(err, jwtplatform.ErrTokenExpired) {
		return auth.ExpiredToken(err)
	}
	if err != nil || claims.Role != jwtplatform.RoleAdmin {
		return auth.InvalidToken(err)
	}
	return s.tokens.RevokeRefreshToken(ctx, claims.ID)
}

func (s *AdminAuthService) issueAndSave(ctx context.Context, adminID uint64) (auth.TokenResponse, error) {
	accessToken, _, err := s.jwt.IssueAccessTokenWithRole(adminID, jwtplatform.RoleAdmin)
	if err != nil {
		return auth.TokenResponse{}, err
	}
	refreshToken, refreshClaims, err := s.jwt.IssueRefreshTokenWithRole(adminID, jwtplatform.RoleAdmin)
	if err != nil {
		return auth.TokenResponse{}, err
	}
	if err := s.tokens.SaveRefreshToken(ctx, refreshClaims.ID, adminID, s.refreshTTL); err != nil {
		return auth.TokenResponse{}, err
	}
	return auth.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: "Bearer", ExpiresIn: int64(s.accessTTL / time.Second)}, nil
}
