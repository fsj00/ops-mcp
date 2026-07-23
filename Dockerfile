# ops-mcp multi-stage image (linux/amd64 & arm64)
FROM golang:1.24-bookworm AS builder

WORKDIR /src
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /out \
	&& go build -trimpath -ldflags="-s -w" -o /out/ops-mcp ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/ops-mcp /app/ops-mcp
COPY plugins /app/plugins
COPY config/ops-mcp.yaml.example /app/config/ops-mcp.yaml
COPY config/hosts.yaml.example /app/config/hosts.yaml
COPY config/databases.yaml.example /app/config/databases.yaml

ENV OPS_MCP_CONFIG=/app/config/ops-mcp.yaml

EXPOSE 20267

ENTRYPOINT ["/app/ops-mcp"]
CMD ["--config", "/app/config/ops-mcp.yaml"]
