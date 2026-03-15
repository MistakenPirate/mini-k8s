package cluster

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	db "github.com/mistakenpirate/mini-k8s/db/sqlc"
)

type handler struct{
	queries *db.Queries
}

func (h *handler) health(w http.ResponseWriter, r *http.Request){
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *handler) list(w http.ResponseWriter, r *http.Request){
	data, err := h.queries.ListClusters(r.Context())
	if err != nil {
		http.Error(w, "Failed to list clusters", http.StatusInternalServerError)
		return
	}
	if data == nil {
    data = []db.Cluster{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

type createBody struct {
    Name string `json:"name"`
}

func (h *handler) create(w http.ResponseWriter, r *http.Request){
	var body createBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" {
    	http.Error(w, "name is required", http.StatusBadRequest)
    	return
	}
	data, err := h.queries.CreateCluster(r.Context(), body.Name)
	if err != nil {
		http.Error(w, "Failed to create cluster", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request){
	id := chi.URLParam(r, "id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "Invalid cluster ID", http.StatusBadRequest)
		return
	}
	data, err := h.queries.GetCluster(r.Context(), uuid)
	if err != nil {
		http.Error(w, "Failed to get cluster", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func RegisterRoutes(r chi.Router, queries *db.Queries) {
    h := &handler{queries: queries}

    r.Route("/clusters", func(r chi.Router) {
		r.Get("/health", h.health)
        r.Get("/", h.list)
        r.Post("/", h.create)
        r.Get("/{id}", h.get)
    })
}