package node

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	db "github.com/mistakenpirate/mini-k8s/db/sqlc"
)

type handler struct {
	queries *db.Queries
}

func (h *handler) listNodesByCluster(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "clusterId")
	clusterUUID, err := uuid.Parse(clusterID)
	if err != nil {
		http.Error(w, "Invalid cluster ID", http.StatusBadRequest)
		return
	}
	data, err := h.queries.ListNodesByCluster(r.Context(), clusterUUID)
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
	Name     string `json:"name"`
	CpuMilli int32  `json:"cpu_millis"`
	MemoryMb int32  `json:"memory_mb"`
}

func (h *handler) createNode(w http.ResponseWriter, r *http.Request) {
	var body createBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	clusterID := chi.URLParam(r, "clusterId")
	clusterUUID, err := uuid.Parse(clusterID)
	if err != nil {
		http.Error(w, "Invalid cluster ID", http.StatusBadRequest)
		return
	}
	data, err := h.queries.CreateNode(r.Context(), db.CreateNodeParams{
		ClusterID: clusterUUID,
		Name:      body.Name,
		CpuMillis: body.CpuMilli,
		MemoryMb:  body.MemoryMb,
	})
	if err != nil {
		http.Error(w, "Failed to create node", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

func (h *handler) getNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeId")
	nodeUUID, err := uuid.Parse(nodeID)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}
	data, err := h.queries.GetNode(r.Context(), nodeUUID)
	if err != nil {
		http.Error(w, "Failed to get node", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

type updateNodeBody struct {
	Status string `json:"status"`
}

func (h *handler) updateNodeStatus(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeId")
	nodeUUID, err := uuid.Parse(nodeID)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}
	var body updateNodeBody
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Status == "" {
		http.Error(w, "status is required", http.StatusBadRequest)
		return
	}
	data, err := h.queries.UpdateNodeStatus(r.Context(), db.UpdateNodeStatusParams{
		Status: body.Status,
		ID:     nodeUUID,
	})
	if err != nil {
		http.Error(w, "Failed to update node", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *handler) deleteNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeId")
	nodeUUID, err := uuid.Parse(nodeID)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}
	err = h.queries.DeleteNode(r.Context(), nodeUUID)
	if err != nil {
		http.Error(w, "Failed to delete node", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func RegisterRoutes(r chi.Router, queries *db.Queries) {
	h := &handler{queries: queries}

	r.Route("/clusters/{clusterId}/nodes", func(r chi.Router) {
		r.Get("/", h.listNodesByCluster)
		r.Post("/", h.createNode)
		r.Get("/{nodeId}", h.getNode)
		r.Patch("/{nodeId}", h.updateNodeStatus)
		r.Delete("/{nodeId}", h.deleteNode)
	})
}
