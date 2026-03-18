package pod

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

var podColumns = []string{"id", "cluster_id", "node_id", "name", "image", "status", "created_at", "updated_at", "cpu_request", "memory_request"}

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

func TestCreatePod(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	podID := uuid.New()

	mock.ExpectQuery("INSERT INTO pods").
		WithArgs(clusterID, "nginx", "nginx:latest", int32(100), int32(128)).
		WillReturnRows(pgxmock.NewRows(podColumns).
			AddRow(podID, clusterID, uuid.NullUUID{}, "nginx", "nginx:latest", "pending", now, now, int32(100), int32(128)))

	body := `{"name": "nginx", "image": "nginx:latest", "cpu_request": 100, "memory_request": 128}`
	req := httptest.NewRequest(http.MethodPost, "/clusters/"+clusterID.String()+"/pods", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var pod db.Pod
	json.NewDecoder(rec.Body).Decode(&pod)
	if pod.Name != "nginx" {
		t.Errorf("expected name nginx, got %s", pod.Name)
	}
	if pod.Image != "nginx:latest" {
		t.Errorf("expected image nginx:latest, got %s", pod.Image)
	}
	if pod.Status != "pending" {
		t.Errorf("expected status pending, got %s", pod.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestCreatePodMissingName(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	body := `{"name": "", "image": "nginx:latest"}`
	req := httptest.NewRequest(http.MethodPost, "/clusters/"+uuid.New().String()+"/pods", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreatePodMissingImage(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	body := `{"name": "nginx", "image": ""}`
	req := httptest.NewRequest(http.MethodPost, "/clusters/"+uuid.New().String()+"/pods", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreatePodInvalidClusterID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	body := `{"name": "nginx", "image": "nginx:latest"}`
	req := httptest.NewRequest(http.MethodPost, "/clusters/bad-id/pods", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListPodsByCluster(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	podID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM pods WHERE cluster_id").
		WithArgs(clusterID).
		WillReturnRows(pgxmock.NewRows(podColumns).
			AddRow(podID, clusterID, uuid.NullUUID{}, "nginx", "nginx:latest", "running", now, now, int32(100), int32(128)))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/pods", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var pods []db.Pod
	json.NewDecoder(rec.Body).Decode(&pods)
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].Name != "nginx" {
		t.Errorf("expected name nginx, got %s", pods[0].Name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestListPodsByClusterWithStatusFilter(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	podID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM pods WHERE cluster_id .+ AND status").
		WithArgs(clusterID, "pending").
		WillReturnRows(pgxmock.NewRows(podColumns).
			AddRow(podID, clusterID, uuid.NullUUID{}, "redis", "redis:7", "pending", now, now, int32(50), int32(64)))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/pods?status=pending", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var pods []db.Pod
	json.NewDecoder(rec.Body).Decode(&pods)
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].Status != "pending" {
		t.Errorf("expected status pending, got %s", pods[0].Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestListPodsByClusterEmpty(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	clusterID := uuid.New()
	mock.ExpectQuery("SELECT .+ FROM pods WHERE cluster_id").
		WithArgs(clusterID).
		WillReturnRows(pgxmock.NewRows(podColumns))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/pods", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var pods []db.Pod
	json.NewDecoder(rec.Body).Decode(&pods)
	if len(pods) != 0 {
		t.Fatalf("expected 0 pods, got %d", len(pods))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestListPodsByClusterInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	req := httptest.NewRequest(http.MethodGet, "/clusters/bad-id/pods", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetPod(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	podID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM pods WHERE id").
		WithArgs(podID).
		WillReturnRows(pgxmock.NewRows(podColumns).
			AddRow(podID, clusterID, uuid.NullUUID{}, "nginx", "nginx:latest", "running", now, now, int32(100), int32(128)))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/pods/"+podID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var pod db.Pod
	json.NewDecoder(rec.Body).Decode(&pod)
	if pod.ID != podID {
		t.Errorf("expected id %s, got %s", podID, pod.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestGetPodInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+uuid.New().String()+"/pods/bad-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdatePodAssignToNode(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	podID := uuid.New()
	nodeID := uuid.New()

	mock.ExpectQuery("UPDATE pods SET node_id").
		WithArgs(uuid.NullUUID{UUID: nodeID, Valid: true}, podID).
		WillReturnRows(pgxmock.NewRows(podColumns).
			AddRow(podID, clusterID, uuid.NullUUID{UUID: nodeID, Valid: true}, "nginx", "nginx:latest", "scheduled", now, now, int32(100), int32(128)))

	body := `{"node_id": "` + nodeID.String() + `"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+clusterID.String()+"/pods/"+podID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var pod db.Pod
	json.NewDecoder(rec.Body).Decode(&pod)
	if pod.Status != "scheduled" {
		t.Errorf("expected status scheduled, got %s", pod.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdatePodStatus(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	now := time.Now()
	clusterID := uuid.New()
	podID := uuid.New()

	mock.ExpectQuery("UPDATE pods SET status").
		WithArgs("running", podID).
		WillReturnRows(pgxmock.NewRows(podColumns).
			AddRow(podID, clusterID, uuid.NullUUID{}, "nginx", "nginx:latest", "running", now, now, int32(100), int32(128)))

	body := `{"status": "running"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+clusterID.String()+"/pods/"+podID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var pod db.Pod
	json.NewDecoder(rec.Body).Decode(&pod)
	if pod.Status != "running" {
		t.Errorf("expected status running, got %s", pod.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdatePodInvalidPodID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	body := `{"status": "running"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+uuid.New().String()+"/pods/bad-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdatePodInvalidNodeID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	podID := uuid.New()
	body := `{"node_id": "not-a-uuid"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+uuid.New().String()+"/pods/"+podID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdatePodMissingFields(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	podID := uuid.New()
	body := `{"something": "else"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+uuid.New().String()+"/pods/"+podID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdatePodInvalidJSON(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	podID := uuid.New()
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+uuid.New().String()+"/pods/"+podID.String(), strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeletePod(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	podID := uuid.New()
	mock.ExpectExec("DELETE FROM pods WHERE").
		WithArgs(podID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/clusters/"+uuid.New().String()+"/pods/"+podID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestDeletePodInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(nil)

	req := httptest.NewRequest(http.MethodDelete, "/clusters/"+uuid.New().String()+"/pods/bad-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
