FROM node:20-bookworm-slim AS frontend
WORKDIR /app/frontend

COPY frontend/package.json frontend/yarn.lock* ./
RUN yarn install --frozen-lockfile

COPY frontend/ ./
RUN yarn build

FROM golang:1.24-bookworm AS builder
WORKDIR /app

ENV CGO_ENABLED=1

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /app/frontend/dist ./cmd/go-mumble-server/frontend-dist

RUN go build -ldflags "-s -w" -o go-mumble-server ./cmd/go-mumble-server

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates openssl \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -r mumble && useradd -r -g mumble -d /data mumble

COPY --from=builder /app/go-mumble-server ./
COPY configs/mumble-server.toml ./mumble-server.toml

RUN mkdir -p /data && chown mumble:mumble /data

USER mumble

ENV MUMBLE_DATABASE_PATH=/data/mumble-server.sqlite
ENV MUMBLE_NETWORK_HOST=0.0.0.0

EXPOSE 64738/tcp
EXPOSE 64738/udp
EXPOSE 64730/tcp

VOLUME ["/data"]

ENTRYPOINT ["./go-mumble-server"]
CMD ["-config", "mumble-server.toml"]
