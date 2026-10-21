ARG RUST_VERSION=1.99
FROM rust:${RUST_VERSION}-bookworm AS build
WORKDIR /src
COPY Cargo.toml Cargo.lock build.rs ./
COPY src src
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/src/target \
    cargo test --release --locked && cargo build --release --locked \
    && cp target/release/prime-bench /prime-bench

FROM gcr.io/distroless/cc-debian12
COPY --from=build /prime-bench /prime-bench
ENV RESULTS_DIR=/results
EXPOSE 8080
ENTRYPOINT ["/prime-bench"]
