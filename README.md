# mini-k8s

Building Kubernetes from scratch in Go to understand how it works under the hood.

Real k8s uses etcd, gRPC, and a control plane with multiple components. This project strips it down to the core ideas state, scheduling, reconciliation using tools I can reason about: a REST API, PostgreSQL, and raw Go.

## Tech Stack

| Tool | Role |
|------|------|
| **Go 1.24** | Language |
| **chi** | HTTP router |
| **pgx/v5** | PostgreSQL driver |
| **sqlc** | SQL → type-safe Go codegen |
| **godotenv** | Environment config |
| **air** | Hot reload during dev |
| **Docker Compose** | Local Postgres |

## Architecture

```
cmd/api/main.go          → entrypoint, router setup
internal/cluster/         → cluster handlers + routes
internal/node/            → node handlers + routes
internal/pod/             → pod handlers + routes
db/migrations/            → SQL schema
db/queries/               → SQL queries (sqlc input)
db/sqlc/                  → generated Go code (sqlc output)
config/                   → env config loader
```

## Build Log

### Phase 1  Scaffolding

Set up the Go project structure, picked the tech stack, wired up the database, and built the first set of cluster APIs.

- [x] Project scaffolding (cmd/api, internal/, db/, config/)
- [x] PostgreSQL via Docker Compose
- [x] sqlc codegen pipeline
- [x] Hot reload with air
- [x] Cluster CRUD: create, list, get

### Phase 2  State Layer

Before you can build a scheduler, you need state. Before you can build a controller, you need something to control. This phase gives the system its memory.

- [x] Node CRUD with cluster scoping
- [x] Pod CRUD with cluster scoping
- [x] Resource capacity on nodes (`cpu_millis`, `memory_mb`)
- [x] Resource requests on pods (`cpu_request`, `memory_request`)
- [x] Unscheduled pod creation (pending, no node assigned)
- [x] Pod assignment to nodes via PATCH
- [x] Status filtering (`?status=pending`) for scheduler polling
- [x] Full lifecycle routes: create → schedule → run → delete
- [x] PATCH/DELETE for all entities
- [x] Health check endpoint

### Phase 3  Scheduler *(next)*

## API Routes

All routes are prefixed with `/api/v1`.

**Clusters**
```
POST   /clusters                                  create a cluster
GET    /clusters                                  list clusters
GET    /clusters/:id                              get cluster
PATCH  /clusters/:id                              update cluster status
DELETE /clusters/:id                              delete cluster
```

**Nodes**
```
POST   /clusters/:id/nodes                        add a node to a cluster
GET    /clusters/:id/nodes                        list nodes in a cluster
GET    /clusters/:id/nodes/:nodeId                get a node
PATCH  /clusters/:id/nodes/:nodeId                update node status
DELETE /clusters/:id/nodes/:nodeId                delete a node
```

**Pods**
```
POST   /clusters/:id/pods                         create a pod (unscheduled)
POST   /clusters/:id/nodes/:nodeId/pods           create a pod on a specific node
GET    /clusters/:id/pods                         list all pods in a cluster
GET    /clusters/:id/pods?status=pending          list pods by status
GET    /clusters/:id/pods/:podId                  get a pod
PATCH  /clusters/:id/pods/:podId                  assign to node or update status
DELETE /clusters/:id/pods/:podId                  delete a pod
```

**Health**
```
GET    /health                                    health check
```

## Running Locally

```bash
# start postgres
docker compose up -d

# copy env
cp .env.example .env

# run migrations
psql $DATABASE_URL -f db/migrations/001_init.sql
psql $DATABASE_URL -f db/migrations/002_resources.sql

# generate sqlc
sqlc generate

# run with hot reload
air
```

## Why?

1. Learned Go a while back but never built anything real with it this fixes that
2. Wanted a strictly typed backend that isn't TypeScript or Python for once
3. Tinkered with k8s at work and was always curious how it actually works
4. Building in public for accountability
