# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o clonacion ./cmd/clonacion

# Run stage
FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/clonacion ./clonacion
EXPOSE 8080
ENTRYPOINT ["./clonacion"]


