package user

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/http/middleware"
	"github.com/example/go-service/internal/http/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetMe(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	profile, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, profile)
}

func (h *Handler) UpdateMe(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	var input UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("请求参数错误", err))
		return
	}
	if input.Nickname == nil && input.Avatar == nil {
		response.WriteError(c, apperror.BadRequest("至少提供一个可更新字段", nil))
		return
	}

	profile, err := h.service.UpdateProfile(c.Request.Context(), userID, input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, profile)
}
