# Methodology

How the numbers are produced, and what they can and cannot tell you.

## The work

Every language runs the same algorithm, written by hand, on top of its own big integer library.

1. Draw a random odd number with exactly `BITS` bits (top bit and low bit set).
2. Trial division by the 168 primes below 1000.
3. Miller-Rabin with `ROUNDS` random bases (20 by default). Squaring uses multiply then mod, the same in all three.
4. Repeat from 1 until a candidate passes.

| | Big integer library | Random source |
|---|---|---|
| Java | `java.math.BigInteger` | `ThreadLocalRandom` |
| Go | `math/big` | `math/rand/v2` ChaCha8, one per worker |
| Rust | `num-bigint` (pure Rust) | `rand::rng()`, thread local |

The built-in primality tests (`BigInteger.isProbablePrime`, `big.Int.ProbablyPrime`) are not used for the measurement because they run different algorithms. Rust uses `num-bigint` and not `rug`, because `rug` wraps GMP, and then the comparison would be against C.

Prime gaps are random, so the cost of one prime varies a lot. Compare throughput over hundreds of primes, not single latencies.

## Three scenarios

**Batch.** Each app generates `BATCH_COUNT` primes of `BATCH_BITS` with one platform thread per CPU, pulling from a shared counter. This measures raw throughput and scaling.

**Server.** Each app exposes the same HTTP contract:

```
GET /prime?bits=512   {"bits":512,"prime":"1340...","ms":1.84}
GET /stats            {"lang":"go","version":"1.27.3","cpus":4,"rss_mb":21.7,"cpu_seconds":12.3}
GET /health           ok
```

Without `bits` the server uses its default (`BITS`, 512 in server runs). An empty, non-numeric or out of range `bits` (64 to 8192) is a 400. Any method other than GET is a 405. The three servers are tested against the same cases.

| | Concurrency model |
|---|---|
| Java | `HttpServer` with a virtual thread per request |
| Go | `net/http`, a goroutine per connection |
| Rust | axum on tokio, CPU work handed to a rayon pool sized to the CPUs |

One shared load generator (`loadgen/`, Go) runs `USERS` virtual users. Each user sends a request, waits for the answer, sleeps `THINK_TIME` with ±50% jitter, and repeats. Users start evenly over `RAMP_UP`. This is a closed model: when the server slows down, users send less, so latency is the number to watch, not throughput alone. Requests slower than 30 seconds count as errors.

The servers run one at a time. The load generator has its own CPU limit (`LOADGEN_CPUS`).

**Gateway.** The same servers expose `GET /aggregate?account=acc-123`, the shape of a typical microservice: three calls in parallel to downstream REST APIs, parse the JSON, combine, answer.

```
GET /accounts/acc-123                130 bytes
GET /accounts/acc-123/transactions   40 transactions, 5.9 KB
GET /rates                           20 exchange rates, 305 bytes

{"accountId":"acc-123","currency":"USD","balanceMinor":1250075,"transactions":40,
 "spentMinor":...,"receivedMinor":...,"balanceEur":11500.69}
```

| | HTTP client | JSON | Fan-out |
|---|---|---|---|
| Java | `java.net.http.HttpClient` | Jackson 3 | one virtual thread per call |
| Go | `net/http` | `encoding/json` | one goroutine per call |
| Rust | reqwest | serde_json | `tokio::try_join!` |

The downstream is one shared mock (`mock/`, Go), so all three call the same thing. It waits `MOCK_LATENCY` ± `MOCK_JITTER` (30 ± 10 ms) before answering, can fail a share of calls (`MOCK_ERROR_RATE`), and has its own CPU limit (`MOCK_CPUS`). The load generator logs the mock's CPU use after each run, to show it is not the bottleneck. A downstream error or timeout (5 s) turns into a 502, which the load generator counts as an error.

For fairness, all three use HTTP/1.1 with keep-alive and a pool that does not cap connections: Go's default of 2 idle connections per host is raised to 4096, reqwest gets the same, and Java's client has no limit by default. Every request waits for the slowest of its three calls, so the gateway p50 starts around the mock's latency and the tail follows its jitter.

The load generator picks a random account per request (`acc-0` to `acc-9999`), with `GATEWAY_USERS` users and `GATEWAY_THINK_TIME` think time. In the CSV these rows have `mode=gateway` and `bits=0`.

## What is measured

One CSV per language and mode, in `results/`, with a row per second (`kind=interval`) and a final `kind=summary` row.

| Column | Source |
|---|---|
| `throughput_ops_s` | completed primes or responses / wall seconds |
| `latency_p50_ms` ... `latency_max_ms` | nearest-rank percentiles of every request in the interval (batch: time per prime) |
| `rss_mb` | `VmRSS` from `/proc/self/status`; peak value on the summary row |
| `cpu_pct` | delta of `utime + stime` from `/proc/self/stat` / wall time. 400 means four CPUs busy |
| `cpus` | CPUs the runtime sees (`availableProcessors`, `GOMAXPROCS`, `available_parallelism`) |

All three read memory and CPU the same way, from the same Linux files. In server and gateway mode the load generator polls `/stats` once per second.

## Limits

- Docker Desktop on a Mac runs Linux in a VM. Numbers are good for comparing the three languages on the same machine, not as absolute figures.
- CPU and memory are capped per container (`APP_CPUS`, `APP_MEMORY`). All three runtimes are container aware and size their thread pools to the cap.
- JVM memory is partly heap sizing policy: with a 2 GB container limit the default max heap is 512 MB, and G1 grows into it under load. In the gateway run, `JAVA_OPTS=-Xmx128m` handled the same load with 0 errors and the same latency, and peak RSS went from 473 to 337 MB. The rest is the JVM itself: metaspace, JIT code cache, carrier threads and HTTP client buffers.
- No JVM tuning: default GC, no warmup phase. The first seconds of the Java run include JIT compilation, and the per-second chart shows it. Set `JAVA_OPTS` to try other flags.
- Rust builds with `--release` and `codegen-units = 1`. Go builds with default flags.
- `num-bigint` is portable Rust. Java's `BigInteger` gets HotSpot intrinsics for its multiply loops and Go's `math/big` has assembly for amd64 and arm64. At 2048 bits and above, this matters more than the language.
