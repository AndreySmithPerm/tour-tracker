# ===== Stage 1: Build =====
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o tour-tracker ./cmd/app

# ===== Stage 2: Run =====
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary
COPY --from=builder /app/tour-tracker .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

# Set ownership
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

ENTRYPOINT ["./tour-tracker"]
