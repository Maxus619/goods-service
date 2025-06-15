package http

import (
	"github.com/gorilla/mux"
	"net/http"
)

func NewRouter(h *Handler) *mux.Router {
	r := mux.NewRouter()

	// Middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Goods endpoints
	api.HandleFunc("/goods/list", h.ListGoods).Methods(http.MethodGet)
	api.HandleFunc("/good/create", h.CreateGood).Methods(http.MethodPost)
	api.HandleFunc("/goods", h.GetGood).Methods(http.MethodGet)
	api.HandleFunc("/good/update", h.UpdateGood).Methods(http.MethodPatch)
	api.HandleFunc("/good/remove", h.DeleteGood).Methods(http.MethodDelete)
	api.HandleFunc("/good/reprioritiize", h.ReprioritizeGood).Methods(http.MethodPatch)

	return r
}
