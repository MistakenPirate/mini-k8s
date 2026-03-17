package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/mistakenpirate/mini-k8s/config"
	db "github.com/mistakenpirate/mini-k8s/db/sqlc"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)

	log.Println("scheduler started")

	// control loop: poll for pending pods every 5 seconds
	for {
		schedule(context.Background(), queries)
		time.Sleep(5 * time.Second)
	}
}

// schedule finds all pending pods and tries to place each one on a node
func schedule(ctx context.Context, q *db.Queries) {
	// grab every pod that hasn't been assigned to a node yet
	pending, err := q.ListPendingPods(ctx)
	if err != nil || len(pending) == 0 {
		return
	}

	for _, pod := range pending {
		// get candidate nodes in the same cluster as the pod
		nodes, err := q.ListNodesByCluster(ctx, pod.ClusterID)
		if err != nil || len(nodes) == 0 {
			continue
		}

		// run the scoring algorithm to find the best-fit node
		best := pickBestNode(ctx, q, nodes, pod)
		if best == nil {
			log.Printf("no node fits pod %s", pod.Name)
			continue
		}

		// bind the pod to the chosen node (also sets status to 'scheduled')
		_, err = q.AssignPodToNode(ctx, db.AssignPodToNodeParams{
			NodeID: uuid.NullUUID{UUID: best.ID, Valid: true},
			ID:     pod.ID,
		})
		if err != nil {
			log.Printf("failed to assign pod %s: %v", pod.Name, err)
			continue
		}
		log.Printf("assigned pod %s -> node %s", pod.Name, best.Name)
	}
}

// pickBestNode scores nodes using a spread strategy:
// it picks the node with the most free resources after placement.
// switch '>' to '<' in the comparison for bin-packing (fill nodes up first).
func pickBestNode(ctx context.Context, q *db.Queries, nodes []db.Node, pod db.Pod) *db.Node {
	var best *db.Node
	bestRemaining := int32(-1)

	for _, node := range nodes {
		// sum up resources already consumed by pods on this node
		assigned, _ := q.ListPodsByNode(ctx, uuid.NullUUID{UUID: node.ID, Valid: true})

		usedCPU, usedMem := int32(0), int32(0)
		for _, p := range assigned {
			usedCPU += p.CpuRequest
			usedMem += p.MemoryRequest
		}

		// calculate what's left on the node
		freeCPU := node.CpuMillis - usedCPU
		freeMem := node.MemoryMb - usedMem

		// skip if the pod doesn't fit
		if freeCPU < pod.CpuRequest || freeMem < pod.MemoryRequest {
			continue
		}

		// spread: prefer the node with the most headroom
		remaining := freeCPU + freeMem
		if remaining > bestRemaining {
			bestRemaining = remaining
			n := node
			best = &n
		}
	}
	return best
}
