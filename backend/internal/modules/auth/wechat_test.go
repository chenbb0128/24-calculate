package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/example/go-service/internal/modules/moderation"
	"github.com/example/go-service/internal/modules/user"
	wechatplatform "github.com/example/go-service/internal/platform/wechat"
	db "github.com/example/go-service/internal/store/sqlc"
)

func TestLoginWithWeChatCreatesAndReusesUser(t *testing.T) {
	users := &fakeWeChatUserStore{}
	client := &fakeWeChatClient{result: wechatplatform.LoginResult{OpenID: "openid-1"}}
	service := NewServiceWithWeChat(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)

	avatar := "https://thirdwx.qlogo.cn/mmopen/example/132"
	first, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code-1", Nickname: "玩家", Avatar: avatar}, "127.0.0.1")
	if err != nil {
		t.Fatalf("first LoginWithWeChat() error = %v", err)
	}
	if first.AccessToken == "" || users.createCalls != 1 || users.byIdentity.Nickname != user.DefaultNickname || users.byIdentity.Avatar != user.DefaultAvatar {
		t.Fatalf("first login state = %+v, users = %+v", first, users)
	}

	second, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code-2", Nickname: "新昵称", Avatar: avatar}, "127.0.0.1")
	if err != nil {
		t.Fatalf("second LoginWithWeChat() error = %v", err)
	}
	if second.AccessToken == "" || users.createCalls != 1 || users.byIdentity.Nickname != user.DefaultNickname || users.byIdentity.Avatar != user.DefaultAvatar {
		t.Fatalf("second login state = %+v, users = %+v", second, users)
	}
}

func TestLoginWithWeChatRejectsInvalidCode(t *testing.T) {
	client := &fakeWeChatClient{err: wechatplatform.ErrInvalidCode}
	service := NewServiceWithWeChat(&fakeWeChatUserStore{}, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)

	_, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "bad-code"}, "127.0.0.1")
	if err == nil || err.Error() != "微信登录凭证无效" {
		t.Fatalf("err = %v, want invalid WeChat code", err)
	}
}

func TestLoginWithWeChatUsesDefaultProfileWhenAuthorizationWasDeclined(t *testing.T) {
	users := &fakeWeChatUserStore{}
	client := &fakeWeChatClient{result: wechatplatform.LoginResult{OpenID: "openid-default"}}
	service := NewServiceWithWeChat(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)

	if _, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code"}, "127.0.0.1"); err != nil {
		t.Fatalf("LoginWithWeChat() error = %v", err)
	}
	if users.byIdentity.Nickname != user.DefaultNickname || users.byIdentity.Avatar != user.DefaultAvatar {
		t.Fatalf("created profile = %+v", users.byIdentity)
	}
}

func TestLoginWithWeChatRejectedFirstNicknameUsesSafeDefault(t *testing.T) {
	users := &fakeWeChatUserStore{}
	client := &fakeWeChatClient{result: wechatplatform.LoginResult{OpenID: "openid-rejected"}}
	service := NewServiceWithWeChat(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)

	if _, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code", Nickname: "违规昵称"}, "127.0.0.1"); err != nil {
		t.Fatalf("LoginWithWeChat() error = %v", err)
	}
	if users.byIdentity.Nickname != user.DefaultNickname || users.byIdentity.NicknameModerationStatus != string(moderation.StatusApproved) {
		t.Fatalf("rejected profile = %+v", users.byIdentity)
	}
}

func TestLoginWithWeChatProviderFailureDoesNotSaveUnreviewedNickname(t *testing.T) {
	users := &fakeWeChatUserStore{}
	client := &fakeWeChatClient{result: wechatplatform.LoginResult{OpenID: "openid-unavailable"}}
	service := NewServiceWithWeChat(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)

	if _, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code", Nickname: "待审核昵称"}, "127.0.0.1"); err != nil {
		t.Fatalf("LoginWithWeChat() error = %v", err)
	}
	if users.byIdentity.Nickname != user.DefaultNickname || users.byIdentity.NicknameModerationStatus != string(moderation.StatusApproved) {
		t.Fatalf("unavailable profile = %+v", users.byIdentity)
	}
}

func TestLoginWithWeChatExistingNamedUserCannotBeRenamedByLoginPayload(t *testing.T) {
	users := &fakeWeChatUserStore{byIdentity: db.User{
		ID: 2, Username: "wx_existing", Nickname: "原昵称", Avatar: user.DefaultAvatar, Status: user.StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved), AvatarModerationStatus: string(moderation.StatusApproved),
	}}
	client := &fakeWeChatClient{result: wechatplatform.LoginResult{OpenID: "openid-existing"}}
	service := NewServiceWithWeChat(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)
	service.SetContentModerator(newAuthModerator(moderation.ProviderResult{Status: moderation.StatusApproved}))

	if _, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code", Nickname: "攻击者昵称"}, "127.0.0.1"); err != nil {
		t.Fatalf("LoginWithWeChat() error = %v", err)
	}
	if users.byIdentity.Nickname != "原昵称" || users.updateCalls != 0 {
		t.Fatalf("existing profile was renamed: %+v, updates=%d", users.byIdentity, users.updateCalls)
	}
}

func TestLoginWithWeChatExistingDefaultProfileDoesNotSyncLoginPayload(t *testing.T) {
	users := &fakeWeChatUserStore{byIdentity: db.User{
		ID: 2, Username: "wx_default", Nickname: user.DefaultNickname, Avatar: user.DefaultAvatar, Status: user.StatusActive,
		NicknameModerationStatus: string(moderation.StatusApproved), AvatarModerationStatus: string(moderation.StatusApproved),
	}}
	client := &fakeWeChatClient{result: wechatplatform.LoginResult{OpenID: "openid-default-existing"}}
	service := NewServiceWithWeChat(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)

	if _, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code", Nickname: "首次授权昵称"}, "127.0.0.1"); err != nil {
		t.Fatalf("first login error = %v", err)
	}
	if users.byIdentity.Nickname != user.DefaultNickname || users.updateCalls != 0 {
		t.Fatalf("existing profile changed by login payload: %+v, updates=%d", users.byIdentity, users.updateCalls)
	}

	if _, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code-2", Nickname: "第二个昵称"}, "127.0.0.1"); err != nil {
		t.Fatalf("second login error = %v", err)
	}
	if users.byIdentity.Nickname != user.DefaultNickname || users.updateCalls != 0 {
		t.Fatalf("existing profile changed by second login payload: %+v, updates=%d", users.byIdentity, users.updateCalls)
	}
}

func TestLoginWithWeChatNewAccountUsesInjectedProfileSynchronizer(t *testing.T) {
	users := &fakeWeChatUserStore{}
	client := &fakeWeChatClient{result: wechatplatform.LoginResult{OpenID: "openid-new-profile"}}
	synchronizer := &fakeWeChatProfileSynchronizer{}
	service := NewServiceWithWeChat(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)
	service.SetWeChatProfileSynchronizer(synchronizer)

	if _, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code", Nickname: "授权昵称", Avatar: "https://thirdwx.qlogo.cn/mmopen/example/132"}, "127.0.0.1"); err != nil {
		t.Fatalf("LoginWithWeChat() error = %v", err)
	}
	if synchronizer.calls != 1 || synchronizer.userID != users.byIdentity.ID || synchronizer.subject != "openid-new-profile" || synchronizer.input.Nickname != "授权昵称" {
		t.Fatalf("profile synchronizer call = %+v", synchronizer)
	}
}

func TestLoginWithWeChatReturnsUnavailableForUpstreamFailure(t *testing.T) {
	client := &fakeWeChatClient{err: errors.New("connection reset")}
	service := NewServiceWithWeChat(&fakeWeChatUserStore{}, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)

	_, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code"}, "127.0.0.1")
	if err == nil || err.Error() != "微信登录暂不可用" {
		t.Fatalf("err = %v, want WeChat unavailable", err)
	}
}

type fakeWeChatClient struct {
	result wechatplatform.LoginResult
	err    error
}

func (f *fakeWeChatClient) ExchangeCode(context.Context, string) (wechatplatform.LoginResult, error) {
	return f.result, f.err
}

type fakeWeChatUserStore struct {
	fakeUserStore
	byIdentity  db.User
	createCalls int
	updateCalls int
}

type fakeWeChatProfileSynchronizer struct {
	calls   int
	userID  uint64
	subject string
	input   user.WeChatProfileInput
	err     error
}

func (f *fakeWeChatProfileSynchronizer) SyncAuthorizedWeChatProfile(_ context.Context, userID uint64, subject string, input user.WeChatProfileInput, _ string) (user.WeChatProfileResult, error) {
	f.calls++
	f.userID = userID
	f.subject = subject
	f.input = input
	return user.WeChatProfileResult{}, f.err
}

func (f *fakeWeChatUserStore) GetUserByProviderSubject(context.Context, string, string) (db.User, error) {
	if f.byIdentity.ID == 0 {
		return db.User{}, sql.ErrNoRows
	}
	return f.byIdentity, nil
}

func (f *fakeWeChatUserStore) CreateUserWithIdentityTx(_ context.Context, userArg db.CreateUserParams, _ db.CreateUserIdentityParams) (uint64, error) {
	f.createCalls++
	f.byIdentity = db.User{
		ID:                       2,
		Username:                 userArg.Username,
		PasswordHash:             userArg.PasswordHash,
		Nickname:                 userArg.Nickname,
		Avatar:                   userArg.Avatar,
		Status:                   userArg.Status,
		NicknameModerationStatus: userArg.NicknameModerationStatus,
		AvatarModerationStatus:   userArg.AvatarModerationStatus,
		ModerationUpdatedAt:      userArg.ModerationUpdatedAt,
		CreatedAt:                userArg.CreatedAt,
		UpdatedAt:                userArg.UpdatedAt,
	}
	return f.byIdentity.ID, nil
}

func (f *fakeWeChatUserStore) UpdateUserProfile(_ context.Context, arg db.UpdateUserProfileParams) error {
	f.updateCalls++
	f.byIdentity.Nickname = arg.Nickname
	f.byIdentity.Avatar = arg.Avatar
	f.byIdentity.NicknameModerationStatus = arg.NicknameModerationStatus
	f.byIdentity.AvatarModerationStatus = arg.AvatarModerationStatus
	f.byIdentity.ModerationUpdatedAt = arg.ModerationUpdatedAt
	f.byIdentity.UpdatedAt = arg.UpdatedAt
	return nil
}

type authModerationProvider struct {
	result moderation.ProviderResult
	err    error
}

func (f authModerationProvider) CheckText(context.Context, moderation.TextCheckRequest) (moderation.ProviderResult, error) {
	return f.result, f.err
}

func (f authModerationProvider) CheckImage(context.Context, moderation.ImageCheckRequest) (moderation.ProviderResult, error) {
	return moderation.ProviderResult{Status: moderation.StatusApproved}, nil
}

func newAuthModerator(result moderation.ProviderResult) *moderation.Service {
	return moderation.NewService(authModerationProvider{result: result}, nil)
}

func newAuthModeratorWithError(err error) *moderation.Service {
	return moderation.NewService(authModerationProvider{err: err}, nil)
}
