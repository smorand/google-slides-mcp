# Build stage
FROM golang:1.21-alpine AS builder

# Install ca-certificates for HTTPS requests and git for module downloads
RUN apk add --no-cache ca-certificates git

# Set working directory
WORKDIR /app

# Copy go mod files first for better cache utilization
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version tagging
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown

# Build the binary with static linking for distroless compatibility
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.CommitSHA=${COMMIT_SHA} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/google-slides-mcp \
    ./cmd/google-slides-mcp

# Runtime stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /app/google-slides-mcp /google-slides-mcp

# Expose the default port
EXPOSE 8080

# Use nonroot user (65532) - already set by distroless:nonroot image
USER nonroot:nonroot

# Run the binary
ENTRYPOINT ["/google-slides-mcp"]
