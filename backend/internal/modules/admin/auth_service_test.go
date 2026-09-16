package admin

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/go-service/internal/config"
	"github.com/example/go-service/internal/modules/auth"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
	db "github.com/example/go-service/internal/store/sqlc"
)

func TestAdminAuthLoginIssuesAdminTokens(t *testing.T) {
	service, manager, tokens := newAdminAuthService(t, activeAdmin(t, 7, "correct-password"))

	result, err := service.Login(context.Background(), LoginInput{Username: " admin ", Password: "correct-password"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	claims, err := manager.ParseAccessToken(result.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.UserID != 7 || claims.Role != jwtplatform.RoleAdmin || tokens.savedJTI == "" {
		t.Fatalf("admin token state was not issued correctly")
	}
}

func TestAdminAuthLoginUsesGenericInvalidCredentials(t *testing.T) {
	service, _, _ := newAdminAuthService(t, activeAdmin(t, 7, "correct-password"))
	wrongPasswordErr := loginAdmin(t, service, "admin", "wrong-password")
	missingUsernameErr := loginAdmin(t, service, "missing", "correct-password")

	if wrongPasswordErr == nil || missingUsernameErr == nil || wrongPasswordErr.Error() != missingUsernameErr.Error() {
		t.Fatalf("credential errors must use the same generic message")
	}
}

func TestAdminAuthRejectsDisabledAdminOnLoginAndRefresh(t *testing.T) {
	disabled := activeAdmin(t, 7, "correct-password")
	disabled.Status = 0
	service, manager, tokens := newAdminAuthService(t, disabled)
	if err := loginAdmin(t, service, "admin", "correct-password"); err == nil {
		t.Fatal("Login() error = nil for disabled admin")
	}
	refreshToken, refreshClaims, err := manager.IssueRefreshTokenWithRole(disabled.ID, jwtplatform.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	tokens.refreshes[refreshClaims.ID] = disabled.ID
	if _, err := service.Refresh(context.Background(), auth.RefreshInput{RefreshToken: refreshToken}); err == nil {
		t.Fatal("Refresh() error = nil for disabled admin")
	}
}

func TestAdminAuthRefreshConsumesTokenAndRequiresMatchingID(t *testing.T) {
	service, manager, tokens := newAdminAuthService(t, activeAdmin(t, 7, "correct-password"))
	refreshToken, claims, err := manager.IssueRefreshTokenWithRole(7, jwtplatform.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	tokens.refreshes[claims.ID] = 7
	if _, err := service.Refresh(context.Background(), auth.RefreshInput{RefreshToken: refreshToken}); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if _, err := service.Refresh(context.Background(), auth.RefreshInput{RefreshToken: refreshToken}); err == nil {
		t.Fatal("Refresh() error = nil after token was consumed")
	}

	mismatchedToken, mismatchedClaims, err := manager.IssueRefreshTokenWithRole(7, jwtplatform.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	tokens.refreshes[mismatchedClaims.ID] = 8
	if _, err := service.Refresh(context.Background(), auth.RefreshInput{RefreshToken: mismatchedToken}); err == nil {
		t.Fatal("Refresh() error = nil when stored ID does not match claims")
	}
}

func TestAdminAuthLogoutRevokesRefreshJTI(t *testing.T) {
	service, manager, tokens := newAdminAuthService(t, activeAdmin(t, 7, "correct-password"))
	refreshToken, claims, err := manager.IssueRefreshTokenWithRole(7, jwtplatform.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), auth.LogoutInput{RefreshToken: refreshToken}); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if tokens.revokedJTI != claims.ID {
		t.Fatal("Logout() did not revoke the submitted refresh token JTI")
	}
}

func loginAdmin(t *testing.T, service *AdminAuthService, username, password string) error {
	t.Helper()
	_, err := service.Login(context.Background(), LoginInput{Username: username, Password: password}, "127.0.0.1")
	return err
}

func activeAdmin(t *testing.T, id uint64, password string) db.AdminAccount {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return db.AdminAccount{ID: id, Username: "admin", PasswordHash: string(hash), Status: 1}
}

func newAdminAuthService(t *testing.T, account db.AdminAccount) (*AdminAuthService, *jwtplatform.Manager, *fakeAdminAuthTokenStore) {
	t.Helper()
	manager, err := jwtplatform.NewManager(config.JWTConfig{
		Secret: "01234567890123456789012345678901", Algorithm: "HS256", Issuer: "go-service", AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	tokens := &fakeAdminAuthTokenStore{refreshes: make(map[string]uint64)}
	return NewAdminAuthService(&fakeAdminAuthStore{account: account}, tokens, manager, time.Minute, time.Hour), manager, tokens
}

type fakeAdminAuthStore struct{ account db.AdminAccount }

func (s *fakeAdminAuthStore) GetAdminAccountByID(_ context.Context, id uint64) (db.AdminAccount, error) {
	if s.account.ID != id {
		return db.AdminAccount{}, sql.ErrNoRows
	}
	return s.account, nil
}

func (s *fakeAdminAuthStore) GetAdminAccountByUsername(_ context.Context, username string) (db.AdminAccount, error) {
	if s.account.Username != username {
		return db.AdminAccount{}, sql.ErrNoRows
	}
	return s.account, nil
}

func (s *fakeAdminAuthStore) TouchAdminLastLogin(context.Context, db.TouchAdminLastLoginParams) error {
	return nil
}

type fakeAdminAuthTokenStore struct {
	refreshes  map[string]uint64
	savedJTI   string
	revokedJTI string
}

func (s *fakeAdminAuthTokenStore) SaveRefreshToken(_ context.Context, jti string, id uint64, _ time.Duration) error {
	s.refreshes[jti] = id
	s.savedJTI = jti
	return nil
}

func (s *fakeAdminAuthTokenStore) ConsumeRefreshToken(_ context.Context, jti string) (uint64, error) {
	id, ok := s.refreshes[jti]
	if !ok {
		return 0, errors.New("refresh token not found")
	}
	delete(s.refreshes, jti)
	return id, nil
}

func (s *fakeAdminAuthTokenStore) RevokeRefreshToken(_ context.Context, jti string) error {
	s.revokedJTI = jti
	delete(s.refreshes, jti)
	return nil
}

func (s *fakeAdminAuthTokenStore) AllowLogin(context.Context, string, int64, time.Duration) (bool, error) {
	return true, nil
}
