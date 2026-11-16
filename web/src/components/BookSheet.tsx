// Book someone in yourself (a phone call, someone at the door) — or move a
// booking: pick the page and a free time, or any time you like.
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useData } from '../lib/data'
import { lengths } from '../lib/booking'
import { addDays, todayISO, weekday } from '../lib/format'
import type { Booking, Slots } from '../lib/types'
import { Icon } from './icons'
import { Select } from './Select'
import { Field, Loading, Modal, Segmented, Spinner, Switch, errMsg, useToast } from './ui'

const timeOf = (iso: string) => iso.slice(11, 16)

export function BookSheet({ typeID, move, onClose, onDone }: { typeID?: number; move?: Booking; onClose: () => void; onDone?: (b: Booking) => void }) {
  const { types } = useData()
  const { toast } = useToast()
  const navigate = useNavigate()
  const usable = (types || []).filter(t => t.status !== 'draft' || t.id === typeID)
  const [tid, setTid] = useState<number>(move?.type_id || typeID || usable[0]?.id || 0)
  const type = (types || []).find(t => t.id === tid)
  const [minutes, setMinutes] = useState<number>(move?.minutes || type?.minutes || 30)
  useEffect(() => { if (type && !move && !type.durations.includes(minutes)) setMinutes(type.durations[0]) }, [type]) // eslint-disable-line react-hooks/exhaustive-deps
  const [from, setFrom] = useState(todayISO())
  const [slots, setSlots] = useState<Slots | null>(null)
  const [day, setDay] = useState('')
  const [start, setStart] = useState('')
  const [any, setAny] = useState(false)
  const [anyDate, setAnyDate] = useState(todayISO())
  const [anyTime, setAnyTime] = useState('10:00')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [note, setNote] = useState('')
  const [tell, setTell] = useState(true)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!tid) return
    let live = true
    setSlots(null)
    api.get<Slots>(`${V1}/types/${tid}/slots?from=${from}&days=14&minutes=${minutes}`).then(s => {
      if (!live) return
      setSlots(s)
      if (!s.days.some(d => d.date === day)) { setDay(s.days[0]?.date || ''); setStart('') }
    }).catch(e => { if (live) { setSlots({ tz: '', minutes, days: [] }); toast(errMsg(e), 'err') } })
    return () => { live = false }
  }, [tid, from, minutes]) // eslint-disable-line react-hooks/exhaustive-deps

  const dayList = useMemo(() => Array.from({ length: 14 }, (_, i) => addDays(from, i)), [from])
  const free = new Map((slots?.days || []).map(d => [d.date, d.slots]))
  const when = any ? `${anyDate}T${anyTime}` : start

  const submit = async () => {
    if (!when) { toast('Pick a time first', 'err'); return }
    setBusy(true)
    try {
      let b: Booking
      if (move) {
        b = await api.post<Booking>(`${V1}/bookings/${move.id}/move`, { start: when, minutes, tell_guest: tell, force: any })
        toast('Moved')
      } else {
        b = await api.post<Booking>(`${V1}/bookings`, { type_id: tid, start: when, minutes, name, email, phone, note, tell_guest: tell && !!email, force: any })
        toast('Booked')
      }
      announceChange()
      onClose()
      if (onDone) onDone(b); else navigate(`/bookings/${b.id}`)
    } catch (e) { toast(errMsg(e), 'err'); setBusy(false) }
  }

  if (!types) return null
  if (usable.length === 0) return (
    <Modal title="Book someone in" onClose={onClose} size="sm">
      <p className="muted">Make a booking page first: it says how long a booking lasts, where it happens and when you are free.</p>
      <div className="row mt"><button type="button" className="btn primary" onClick={() => { onClose(); navigate('/types/new') }}>New booking page</button></div>
    </Modal>
  )
  return (
    <Modal title={move ? `Move: ${move.title} with ${move.name || move.email}` : 'Book someone in'} onClose={onClose} size="lg"
      footer={<>
        <span className="grow muted small">{when ? `${weekday(when.slice(0, 10), true)} ${when.slice(0, 10)} at ${when.slice(11, 16)}` : 'No time picked yet'}</span>
        <button type="button" className="btn" onClick={onClose}>Cancel</button>
        <button type="button" className="btn primary" onClick={submit} disabled={busy || !when || (!move && !name.trim() && !email.trim())}>{busy && <Spinner />}{move ? 'Move it' : 'Book'}</button>
      </>}>
      <div className="stack loose">
        {!move && <div className="two-col tight">
          <Field label="Booking page"><Select ariaLabel="Booking page" value={String(tid)} onChange={v => setTid(Number(v))} options={usable.map(t => ({ value: String(t.id), label: `${t.emoji ? t.emoji + ' ' : ''}${t.title}`, hint: lengths(t.durations) }))} /></Field>
          {type && type.durations.length > 1 && <Field label="How long"><Segmented size="sm" value={minutes} onChange={setMinutes} options={type.durations.map(d => ({ value: d, label: lengths([d]) }))} /></Field>}
        </div>}
        <div className="row wrap">
          <Switch checked={any} onChange={setAny} label="Any time" />
          <span className="small muted">{any ? 'Any day and time you like, even outside your hours or over another booking.' : 'Only the free times of the page — what people see on it.'}</span>
        </div>
        {any ? <div className="two-col tight">
          <Field label="Day"><input type="date" value={anyDate} onChange={e => setAnyDate(e.target.value)} /></Field>
          <Field label="Time"><input type="time" value={anyTime} onChange={e => setAnyTime(e.target.value)} /></Field>
        </div> : <div className="book-pick">
          <div className="book-days">
            <button type="button" className="btn ghost sm icon-only" aria-label="Earlier" disabled={from <= todayISO()} onClick={() => setFrom(addDays(from, -14) < todayISO() ? todayISO() : addDays(from, -14))}><Icon name="chevron-left" size={16} /></button>
            <div className="book-day-strip">
              {dayList.map(d => {
                const n = free.get(d)?.length || 0
                return <button key={d} type="button" className={`book-day ${d === day ? 'on' : ''} ${n ? '' : 'none'}`} disabled={!n} onClick={() => { setDay(d); setStart('') }}>
                  <span className="small">{weekday(d)}</span><b>{Number(d.slice(8))}</b><span className="tiny muted">{n ? n : '–'}</span>
                </button>
              })}
            </div>
            <button type="button" className="btn ghost sm icon-only" aria-label="Later" onClick={() => setFrom(addDays(from, 14))}><Icon name="chevron-right" size={16} /></button>
          </div>
          {!slots ? <Loading /> : (free.get(day) || []).length === 0 ? <p className="muted small">No free times in these two weeks. Try later, or switch on “Any time”.</p>
            : <div className="book-slots">{(free.get(day) || []).map(s => (
              <button key={s.start} type="button" className={`book-slot ${start === s.start ? 'on' : ''}`} onClick={() => setStart(s.start)}>{timeOf(s.start)}{type && type.capacity > 1 && <span className="tiny">{s.seats} left</span>}</button>
            ))}</div>}
          {slots?.calendar_warning && <p className="small warn-text">Pocket Calendar could not be checked: {slots.calendar_warning}</p>}
        </div>}
        {!move && <div className="two-col tight">
          <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} autoComplete="off" /></Field>
          <Field label="Email" help="For their confirmation and reminder."><input type="email" value={email} onChange={e => setEmail(e.target.value)} autoComplete="off" /></Field>
          <Field label="Phone"><input type="tel" value={phone} onChange={e => setPhone(e.target.value)} autoComplete="off" /></Field>
          <Field label="Your note" help="Only you see it."><input value={note} onChange={e => setNote(e.target.value)} /></Field>
        </div>}
        <div className="row"><Switch checked={tell} onChange={setTell} label="Email them" /><span className="small muted">{move ? 'Send them the new time (through Pocket Mail).' : 'Send them a confirmation (through Pocket Mail).'}</span></div>
      </div>
    </Modal>
  )
}
