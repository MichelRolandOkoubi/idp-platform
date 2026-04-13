# ── Stage 1: Builder ──────────────────────────────────────────────────────────
FROM rust:1.75-slim AS builder

WORKDIR /app

# Install deps
RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Cache dependencies
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {}" > src/main.rs
RUN cargo build --release
RUN rm -f target/release/deps/idpctl*

# Build actual binary
COPY src ./src
RUN cargo build --release

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/target/release/idpctl /usr/local/bin/idpctl

RUN useradd -r -s /bin/false idpctl
USER idpctl

ENTRYPOINT ["idpctl"]