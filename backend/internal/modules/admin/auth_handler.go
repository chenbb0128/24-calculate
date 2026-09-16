package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/http/response"
	"github.com/example/go-service/internal/modules/auth"
)

type AdminAuthHandler struct{ service *AdminAuthService }

func NewAdminAuthHandler(service *AdminAuthService) *AdminAuthHandler {
	return &AdminAuthHandler{service: service}
}

func (h *AdminAuthHandler) Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("请求参数错误", err))
		return
	}
	result, err := h.service.Login(c.Request.Context(), input, c.ClientIP())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *AdminAuthHandler) Refresh(c *gin.Context) {
	var input auth.RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("请求参数错误", err))
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *AdminAuthHandler) Logout(c *gin.Context) {
	var input auth.LogoutInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("请求参数错误", err))
		return
	}
	if err := h.service.Logout(c.Request.Context(), input); err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, map[string]bool{"revoked": true})
}
