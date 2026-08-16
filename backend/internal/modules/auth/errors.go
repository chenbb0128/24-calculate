package auth

import (
	"net/http"

	"github.com/example/go-service/internal/apperror"
)

func InvalidWeChatCode(err error) error {
	return apperror.New(20004, http.StatusUnauthorized, "微信登录凭证无效", err)
}

func WeChatUnavailable(err error) error {
	return apperror.New(50001, http.StatusServiceUnavailable, "微信登录暂不可用", err)
}

func MissingToken(err error) error {
	return apperror.New(20001, http.StatusUnauthorized, "Token 缺失", err)
}

func InvalidToken(err error) error {
	return apperror.New(20002, http.StatusUnauthorized, "Token 无效", err)
}

func ExpiredToken(err error) error {
	return apperror.New(20003, http.StatusUnauthorized, "Token 已过期", err)
}

func InvalidCredentials(err error) error {
	return apperror.New(30004, http.StatusUnauthorized, "用户名或密码错误", err)
}

func TooManyAttempts(err error) error {
	return apperror.New(10006, http.StatusTooManyRequests, "请求过于频繁，请稍后再试", err)
}
