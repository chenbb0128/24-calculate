package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/http/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("请求参数错误", err))
		return
	}
	result, err := h.service.Register(c.Request.Context(), input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) Login(c *gin.Context) {
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

func (h *Handler) WeChatLogin(c *gin.Context) {
	var input WeChatLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("请求参数错误", err))
		return
	}
	result, err := h.service.LoginWithWeChat(c.Request.Context(), input, c.ClientIP())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) DevLogin(c *gin.Context) {
	var input DevLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("璇锋眰鍙傛暟閿欒", err))
		return
	}
	result, err := h.service.DevLogin(c.Request.Context(), input, c.ClientIP())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Refresh(c *gin.Context) {
	var input RefreshInput
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

func (h *Handler) Logout(c *gin.Context) {
	var input LogoutInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("请求参数错误", err))
		return
	}
	if err := h.service.Logout(c.Request.Context(), input); err != nil {
		response.WriteError(c, err)
		return
	}
	response.NoContent(c)
}
