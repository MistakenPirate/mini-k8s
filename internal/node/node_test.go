package node

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	db "github.com/mistakenpirate/mini-k8s/db/sqlc"
	"github.com/pashagolub/pgxmock/v4"
)

var nodeColumns = []string{"id", "cluster_id", "name", "status", "created_at", "updated_at", "cpu_millis", "memory_mb"}

func setupRouter(t *testing.T) (chi.Router, pgxmock.PgxConnIface) {
	t.Helper()
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	queries := db.New(mock)
	r := chi.NewRouter()
	RegisterRoutes(r, queries)
	return r, mock
}

func TestListNodesByCluster(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	nodeID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM nodes WHERE").
		WithArgs(clusterID).
		WillReturnRows(pgxmock.NewRows(nodeColumns).
			AddRow(nodeID, clusterID, "node-1", "ready", now, now, int32(1000), int32(1024)))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/nodes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var nodes []db.Node
	json.NewDecoder(rec.Body).Decode(&nodes)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "node-1" {
		t.Errorf("expected name node-1, got %s", nodes[0].Name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestListNodesByClusterEmpty(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	clusterID := uuid.New()
	mock.ExpectQuery("SELECT .+ FROM nodes WHERE").
		WithArgs(clusterID).
		WillReturnRows(pgxmock.NewRows(nodeColumns))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/nodes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var nodes []db.Node
	json.NewDecoder(rec.Body).Decode(&nodes)
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestListNodesByClusterInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	req := httptest.NewRequest(http.MethodGet, "/clusters/bad-id/nodes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateNode(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	nodeID := uuid.New()

	mock.ExpectQuery("INSERT INTO nodes").
		WithArgs(clusterID, "worker-1", int32(2000), int32(2048)).
		WillReturnRows(pgxmock.NewRows(nodeColumns).
			AddRow(nodeID, clusterID, "worker-1", "pending", now, now, int32(2000), int32(2048)))

	body := `{"name": "worker-1", "cpu_millis": 2000, "memory_mb": 2048}`
	req := httptest.NewRequest(http.MethodPost, "/clusters/"+clusterID.String()+"/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var node db.Node
	json.NewDecoder(rec.Body).Decode(&node)
	if node.Name != "worker-1" {
		t.Errorf("expected name worker-1, got %s", node.Name)
	}
	if node.CpuMillis != 2000 {
		t.Errorf("expected cpu_millis 2000, got %d", node.CpuMillis)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestCreateNodeMissingName(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	body := `{"name": "", "cpu_millis": 1000, "memory_mb": 1024}`
	req := httptest.NewRequest(http.MethodPost, "/clusters/"+uuid.New().String()+"/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateNodeInvalidClusterID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	body := `{"name": "worker-1", "cpu_millis": 1000, "memory_mb": 1024}`
	req := httptest.NewRequest(http.MethodPost, "/clusters/bad-id/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetNode(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	nodeID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM nodes WHERE").
		WithArgs(nodeID).
		WillReturnRows(pgxmock.NewRows(nodeColumns).
			AddRow(nodeID, clusterID, "node-1", "ready", now, now, int32(1000), int32(1024)))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/nodes/"+nodeID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var node db.Node
	json.NewDecoder(rec.Body).Decode(&node)
	if node.ID != nodeID {
		t.Errorf("expected id %s, got %s", nodeID, node.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestGetNodeInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+uuid.New().String()+"/nodes/bad-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	nodeID := uuid.New()

	mock.ExpectQuery("UPDATE nodes SET").
		WithArgs("ready", nodeID).
		WillReturnRows(pgxmock.NewRows(nodeColumns).
			AddRow(nodeID, clusterID, "node-1", "ready", now, now, int32(1000), int32(1024)))

	body := `{"status": "ready"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+clusterID.String()+"/nodes/"+nodeID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var node db.Node
	json.NewDecoder(rec.Body).Decode(&node)
	if node.Status != "ready" {
		t.Errorf("expected status ready, got %s", node.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdateNodeStatusMissingStatus(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	nodeID := uuid.New()
	body := `{"status": ""}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+uuid.New().String()+"/nodes/"+nodeID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateNodeStatusInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	body := `{"status": "ready"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+uuid.New().String()+"/nodes/bad-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteNode(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	nodeID := uuid.New()
	mock.ExpectExec("DELETE FROM nodes WHERE").
		WithArgs(nodeID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/clusters/"+uuid.New().String()+"/nodes/"+nodeID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestDeleteNodeInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	req := httptest.NewRequest(http.MethodDelete, "/clusters/"+uuid.New().String()+"/nodes/bad-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
