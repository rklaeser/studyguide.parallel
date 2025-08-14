# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy the pkg module first
COPY pkg/ ./pkg/

# Copy c module files  
COPY c/go.mod c/go.sum* ./
COPY c/cmd/ ./cmd/
COPY c/*.go ./

# Update go.mod to use local pkg
RUN go mod edit -replace studyguide.parallel/pkg=./pkg

RUN CGO_ENABLED=0 GOOS=linux go build -o pipelined-processor ./cmd/processor

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/pipelined-processor .

RUN mkdir -p /data/c/output

ENTRYPOINT ["./pipelined-processor"]