# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for modules
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the binary
RUN go build -o godns main.go

# Final stage
FROM alpine:3.20

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/godns .

# Expose DNS port
EXPOSE 1053/udp
EXPOSE 1053/tcp

# Expose optional HTTP API port
EXPOSE 8080

ENTRYPOINT ["./godns"]
