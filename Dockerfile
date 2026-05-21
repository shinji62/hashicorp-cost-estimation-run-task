# Global build arguments for multi-arch support
ARG TARGETOS
ARG TARGETARCH

# ==========================================
# Stage 1: Build the Main Application
# ==========================================
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

RUN apk add --no-cache git ca-certificates make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Added GOARCH=$TARGETARCH to ensure multi-arch compatibility
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w" -o cost-estimation-run-task .

# ==========================================
# Stage 2: Build c3x from Source
# ==========================================
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS c3x-builder

ARG TARGETOS
ARG TARGETARCH
# Change this value or pass it via --build-arg to force a fresh git clone
ARG C3X_VERSION=main

WORKDIR /c3x

RUN apk add --no-cache git ca-certificates make

# Clone and checkout specific version (avoids bad caching)
RUN git clone https://github.com/c3xdev/c3x.git . && \
    git checkout ${C3X_VERSION}

# Removed the 'go clean' and debugging hex dumps
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w" -o /c3x/c3x ./cmd/c3x

# ==========================================
# Stage 3: Final Runtime Image
# ==========================================
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

# Create non-root user first so we can assign ownership on copy
RUN addgroup -g 1000 runtask && \
    adduser -D -u 1000 -G runtask runtask

# Copy binaries, set permissions, and change ownership in a single layer
COPY --from=builder --chmod=755 --chown=runtask:runtask /app/cost-estimation-run-task /usr/local/bin/
COPY --from=c3x-builder --chmod=755 --chown=runtask:runtask /c3x/c3x /usr/local/bin/

USER runtask

EXPOSE 22180

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:22180/healthcheck || exit 1

ENTRYPOINT ["cost-estimation-run-task"]
CMD ["--addr", "22180", "--path", "/runtask"]