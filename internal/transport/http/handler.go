package http

import (
	"encoding/json"
	"errors"
	"goods-service/internal/models"
	"goods-service/internal/service"
	"net/http"
	"strconv"
)

type ErrorResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type Handler struct {
	goodService *service.GoodService
}

func NewHandler(goodService *service.GoodService) *Handler {
	return &Handler{goodService: goodService}
}

func (h *Handler) CreateGood(w http.ResponseWriter, r *http.Request) {
	projectId, err := getProjectId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid project ID")
		return
	}

	var good models.Good
	if err := json.NewDecoder(r.Body).Decode(&good); err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid request payload")
		return
	}

	good.ProjectID = projectId

	if err := h.goodService.CreateGood(r.Context(), &good); err != nil {
		respondWithError(w, http.StatusInternalServerError, 5, "Internal server error")
		return
	}

	respondWithJSON(w, http.StatusCreated, good)
}

func (h *Handler) UpdateGood(w http.ResponseWriter, r *http.Request) {
	id, err := getId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid good ID")
		return
	}

	projectId, err := getProjectId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid project ID")
		return
	}

	var good models.Good
	if err := json.NewDecoder(r.Body).Decode(&good); err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid request payload")
		return
	}

	good.ID = id
	good.ProjectID = projectId

	if err := h.goodService.UpdateGood(r.Context(), &good); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			respondWithError(w, http.StatusBadRequest, 3, "errors.common.notFound")
			return
		}

		respondWithError(w, http.StatusInternalServerError, 5, "Internal server error")
		return
	}

	respondWithJSON(w, http.StatusOK, good)
}

func (h *Handler) GetGood(w http.ResponseWriter, r *http.Request) {
	id, err := getId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid good ID")
		return
	}

	projectId, err := getProjectId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid project ID")
		return
	}

	good, err := h.goodService.GetGood(r.Context(), id, projectId)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 3, "errors.common.notFound")
		return
	}

	respondWithJSON(w, http.StatusOK, good)
}

type DeleteResponse struct {
	Id         int  `json:"id"`
	CampaignId int  `json:"campaignId"`
	Removed    bool `json:"removed"`
}

func (h *Handler) DeleteGood(w http.ResponseWriter, r *http.Request) {
	id, err := getId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid good ID")
		return
	}

	projectId, err := getProjectId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid project ID")
		return
	}

	if err := h.goodService.DeleteGood(r.Context(), id, projectId); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			respondWithError(w, http.StatusBadRequest, 3, "errors.common.notFound")
			return
		}

		respondWithError(w, http.StatusInternalServerError, 4, "Internal server error")
		return
	}

	deleteResponse := DeleteResponse{
		Id:         id,
		CampaignId: projectId,
		Removed:    true,
	}

	respondWithJSON(w, http.StatusOK, deleteResponse)
}

func (h *Handler) ReprioritizeGood(w http.ResponseWriter, r *http.Request) {
	id, err := getId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid good ID")
		return
	}

	projectId, err := getProjectId(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid project ID")
		return
	}

	var req struct {
		NewPriority int `json:"newPriority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, 4, "Invalid request payload")
		return
	}

	if req.NewPriority < 1 {
		respondWithError(w, http.StatusBadRequest, 4, "Priority must be greater than 0")
		return
	}

	goods, err := h.goodService.ReprioritizeGood(r.Context(), id, projectId, req.NewPriority)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			respondWithError(w, http.StatusBadRequest, 3, "errors.common.notFound")
			return
		}

		respondWithError(w, http.StatusInternalServerError, 5, "Internal server error")
		return
	}

	respondWithJSON(w, http.StatusOK, goods)
}

type PaginatedResponse struct {
	Meta struct {
		Total   int `json:"total"`   // Общее количество записей
		Removed int `json:"removed"` // Количество удаленных записей
		Limit   int `json:"limit"`   // Размер страницы
		Offset  int `json:"offset"`  // Смещение
	} `json:"meta"`
	Goods []models.Good `json:"goods"` // Список товаров
}

func (h *Handler) ListGoods(w http.ResponseWriter, r *http.Request) {
	limit, offset := getPaginationParams(r)

	// Получаем товары
	goods, err := h.goodService.ListGoods(r.Context(), limit, offset)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, 5, "Internal server error")
		return
	}

	// Получаем общее количество записей и количество удаленных записей
	total, err := h.goodService.GetTotalCount(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, 5, "Internal server error")
		return
	}

	removed, err := h.goodService.GetRemovedCount(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, 5, "Internal server error")
		return
	}

	// Формируем ответ с пагинацией
	response := PaginatedResponse{
		Goods: goods,
	}
	response.Meta.Total = total
	response.Meta.Removed = removed
	response.Meta.Limit = limit
	response.Meta.Offset = offset

	respondWithJSON(w, http.StatusOK, response)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// getId Извлекает id из URL
func getId(r *http.Request) (id int, err error) {
	idStr := r.URL.Query().Get("projectId")
	if idStr == "" {
		return 0, errors.New("projectId is required")
	}

	id, err = strconv.Atoi(idStr)
	if err != nil {
		return 0, errors.New("invalid projectId parameter")
	}

	if id <= 0 {
		return 0, errors.New("id must be positive numbers")
	}

	return id, nil
}

// getProjectId Извлекает projectId из URL
func getProjectId(r *http.Request) (projectId int, err error) {
	projectIdStr := r.URL.Query().Get("projectId")
	if projectIdStr == "" {
		return 0, errors.New("projectId is required")
	}

	projectId, err = strconv.Atoi(projectIdStr)
	if err != nil {
		return 0, errors.New("invalid projectId parameter")
	}

	if projectId <= 0 {
		return 0, errors.New("projectId must be positive numbers")
	}

	return projectId, nil
}

// getPaginationParams Извлекает параметры пагинации из query параметров
func getPaginationParams(r *http.Request) (limit int, offset int) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit = 10
	offset = 0

	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	if limit > 100 {
		limit = 100
	}

	return limit, offset
}

func respondWithError(w http.ResponseWriter, status int, code int, message string) {
	response := ErrorResponse{
		Code:    code,
		Message: message,
		Details: struct{}{},
	}
	respondWithJSON(w, status, response)
}
