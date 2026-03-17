# mini-k8s

Building Kubernetes from scratch in Go to understand how it works under the hood.

Real k8s uses etcd, gRPC, and a control plane with multiple components. This project strips it down to the core ideas state, scheduling, reconciliation using tools I can reason about: a REST API, PostgreSQL, and raw Go.

<img width="1326" height="984" alt="image" src="https://github.com/user-attachments/assets/b84bfce1-d468-445e-8e07-5e43e56d3625" />

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
