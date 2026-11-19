# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install git and build essentials
RUN apk add --no-cache git ca-certificates tzdata

# Cache dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build static executable
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o smartcoop cmd/server/main.go

# Final runtime stage
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /build/smartcoop /app/smartcoop

# Copy templates and static assets
COPY web /app/web
COPY .env.example /app/.env.example

EXPOSE 3000

ENTRYPOINT ["/app/smartcoop"]

