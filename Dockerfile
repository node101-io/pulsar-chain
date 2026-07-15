FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out \
  && CGO_ENABLED=0 GOFLAGS="-tags=purego" go build -o /out/pulsard ./cmd/pulsard \
  && CGO_ENABLED=0 GOFLAGS="-tags=purego" go build -o /out/pulsar-devtools ./scripts/devtools

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
  && apt-get install -y --no-install-recommends bash ca-certificates curl python3 \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/pulsar

COPY --from=builder /out/pulsard /usr/local/bin/pulsard
COPY --from=builder /out/pulsar-devtools /usr/local/bin/pulsar-devtools
COPY config.yml ./config.yml
COPY scripts/docker_entrypoint.sh ./scripts/docker_entrypoint.sh
COPY scripts/setup_local_testnet.sh ./scripts/setup_local_testnet.sh
COPY scripts/setup_local_testnet_helper.py ./scripts/setup_local_testnet_helper.py

RUN chmod +x /usr/local/bin/pulsard /usr/local/bin/pulsar-devtools \
  /opt/pulsar/scripts/docker_entrypoint.sh /opt/pulsar/scripts/setup_local_testnet.sh

ENTRYPOINT ["/opt/pulsar/scripts/docker_entrypoint.sh"]
