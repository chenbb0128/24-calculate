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
	"github.com/example/go-service/internal/modules/moderation"
	"github.com/example/go-service/internal/modules/user"
	jwtplatform "github.com/example/go-service/internal/platform/jwt"
	taptapplatform "github.com/example/go-service/internal/platform/taptap"
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

type AccessTokenRevocationStore interface {
	RevokeAccessToken(context.Context, string, time.Duration) error
}

type WelcomeEnqueuer interface {
	EnqueueUserWelcome(ctx context.Context, userID uint64) error
}

type WeChatLoginClient interface {
	ExchangeCode(ctx context.Context, code string) (wechatplatform.LoginResult, error)
}

type WeChatProfileSynchronizer interface {
	SyncAuthorizedWeChatProfile(context.Context, uint64, string, user.WeChatProfileInput, string) (user.WeChatProfileResult, error)
}

type TapTapLoginClient interface {
	ExchangeCode(ctx context.Context, code string) (taptapplatform.LoginResult, error)
}

type Service struct {
	users                     UserStore
	tokens                    TokenStore
	jwt                       *jwtplatform.Manager
	accessTTL                 time.Duration
	refreshTTL                time.Duration
	loginLimit                int64
	loginWindow               time.Duration
	welcome                   WelcomeEnqueuer
	logger                    *slog.Logger
	wechat                    WeChatLoginClient
	taptap                    TapTapLoginClient
	moderator                 *moderation.Service
	wechatProfileSynchronizer WeChatProfileSynchronizer
}

// SetContentModerator is retained for compatibility with existing composition
// code. WeChat profile changes are now handled by the user-module synchronizer;
// login itself never applies profile fields to an existing account.
func (s *Service) SetContentModerator(moderator *moderation.Service) {
	if s != nil {
		s.moderator = moderator
	}
}

func (s *Service) SetWeChatProfileSynchronizer(synchronizer WeChatProfileSynchronizer) {
	if s != nil {
		s.wechatProfileSynchronizer = synchronizer
	}
}

func NewService(users UserStore, tokens TokenStore, manager *jwtplatform.Manager, accessTTL, refreshTTL time.Duration, welcome WelcomeEnqueuer, logger *slog.Logger) *Service {
	return NewServiceWithWeChat(users, tokens, manager, accessTTL, refreshTTL, welcome, logger, nil)
}

func NewServiceWithWeChat(users UserStore, tokens TokenStore, manager *jwtplatform.Manager, accessTTL, refreshTTL time.Duration, welcome WelcomeEnqueuer, logger *slog.Logger, wechatClient WeChatLoginClient) *Service {
	return NewServiceWithWeChatAndTapTap(users, tokens, manager, accessTTL, refreshTTL, welcome, logger, wechatClient, nil)
}

func NewServiceWithWeChatAndTapTap(users UserStore, tokens TokenStore, manager *jwtplatform.Manager, accessTTL, refreshTTL time.Duration, welcome WelcomeEnqueuer, logger *slog.Logger, wechatClient WeChatLoginClient, tapTapClient TapTapLoginClient) *Service {
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
		taptap:      tapTapClient,
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
	return s.loginWithExternal(ctx, externalLoginInput{
		Code: input.Code, Nickname: input.Nickname, Avatar: input.Avatar,
	}, ip, wechatProvider, "wx_", user.NormalizeWeChatAvatar,
		func(ctx context.Context, code string) (externalLoginResult, error) {
			if s.wechat == nil {
				return externalLoginResult{}, WeChatUnavailable(nil)
			}
			result, err := s.wechat.ExchangeCode(ctx, code)
			return externalLoginResult{OpenID: result.OpenID}, err
		},
		func(err error) error {
			if errors.Is(err, wechatplatform.ErrInvalidCode) {
				return InvalidWeChatCode(err)
			}
			if errors.Is(err, wechatplatform.ErrNotConfigured) {
				return WeChatUnavailable(err)
			}
			return WeChatUnavailable(err)
		},
		InvalidWeChatCode, WeChatUnavailable)
}

func (s *Service) LoginWithTapTap(ctx context.Context, input TapTapLoginInput, ip string) (TokenResponse, error) {
	return s.loginWithExternal(ctx, externalLoginInput{
		Code: input.Code, Nickname: input.Nickname, Avatar: input.Avatar,
	}, ip, tapTapProvider, "tt_", user.NormalizeAvatar,
		func(ctx context.Context, code string) (externalLoginResult, error) {
			if s.taptap == nil {
				return externalLoginResult{}, TapTapUnavailable(nil)
			}
			result, err := s.taptap.ExchangeCode(ctx, code)
			return externalLoginResult{OpenID: result.OpenID}, err
		},
		func(err error) error {
			if errors.Is(err, taptapplatform.ErrInvalidCode) {
				return InvalidTapTapCode(err)
			}
			if errors.Is(err, taptapplatform.ErrNotConfigured) {
				return TapTapUnavailable(err)
			}
			return TapTapUnavailable(err)
		},
		InvalidTapTapCode, TapTapUnavailable)
}

type externalLoginInput struct {
	Code     string
	Nickname string
	Avatar   string
}

type externalLoginResult struct {
	OpenID string
}

type externalLoginExchange func(context.Context, string) (externalLoginResult, error)
type externalLoginErrorMapper func(error) error
type externalAvatarNormalizer func(string) (string, error)

func (s *Service) loginWithExternal(ctx context.Context, input externalLoginInput, ip, provider, usernamePrefix string, normalizeAvatar externalAvatarNormalizer, exchange externalLoginExchange, mapExchangeError externalLoginErrorMapper, invalidCode func(error) error, unavailable func(error) error) (TokenResponse, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" || len(code) > 512 {
		return TokenResponse{}, invalidCode(nil)
	}
	allowed, err := s.tokens.AllowLogin(ctx, ip, s.loginLimit, s.loginWindow)
	if err != nil {
		return TokenResponse{}, err
	}
	if !allowed {
		return TokenResponse{}, TooManyAttempts(nil)
	}
	loginResult, err := exchange(ctx, code)
	if err != nil {
		mapped := mapExchangeError(err)
		if mapped != nil {
			return TokenResponse{}, mapped
		}
		return TokenResponse{}, unavailable(err)
	}
	openID := strings.TrimSpace(loginResult.OpenID)
	if openID == "" {
		return TokenResponse{}, invalidCode(nil)
	}
	users, ok := s.users.(WeChatUserStore)
	if !ok {
		return TokenResponse{}, unavailable(fmt.Errorf("user store does not support identities"))
	}

	account, err := users.GetUserByProviderSubject(ctx, provider, openID)
	created := false
	if errors.Is(err, sql.ErrNoRows) {
		account, err = s.createExternalUser(ctx, users, provider, usernamePrefix, openID, input, normalizeAvatar)
		if err != nil {
			if store.IsDuplicateEntry(err) {
				account, err = users.GetUserByProviderSubject(ctx, provider, openID)
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
	if provider == wechatProvider {
		if created && s.wechatProfileSynchronizer != nil && (strings.TrimSpace(input.Nickname) != "" || strings.TrimSpace(input.Avatar) != "") {
			if _, err := s.wechatProfileSynchronizer.SyncAuthorizedWeChatProfile(ctx, account.ID, openID, user.WeChatProfileInput{
				Nickname: input.Nickname,
				Avatar:   input.Avatar,
			}, "wechat_login_initial"); err != nil {
				return TokenResponse{}, err
			}
		}
	} else if err := s.syncExternalProfile(ctx, users, &account, input, normalizeAvatar, unavailable); err != nil {
		return TokenResponse{}, err
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
const tapTapProvider = "taptap"

func (s *Service) createExternalUser(ctx context.Context, users WeChatUserStore, provider, usernamePrefix, openID string, input externalLoginInput, normalizeAvatar externalAvatarNormalizer) (db.User, error) {
	nickname := user.DefaultNickname
	nicknameStatus := string(moderation.StatusApproved)
	if provider != wechatProvider {
		var err error
		nickname, err = user.NormalizeNickname(input.Nickname)
		if err != nil {
			return db.User{}, apperror.BadRequest(err.Error(), err)
		}
	}
	avatar := user.DefaultAvatar
	if provider != wechatProvider {
		var err error
		avatar, err = normalizeAvatar(input.Avatar)
		if err != nil {
			return db.User{}, apperror.BadRequest(err.Error(), err)
		}
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
	username := externalUsername(usernamePrefix, openID)
	id, err := users.CreateUserWithIdentityTx(ctx, db.CreateUserParams{
		Username:                 username,
		PasswordHash:             string(hash),
		Nickname:                 nickname,
		Avatar:                   avatar,
		Status:                   user.StatusActive,
		NicknameModerationStatus: nicknameStatus,
		AvatarModerationStatus:   string(moderation.StatusApproved),
		CreatedAt:                now,
		UpdatedAt:                now,
	}, db.CreateUserIdentityParams{
		Provider:        provider,
		ProviderSubject: openID,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		return db.User{}, err
	}
	return db.User{
		ID:                       id,
		Username:                 username,
		PasswordHash:             string(hash),
		Nickname:                 nickname,
		Avatar:                   avatar,
		Status:                   user.StatusActive,
		NicknameModerationStatus: nicknameStatus,
		AvatarModerationStatus:   string(moderation.StatusApproved),
		CreatedAt:                now,
		UpdatedAt:                now,
	}, nil
}

type weChatProfileUpdater interface {
	UpdateUserProfile(context.Context, db.UpdateUserProfileParams) error
}

func (s *Service) syncExternalProfile(ctx context.Context, users WeChatUserStore, account *db.User, input externalLoginInput, normalizeAvatar externalAvatarNormalizer, unavailable func(error) error) error {
	if account == nil || account.ID == 0 {
		return unavailable(errors.New("external account is invalid"))
	}
	nickname := strings.TrimSpace(account.Nickname)
	if nickname == "" {
		nickname = user.DefaultNickname
	}
	nicknameStatus := strings.TrimSpace(account.NicknameModerationStatus)
	avatar := strings.TrimSpace(account.Avatar)
	if avatar == "" {
		avatar = user.DefaultAvatar
	}
	avatarStatus := strings.TrimSpace(account.AvatarModerationStatus)
	if strings.TrimSpace(input.Nickname) != "" {
		var err error
		nickname, err = user.NormalizeNickname(input.Nickname)
		if err != nil {
			return apperror.BadRequest(err.Error(), err)
		}
		nicknameStatus = string(moderation.StatusApproved)
	}
	if strings.TrimSpace(input.Avatar) != "" {
		var err error
		avatar, err = normalizeAvatar(input.Avatar)
		if err != nil {
			return apperror.BadRequest(err.Error(), err)
		}
		avatarStatus = string(moderation.StatusApproved)
	}
	if nickname == account.Nickname && avatar == account.Avatar && nicknameStatus == account.NicknameModerationStatus && avatarStatus == account.AvatarModerationStatus {
		return nil
	}
	updater, ok := users.(weChatProfileUpdater)
	if !ok {
		return unavailable(fmt.Errorf("user store does not support profile sync"))
	}
	now := time.Now().UTC()
	if err := updater.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		Nickname: nickname, Avatar: avatar,
		NicknameModerationStatus: nicknameStatus,
		AvatarModerationStatus:   avatarStatus,
		ModerationUpdatedAt:      &now,
		UpdatedAt:                now, ID: account.ID,
	}); err != nil {
		return err
	}
	account.Nickname, account.Avatar = nickname, avatar
	account.NicknameModerationStatus, account.AvatarModerationStatus = nicknameStatus, avatarStatus
	account.ModerationUpdatedAt, account.UpdatedAt = &now, now
	return nil
}

func wechatUsername(openID string) string {
	return externalUsername("wx_", openID)
}

func externalUsername(prefix, subject string) string {
	digest := sha256.Sum256([]byte(subject))
	return fmt.Sprintf("%s%x", prefix, digest[:12])
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

func (s *Service) ParseAccessToken(value string) (*jwtplatform.Claims, error) {
	claims, err := s.jwt.ParseAccessToken(strings.TrimSpace(value))
	if errors.Is(err, jwtplatform.ErrTokenExpired) {
		return nil, ExpiredToken(err)
	}
	if err != nil {
		return nil, InvalidToken(err)
	}
	return claims, nil
}

func (s *Service) Logout(ctx context.Context, input LogoutInput) error {
	claims, err := s.jwt.ParseRefreshToken(strings.TrimSpace(input.RefreshToken))
	if errors.Is(err, jwtplatform.ErrTokenExpired) {
		return ExpiredToken(err)
	}
	if err != nil {
		return InvalidToken(err)
	}
	return s.tokens.RevokeRefreshToken(ctx, claims.ID)
}

func (s *Service) LogoutWithAccess(ctx context.Context, accessClaims *jwtplatform.Claims, input LogoutInput) error {
	if accessClaims == nil || accessClaims.UserID == 0 || strings.TrimSpace(accessClaims.ID) == "" {
		return InvalidToken(nil)
	}
	claims, err := s.jwt.ParseRefreshToken(strings.TrimSpace(input.RefreshToken))
	if errors.Is(err, jwtplatform.ErrTokenExpired) {
		return ExpiredToken(err)
	}
	if err != nil {
		return InvalidToken(err)
	}
	if claims.UserID != accessClaims.UserID {
		return InvalidToken(nil)
	}
	if err := s.tokens.RevokeRefreshToken(ctx, claims.ID); err != nil {
		return err
	}
	if revoker, ok := s.tokens.(AccessTokenRevocationStore); ok && accessClaims.ExpiresAt != nil {
		ttl := time.Until(accessClaims.ExpiresAt.Time)
		if ttl > 0 {
			if err := revoker.RevokeAccessToken(ctx, accessClaims.ID, ttl); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRegisterInput(input RegisterInput) (string, string, string, string, error) {
	username := strings.TrimSpace(input.Username)
	password := input.Password
	nickname, nicknameErr := user.NormalizeNickname(input.Nickname)
	if nicknameErr != nil {
		return "", "", "", "", apperror.BadRequest(nicknameErr.Error(), nicknameErr)
	}
	avatar, avatarErr := user.NormalizeAvatar(input.Avatar)
	if avatarErr != nil {
		return "", "", "", "", apperror.BadRequest(avatarErr.Error(), avatarErr)
	}
	if len([]rune(username)) < 3 || len([]rune(username)) > 64 {
		return "", "", "", "", apperror.BadRequest("用户名长度必须为 3 到 64 个字符", nil)
	}
	if len([]byte(password)) < 8 || len([]byte(password)) > 72 {
		return "", "", "", "", apperror.BadRequest("密码长度必须为 8 到 72 个字节", nil)
	}
	return username, password, nickname, avatar, nil
}
