import { Bar, BarChart, CartesianGrid, Cell, LabelList, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { LANGS, type Lang, type Result, type SeriesPoint } from './data'
import { formatNumber } from './format'
import { useTheme } from './theme'

type TooltipEntry = { dataKey?: string | number; value?: number | string; color?: string; payload?: Record<string, unknown> }

function ChartTooltip({ active, payload, label, unit, title }: {
  active?: boolean
  payload?: TooltipEntry[]
  label?: string | number
  unit: string
  title: (label: string | number | undefined, payload: TooltipEntry[]) => string
}) {
  if (!active || !payload?.length) return null
  return (
    <div className="tooltip">
      <div className="tooltip-title">{title(label, payload)}</div>
      {payload.map((entry) => (
        <div className="tooltip-row" key={String(entry.dataKey)}>
          <span className="line-key" style={{ background: entry.color }} />
          <strong>{formatNumber(Number(entry.value))} {unit}</strong>
          <span className="secondary">{String(entry.payload?.lang ?? entry.dataKey)}</span>
        </div>
      ))}
    </div>
  )
}

export function SummaryBars({ results, title, unit, value }: {
  results: Result[]
  title: string
  unit: string
  value: (result: Result) => number
}) {
  const theme = useTheme()
  const data = results.map((result) => ({ lang: result.lang, value: value(result) }))
  return (
    <figure className="chart">
      <figcaption>{title} <span className="unit">({unit})</span></figcaption>
      <ResponsiveContainer width="100%" height={40 + data.length * 36}>
        <BarChart data={data} layout="vertical" margin={{ top: 4, right: 64, bottom: 4, left: 0 }} barCategoryGap={8}>
          <XAxis type="number" hide domain={[0, 'dataMax']} />
          <YAxis type="category" dataKey="lang" width={96} tickLine={false} axisLine={{ stroke: theme.axis }} tick={{ fill: theme.muted, fontSize: 12 }} />
          <Tooltip cursor={false} content={<ChartTooltip unit={unit} title={() => title} />} />
          <Bar dataKey="value" radius={[0, 4, 4, 0]} isAnimationActive={false} stroke={theme.surface} strokeWidth={2}>
            {data.map((entry) => <Cell key={entry.lang} fill={theme.series[entry.lang]} />)}
            <LabelList dataKey="value" position="right" formatter={(v) => formatNumber(Number(v))} className="bar-label" />
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </figure>
  )
}

export function TimeSeries({ data, langs, title, unit }: { data: SeriesPoint[]; langs: Lang[]; title: string; unit: string }) {
  const theme = useTheme()
  return (
    <figure className="chart">
      <figcaption>{title} <span className="unit">({unit})</span></figcaption>
      <ResponsiveContainer width="100%" height={220}>
        <LineChart data={data} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
          <CartesianGrid vertical={false} stroke={theme.grid} />
          <XAxis dataKey="t" type="number" domain={['dataMin', 'dataMax']} tickLine={false} axisLine={{ stroke: theme.axis }} tick={{ fill: theme.muted, fontSize: 12 }} unit="s" />
          <YAxis width={48} tickLine={false} axisLine={false} tick={{ fill: theme.muted, fontSize: 12 }} tickFormatter={(v) => formatNumber(Number(v))} />
          <Tooltip cursor={{ stroke: theme.axis }} content={<ChartTooltip unit={unit} title={(t) => `${t} s`} />} />
          {LANGS.filter((lang) => langs.includes(lang)).map((lang) => (
            <Line key={lang} dataKey={lang} name={lang} stroke={theme.series[lang]} strokeWidth={2} dot={false} activeDot={{ r: 4, stroke: theme.surface, strokeWidth: 2 }} isAnimationActive={false} connectNulls />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </figure>
  )
}

export function Legend({ langs }: { langs: Lang[] }) {
  const theme = useTheme()
  return (
    <ul className="legend">
      {langs.map((lang) => (
        <li key={lang}><span className="line-key" style={{ background: theme.series[lang] }} />{lang}</li>
      ))}
    </ul>
  )
}
