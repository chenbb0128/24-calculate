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

func UsernameExists(err error) error {
	return apperror.New(30003, http.StatusConflict, "用户名已存在", err)
}

func InvalidCredentials(err error) error {
	return apperror.New(30004, http.StatusUnauthorized, "用户名或密码错误", err)
}
