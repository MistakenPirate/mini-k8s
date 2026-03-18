package cluster

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"context"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	db "github.com/mistakenpirate/mini-k8s/db/sqlc"
	"github.com/pashagolub/pgxmock/v4"
)

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

func TestListClusters(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	now := time.Now()
	id := uuid.New()
	rows := pgxmock.NewRows([]string{"id", "name", "status", "created_at", "updated_at"}).
		AddRow(id, "test-cluster", "active", now, now)
	mock.ExpectQuery("SELECT .+ FROM clusters").WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/clusters", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var clusters []db.Cluster
	json.NewDecoder(rec.Body).Decode(&clusters)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Name != "test-cluster" {
		t.Errorf("expected name test-cluster, got %s", clusters[0].Name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestListClustersEmpty(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	rows := pgxmock.NewRows([]string{"id", "name", "status", "created_at", "updated_at"})
	mock.ExpectQuery("SELECT .+ FROM clusters").WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/clusters", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var clusters []db.Cluster
	json.NewDecoder(rec.Body).Decode(&clusters)
	if len(clusters) != 0 {
		t.Fatalf("expected 0 clusters, got %d", len(clusters))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestCreateCluster(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	now := time.Now()
	id := uuid.New()
	mock.ExpectQuery("INSERT INTO clusters").
		WithArgs("my-cluster").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "status", "created_at", "updated_at"}).
			AddRow(id, "my-cluster", "pending", now, now))

	body := `{"name": "my-cluster"}`
	req := httptest.NewRequest(http.MethodPost, "/clusters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var cluster db.Cluster
	json.NewDecoder(rec.Body).Decode(&cluster)
	if cluster.Name != "my-cluster" {
		t.Errorf("expected name my-cluster, got %s", cluster.Name)
	}
	if cluster.Status != "pending" {
		t.Errorf("expected status pending, got %s", cluster.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestCreateClusterMissingName(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPost, "/clusters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateClusterInvalidJSON(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	req := httptest.NewRequest(http.MethodPost, "/clusters", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetCluster(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	now := time.Now()
	id := uuid.New()
	mock.ExpectQuery("SELECT .+ FROM clusters WHERE").
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "status", "created_at", "updated_at"}).
			AddRow(id, "test-cluster", "active", now, now))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var cluster db.Cluster
	json.NewDecoder(rec.Body).Decode(&cluster)
	if cluster.ID != id {
		t.Errorf("expected id %s, got %s", id, cluster.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestGetClusterInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/clusters/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateClusterStatus(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	now := time.Now()
	id := uuid.New()
	mock.ExpectQuery("UPDATE clusters SET").
		WithArgs("active", id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "status", "created_at", "updated_at"}).
			AddRow(id, "test-cluster", "active", now, now))

	body := `{"status": "active"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+id.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var cluster db.Cluster
	json.NewDecoder(rec.Body).Decode(&cluster)
	if cluster.Status != "active" {
		t.Errorf("expected status active, got %s", cluster.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdateClusterStatusMissingStatus(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	body := `{"status": ""}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/"+uuid.New().String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateClusterStatusInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	body := `{"status": "active"}`
	req := httptest.NewRequest(http.MethodPatch, "/clusters/bad-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteCluster(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	id := uuid.New()
	mock.ExpectExec("DELETE FROM clusters WHERE").
		WithArgs(id).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/clusters/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestDeleteClusterInvalidID(t *testing.T) {
	r, mock := setupRouter(t)
	defer mock.Close(context.Background())

	req := httptest.NewRequest(http.MethodDelete, "/clusters/bad-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
