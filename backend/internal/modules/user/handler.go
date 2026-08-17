package user

import (
	"io"
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

func (h *Handler) UploadAvatar(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.WriteError(c, apperror.BadRequest("请上传名为 file 的图片文件", err))
		return
	}
	if file.Size <= 0 {
		response.WriteError(c, apperror.BadRequest("头像文件不能为空", nil))
		return
	}
	maxBytes := h.service.avatarMaxBytes
	if maxBytes <= 0 {
		maxBytes = 2 << 20
	}
	if file.Size > maxBytes {
		response.WriteError(c, apperror.BadRequest("头像文件不能超过 2 MB", nil))
		return
	}
	opened, err := file.Open()
	if err != nil {
		response.WriteError(c, apperror.BadRequest("无法读取头像文件", err))
		return
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, maxBytes+1))
	if err != nil {
		response.WriteError(c, apperror.BadRequest("无法读取头像文件", err))
		return
	}
	if int64(len(data)) > maxBytes {
		response.WriteError(c, apperror.BadRequest("头像文件不能超过 2 MB", nil))
		return
	}
	result, err := h.service.UploadAvatar(c.Request.Context(), userID, data, h.service.avatarMaxDimension)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}
