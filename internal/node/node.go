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

type registerBody struct {
	NodeName  string `json:"node_name"`
	ClusterID string `json:"cluster_id,omitempty"`
	CpuMillis int32  `json:"cpu_millis"`
	MemoryMb  int32  `json:"memory_mb"`
}

// registerNode lets a kubelet self-register via the API.
// If cluster_id is omitted and exactly one cluster exists, it auto-joins that cluster.
func (h *handler) registerNode(w http.ResponseWriter, r *http.Request) {
	var body registerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NodeName == "" {
		http.Error(w, "node_name is required", http.StatusBadRequest)
		return
	}

	var clusterUUID uuid.UUID

	if body.ClusterID != "" {
		parsed, err := uuid.Parse(body.ClusterID)
		if err != nil {
			http.Error(w, "invalid cluster_id", http.StatusBadRequest)
			return
		}
		// verify cluster exists
		_, err = h.queries.GetCluster(r.Context(), parsed)
		if err != nil {
			http.Error(w, "cluster not found", http.StatusNotFound)
			return
		}
		clusterUUID = parsed
	} else {
		// auto-discover: pick the only cluster
		clusters, err := h.queries.ListClusters(r.Context())
		if err != nil {
			http.Error(w, "failed to list clusters", http.StatusInternalServerError)
			return
		}
		if len(clusters) == 0 {
			http.Error(w, "no clusters exist — create a cluster first", http.StatusNotFound)
			return
		}
		if len(clusters) > 1 {
			http.Error(w, "multiple clusters exist — pass cluster_id to pick one", http.StatusConflict)
			return
		}
		clusterUUID = clusters[0].ID
	}

	// check if node already exists (rejoin)
	existing, err := h.queries.GetNodeByName(r.Context(), db.GetNodeByNameParams{
		ClusterID: clusterUUID,
		Name:      body.NodeName,
	})
	if err == nil {
		// node exists — rejoin, mark ready
		updated, err := h.queries.UpdateNodeStatus(r.Context(), db.UpdateNodeStatusParams{
			Status: "ready",
			ID:     existing.ID,
		})
		if err != nil {
			http.Error(w, "failed to update node status", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
		return
	}

	// new node — register it
	cpuMillis := body.CpuMillis
	if cpuMillis == 0 {
		cpuMillis = 1000
	}
	memoryMb := body.MemoryMb
	if memoryMb == 0 {
		memoryMb = 1024
	}

	node, err := h.queries.CreateNode(r.Context(), db.CreateNodeParams{
		ClusterID: clusterUUID,
		Name:      body.NodeName,
		CpuMillis: cpuMillis,
		MemoryMb:  memoryMb,
	})
	if err != nil {
		http.Error(w, "failed to register node", http.StatusInternalServerError)
		return
	}

	// mark ready immediately
	node, err = h.queries.UpdateNodeStatus(r.Context(), db.UpdateNodeStatusParams{
		Status: "ready",
		ID:     node.ID,
	})
	if err != nil {
		http.Error(w, "failed to update node status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

// listPodsByNode returns pods assigned to a specific node, with optional ?status= filter
func (h *handler) listPodsByNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeId")
	nodeUUID, err := uuid.Parse(nodeID)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	nullUUID := uuid.NullUUID{UUID: nodeUUID, Valid: true}
	status := r.URL.Query().Get("status")

	var pods []db.Pod

	switch status {
	case "scheduled":
		pods, err = h.queries.ListScheduledPodsByNode(r.Context(), nullUUID)
	case "running":
		pods, err = h.queries.ListRunningPodsByNode(r.Context(), nullUUID)
	default:
		pods, err = h.queries.ListPodsByNode(r.Context(), nullUUID)
	}

	if err != nil {
		http.Error(w, "failed to list pods", http.StatusInternalServerError)
		return
	}
	if pods == nil {
		pods = []db.Pod{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pods)
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

	r.Post("/nodes/register", h.registerNode)

	r.Get("/nodes/{nodeId}/pods", h.listPodsByNode)
	r.Patch("/nodes/{nodeId}", h.updateNodeStatus)

	r.Route("/clusters/{clusterId}/nodes", func(r chi.Router) {
		r.Get("/", h.listNodesByCluster)
		r.Post("/", h.createNode)
		r.Get("/{nodeId}", h.getNode)
		r.Patch("/{nodeId}", h.updateNodeStatus)
		r.Delete("/{nodeId}", h.deleteNode)
	})
}
