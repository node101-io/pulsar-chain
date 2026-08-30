# syntax=docker/dockerfile:1.10

FROM golang:1.26.5-alpine3.24@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder

RUN apk upgrade --no-cache libcrypto3 libssl3

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  mkdir -p /out \
  && CGO_ENABLED=0 GOFLAGS="-tags=purego" go build -o /out/pulsard ./cmd/pulsard \
  && CGO_ENABLED=0 GOFLAGS="-tags=purego" go build -o /out/pulsar-devtools ./scripts/devtools

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS runtime

RUN apk upgrade --no-cache libcrypto3 libssl3 \
  && apk add --no-cache bash ca-certificates curl python3

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
