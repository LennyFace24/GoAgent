FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o goagent ./cmd/main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/goagent .
COPY --from=builder /app/config.docker.yaml ./config.yaml
RUN mkdir -p data/conversations
EXPOSE 8080
CMD ["./goagent"]
