import { useEffect, useState } from 'react'
import { Legend, SummaryBars, TimeSeries } from './charts'
import { loadRuns, timeSeries, type Mode, type Result, type Run } from './data'
import { formatNumber, formatRunId } from './format'

const MODES: { mode: Mode; title: string; describe: (r: Result) => string }[] = [
  {
    mode: 'batch',
    title: 'Batch',
    describe: (r) => `${formatNumber(r.summary.completed)} primes of ${r.summary.bits} bits, ${r.summary.concurrency} worker threads, ${r.summary.cpus} CPUs`,
  },
  {
    mode: 'server',
    title: 'Server under load',
    describe: (r) => `${formatNumber(r.summary.concurrency)} virtual users requesting ${r.summary.bits}-bit primes over HTTP, ${r.summary.cpus} CPUs`,
  },
  {
    mode: 'gateway',
    title: 'Gateway calling downstream APIs',
    describe: (r) =>
      `${formatNumber(r.summary.concurrency)} virtual users, each request fans out to 3 downstream HTTP calls and parses their JSON, ${r.summary.cpus} CPUs`,
  },
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

export default function App() {
  const [runs, setRuns] = useState<Run[]>()
  const [runId, setRunId] = useState<string>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    loadRuns()
      .then((loaded) => {
        setRuns(loaded)
        setRunId(loaded[0]?.id)
      })
      .catch((e: unknown) => setError(String(e)))
  }, [])

  const run = runs?.find((r) => r.id === runId)
  const history = runs?.flatMap((r) => r.results) ?? []

  return (
    <main>
      <header className="page-header">
        <div>
          <h1>Prime Bench</h1>
          <p className="secondary">Java, Go and Rust generating big probable primes with the same Miller-Rabin test.</p>
        </div>
        {runs && runs.length > 0 && (
          <label className="run-picker">
            <span className="secondary">Run</span>
            <select value={runId} onChange={(e) => setRunId(e.target.value)}>
              {runs.map((r) => <option key={r.id} value={r.id}>{formatRunId(r.id)}</option>)}
            </select>
          </label>
        )}
      </header>

      {error && <p className="notice">Could not load results: {error}</p>}
      {runs?.length === 0 && <p className="notice">No results yet. Run <code>make bench</code> and reload.</p>}

      {run && MODES.map(({ mode, title, describe }) => {
        const results = run.results.filter((r) => r.mode === mode)
        return results.length > 0 && <ModeSection key={mode} title={title} describe={describe} results={results} />
      })}

      {history.length > 0 && (
        <section>
          <header className="section-header">
            <div>
              <h2>All runs</h2>
              <p className="secondary">Every summary on disk. Compare language versions across runs here.</p>
            </div>
          </header>
          <ResultsTable results={history} showRun />
        </section>
      )}
    </main>
  )
}
