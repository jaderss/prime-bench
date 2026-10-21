export const LANGS = ['java', 'go', 'rust'] as const
export type Lang = (typeof LANGS)[number]
export type Mode = 'batch' | 'server' | 'gateway'

const NUMERIC = [
  'elapsed_s', 'cpus', 'bits', 'concurrency', 'completed', 'errors', 'throughput_ops_s',
  'latency_p50_ms', 'latency_p95_ms', 'latency_p99_ms', 'latency_max_ms', 'rss_mb', 'cpu_pct',
] as const

type Numeric = (typeof NUMERIC)[number]

export type Row = Record<Numeric, number> & {
  kind: 'interval' | 'summary'
  lang: Lang
  lang_version: string
  mode: Mode
  run_id: string
}

export type Result = {
  file: string
  runId: string
  mode: Mode
  lang: Lang
  version: string
  intervals: Row[]
  summary: Row
}

export type Run = {
  id: string
  results: Result[]
}

export function parseCsv(text: string): Row[] {
  const [header, ...lines] = text.trim().split(/\r?\n/)
  const columns = header.split(',')
  return lines.map((line) => {
    const cells = line.split(',')
    const row: Record<string, string | number> = {}
    columns.forEach((column, i) => {
      row[column] = (NUMERIC as readonly string[]).includes(column) ? Number(cells[i]) : cells[i]
    })
    return row as Row
  })
}

function toResult(file: string, rows: Row[]): Result | undefined {
  const summary = rows.find((row) => row.kind === 'summary')
  if (!summary || !(LANGS as readonly string[]).includes(summary.lang)) return undefined
  return {
    file,
    runId: summary.run_id,
    mode: summary.mode,
    lang: summary.lang,
    version: summary.lang_version,
    intervals: rows.filter((row) => row.kind === 'interval'),
    summary,
  }
}

export async function loadRuns(): Promise<Run[]> {
  const listing: { name: string; type: string }[] = await fetch('results/').then((r) => r.json())
  const files = listing.filter((entry) => entry.type === 'file' && entry.name.endsWith('.csv'))
  const results = await Promise.all(
    files.map(async ({ name }) => toResult(name, parseCsv(await fetch(`results/${encodeURIComponent(name)}`).then((r) => r.text())))),
  )
  const byRun = Map.groupBy(results.filter((r): r is Result => r !== undefined), (r) => r.runId)
  return [...byRun]
    .map(([id, runResults]) => ({ id, results: runResults.sort((a, b) => LANGS.indexOf(a.lang) - LANGS.indexOf(b.lang)) }))
    .sort((a, b) => b.id.localeCompare(a.id))
}

export type SeriesPoint = { t: number } & Partial<Record<Lang, number>>

export function timeSeries(results: Result[], metric: Numeric): SeriesPoint[] {
  const points = new Map<number, SeriesPoint>()
  for (const result of results) {
    for (const row of result.intervals) {
      const t = Math.round(row.elapsed_s)
      const point = points.get(t) ?? { t }
      point[result.lang] = row[metric]
      points.set(t, point)
    }
  }
  return [...points.values()].sort((a, b) => a.t - b.t)
}
