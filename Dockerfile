FROM golang:1.26-bookworm

RUN apt-get update \
  && apt-get install -y --no-install-recommends python3 ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOFLAGS="-tags=purego" go build -o /usr/local/bin/pulsard ./cmd/pulsard
RUN chmod +x /app/scripts/docker_entrypoint.sh /app/scripts/setup_local_testnet.sh

ENTRYPOINT ["/app/scripts/docker_entrypoint.sh"]
CMD ["start", "--home", "/validator"]
