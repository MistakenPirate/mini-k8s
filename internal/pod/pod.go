package pod

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

type createBody struct {
	Name          string `json:"name"`
	Image         string `json:"image"`
	CpuRequest    int32  `json:"cpu_request"`
	MemoryRequest int32  `json:"memory_request"`
}

func (h *handler) createPod(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "clusterId")
	clusterUUID, err := uuid.Parse(clusterID)
	if err != nil {
		http.Error(w, "Invalid cluster ID", http.StatusBadRequest)
		return
	}

	var body createBody
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil || body.Name == "" || body.Image == "" {
		http.Error(w, "name and image are required", http.StatusBadRequest)
		return
	}

	data, err := h.queries.CreatePod(r.Context(), db.CreatePodParams{
		ClusterID:     clusterUUID,
		Name:          body.Name,
		Image:         body.Image,
		CpuRequest:    body.CpuRequest,
		MemoryRequest: body.MemoryRequest,
	})
	if err != nil {
		http.Error(w, "Failed to create pod", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

func (h *handler) listPodsByCluster(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "clusterId")
	clusterUUID, err := uuid.Parse(clusterID)
	if err != nil {
		http.Error(w, "Invalid cluster ID", http.StatusBadRequest)
		return
	}

	status := r.URL.Query().Get("status")
	if status != "" {
		data, err := h.queries.ListPodsByStatus(r.Context(), db.ListPodsByStatusParams{
			ClusterID: clusterUUID,
			Status:    status,
		})
		if err != nil {
			http.Error(w, "Failed to list pods", http.StatusInternalServerError)
			return
		}
		if data == nil {
			data = []db.Pod{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
		return
	}

	data, err := h.queries.ListPodsByCluster(r.Context(), clusterUUID)
	if err != nil {
		http.Error(w, "Failed to list pods", http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []db.Pod{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *handler) getPod(w http.ResponseWriter, r *http.Request) {
	podID := chi.URLParam(r, "podId")
	podUUID, err := uuid.Parse(podID)
	if err != nil {
		http.Error(w, "Invalid pod ID", http.StatusBadRequest)
		return
	}

	data, err := h.queries.GetPod(r.Context(), podUUID)
	if err != nil {
		http.Error(w, "Failed to get pod", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

type assignBody struct {
	NodeID string `json:"node_id"`
}

type updateStatusBody struct {
	Status string `json:"status"`
}

func (h *handler) updatePod(w http.ResponseWriter, r *http.Request) {
	podID := chi.URLParam(r, "podId")
	podUUID, err := uuid.Parse(podID)
	if err != nil {
		http.Error(w, "Invalid pod ID", http.StatusBadRequest)
		return
	}

	var raw map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&raw)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Assign to node
	if nodeIDStr, ok := raw["node_id"].(string); ok {
		nodeUUID, err := uuid.Parse(nodeIDStr)
		if err != nil {
			http.Error(w, "Invalid node ID", http.StatusBadRequest)
			return
		}
		data, err := h.queries.AssignPodToNode(r.Context(), db.AssignPodToNodeParams{
			NodeID: uuid.NullUUID{UUID: nodeUUID, Valid: true},
			ID:     podUUID,
		})
		if err != nil {
			http.Error(w, "Failed to assign pod", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
		return
	}

	// Update status
	if status, ok := raw["status"].(string); ok {
		data, err := h.queries.UpdatePodStatus(r.Context(), db.UpdatePodStatusParams{
			Status: status,
			ID:     podUUID,
		})
		if err != nil {
			http.Error(w, "Failed to update pod status", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
		return
	}

	http.Error(w, "node_id or status is required", http.StatusBadRequest)
}

func (h *handler) deletePod(w http.ResponseWriter, r *http.Request) {
	podID := chi.URLParam(r, "podId")
	podUUID, err := uuid.Parse(podID)
	if err != nil {
		http.Error(w, "Invalid pod ID", http.StatusBadRequest)
		return
	}
	err = h.queries.DeletePod(r.Context(), podUUID)
	if err != nil {
		http.Error(w, "Failed to delete pod", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func RegisterRoutes(r chi.Router, queries *db.Queries) {
	h := &handler{queries: queries}

	r.Post("/clusters/{clusterId}/pods", h.createPod)
	r.Get("/clusters/{clusterId}/pods", h.listPodsByCluster)
	r.Get("/clusters/{clusterId}/pods/{podId}", h.getPod)
	r.Patch("/clusters/{clusterId}/pods/{podId}", h.updatePod)
	r.Delete("/clusters/{clusterId}/pods/{podId}", h.deletePod)
}
