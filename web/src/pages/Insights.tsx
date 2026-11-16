// How booking goes: bookings per week, which pages, what time of day, how
// many came, were paid or cancelled.
import { useState } from 'react'
import { api, V1 } from '../lib/api'
import { money } from '../lib/booking'
import { fmtDate } from '../lib/format'
import type { Insights as InsightsT } from '../lib/types'
import { useCrumb } from '../components/Layout'
import { Bars, HBars } from '../components/Chart'
import { Empty, Loading, Segmented, useAsync } from '../components/ui'

export default function Insights() {
  const [weeks, setWeeks] = useState(12)
  const { data } = useAsync(() => api.get<InsightsT>(`${V1}/insights?weeks=${weeks}`), [weeks])
  useCrumb(undefined)
  const s = data?.statuses || {}
  const booked = (s.done || 0) + (s.confirmed || 0) + (s.no_show || 0) + (s.pending || 0)
  const off = (s.cancelled || 0) + (s.declined || 0)
  const past = (s.done || 0) + (s.no_show || 0)
  const hours = (data?.weeks || []).reduce((n, w) => n + w.minutes, 0) / 60
  return (
    <div className="page stack loose">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Insights</h1><p className="muted">{data ? `Since ${fmtDate(data.from)}` : ' '}</p></div>
        <Segmented size="sm" value={weeks} onChange={setWeeks} options={[{ value: 4, label: '4 weeks' }, { value: 12, label: '12 weeks' }, { value: 26, label: '6 months' }, { value: 52, label: 'A year' }]} label="Over" />
      </div>
      {!data ? <Loading /> : booked + off === 0 ? <Empty icon="chart" title="Nothing to count yet"><p>Once people book, you see here how many, for what and when.</p></Empty> : <>
        <div className="stats-row">
          <div className="stat"><div className="stat-l">Bookings</div><div className="stat-v tnum">{booked}</div><div className="stat-s small">{hours.toFixed(hours < 10 ? 1 : 0)} hours</div></div>
          <div className="stat"><div className="stat-l">Cancelled</div><div className="stat-v tnum">{off}</div><div className="stat-s small">{booked + off ? Math.round(off * 100 / (booked + off)) : 0}% of all</div></div>
          <div className="stat"><div className="stat-l">Did not come</div><div className="stat-v tnum">{s.no_show || 0}</div><div className="stat-s small">{past ? Math.round((s.no_show || 0) * 100 / past) : 0}% of those that happened</div></div>
          <div className="stat"><div className="stat-l">Paid</div><div className="stat-v tnum">{data.paid ? money(data.paid, data.currency) : '—'}</div><div className="stat-s small">{data.views} page visits</div></div>
        </div>
        <section className="card">
          <h2 className="card-h">Bookings per week</h2>
          <Bars items={data.weeks.map(w => ({ label: fmtDate(w.start, false), value: w.booked, hint: `${w.cancelled} cancelled` }))} format={v => String(v)} height={190} />
        </section>
        <div className="two-grid">
          <section className="card">
            <h2 className="card-h">By page</h2>
            <HBars items={data.types.map(t => ({ label: `${t.emoji ? t.emoji + ' ' : ''}${t.title}`, value: t.count }))} format={v => String(v)} />
          </section>
          <section className="card">
            <h2 className="card-h">Time of day</h2>
            <Bars items={data.hours.map((n, h) => ({ label: String(h).padStart(2, '0'), value: n })).filter((x, h) => x.value > 0 || (h >= 8 && h <= 18))} format={v => String(v)} height={150} />
          </section>
        </div>
      </>}
    </div>
  )
}
