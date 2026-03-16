package node

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

func (h *handler) listNodesByCluster(w http.ResponseWriter, r *http.Request){
	clusterID := chi.URLParam(r, "clusterId")
	uuid, err := uuid.Parse(clusterID)
	if err != nil {
		http.Error(w, "Invalid cluster ID", http.StatusBadRequest)
		return
	}
	data, err := h.queries.ListNodesByCluster(r.Context(), uuid)
	if err != nil {
		http.Error(w, "Failed to list nodes", http.StatusInternalServerError)
		return
	}
	if data == nil {
    data = []db.Node{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

type createBody struct {
    Name string `json:"name"`
}

func (h *handler) createNode(w http.ResponseWriter, r *http.Request){
	var body createBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" {
    	http.Error(w, "name is required", http.StatusBadRequest)
    	return
	}
	clusterID := chi.URLParam(r, "clusterId")
	uuid, err := uuid.Parse(clusterID)
	if err != nil {
		http.Error(w, "Invalid cluster ID", http.StatusBadRequest)
		return
	}
	data, err := h.queries.CreateNode(r.Context(), db.CreateNodeParams{
		ClusterID: uuid,
		Name:      body.Name,
	})
	if err != nil {
		http.Error(w, "Failed to create node", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}


func RegisterRoutes(r chi.Router, queries *db.Queries) {
    h := &handler{queries: queries}

    r.Route("/clusters/{clusterId}/nodes", func(r chi.Router) {
        r.Get("/", h.listNodesByCluster)
        r.Post("/", h.createNode)
    })
}