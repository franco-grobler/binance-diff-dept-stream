default:
    just --list

# Run application
[group("Dev")]
run:
    go run ./cmd/streamer/

# Build key replacer image
[group("Podman")]
build-container:
    podman build \
      -f Containerfile \
      -t binance-orderbook

# Run key replacer image
[group("Podman")]
run-container: build-container
    podman run -it --rm binance-orderbook
