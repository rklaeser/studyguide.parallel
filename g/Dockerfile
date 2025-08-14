FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY pkg/ /pkg/
COPY g_multithreaded_microservice/go.mod ./
COPY g_multithreaded_microservice/go.sum ./
RUN go mod download

COPY g_multithreaded_microservice/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o service cmd/service/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/service .

RUN mkdir -p /data/input /data/output

EXPOSE 8080

ENTRYPOINT ["./service"]