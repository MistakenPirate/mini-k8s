package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/mistakenpirate/mini-k8s/config"
	db "github.com/mistakenpirate/mini-k8s/db/sqlc"
	"github.com/mistakenpirate/mini-k8s/internal/cluster"
	"github.com/mistakenpirate/mini-k8s/internal/node"
	"github.com/mistakenpirate/mini-k8s/internal/pod"
)

func main(){
	_ = godotenv.Load()
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil{
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil{
		log.Fatalf("Unable to ping database: %v", err)
	}

	log.Println("Connected to database")

	queries := db.New(pool)
	
	r := chi.NewRouter()
    r.Use(middleware.Logger)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Route("/api/v1", func(r chi.Router) {
		cluster.RegisterRoutes(r, queries)
		node.RegisterRoutes(r, queries)
		pod.RegisterRoutes(r, queries)
	})

	log.Printf("server listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
    	log.Fatalf("server error: %v", err)
	}

}
