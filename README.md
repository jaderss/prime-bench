# prime-bench

[![CI](https://github.com/jaderss/prime-bench/actions/workflows/ci.yml/badge.svg)](https://github.com/jaderss/prime-bench/actions/workflows/ci.yml) [![Quality gate](https://sonarcloud.io/api/project_badges/measure?project=jaderss_prime-bench&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=jaderss_prime-bench) [![Coverage](https://sonarcloud.io/api/project_badges/measure?project=jaderss_prime-bench&metric=coverage)](https://sonarcloud.io/summary/new_code?id=jaderss_prime-bench)

Java, Go and Rust on the same work, in three scenarios: a batch job generating big probable primes on all CPUs, an HTTP server generating them for thousands of virtual users, and a gateway endpoint that calls three downstream REST APIs in parallel and parses their JSON. The gateway also runs on Spring Boot, on the JVM and as a GraalVM native image. Everything runs on Docker Compose or on Docker Desktop's Kubernetes. Every run writes a CSV with throughput, latency percentiles, memory and CPU, and a React dashboard reads them.

The point is to compare the concurrency models on CPU-bound and on I/O-bound work: Java virtual threads, Go goroutines, and Rust tokio (with a rayon pool for the CPU work), and to show what a framework and native compilation cost on top.

## Running

You need git, `make` and Docker running (Docker Desktop on macOS and Windows). The defaults use 5 CPUs in Docker: 2 for the app under test, 2 for the load generator and 1 for the mock downstream. With fewer, lower `APP_CPUS`, `LOADGEN_CPUS` and `MOCK_CPUS` (see below). No JDK, Go, Rust or Node is needed on the host.

```bash
git clone https://github.com/jaderss/prime-bench.git
cd prime-bench
make bench        # build, then batch and server for java, go and rust, and the gateway for those plus spring
make dashboard    # open http://localhost:8080
make down         # stop everything
```

`make bench` takes about 12 minutes with the defaults. The first build pulls the base images and compiles the native image, a few minutes more. CSVs land in `results/`, one per language and mode, named after the run (`20261021T172045Z_server_go.csv`). `make help` lists every target.

For a quick first look, about four minutes:

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
make bench LOADGEN_CPUS=1                # smaller Docker setup, 4 CPUs
make dashboard DASHBOARD_PORT=9090       # 8080 already taken
```

### Resources

Every app gets the same resources in every scenario, on compose and on Kubernetes. They live in `.env`:

| Key | Default | What it sets |
|---|---|---|
| `APP_CPUS` | 2 | CPU limit for each app |
| `APP_MEMORY_MB` | 1536 | Memory limit for each app. On Kubernetes the request equals the limit |
| `HEAP_PERCENT` | 75 | Max heap as a share of the memory limit: `-XX:MaxRAMPercentage` on the JVM, `-XX:MaximumHeapSizePercent` on the native image |
| `JVM_GC` | ParallelGC | Garbage collector for plain Java and Spring on the JVM. The native image only has the serial collector |
| `JAVA_OPTS` | empty | Extra JVM flags, added after the ones above |

The JVM values follow the [Microsoft guide for Java in containers](https://learn.microsoft.com/en-us/azure/developer/java/containers/overview): 2 cores, ParallelGC and 75% of the memory for the heap. Without flags, the JVM uses only 25% of the container limit for the heap and picks SerialGC below 1792 MB.

### On Kubernetes

The same benchmark runs on Docker Desktop's built-in Kubernetes (Settings > Kubernetes > Enable, one node). Every `kubectl` call names `--context docker-desktop`, so nothing reaches another cluster.

```bash
make k8s-bench    # build, load the images, deploy, run server and gateway, one language at a time
make k8s-down     # delete the prime-bench namespace
```

Results land in `results/` as `k8s-server` and `k8s-gateway` and show in the dashboard's Kubernetes tab. Each app is a Deployment with the same CPU and memory limits as in compose, readiness and liveness probes, and a non-root security context. The load generator runs as a Job and prints its CSV to its log, which `scripts/k8s-run.sh` saves.

Docker Desktop can provision the cluster with kubeadm or kind. A kubeadm node shares Docker's image store. A kind node does not, so `scripts/k8s-load.sh` streams the images into it with the node's own `ctr`. For that it starts a short-lived root pod with the node's filesystem mounted and deletes it when done. That is fine on a local throwaway cluster, and it is why the script always targets the `docker-desktop` context, whatever context is active.

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
docker compose --profile tools build loadgen mock
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
| `range of CPUs is from 0.01 to N` | Docker has fewer CPUs than `APP_CPUS`, `LOADGEN_CPUS` or `MOCK_CPUS`. Lower them, or raise the CPUs in Docker Desktop settings. |
| `Cannot connect to the Docker daemon` | Start Docker Desktop. |
| `port is already allocated` | Another process uses 8080 to 8085. Stop it, or use `DASHBOARD_PORT` for the dashboard. |
| Dashboard says "No results yet" | Run `make bench` (or `make k8s-bench` for the Kubernetes tab) first, then reload. |
| `connection to the server kubernetes.docker.internal:6443 was refused` | Kubernetes is off. Enable it in Docker Desktop settings and wait until it shows as running. |
| Pods stuck in `ErrImageNeverPull` | The images are not on the node. Run `make k8s-up`, which loads them. |
| `make k8s-bench` times out on a rollout | A previous run left an app scaled up. Run `make k8s-down` and start again. |

## Upgrading a language

Versions live in `.env` and go into each Dockerfile as a build argument. Images are tagged with the version, so old and new builds sit side by side.

```
JAVA_VERSION=27
GO_VERSION=1.27
RUST_VERSION=1.99
NODE_VERSION=26
NGINX_VERSION=1.31
SPRING_JAVA_VERSION=25
```

Change a line, run `make bench`, and the dashboard's "All runs" table shows both versions next to each other. CI builds and tests Java, Go and Rust on the current and the previous release, and runs every app, Spring and the native image included, in a short benchmark. Spring stays on Java 25 because that is the newest JDK GraalVM Community ships.

## Results

One run on an Apple M4 Max with Docker Desktop, on AC power. Java 27, Go 1.27.3, Rust 1.99.1, Spring Boot 4.1.1 on Java 25. Every app had 2 CPUs and 1.5 GB, with the JVM settings from [Resources](#resources). Every run finished with 0 errors.

Since every app gets the same limits, the comparison between languages matters more than the absolute numbers. On slower cores primes/s goes down, and the server and gateway reach saturation with fewer users. The ratios should hold: CPU per request, the p99 gap under load and memory.

![Dashboard, gateway run](docs/dashboard.jpg)

**Batch**, 200 primes of 2048 bits, 2 threads:

| | Primes/s | p50 ms | p99 ms | Peak RSS MB |
|---|---|---|---|---|
| Java | 9.26 | 168 | 786 | 110 |
| Go | 8.72 | 166 | 1032 | 13.9 |
| Rust | 6.31 | 260 | 1045 | 2.7 |

**Server**, 1000 virtual users, 512-bit primes, 2 s think time, 60 s:

| | Requests/s | p50 ms | p99 ms | Peak RSS MB | CPU % |
|---|---|---|---|---|---|
| Java | 446 | 3.3 | 13.4 | 264 | 122 |
| Go | 428 | 46.9 | 531 | 19.6 | 178 |
| Rust | 446 | 5.3 | 28.6 | 4.2 | 145 |

**Gateway**, 4000 virtual users, 3 downstream calls per request (30 ± 10 ms each), 2 s think time, 60 s:

| | Requests/s | p50 ms | p99 ms | Peak RSS MB | CPU % |
|---|---|---|---|---|---|
| Java (no framework) | 1751 | 36.8 | 41.2 | 272 | 77 |
| Spring Boot, JVM | 1751 | 36.8 | 41.7 | 1359 | 105 |
| Spring Boot, native | 1749 | 37.6 | 61.1 | 462 | 131 |
| Go | 1752 | 36.7 | 41.1 | 29.5 | 31 |
| Rust | 1751 | 36.6 | 41.0 | 19.1 | 17 |

The mock downstream used 14 to 18% of its CPU in each gateway run, so it was not the limit.

What the numbers show:

- At 2048 bits Java leads, Go is close and Rust is about 30% slower. `num-bigint` is portable Rust, while `BigInteger` has JIT intrinsics and `math/big` has assembly.
- At 512 bits Go needs more CPU per prime. For the same 430 to 450 requests per second, Go uses 178% CPU, Rust 145% and Java 122%.
- Under load that extra CPU becomes latency. Go runs at 89% of its CPU limit, Java at 61%. Near saturation the queue grows much faster than the load, so Go has about the same throughput as Java with 40 times the p99.
- Go's garbage collector is not the main cost. The heap stays small, so the collector runs often, but it takes a few percent of the CPU. `GOGC=400` cuts the p99 from 531 to 107 ms with about the same CPU. Try it with `make server LANGS=go GOGC=400`.
- In the gateway the work is mostly waiting on other services, so all five give the same throughput and latency, except Spring native. The difference is the cost. For the same 1750 requests per second, Java uses 4.5 times the CPU of Rust, and Go 1.8 times.
- Spring Boot on the JVM costs about 35% more CPU than plain Java for the same gateway (105% against 77%), with the same latency. That is the framework: servlet filters, message converters, validation and observability hooks.
- The GraalVM native build starts in 0.05 s and is lean at low load (135 MB and 17% CPU at 100 users, against 519 MB and 26% on the JVM). Under sustained load it uses more CPU than the JVM (131% against 105%), a third of its memory, and has a worse p99 (61 ms against 42 ms). GraalVM Community has no JIT to re-optimize hot code and only the serial collector, so native is for fast startup and small footprints, not for peak throughput.
- JVM memory follows the heap limit more than the work. ParallelGC grows the heap toward 75% of the container when the load asks for it, and does not give it back. Plain Java peaked at 272 MB in this run and at 1160 MB in the Kubernetes run of the same gateway. Spring peaked at 1.36 to 1.46 GB. With `JAVA_OPTS=-Xmx128m` plain Java handles the same load at 358 MB with the same p99, so the live data is small. Go stays at 30 MB and Rust at 19 MB.
- At 1 CPU per app the Spring gateway fails at this load: it needs more than one core, the backlog fills the heap and the JVM runs out of memory. At 2 CPUs and 1 GB it passed on compose but was close to the limit, and on Kubernetes it lost half the throughput. At 1.5 GB it passes everywhere. Java, Go and Rust passed in all three setups.
- On Docker Desktop's Kubernetes the same runs land within noise of compose: gateway p99 41.0 to 43.7 ms for the JVM, Go and Rust, 62.8 ms for Spring native, and the same CPU. Go's server p99 moves a lot between runs near saturation (173 ms on Kubernetes, 531 ms on compose), while Java and Rust stay within a few milliseconds.

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

Each Dockerfile runs the language's tests before it builds the image, so `make build` fails on a broken test. The same prime and composite vectors (Mersenne primes, Carmichael numbers, a strong pseudoprime to bases 2, 3, 5 and 7) are checked in all three languages, and the gateway in every language is tested over HTTP against a stub downstream. CI also runs SonarQube Cloud with coverage from JaCoCo and Go.

With the toolchains installed:

```bash
cd apps/java   && ./mvnw verify           # also checks formatting
cd apps/spring && ./mvnw verify           # also checks formatting
cd apps/go     && go test ./...
cd apps/rust   && cargo test
cd mock        && go test ./...
cd loadgen     && go test ./...
cd dashboard && npm ci && npm run dev   # dashboard with hot reload on :5173, reads ../results
```
