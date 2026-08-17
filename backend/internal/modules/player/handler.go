package player

import (
	"net/http"
	"strconv"

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

func (h *Handler) Bootstrap(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.Bootstrap(c.Request.Context(), userID)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Leaderboard(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	result, err := h.service.LeaderboardScopedPage(c.Request.Context(), userID, c.Param("mode"), LeaderboardQuery{
		Scope: c.Query("scope"), Period: c.Query("period"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) StartEndlessRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.StartEndlessRun(c.Request.Context(), userID)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) SubmitEndlessRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input EndlessRunSubmissionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.SubmitEndlessRun(c.Request.Context(), userID, c.Param("run_id"), input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) ResumeEndlessRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.ResumeEndlessRun(c.Request.Context(), userID, c.Param("run_id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) StartCampaignRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input CampaignRunStartInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.StartCampaignRun(c.Request.Context(), userID, input.LevelID)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) SubmitCampaignRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input CampaignRunSubmissionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.SubmitCampaignRun(c.Request.Context(), userID, c.Param("run_id"), input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) ResumeCampaignRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.ResumeCampaignRun(c.Request.Context(), userID, c.Param("run_id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) StartDailyRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.StartDailyRun(c.Request.Context(), userID)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) SubmitDailyRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input DailyRunSubmissionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.SubmitDailyRun(c.Request.Context(), userID, c.Param("run_id"), input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) ResumeDailyRun(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.ResumeDailyRun(c.Request.Context(), userID, c.Param("run_id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) JoinMatchmaking(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input JoinMatchmakingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.JoinMatchmaking(c.Request.Context(), userID, input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) GetMatchmakingStatus(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.GetMatchmakingStatus(c.Request.Context(), userID, c.Query("ticket_id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) CancelMatchmaking(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input struct {
		TicketID string `json:"ticket_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.CancelMatchmaking(c.Request.Context(), userID, input.TicketID)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) CreateFriendRoom(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.CreateFriendRoom(c.Request.Context(), userID)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) JoinFriendRoom(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.JoinFriendRoom(c.Request.Context(), userID, c.Param("room_code"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) GetFriendRoom(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.GetFriendRoomForUser(c.Request.Context(), userID, c.Param("room_code"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) LeaveFriendRoom(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	if err := h.service.LeaveFriendRoom(c.Request.Context(), userID, c.Param("room_code")); err != nil {
		response.WriteError(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ReadyFriendRoom(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input FriendRoomReadyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.ReadyFriendRoom(c.Request.Context(), userID, c.Param("room_code"), input.Ready)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) StartFriendRoom(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.StartFriendRoom(c.Request.Context(), userID, c.Param("room_code"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) UpdateFriendMatchProgress(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input FriendMatchProgressInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.UpdateFriendMatchProgress(c.Request.Context(), userID, c.Param("room_code"), input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) GetFriendMatchProgress(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.GetFriendMatchProgress(c.Request.Context(), userID, c.Param("room_code"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) SubmitFriendMatch(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input FriendMatchSubmissionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.SubmitFriendMatch(c.Request.Context(), userID, c.Param("room_code"), input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) CompleteLevel(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	levelID, err := strconv.Atoi(c.Param("level_id"))
	if err != nil {
		response.WriteError(c, apperror.BadRequest("level_id is invalid", err))
		return
	}
	var input CompleteLevelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.CompleteLevel(c.Request.Context(), userID, levelID, input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) CompleteDaily(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input CompleteDailyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.CompleteDaily(c.Request.Context(), userID, input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) PurchaseSkin(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input PurchaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.PurchaseSkinWithKey(c.Request.Context(), userID, c.Param("skin_id"), input.IdempotencyKey)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) EquipSkin(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.EquipSkin(c.Request.Context(), userID, c.Param("skin_id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) PurchaseCosmetic(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input PurchaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.PurchaseCosmeticWithKey(c.Request.Context(), userID, c.Param("cosmetic_id"), input.IdempotencyKey)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) EquipCosmetic(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	result, err := h.service.EquipCosmetic(c.Request.Context(), userID, c.Param("cosmetic_id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) UpdatePreferences(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	var input PlayerPreferencesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.WriteError(c, apperror.BadRequest("request body is invalid", err))
		return
	}
	result, err := h.service.UpdatePlayerPreferences(c.Request.Context(), userID, input)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, http.StatusOK, result)
}
