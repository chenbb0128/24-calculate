package user

import (
	"net/http"

	"github.com/example/go-service/internal/apperror"
)

func NotFound(err error) error {
	return apperror.New(30001, http.StatusNotFound, "用户不存在", err)
}

func Disabled(err error) error {
	return apperror.New(30002, http.StatusForbidden, "用户已禁用", err)
}

func AvatarTooLarge(err error) error {
	return apperror.New(10007, http.StatusRequestEntityTooLarge, "头像文件不能超过 2 MB", err)
}

func InvalidWeChatProfileCode(err error) error {
	return apperror.New(20004, http.StatusUnauthorized, "微信登录凭证无效", err)
}

func WeChatProfileUnavailable(err error) error {
	return apperror.NewBusiness("WECHAT_PROFILE_SYNC_UNAVAILABLE", http.StatusServiceUnavailable, "微信资料同步暂不可用", err)
}

func WeChatProfileIdentityMismatch(err error) error {
	return apperror.New(apperror.CodeForbidden, http.StatusForbidden, "微信账号与当前账号不匹配", err)
}

func WeChatProfileCodeUsed(err error) error {
	return apperror.Conflict("微信登录凭证已使用，请重新授权", err)
}

func WeChatProfileRateLimited(err error) error {
	return apperror.New(10006, http.StatusTooManyRequests, "资料同步过于频繁，请稍后再试", err)
}
func UsernameExists(err error) error {
	return apperror.New(30003, http.StatusConflict, "用户名已存在", err)
}

func InvalidCredentials(err error) error {
	return apperror.New(30004, http.StatusUnauthorized, "用户名或密码错误", err)
}
