FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY d_distributed_sequential/go.mod d_distributed_sequential/go.sum* ./
RUN go mod download

COPY pkg/ ./pkg/
COPY d_distributed_sequential/cmd/ ./cmd/

RUN CGO_ENABLED=0 GOOS=linux go build -o processor ./cmd/processor

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/processor .

RUN mkdir -p /data/input /data/output

ENTRYPOINT ["./processor"]