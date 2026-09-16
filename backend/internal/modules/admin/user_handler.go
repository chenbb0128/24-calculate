package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/example/go-service/internal/apperror"
	"github.com/example/go-service/internal/http/response"
)

type AdminUserHandler struct{ service *AdminUserService }

func NewAdminUserHandler(service *AdminUserService) *AdminUserHandler {
	return &AdminUserHandler{service: service}
}

func (h *AdminUserHandler) List(c *gin.Context) {
	result, err := h.service.List(c.Request.Context(), ListAdminUsersInput{
		Query: c.Query("q"), Status: c.Query("status"), Platform: c.Query("platform"),
		Page: parseAdminUserQueryInt(c.Query("page")), PageSize: parseAdminUserQueryInt(c.Query("page_size")),
	})
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *AdminUserHandler) Detail(c *gin.Context) {
	id, err := parseAdminUserID(c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.Detail(c.Request.Context(), id)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *AdminUserHandler) Disable(c *gin.Context) {
	h.setStatus(c, true)
}

func (h *AdminUserHandler) Enable(c *gin.Context) {
	h.setStatus(c, false)
}

func (h *AdminUserHandler) setStatus(c *gin.Context, disable bool) {
	id, err := parseAdminUserID(c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var result AdminUserStatusResponse
	if disable {
		result, err = h.service.Disable(c.Request.Context(), id)
	} else {
		result, err = h.service.Enable(c.Request.Context(), id)
	}
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func parseAdminUserID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, apperror.BadRequest("用户 ID 无效", err)
	}
	return id, nil
}

func parseAdminUserQueryInt(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
