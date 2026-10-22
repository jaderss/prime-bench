# prime-bench

Benchmark of Java, Go and Rust on the same work, plus Spring Boot (JVM and GraalVM native) for the gateway. Built 2026-10-21. Public at github.com/jaderss/prime-bench. README.md has the results, METHODOLOGY.md the contracts and limits. This file is what a session needs to keep working on it.

## What exists

| Part | What it is |
|---|---|
| `apps/java` | Java 27, no framework. `HttpServer` + `HttpClient` on virtual threads, Jackson 3 (first dependency, shipped in `target/lib` next to the jar), Maven wrapper, JUnit 6, Spotless |
| `apps/go` | Go 1.27, standard library only. `metrics/` package is shared with `loadgen` and `mock` through `replace` directives |
| `apps/rust` | Rust 1.99, axum, tokio, rayon (CPU work), num-bigint, reqwest, serde |
| `apps/spring` | Spring Boot 4.1.1 on Java 25, gateway only. Two Dockerfile targets: `jvm` (Temurin) and `native` (GraalVM Community 25 on distroless) |
| `mock` | Go mock of three downstream APIs (account, transactions, rates), latency, jitter and error rate from env |
| `loadgen` | Go load generator, closed model, one CSV per run. `SCENARIO=prime\|gateway`, `MODE_PREFIX`, `CSV_STDOUT` for Kubernetes |
| `dashboard` | React 19, Vite 8, Recharts 3, nginx. Tabs: Docker Compose and Kubernetes (`#kubernetes`) |
| `k8s`, `scripts` | Plain manifests rendered with `.env` values by sed, `k8s-run.sh` (one language per run), `k8s-load.sh` (images into a kind node) |

Three scenarios: batch (primes on all CPUs), server (`GET /prime`), gateway (`GET /aggregate`, three downstream calls in parallel). Spring only runs the gateway (`GATEWAY_LANGS`).

## Commands

```bash
make bench              # about 12 min: batch + server for java go rust, gateway for those + spring + spring-native
make k8s-bench          # same server + gateway on Docker Desktop Kubernetes
make dashboard          # http://localhost:8080 (hard reload after a rebuild, the browser caches the bundle)
make k8s-down; make down
```

Smoke runs: `make bench BATCH_COUNT=40 USERS=200 GATEWAY_USERS=200 DURATION=15s`. CI runs the same with `APP_CPUS=2 LOADGEN_CPUS=1 MOCK_CPUS=1` and expects exactly 11 CSVs (3 batch, 3 server, 5 gateway). Change that count in `.github/workflows/ci.yml` when a language or scenario is added.

Tests per module: `apps/java` and `apps/spring` `./mvnw verify` (local JDK: `JAVA_HOME=~/Library/Java/JavaVirtualMachines/temurin-25.0.4/Contents/Home`), Go modules `go test ./...`, Rust `cargo test`, dashboard `npm run lint && npm run build`. Go and Rust have no local toolchain on this Mac: run them in `golang:1.27` / `rust:1.99` containers from the repo root (Go modules reference `../apps/go`).

## Conventions

- Java style from the java-engineer skill: no comments, no Javadoc, records, constructor injection, Palantir format via Spotless.
- Commits: one short present-tense paragraph, no AI attribution. prime-bench commits go straight to `main`. Other portfolio repos use a branch and PR; the user merges unless they say otherwise.
- `.env` holds versions and workload only, no secrets. Images are tagged with their version (`prime-bench/java:27`, `prime-bench/mock:go1.27`).
- Gateway contract, the same in all four: money in integer minor units (`balanceEurMinor`), a missing downstream field or any downstream failure is a 502 with body `downstream unavailable`, the detail goes to the server log without the account id.
- CSV schema and percentiles (nearest rank) must stay identical across languages. CI fails if headers differ.
- Dashboard series colors are validated with the dataviz skill: display order java, spring-native, spring, go, rust. Other orders fail the CVD checks.

## SonarQube Cloud

- Org `jaderss`, project `jaderss_prime-bench`. Scan runs in CI (`sonar` job, `sonar-project.properties`), Automatic Analysis is off. Coverage: JaCoCo for both Maven modules, Go profiles for `apps/go`, `mock`, `loadgen`. Rust and TypeScript coverage are not imported.
- The gate requires 80% coverage on new code. Glue code without tests fails it: test the HTTP layer (router or server started on port 0).
- API token in the macOS Keychain, service `sonarcloud-token`: `curl -H "Authorization: Bearer $(security find-generic-password -s sonarcloud-token -w)" ...`. Never print it.
- Resolved on purpose: `text:S8566` on `loadgen/go.mod` and `mock/go.mod` (false positive, no external deps), `kubernetes:S5332` plain HTTP in the cluster (accepted, TLS would distort the benchmark).

## Kubernetes

- Every kubectl call uses `--context docker-desktop`. The kubeconfig also has work clusters (`poupex`, `stage`): never run anything without that flag.
- Docker Desktop provisions the cluster with kind (node `desktop-control-plane`), which does not see Docker's images. `k8s-load.sh` streams them in through a short-lived root pod that runs the node's own `ctr` with the node's dynamic loader (no shell or chroot in the node images). On kubeadm (node `docker-desktop`) it skips.
- Do not run the manifests through kustomize: it strips quotes and env values like `"512"` become numbers.

## Gotchas met while building

- zsh: `set -- $var` and `$K` with spaces do not word split, `path` is tied to `PATH`, `======` is special. Use functions and arrays.
- Spring Boot 4: `RestClient.Builder` needs `spring-boot-starter-restclient`. start.spring.io writes the version as `4.1.1.RELEASE`, Maven wants `4.1.1`.
- GraalVM native: link with `--static-nolibc` (distroless has no libz), and copy the binary to a path other than `/app` (the builder image has that directory). Register downstream DTOs with `@RegisterReflectionForBinding`.
- Jackson missing fields: Java and Spring enable `FAIL_ON_MISSING_CREATOR_PROPERTIES` and `FAIL_ON_NULL_CREATOR_PROPERTIES`; Go uses pointer fields and `validate()`.
- The Go HTTP client default keeps 2 idle connections per host. All three clients use an uncapped keep-alive pool for fairness.
- Mac `head -q` does not exist; use `awk 'FNR == 1'` in scripts meant to run on both.

## Results to keep in mind

Gateway, 4000 users, same on Compose and Kubernetes: p99 41 ms for all except Spring native (60 ms); CPU Rust 22%, Go 37 to 40%, Java 88%, Spring 124%, Spring native 137%; memory Rust 20 MB, Go 30 MB, JVM 450 to 510 MB. Server (CPU-bound, 512 bits): Go p99 about 400 ms against 10 ms for Java, caused by more CPU per prime in `math/big`, not by GC.

## Open ideas

- Native image with a capped heap and at low load, to show where native wins.
- A density run on Kubernetes: replicas per language on one node.
- Rust coverage in Sonar (needs an LCOV report).
