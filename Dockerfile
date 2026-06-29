FROM golang:1.26-bookworm

RUN apt-get update \
  && apt-get install -y --no-install-recommends python3 ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

ENV HOME=/data \
    BIN_DIR=/data/bin \
    BINARY_PATH=/data/bin/pulsard \
    COMPAT_BINARY_PATH=/data/bin/pulsar-chaind \
    GOCACHE=/tmp/go-build \
    API_BIND_HOST=0.0.0.0 \
    START_VALIDATORS=1

COPY go.mod go.sum ./
RUN go mod download

COPY . .

VOLUME ["/data"]

EXPOSE 26656 26657 1317 9090 26666 26667 1318 9091 26676 26677 1319 9092

CMD ["./scripts/setup_local_testnet.sh", "3"]
