package auth

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/example/go-service/internal/modules/user"
	wechatplatform "github.com/example/go-service/internal/platform/wechat"
	db "github.com/example/go-service/internal/store/sqlc"
)

func TestLoginWithWeChatCreatesAndReusesUser(t *testing.T) {
	users := &fakeWeChatUserStore{}
	client := &fakeWeChatClient{result: wechatplatform.LoginResult{OpenID: "openid-1"}}
	service := NewServiceWithWeChat(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, client)

	first, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code-1", Nickname: "玩家"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("first LoginWithWeChat() error = %v", err)
	}
	if first.AccessToken == "" || users.createCalls != 1 || users.byIdentity.Nickname != "玩家" {
		t.Fatalf("first login state = %+v, users = %+v", first, users)
	}

	second, err := service.LoginWithWeChat(context.Background(), WeChatLoginInput{Code: "code-2", Nickname: "新昵称"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("second LoginWithWeChat() error = %v", err)
	}
	if second.AccessToken == "" || users.createCalls != 1 {
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
		ID:           2,
		Username:     userArg.Username,
		PasswordHash: userArg.PasswordHash,
		Nickname:     userArg.Nickname,
		Avatar:       userArg.Avatar,
		Status:       userArg.Status,
		CreatedAt:    userArg.CreatedAt,
		UpdatedAt:    userArg.UpdatedAt,
	}
	return f.byIdentity.ID, nil
}
