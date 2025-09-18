# Build stage
FROM golang:1.20-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gateway cmd/server/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates postgresql-client
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/gateway .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/migrations ./migrations

# Create data directory for any local files
RUN mkdir -p /root/data

# Expose port
EXPOSE 8080

# Command to run
CMD ["./gateway", "-config", "configs/providers.yaml"]