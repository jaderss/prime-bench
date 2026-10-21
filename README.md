# prime-bench

[![CI](https://github.com/jaderss/prime-bench/actions/workflows/ci.yml/badge.svg)](https://github.com/jaderss/prime-bench/actions/workflows/ci.yml) [![Quality gate](https://sonarcloud.io/api/project_badges/measure?project=jaderss_prime-bench&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=jaderss_prime-bench) [![Coverage](https://sonarcloud.io/api/project_badges/measure?project=jaderss_prime-bench&metric=coverage)](https://sonarcloud.io/summary/new_code?id=jaderss_prime-bench)

Java, Go and Rust on the same work, in three scenarios: a batch job generating big probable primes on all CPUs, an HTTP server generating them for thousands of virtual users, and a gateway endpoint that calls three downstream REST APIs in parallel and parses their JSON. Every run writes a CSV with throughput, latency percentiles, memory and CPU, and a React dashboard reads them.

The point is to compare the concurrency models on CPU-bound and on I/O-bound work: Java virtual threads, Go goroutines, and Rust tokio (with a rayon pool for the CPU work).

## Running

You need git, `make` and Docker running (Docker Desktop on macOS and Windows). The defaults use 6 CPUs in Docker: 4 for the app under test and 2 for the load generator. With fewer, lower `APP_CPUS` and `LOADGEN_CPUS` (see below). No JDK, Go, Rust or Node is needed on the host.

```bash
git clone https://github.com/jaderss/prime-bench.git
cd prime-bench
make bench        # build, then batch, server and gateway runs for java, go and rust
make dashboard    # open http://localhost:8080
make down         # stop everything
```

`make bench` takes about eight minutes with the defaults. The first build pulls the base images and takes a few minutes more. CSVs land in `results/`, one per language and mode, named after the run (`20261021T172045Z_server_go.csv`). `make help` lists every target.

For a quick first look, about two minutes:

```bash
make bench BATCH_COUNT=40 USERS=200 GATEWAY_USERS=200 DURATION=15s
```

Any value from `.env` can be overridden on the command line:

```bash
make batch LANGS=rust                    # one mode, one language
make server USERS=5000 THINK_TIME=1s     # heavier load
make gateway MOCK_LATENCY=80ms           # slower downstream APIs
make gateway MOCK_ERROR_RATE=0.01        # 1% of downstream calls fail
make gateway GATEWAY_LANGS="spring spring-native"   # only the Spring variants
make bench JAVA_VERSION=25 LANGS=java    # same benchmark on another JDK
make bench APP_CPUS=2 LOADGEN_CPUS=1     # smaller Docker setup
make dashboard DASHBOARD_PORT=9090       # 8080 already taken
```

### On Kubernetes

The same benchmark runs on Docker Desktop's built-in Kubernetes (Settings > Kubernetes > Enable, one node). Every `kubectl` call names `--context docker-desktop`, so nothing reaches another cluster.

```bash
make k8s-bench    # build, load the images, deploy, run server and gateway, one language at a time
make k8s-down     # delete the prime-bench namespace
```

Results land in `results/` as `k8s-server` and `k8s-gateway` and show in the dashboard's Kubernetes tab. Each app is a Deployment with the same CPU and memory limits as in compose, readiness and liveness probes, and a non-root security context. The load generator runs as a Job and prints its CSV to its log, which `scripts/k8s-run.sh` saves.

Docker Desktop can provision the cluster with kubeadm or kind. A kubeadm node shares Docker's image store. A kind node does not, so `scripts/k8s-load.sh` streams the images into it with the node's own `ctr`.

### Calling a server by hand

```bash
docker compose up -d --build go
curl "localhost:8082/prime?bits=512"
curl localhost:8082/stats
docker compose down
```

| Service | Port |
|---|---|
| java | 8081 |
| go | 8082 |
| rust | 8083 |
| spring | 8084 |
| spring-native | 8085 |
| dashboard | 8080 |

```bash
docker compose --profile tools up -d mock go
curl "localhost:8082/aggregate?account=acc-123"
```

### Without make

The make targets are thin wrappers around compose. Set `RUN_ID` yourself, otherwise every run writes to `local_*.csv`. On Linux, also export your user id so the containers can write to `results/`.

```bash
export HOST_UID=$(id -u) HOST_GID=$(id -g)   # Linux only
docker compose build java go rust
docker compose --profile tools build loadgen
docker compose run --rm -e MODE=batch -e BITS=2048 -e RUN_ID=run1 go
docker compose up -d go
docker compose run --rm -e TARGET=http://go:8080 -e RUN_ID=run1 loadgen
docker compose down
```

### Without Docker

Each app also runs on its own toolchain (JDK 25+, Go 1.25+, Rust 1.85+, Node 22.12+). Memory and CPU are read from Linux `/proc`, so those two columns are 0 on macOS and Windows.

```bash
cd apps/go   && MODE=batch RESULTS_DIR=../../results go run .
cd apps/rust && MODE=batch RESULTS_DIR=../../results cargo run --release
cd apps/java && ./mvnw package && MODE=batch RESULTS_DIR=../../results java -jar target/prime-bench.jar
cd loadgen   && TARGET=http://localhost:8080 USERS=200 RESULTS_DIR=../results go run .
```

Without `MODE=batch` an app starts its server on port 8080 (`PORT` to change).

### Troubleshooting

| Error | Fix |
|---|---|
| `range of CPUs is from 0.01 to N` | Docker has fewer CPUs than `APP_CPUS` or `LOADGEN_CPUS`. Lower them, or raise the CPUs in Docker Desktop settings. |
| `Cannot connect to the Docker daemon` | Start Docker Desktop. |
| `port is already allocated` | Another process uses 8080 to 8083. Stop it, or use `DASHBOARD_PORT` for the dashboard. |
| Dashboard says "No results yet" | Run `make bench` first, then reload. |

## Upgrading a language

Versions live in `.env` and go into each Dockerfile as a build argument. Images are tagged with the version, so old and new builds sit side by side.

```
JAVA_VERSION=27
GO_VERSION=1.27
RUST_VERSION=1.99
NODE_VERSION=26
NGINX_VERSION=1.31
```

Change a line, run `make bench`, and the dashboard's "All runs" table shows both versions next to each other. CI builds and tests every app on the current and the previous release.

## Results

One run on an Apple M4 Max, Docker Desktop, 4 CPUs and 2 GB per app. Java 27, Go 1.27.3, Rust 1.99.1. Every run finished with 0 errors.

![Dashboard, gateway run](docs/dashboard.jpg)

**Batch**, 200 primes of 2048 bits, 4 threads:

| | Primes/s | p50 ms | p99 ms | Peak RSS MB |
|---|---|---|---|---|
| Java | 17.2 | 176 | 981 | 141 |
| Go | 15.2 | 202 | 884 | 14.0 |
| Rust | 11.9 | 228 | 1430 | 2.8 |

**Server**, 2000 virtual users, 512-bit primes, 2 s think time, 60 s:

| | Requests/s | p50 ms | p99 ms | Peak RSS MB | CPU % |
|---|---|---|---|---|---|
| Java | 891 | 2.7 | 10.0 | 275 | 236 |
| Go | 864 | 35.4 | 402 | 22.9 | 346 |
| Rust | 891 | 4.1 | 16.0 | 4.9 | 292 |

**Gateway**, 4000 virtual users, 3 downstream calls per request (30 ± 10 ms each), 2 s think time, 60 s:

| | Requests/s | p50 ms | p99 ms | Peak RSS MB | CPU % |
|---|---|---|---|---|---|
| Java (no framework) | 1753 | 36.7 | 41.2 | 463 | 88 |
| Spring Boot, JVM | 1754 | 36.8 | 41.4 | 490 | 124 |
| Spring Boot, native | 1753 | 37.6 | 61.3 | 466 | 137 |
| Go | 1756 | 36.6 | 41.0 | 30.0 | 40 |
| Rust | 1753 | 36.6 | 41.0 | 20.0 | 24 |

The mock downstream used 15% of its CPU in each gateway run, so it was not the limit.

What the numbers show:

- At 2048 bits Java leads, Go is close and Rust is 20 to 30% slower. `num-bigint` is portable Rust, while `BigInteger` has JIT intrinsics and `math/big` has assembly.
- At 512 bits Go falls behind. A batch run of 4000 primes gives Java about 1420/s, Rust 1265/s and Go 990/s, so Go spends more CPU per prime on small numbers.
- Under load that extra CPU turns into latency. Go serves the same traffic at 87% CPU, Java at 59%, and queueing near saturation grows much faster than load. Same throughput, 40x the p99.
- Go's garbage collector is not the main cost. The heap stays around 5 MB, so it runs about 200 cycles per second, but those take 3% of the CPU. `GOGC=400` cuts the p99 to about 120 ms with the same CPU use. Try it with `make server LANGS=go GOGC=400`.
- In the gateway, where the work is waiting on other services, all three give the same throughput and latency. The cost is what differs: for the same 1750 requests per second, Java uses 3.7 times the CPU of Rust and Go 1.6 times.
- Spring Boot on the JVM costs about 40% more CPU than plain Java for the same gateway (124% against 88%), with the same latency and memory. That is the framework: servlet filters, message converters, validation and observability hooks.
- The GraalVM native build starts in 0.05 s and is lean at low load (140 MB and 18% CPU at 100 users, against 335 MB and 43% on the JVM). Under sustained load it gives that back: more CPU than the JVM, as much memory and a worse p99. GraalVM Community has no JIT to re-optimize hot code and only the serial collector, so native is for fast startup and small idle footprints, not for peak throughput.
- On Docker Desktop's Kubernetes the same runs land within noise of compose: p99 41.0 to 41.4 ms for the JVM, Go and Rust gateways, 60 ms for Spring native, and the same CPU and memory. The extra hops through Kubernetes Services cost nothing measurable at this load, and the ranking holds.
- Memory is where the runtimes differ most. In the gateway run: 473 MB for the JVM, 31.5 MB for Go, 19.8 MB for Rust. With `JAVA_OPTS=-Xmx128m` Java handles the same load at 337 MB, so most of the JVM's memory is the runtime itself, not the heap.

Run it on your own machine before you quote any of this. See [METHODOLOGY.md](METHODOLOGY.md) for the limits.

## Layout

```
apps/java     Java 27, no framework. HttpServer and HttpClient on virtual threads, Jackson, Maven, JUnit
apps/go       Go 1.27, standard library only
apps/rust     Rust 1.99, axum, tokio, rayon, num-bigint, reqwest, serde
apps/spring   Spring Boot 4.1 on Java 25: the gateway only, as a JVM image and a GraalVM native image
loadgen       Go load generator, shares the CSV writer with apps/go
mock          Go mock of three downstream REST APIs, with latency and error injection
k8s           Kubernetes manifests, rendered with the values from .env
scripts       k8s-run.sh runs one language on Kubernetes, k8s-load.sh loads images into a kind node
dashboard     React 19, Vite, Recharts, served by nginx
results       CSV output, mounted into every container
```

Each app reads its settings from the environment: `MODE` (`batch` or `server`), `BITS`, `COUNT`, `CONCURRENCY`, `ROUNDS`, `RUN_ID`, `DOWNSTREAM_URL`. The CSV schema and the HTTP contract are in [METHODOLOGY.md](METHODOLOGY.md), together with what the numbers do not tell you.

## Testing

Each Dockerfile runs the language's tests before it builds the image, so `make build` fails on a broken test. The same prime and composite vectors (Mersenne primes, Carmichael numbers, a strong pseudoprime to bases 2, 3, 5 and 7) are checked in all three languages.

With the toolchains installed:

```bash
cd apps/java && ./mvnw verify           # also checks formatting
cd apps/go   && go test ./...
cd apps/rust && cargo test
cd dashboard && npm ci && npm run dev   # dashboard with hot reload on :5173, reads ../results
```
