# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /speedtest-tracker-webhook

# Final stage
FROM alpine:latest

WORKDIR /

COPY --from=builder /speedtest-tracker-webhook /speedtest-tracker-webhook

EXPOSE 8080

ENTRYPOINT ["/speedtest-tracker-webhook"]
