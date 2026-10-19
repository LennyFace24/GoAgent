FROM node:20-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

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
COPY --from=builder /app/internal/prompt ./internal/prompt
COPY --from=frontend /src/frontend/dist ./frontend/dist
RUN mkdir -p data/conversations
EXPOSE 8080
CMD ["./goagent"]
