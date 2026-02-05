# Build Stage
FROM golang:1.25-debian AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o streamer ./cmd/streamer

# Final Stage (Distroless for security and small size)
# hadolint ignore[DL3006]
FROM gcr.io/distroless/static-debian12

WORKDIR /

# Copy binary from builder
COPY --from=builder /app/streamer /streamer

# Run as non-root user (good practice)
USER nonroot:nonroot

ENTRYPOINT ["/streamer"]
