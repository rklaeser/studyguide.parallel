# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy the pkg module first
COPY pkg/ ./pkg/

# Copy b module files  
COPY b/go.mod b/go.sum* ./
COPY b/cmd/ ./cmd/
COPY b/*.go ./

# Update go.mod to use local pkg
RUN go mod edit -replace studyguide.parallel/pkg=./pkg

RUN CGO_ENABLED=0 GOOS=linux go build -o tile-parallel-processor ./cmd/processor

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/tile-parallel-processor .

RUN mkdir -p /data/b/output

ENTRYPOINT ["./tile-parallel-processor"]