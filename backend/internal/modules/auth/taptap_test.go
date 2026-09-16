package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	taptapplatform "github.com/example/go-service/internal/platform/taptap"
)

func TestLoginWithTapTapCreatesAndReusesUser(t *testing.T) {
	users := &fakeWeChatUserStore{}
	client := &fakeTapTapClient{result: taptapplatform.LoginResult{OpenID: "tap-openid-1"}}
	service := NewServiceWithWeChatAndTapTap(users, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, nil, client)

	first, err := service.LoginWithTapTap(context.Background(), TapTapLoginInput{Code: "code-1", Nickname: "Tap玩家"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("first LoginWithTapTap() error = %v", err)
	}
	if first.AccessToken == "" || users.createCalls != 1 || users.byIdentity.Nickname != "Tap玩家" {
		t.Fatalf("first login state = %+v, users = %+v", first, users)
	}

	second, err := service.LoginWithTapTap(context.Background(), TapTapLoginInput{Code: "code-2", Nickname: "新昵称"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("second LoginWithTapTap() error = %v", err)
	}
	if second.AccessToken == "" || users.createCalls != 1 || users.byIdentity.Nickname != "新昵称" {
		t.Fatalf("second login state = %+v, users = %+v", second, users)
	}
}

func TestLoginWithTapTapRejectsInvalidCode(t *testing.T) {
	client := &fakeTapTapClient{err: taptapplatform.ErrInvalidCode}
	service := NewServiceWithWeChatAndTapTap(&fakeWeChatUserStore{}, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, nil, client)

	_, err := service.LoginWithTapTap(context.Background(), TapTapLoginInput{Code: "bad-code"}, "127.0.0.1")
	if err == nil || err.Error() != "TapTap 登录凭证无效" {
		t.Fatalf("err = %v, want invalid TapTap code", err)
	}
}

func TestLoginWithTapTapReturnsUnavailableForUpstreamFailure(t *testing.T) {
	client := &fakeTapTapClient{err: errors.New("connection reset")}
	service := NewServiceWithWeChatAndTapTap(&fakeWeChatUserStore{}, &fakeTokenStore{}, newTestJWTManager(t), time.Minute, time.Hour, nil, nil, nil, client)

	_, err := service.LoginWithTapTap(context.Background(), TapTapLoginInput{Code: "code"}, "127.0.0.1")
	if err == nil || err.Error() != "TapTap 登录暂不可用" {
		t.Fatalf("err = %v, want TapTap unavailable", err)
	}
}

type fakeTapTapClient struct {
	result taptapplatform.LoginResult
	err    error
}

func (f *fakeTapTapClient) ExchangeCode(context.Context, string) (taptapplatform.LoginResult, error) {
	return f.result, f.err
}
