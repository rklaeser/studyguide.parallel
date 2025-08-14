# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy the pkg module first
COPY pkg/ ./pkg/

# Copy a module files  
COPY a/go.mod a/go.sum* ./
COPY a/cmd/ ./cmd/
COPY a/*.go ./

# Update go.mod to use local pkg
RUN go mod edit -replace studyguide.parallel/pkg=./pkg

RUN CGO_ENABLED=0 GOOS=linux go build -o sequential-processor ./cmd/processor

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/sequential-processor .

RUN mkdir -p /data/a/output

ENTRYPOINT ["./sequential-processor"]