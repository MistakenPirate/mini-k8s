FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /api ./cmd/api

FROM alpine:3.21

COPY --from=builder /api /api
COPY db/migrations /migrations

EXPOSE 3000

CMD ["/api"]
