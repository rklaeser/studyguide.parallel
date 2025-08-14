FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy the pkg module first
COPY pkg/ ./pkg/

# Copy d module files
COPY d/go.mod d/go.sum* ./
COPY d/cmd/ ./cmd/

# Update go.mod to use local pkg
RUN go mod edit -replace studyguide.parallel/pkg=./pkg

RUN CGO_ENABLED=0 GOOS=linux go build -o processor ./cmd/processor

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/processor .

RUN mkdir -p /d/output

ENTRYPOINT ["./processor"]