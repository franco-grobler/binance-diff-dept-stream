default:
    just --list

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
