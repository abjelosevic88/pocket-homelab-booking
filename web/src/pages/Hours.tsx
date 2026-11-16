// Weekly hours people can book in, and the days that differ (days off,
// a late start). A page uses one set of hours; most people need one.
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange } from '../lib/data'
import { WEEKDAYS, ZONES } from '../lib/booking'
import { fmtDate, todayISO, weekday } from '../lib/format'
import type { Override, Schedule, Span } from '../lib/types'
import { useCrumb } from '../components/Layout'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Confirm, Field, Loading, Spinner, Switch, errMsg, useAsync, useToast } from '../components/ui'

export default function Hours() {
  const { settings } = useApp()
  const { toast } = useToast()
  const { data, reload } = useAsync(() => api.get<{ schedules: Schedule[]; tz: string }>(`${V1}/schedules`), [])
  useCrumb(undefined)
  const add = async () => {
    try { await api.post(`${V1}/schedules`, { name: 'Other hours' }); await reload(); toast('Added') } catch (e) { toast(errMsg(e), 'err') }
  }
  return (
    <div className="page stack loose">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Hours & days off</h1><p className="muted">People can only book inside these hours. Times already taken by another booking{settings?.check_calendar ? ' or by something in Pocket Calendar' : ''} are left out.</p></div>
        <button type="button" className="btn" onClick={add}><Icon name="plus" size={15} />Another set of hours</button>
      </div>
      {settings && <div className="callout info"><Icon name="calendar" size={15} /><div>{settings.check_calendar
        ? <>Times you are busy in <b>Pocket Calendar</b> cannot be booked{settings.calendar_via ? '' : <> — but Pocket Calendar is not connected yet ({settings.calendar_detail || 'Settings → Connected apps'})</>}.</>
        : <>Pocket Calendar is not checked for busy times.</>} <Link to="/settings/calendar">Change</Link></div></div>}
      {!data ? <Loading /> : data.schedules.map(s => <ScheduleCard key={s.id} s={s} only={data.schedules.length === 1} onChanged={reload} />)}
    </div>
  )
}

function ScheduleCard({ s: s0, only, onChanged }: { s: Schedule; only: boolean; onChanged: () => void }) {
  const { toast } = useToast()
  const [s, setS] = useState(s0)
  const [busy, setBusy] = useState(false)
  const [del, setDel] = useState(false)
  const [newDay, setNewDay] = useState('')
  useEffect(() => setS(s0), [s0])
  const dirty = JSON.stringify(s) !== JSON.stringify(s0)
  const setDay = (d: string, spans: Span[]) => setS(x => ({ ...x, hours: { ...x.hours, [d]: spans } }))
  const setOv = (i: number, o: Partial<Override>) => setS(x => ({ ...x, overrides: x.overrides.map((v, j) => j === i ? { ...v, ...o } : v) }))
  const save = async () => {
    setBusy(true)
    try { await api.put(`${V1}/schedules/${s.id}`, s); announceChange(); onChanged(); toast('Saved') } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const addDay = () => {
    if (!newDay || s.overrides.some(o => o.day === newDay)) return
    setS(x => ({ ...x, overrides: [...x.overrides, { day: newDay, hours: [], note: '' }].sort((a, b) => a.day.localeCompare(b.day)) }))
    setNewDay('')
  }
  const upcoming = s.overrides.map((o, i) => ({ o, i })).filter(x => x.o.day >= todayISO())
  return (
    <section className="card stack">
      <div className="row wrap">
        <input className="grow title-in" value={s.name} onChange={e => setS({ ...s, name: e.target.value })} aria-label="Name of these hours" />
        <div className="tz-pick"><Select ariaLabel="Time zone" value={s.tz} onChange={v => setS({ ...s, tz: v })} options={[...new Set([s.tz, ...ZONES])].filter(Boolean).map(z => ({ value: z, label: z }))} /></div>
        {!only && <Switch size="sm" checked={s.is_default} onChange={v => setS({ ...s, is_default: v })} label="Default" />}
        {!only && <button type="button" className="btn ghost sm icon-only" aria-label="Delete these hours" onClick={() => setDel(true)}><Icon name="trash" size={15} /></button>}
      </div>
      <p className="muted small">{s.pages ? `${s.pages} ${s.pages === 1 ? 'page uses' : 'pages use'} these hours.` : 'No page uses these hours yet.'}{s.is_default ? ' New pages start with them.' : ''}</p>
      <div className="week-hours">
        {WEEKDAYS.map(({ d, label }) => {
          const spans = s.hours[d] || []
          return (
            <div key={d} className={`wh-row ${spans.length ? '' : 'off'}`}>
              <Switch size="sm" checked={spans.length > 0} onChange={v => setDay(d, v ? [['09:00', '17:00']] : [])} label={label} />
              <span className="wh-day">{label}</span>
              <div className="wh-spans">
                {spans.length === 0 ? <span className="muted small">Not bookable</span> : spans.map((sp, i) => (
                  <span key={i} className="wh-span">
                    <input type="time" value={sp[0]} onChange={e => setDay(d, spans.map((x, j) => j === i ? [e.target.value, x[1]] : x))} aria-label={`${label} from`} />
                    <span className="muted">–</span>
                    <input type="time" value={sp[1]} onChange={e => setDay(d, spans.map((x, j) => j === i ? [x[0], e.target.value] : x))} aria-label={`${label} until`} />
                    <button type="button" className="btn ghost sm icon-only" aria-label="Remove" onClick={() => setDay(d, spans.filter((_, j) => j !== i))}><Icon name="x" size={13} /></button>
                  </span>
                ))}
                {spans.length > 0 && spans.length < 4 && <button type="button" className="btn ghost sm" onClick={() => setDay(d, [...spans, [spans[spans.length - 1][1] < '18:00' ? spans[spans.length - 1][1] : '18:00', '20:00']])}><Icon name="plus" size={13} />Add</button>}
              </div>
            </div>
          )
        })}
      </div>
      <h3 className="card-h">Days that differ</h3>
      <p className="muted small">A day off, a holiday, or other hours on one day.</p>
      {upcoming.length === 0 && <p className="muted small">None coming up.</p>}
      {upcoming.map(({ o, i }) => (
        <div key={o.day} className="ov-row">
          <b className="ov-day">{weekday(o.day)}, {fmtDate(o.day)}</b>
          <Switch size="sm" checked={o.hours.length > 0} onChange={v => setOv(i, { hours: v ? [['09:00', '13:00']] : [] })} label="Bookable" />
          {o.hours.length === 0 ? <span className="muted small">Day off</span> : o.hours.map((sp, j) => (
            <span key={j} className="wh-span">
              <input type="time" value={sp[0]} onChange={e => setOv(i, { hours: o.hours.map((x, k) => k === j ? [e.target.value, x[1]] : x) })} aria-label="From" />
              <span className="muted">–</span>
              <input type="time" value={sp[1]} onChange={e => setOv(i, { hours: o.hours.map((x, k) => k === j ? [x[0], e.target.value] : x) })} aria-label="Until" />
            </span>
          ))}
          <input className="grow" value={o.note || ''} onChange={e => setOv(i, { note: e.target.value })} placeholder="Why (only you see it)" aria-label="Note" />
          <button type="button" className="btn ghost sm icon-only" aria-label="Remove the day" onClick={() => setS(x => ({ ...x, overrides: x.overrides.filter((_, k) => k !== i) }))}><Icon name="trash" size={14} /></button>
        </div>
      ))}
      <div className="row wrap">
        <Field label="Add a day"><input type="date" value={newDay} min={todayISO()} onChange={e => setNewDay(e.target.value)} /></Field>
        <button type="button" className="btn sm align-end" onClick={addDay} disabled={!newDay}><Icon name="plus" size={14} />Add the day</button>
      </div>
      <div className="row">
        <button type="button" className="btn primary" onClick={save} disabled={!dirty || busy}>{busy && <Spinner />}Save</button>
        {dirty && <button type="button" className="btn ghost" onClick={() => setS(s0)}>Undo changes</button>}
      </div>
      {del && <Confirm title={`Delete “${s.name}”?`} message="Pages that use these hours switch to your default hours." confirmLabel="Delete"
        onCancel={() => setDel(false)} onConfirm={async () => { try { await api.del(`${V1}/schedules/${s.id}`); onChanged(); announceChange() } catch (e) { toast(errMsg(e), 'err') } setDel(false) }} />}
    </section>
  )
}
