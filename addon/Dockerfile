# Multi-stage build for Home Assistant addon. Build with context at repo root:
#   docker build -f addon/Dockerfile --build-arg BUILD_FROM=ghcr.io/... .
ARG BUILD_FROM
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

RUN go build -ldflags "-s -w" -tags embed_frontend -o go-mumble-server ./cmd/go-mumble-server

FROM ${BUILD_FROM}

COPY --from=builder /app/go-mumble-server /usr/bin/go-mumble-server
COPY addon/rootfs /
