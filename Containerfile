ARG GO_VERSION=1.25.5

FROM public.ecr.aws/docker/library/golang:${GO_VERSION}-trixie AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . ./

# Build application
# Ensure executable
RUN go build -o main ./cmd/streamer/ \
  && chmod +x main

# FROM debian:bookworm-slim AS dependencies
#
# COPY deploy/bin/install-deps.sh /tmp/install-deps.sh
# RUN chmod +x /tmp/install-deps.sh && ./tmp/install-deps.sh
#
FROM gcr.io/distroless/static-debian12:nonroot AS final

WORKDIR /app

COPY --from=builder /app/main /app/main

ENTRYPOINT [ "/app/main" ]
