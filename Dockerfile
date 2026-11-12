# Build stage
FROM golang:1.27.4-alpine3.24 AS builder

# Install git for version info
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Version information (passed by the release workflow)
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o prox main.go

# Final stage
FROM alpine:3.24

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -s /bin/sh prox

# Set working directory
WORKDIR /home/prox

# Copy binary from builder stage
COPY --from=builder /app/prox /usr/local/bin/prox

# Change ownership and make executable
RUN chown prox:prox /usr/local/bin/prox && chmod +x /usr/local/bin/prox

# Switch to non-root user
USER prox

# Set entrypoint
ENTRYPOINT ["prox"]

# Default command
CMD ["--help"]
