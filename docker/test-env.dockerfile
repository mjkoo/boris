# Multi-stage build: compile boris from source, then set up a minimal
# runtime image with the boris repo checked out as a known workspace.

# --- Builder stage ---
FROM golang:1.25-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /boris ./cmd/boris

# --- Runtime stage ---
FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    bash \
    coreutils \
    grep \
    findutils \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Clone boris repo at a known commit for a reproducible workspace
RUN git clone https://github.com/mjkoo/boris.git /workspace \
    && cd /workspace \
    && git checkout 2cc83a9 \
    && git config --global --add safe.directory /workspace

COPY --from=builder /boris /usr/local/bin/boris

WORKDIR /workspace
EXPOSE 8080

ENTRYPOINT ["boris", "--transport", "http", "--port", "8080", "--anthropic-compat"]
