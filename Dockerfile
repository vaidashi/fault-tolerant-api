FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -o fault-tolerant-api ./cmd/api

# Create a minimal image
FROM alpine:3.16

WORKDIR /app

# Copy the binary from the builder
COPY --from=builder /app/fault-tolerant-api .

# Set the command to run the binary
ENTRYPOINT ["./fault-tolerant-api"]