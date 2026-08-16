package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/modules/user"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
	wechatplatform "github.com/example/go-service/internal/platform/wechat"
	"github.com/example/go-service/internal/store"
	db "github.com/example/go-service/internal/store/sqlc"
)

type UserStore interface {
	GetUserByID(ctx context.Context, id uint64) (db.User, error)
	GetUserByUsername(ctx context.Context, username string) (db.User, error)
	CreateUserTx(ctx context.Context, arg db.CreateUserParams) (uint64, error)
}

type WeChatUserStore interface {
	UserStore
	GetUserByProviderSubject(ctx context.Context, provider, subject string) (db.User, error)
	CreateUserWithIdentityTx(ctx context.Context, userArg db.CreateUserParams, identityArg db.CreateUserIdentityParams) (uint64, error)
}

type TokenStore interface {
	SaveRefreshToken(ctx context.Context, jti string, userID uint64, ttl time.Duration) error
	ConsumeRefreshToken(ctx context.Context, jti string) (uint64, error)
	RevokeRefreshToken(ctx context.Context, jti string) error
	AllowLogin(ctx context.Context, ip string, limit int64, window time.Duration) (bool, error)
}

type WelcomeEnqueuer interface {
	EnqueueUserWelcome(ctx context.Context, userID uint64) error
}

type WeChatLoginClient interface {
	ExchangeCode(ctx context.Context, code string) (wechatplatform.LoginResult, error)
}

type Service struct {
	users       UserStore
	tokens      TokenStore
	jwt         *jwtplatform.Manager
	accessTTL   time.Duration
	refreshTTL  time.Duration
	loginLimit  int64
	loginWindow time.Duration
	welcome     WelcomeEnqueuer
	logger      *slog.Logger
	wechat      WeChatLoginClient
}

func NewService(users UserStore, tokens TokenStore, manager *jwtplatform.Manager, accessTTL, refreshTTL time.Duration, welcome WelcomeEnqueuer, logger *slog.Logger) *Service {
	return NewServiceWithWeChat(users, tokens, manager, accessTTL, refreshTTL, welcome, logger, nil)
}

func NewServiceWithWeChat(users UserStore, tokens TokenStore, manager *jwtplatform.Manager, accessTTL, refreshTTL time.Duration, welcome WelcomeEnqueuer, logger *slog.Logger, wechatClient WeChatLoginClient) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		users:       users,
		tokens:      tokens,
		jwt:         manager,
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
		loginLimit:  5,
		loginWindow: time.Minute,
		welcome:     welcome,
		logger:      logger,
		wechat:      wechatClient,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (UserResponse, error) {
	username, password, nickname, avatar, err := validateRegisterInput(input)
	if err != nil {
		return UserResponse{}, err
	}

	existing, err := s.users.GetUserByUsername(ctx, username)
	if err == nil && existing.ID != 0 {
		return UserResponse{}, user.UsernameExists(nil)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return UserResponse{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return UserResponse{}, err
	}
	now := time.Now().UTC()
	id, err := s.users.CreateUserTx(ctx, db.CreateUserParams{
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Avatar:       avatar,
		Status:       user.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		if store.IsDuplicateEntry(err) {
			return UserResponse{}, user.UsernameExists(err)
		}
		return UserResponse{}, err
	}
	if s.welcome != nil {
		if err := s.welcome.EnqueueUserWelcome(ctx, id); err != nil {
			s.logger.ErrorContext(ctx, "enqueue user welcome task failed", "user_id", id, "error", err)
		}
	}

	return UserResponse{
		ID:        id,
		Username:  username,
		Nickname:  nickname,
		Avatar:    avatar,
		Status:    user.StatusActive,
		CreatedAt: now.Format(time.RFC3339),
		UpdatedAt: now.Format(time.RFC3339),
	}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput, ip string) (TokenResponse, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" || input.Password == "" {
		return TokenResponse{}, InvalidCredentials(nil)
	}
	allowed, err := s.tokens.AllowLogin(ctx, ip, s.loginLimit, s.loginWindow)
	if err != nil {
		return TokenResponse{}, err
	}
	if !allowed {
		return TokenResponse{}, TooManyAttempts(nil)
	}

	account, err := s.users.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return TokenResponse{}, InvalidCredentials(err)
	}
	if err != nil {
		return TokenResponse{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(input.Password)) != nil {
		return TokenResponse{}, InvalidCredentials(nil)
	}
	if account.Status == user.StatusDisabled {
		return TokenResponse{}, user.Disabled(nil)
	}

	pair, jti, err := issueTokenPair(s.jwt, account.ID, s.accessTTL)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := s.tokens.SaveRefreshToken(ctx, jti, account.ID, s.refreshTTL); err != nil {
		return TokenResponse{}, err
	}
	return pair, nil
}

// DevLogin creates or reuses one of a small set of local-only test users. The
// route is registered only outside production, so it cannot be used by the
// deployed WeChat service.
func (s *Service) DevLogin(ctx context.Context, input DevLoginInput, ip string) (TokenResponse, error) {
	if input.Slot < 1 || input.Slot > 9 {
		return TokenResponse{}, apperror.BadRequest("dev login slot must be between 1 and 9", nil)
	}
	allowed, err := s.tokens.AllowLogin(ctx, ip, s.loginLimit, s.loginWindow)
	if err != nil {
		return TokenResponse{}, err
	}
	if !allowed {
		return TokenResponse{}, TooManyAttempts(nil)
	}

	username := "dev_player_" + strconv.Itoa(input.Slot)
	account, err := s.users.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return TokenResponse{}, fmt.Errorf("generate dev account secret: %w", err)
		}
		hash, err := bcrypt.GenerateFromPassword(secret, bcrypt.MinCost)
		if err != nil {
			return TokenResponse{}, fmt.Errorf("hash dev account secret: %w", err)
		}
		now := time.Now().UTC()
		id, createErr := s.users.CreateUserTx(ctx, db.CreateUserParams{
			Username:     username,
			PasswordHash: string(hash),
			Nickname:     "Dev Player " + strconv.Itoa(input.Slot),
			Status:       user.StatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		if createErr != nil {
			if !store.IsDuplicateEntry(createErr) {
				return TokenResponse{}, createErr
			}
			account, err = s.users.GetUserByUsername(ctx, username)
		} else {
			account = db.User{ID: id, Username: username, Nickname: "Dev Player " + strconv.Itoa(input.Slot), Status: user.StatusActive, CreatedAt: now, UpdatedAt: now}
			err = nil
		}
	}
	if account.ID != 0 {
		err = nil
	}
	if err != nil {
		return TokenResponse{}, fmt.Errorf("dev account lookup: %w", err)
	}
	if account.Status == user.StatusDisabled {
		return TokenResponse{}, user.Disabled(nil)
	}

	pair, jti, err := issueTokenPair(s.jwt, account.ID, s.accessTTL)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := s.tokens.SaveRefreshToken(ctx, jti, account.ID, s.refreshTTL); err != nil {
		return TokenResponse{}, err
	}
	return pair, nil
}

func (s *Service) LoginWithWeChat(ctx context.Context, input WeChatLoginInput, ip string) (TokenResponse, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" || len(code) > 512 {
		return TokenResponse{}, InvalidWeChatCode(nil)
	}
	allowed, err := s.tokens.AllowLogin(ctx, ip, s.loginLimit, s.loginWindow)
	if err != nil {
		return TokenResponse{}, err
	}
	if !allowed {
		return TokenResponse{}, TooManyAttempts(nil)
	}
	if s.wechat == nil {
		return TokenResponse{}, WeChatUnavailable(nil)
	}
	loginResult, err := s.wechat.ExchangeCode(ctx, code)
	if err != nil {
		if errors.Is(err, wechatplatform.ErrNotConfigured) {
			return TokenResponse{}, WeChatUnavailable(err)
		}
		return TokenResponse{}, InvalidWeChatCode(err)
	}
	openID := strings.TrimSpace(loginResult.OpenID)
	if openID == "" {
		return TokenResponse{}, InvalidWeChatCode(nil)
	}
	users, ok := s.users.(WeChatUserStore)
	if !ok {
		return TokenResponse{}, WeChatUnavailable(fmt.Errorf("user store does not support identities"))
	}

	account, err := users.GetUserByProviderSubject(ctx, wechatProvider, openID)
	created := false
	if errors.Is(err, sql.ErrNoRows) {
		account, err = s.createWeChatUser(ctx, users, openID, input)
		if err != nil {
			if store.IsDuplicateEntry(err) {
				account, err = users.GetUserByProviderSubject(ctx, wechatProvider, openID)
			} else {
				return TokenResponse{}, err
			}
		} else {
			created = true
		}
	}
	if err != nil {
		return TokenResponse{}, err
	}
	if account.Status == user.StatusDisabled {
		return TokenResponse{}, user.Disabled(nil)
	}

	pair, jti, err := issueTokenPair(s.jwt, account.ID, s.accessTTL)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := s.tokens.SaveRefreshToken(ctx, jti, account.ID, s.refreshTTL); err != nil {
		return TokenResponse{}, err
	}
	if created && s.welcome != nil {
		if err := s.welcome.EnqueueUserWelcome(ctx, account.ID); err != nil {
			s.logger.ErrorContext(ctx, "enqueue user welcome task failed", "user_id", account.ID, "error", err)
		}
	}
	return pair, nil
}

const wechatProvider = "wechat"

func (s *Service) createWeChatUser(ctx context.Context, users WeChatUserStore, openID string, input WeChatLoginInput) (db.User, error) {
	nickname := strings.TrimSpace(input.Nickname)
	avatar := strings.TrimSpace(input.Avatar)
	if len([]rune(nickname)) > 100 || len([]rune(avatar)) > 500 {
		return db.User{}, apperror.BadRequest("用户资料长度超出限制", nil)
	}
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return db.User{}, fmt.Errorf("generate external account secret: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword(passwordBytes, bcrypt.DefaultCost)
	if err != nil {
		return db.User{}, fmt.Errorf("hash external account secret: %w", err)
	}
	now := time.Now().UTC()
	username := wechatUsername(openID)
	id, err := users.CreateUserWithIdentityTx(ctx, db.CreateUserParams{
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Avatar:       avatar,
		Status:       user.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, db.CreateUserIdentityParams{
		Provider:        wechatProvider,
		ProviderSubject: openID,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		return db.User{}, err
	}
	return db.User{
		ID:           id,
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Avatar:       avatar,
		Status:       user.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func wechatUsername(openID string) string {
	digest := sha256.Sum256([]byte(openID))
	return fmt.Sprintf("wx_%x", digest[:12])
}

func (s *Service) Refresh(ctx context.Context, input RefreshInput) (TokenResponse, error) {
	claims, err := s.jwt.ParseRefreshToken(strings.TrimSpace(input.RefreshToken))
	if errors.Is(err, jwtplatform.ErrTokenExpired) {
		return TokenResponse{}, ExpiredToken(err)
	}
	if err != nil {
		return TokenResponse{}, InvalidToken(err)
	}

	userID, err := s.tokens.ConsumeRefreshToken(ctx, claims.ID)
	if err != nil {
		return TokenResponse{}, InvalidToken(err)
	}
	if userID != claims.UserID {
		return TokenResponse{}, InvalidToken(nil)
	}
	account, err := s.users.GetUserByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return TokenResponse{}, user.NotFound(err)
	}
	if err != nil {
		return TokenResponse{}, err
	}
	if account.Status == user.StatusDisabled {
		return TokenResponse{}, user.Disabled(nil)
	}

	pair, jti, err := issueTokenPair(s.jwt, account.ID, s.accessTTL)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := s.tokens.SaveRefreshToken(ctx, jti, account.ID, s.refreshTTL); err != nil {
		return TokenResponse{}, err
	}
	return pair, nil
}

func (s *Service) Logout(ctx context.Context, input LogoutInput) error {
	claims, err := s.jwt.ParseRefreshToken(strings.TrimSpace(input.RefreshToken))
	if errors.Is(err, jwtplatform.ErrTokenExpired) {
		return ExpiredToken(err)
	}
	if err != nil {
		return InvalidToken(err)
	}
	if err := s.tokens.RevokeRefreshToken(ctx, claims.ID); err != nil {
		return err
	}
	return nil
}

func validateRegisterInput(input RegisterInput) (string, string, string, string, error) {
	username := strings.TrimSpace(input.Username)
	password := input.Password
	nickname := strings.TrimSpace(input.Nickname)
	avatar := strings.TrimSpace(input.Avatar)
	if len([]rune(username)) < 3 || len([]rune(username)) > 64 {
		return "", "", "", "", apperror.BadRequest("用户名长度必须为 3 到 64 个字符", nil)
	}
	if len([]byte(password)) < 8 || len([]byte(password)) > 72 {
		return "", "", "", "", apperror.BadRequest("密码长度必须为 8 到 72 个字节", nil)
	}
	if len([]rune(nickname)) > 100 || len([]rune(avatar)) > 500 {
		return "", "", "", "", apperror.BadRequest("用户资料长度超出限制", nil)
	}
	return username, password, nickname, avatar, nil
}
