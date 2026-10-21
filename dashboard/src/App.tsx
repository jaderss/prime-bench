import { useEffect, useState } from 'react'
import { Legend, SummaryBars, TimeSeries } from './charts'
import { loadRuns, timeSeries, type Mode, type Result, type Run } from './data'
import { formatNumber, formatRunId } from './format'

const describeServer = (r: Result) =>
  `${formatNumber(r.summary.concurrency)} virtual users requesting ${r.summary.bits}-bit primes over HTTP, ${r.summary.cpus} CPUs`

const describeGateway = (r: Result) =>
  `${formatNumber(r.summary.concurrency)} virtual users, each request fans out to 3 downstream HTTP calls and parses their JSON, ${r.summary.cpus} CPUs`

const MODES: { mode: Mode; title: string; describe: (r: Result) => string }[] = [
  {
    mode: 'batch',
    title: 'Batch',
    describe: (r) => `${formatNumber(r.summary.completed)} primes of ${r.summary.bits} bits, ${r.summary.concurrency} worker threads, ${r.summary.cpus} CPUs`,
  },
  {
    mode: 'server',
    title: 'Server under load',
    describe: describeServer,
  },
  {
    mode: 'gateway',
    title: 'Gateway calling downstream APIs',
    describe: describeGateway,
  },
  { mode: 'k8s-server', title: 'Server on Kubernetes', describe: describeServer },
  { mode: 'k8s-gateway', title: 'Gateway on Kubernetes', describe: describeGateway },
]

const COLUMNS: { label: string; value: (r: Result) => string }[] = [
  { label: 'Throughput /s', value: (r) => formatNumber(r.summary.throughput_ops_s) },
  { label: 'p50 ms', value: (r) => formatNumber(r.summary.latency_p50_ms) },
  { label: 'p95 ms', value: (r) => formatNumber(r.summary.latency_p95_ms) },
  { label: 'p99 ms', value: (r) => formatNumber(r.summary.latency_p99_ms) },
  { label: 'Max ms', value: (r) => formatNumber(r.summary.latency_max_ms) },
  { label: 'Peak RSS MB', value: (r) => formatNumber(r.summary.rss_mb) },
  { label: 'CPU %', value: (r) => formatNumber(r.summary.cpu_pct) },
  { label: 'Completed', value: (r) => formatNumber(r.summary.completed) },
  { label: 'Errors', value: (r) => formatNumber(r.summary.errors) },
  { label: 'Seconds', value: (r) => formatNumber(r.summary.elapsed_s) },
]

function ResultsTable({ results, showRun = false }: { results: Result[]; showRun?: boolean }) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {showRun && <th>Run</th>}
            {showRun && <th>Mode</th>}
            <th>Language</th>
            <th>Version</th>
            {COLUMNS.map((c) => <th key={c.label} className="num">{c.label}</th>)}
          </tr>
        </thead>
        <tbody>
          {results.map((r) => (
            <tr key={r.file}>
              {showRun && <td>{formatRunId(r.runId)}</td>}
              {showRun && <td>{r.mode}</td>}
              <td>{r.lang}</td>
              <td>{r.version}</td>
              {COLUMNS.map((c) => <td key={c.label} className="num">{c.value(r)}</td>)}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function ModeSection({ title, describe, results }: { title: string; describe: (r: Result) => string; results: Result[] }) {
  const langs = results.map((r) => r.lang)
  return (
    <section>
      <header className="section-header">
        <div>
          <h2>{title}</h2>
          <p className="secondary">{describe(results[0])}</p>
        </div>
        <Legend langs={langs} />
      </header>
      <div className="grid">
        <SummaryBars results={results} title="Throughput" unit="ops/s" value={(r) => r.summary.throughput_ops_s} />
        <SummaryBars results={results} title="p99 latency" unit="ms" value={(r) => r.summary.latency_p99_ms} />
        <SummaryBars results={results} title="Peak memory" unit="MB RSS" value={(r) => r.summary.rss_mb} />
      </div>
      <div className="grid">
        <TimeSeries data={timeSeries(results, 'throughput_ops_s')} langs={langs} title="Throughput per second" unit="ops/s" />
        <TimeSeries data={timeSeries(results, 'latency_p99_ms')} langs={langs} title="p99 latency per second" unit="ms" />
        <TimeSeries data={timeSeries(results, 'rss_mb')} langs={langs} title="Memory" unit="MB RSS" />
      </div>
      <ResultsTable results={results} />
    </section>
  )
}

const ENVIRONMENTS: { id: string; label: string; modes: Mode[]; empty: string }[] = [
  { id: 'compose', label: 'Docker Compose', modes: ['batch', 'server', 'gateway'], empty: 'make bench' },
  { id: 'kubernetes', label: 'Kubernetes', modes: ['k8s-server', 'k8s-gateway'], empty: 'make k8s-bench' },
]

const showEnvironment = (id: string) => {
  window.location.hash = id
}

const environmentFromHash = () => ENVIRONMENTS.find((e) => `#${e.id}` === window.location.hash) ?? ENVIRONMENTS[0]

export default function App() {
  const [runs, setRuns] = useState<Run[]>()
  const [environment, setEnvironment] = useState(environmentFromHash)
  const [selectedRunId, setSelectedRunId] = useState<string>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    loadRuns()
      .then(setRuns)
      .catch((e: unknown) => setError(String(e)))
    const onHashChange = () => setEnvironment(environmentFromHash())
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  const environmentRuns = (runs ?? [])
    .map((r) => ({ ...r, results: r.results.filter((result) => environment.modes.includes(result.mode)) }))
    .filter((r) => r.results.length > 0)
  const run = environmentRuns.find((r) => r.id === selectedRunId) ?? environmentRuns[0]
  const history = environmentRuns.flatMap((r) => r.results)

  const selectEnvironment = (id: string) => {
    showEnvironment(id)
    setSelectedRunId(undefined)
  }

  return (
    <main>
      <header className="page-header">
        <div>
          <h1>Prime Bench</h1>
          <p className="secondary">Java, Go and Rust on CPU-bound prime generation and an I/O-bound gateway, with Spring Boot on the JVM and as a native image in the gateway.</p>
        </div>
        {environmentRuns.length > 0 && (
          <label className="run-picker">
            <span className="secondary">Run</span>
            <select value={run?.id} onChange={(e) => setSelectedRunId(e.target.value)}>
              {environmentRuns.map((r) => <option key={r.id} value={r.id}>{formatRunId(r.id)}</option>)}
            </select>
          </label>
        )}
      </header>

      <nav className="tabs" role="tablist" aria-label="Where the benchmark ran">
        {ENVIRONMENTS.map((e) => (
          <button
            key={e.id}
            role="tab"
            aria-selected={e.id === environment.id}
            className="tab"
            onClick={() => selectEnvironment(e.id)}
          >
            {e.label}
          </button>
        ))}
      </nav>

      {error && <p className="notice">Could not load results: {error}</p>}
      {runs && environmentRuns.length === 0 && (
        <p className="notice">No {environment.label} results yet. Run <code>{environment.empty}</code> and reload.</p>
      )}

      {run && MODES.filter(({ mode }) => environment.modes.includes(mode)).map(({ mode, title, describe }) => {
        const results = run.results.filter((r) => r.mode === mode)
        return results.length > 0 && <ModeSection key={mode} title={title} describe={describe} results={results} />
      })}

      {history.length > 0 && (
        <section>
          <header className="section-header">
            <div>
              <h2>All runs</h2>
              <p className="secondary">Every {environment.label} summary on disk. Compare language versions across runs here.</p>
            </div>
          </header>
          <ResultsTable results={history} showRun />
        </section>
      )}
    </main>
  )
}
