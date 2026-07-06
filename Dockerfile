# syntax=docker/dockerfile:1

# Build stage. Runs on the build host's native platform and cross-compiles a
# static binary for the target platform, so multi-arch builds need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

# Module manifest first for layer caching (this project is stdlib-only).
COPY go.mod ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/planka-mcp ./cmd/planka-mcp

# Production stage.
FROM alpine:3.20 AS production

ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

LABEL org.opencontainers.image.title="Planka MCP Server (Go)" \
      org.opencontainers.image.description="MCP server for Planka - Real-Time Collaborative Kanban Board" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.source="https://github.com/adambenhassen/planka-mcp-go" \
      org.opencontainers.image.url="https://github.com/adambenhassen/planka-mcp-go" \
      org.opencontainers.image.documentation="https://github.com/adambenhassen/planka-mcp-go#readme" \
      org.opencontainers.image.vendor="adambenhassen" \
      org.opencontainers.image.licenses="MIT"

# Non-root user for security.
RUN addgroup -g 1001 -S mcpuser && \
    adduser -u 1001 -S mcpuser -G mcpuser

COPY --from=builder /out/planka-mcp /usr/local/bin/planka-mcp

USER mcpuser

# Runtime defaults (override at runtime). stdio is the default single-client mode.
ENV MCP_TRANSPORT=stdio \
    MCP_PORT=3001 \
    PLANKA_BASE_URL=http://localhost:3000

# Only used in SSE mode.
EXPOSE 3001

# Health check is only meaningful in SSE mode; a no-op in stdio mode.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD if [ "$MCP_TRANSPORT" = "sse" ]; then wget --no-verbose --tries=1 --spider "http://localhost:${MCP_PORT}/health" || exit 1; else exit 0; fi

ENTRYPOINT ["/usr/local/bin/planka-mcp"]
