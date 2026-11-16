// The bookings: what is coming up (by day), what waits for your yes, what
// happened — with the booking you opened beside the list (on a phone, on
// its own page).
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData, useOnChange } from '../lib/data'
import { APP_INFO, STATUS, TARGETS, locInfo, money } from '../lib/booking'
import { ago, fmtDate, fmtDateTime, todayISO, weekday, addDays } from '../lib/format'
import type { AppState, Booking, Made } from '../lib/types'
import { openCreate, useCrumb } from '../components/Layout'
import { BookSheet } from '../components/BookSheet'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Avatar, Confirm, Empty, Loading, type MenuItem, MenuButton, Modal, Segmented, Spinner, Switch, errMsg, useAsync, useDebounce, useMediaQuery, useToast } from '../components/ui'

type List = { bookings: Booking[]; total: number }

const VIEWS = [{ value: 'upcoming', label: 'Upcoming' }, { value: 'pending', label: 'To confirm' }, { value: 'past', label: 'Past' }, { value: 'cancelled', label: 'Cancelled' }, { value: 'all', label: 'All' }]

export function dayHeading(d: string): string {
  const t = todayISO()
  if (d === t) return 'Today'
  if (d === addDays(t, 1)) return 'Tomorrow'
  if (d === addDays(t, -1)) return 'Yesterday'
  return `${weekday(d, true)}, ${fmtDate(d, d.slice(0, 4) !== t.slice(0, 4))}`
}

export default function Bookings() {
  const { id } = useParams()
  const openID = Number(id || 0)
  const [sp, setSp] = useSearchParams()
  const navigate = useNavigate()
  const { types, summary } = useData()
  const wide = useMediaQuery('(min-width: 1100px)')
  const view = sp.get('view') || 'upcoming'
  const typeID = Number(sp.get('type') || 0)
  const [q, setQ] = useState(sp.get('q') || '')
  const dq = useDebounce(q, 250)
  const [limit, setLimit] = useState(100)
  const qs = `view=${view}&type_id=${typeID || ''}&q=${encodeURIComponent(dq)}&limit=${limit}`
  const { data, reload } = useAsync(() => api.get<List>(`${V1}/bookings?${qs}`), [qs])
  useOnChange(reload)
  useEffect(() => { const t = window.setInterval(() => { if (document.visibilityState === 'visible') reload() }, 60000); return () => window.clearInterval(t) }, [reload])
  const set = (k: string, v: string) => { const n = new URLSearchParams(sp); if (v) n.set(k, v); else n.delete(k); setSp(n, { replace: true }) }
  const keep = sp.toString() ? `?${sp.toString()}` : ''
  const open = (bid: number) => navigate(`/bookings/${bid}${keep}`)
  const cur = types?.find(t => t.id === typeID)
  useCrumb(openID && !wide ? 'Booking' : cur?.title)

  const groups: [string, Booking[]][] = []
  for (const b of data?.bookings || []) {
    const last = groups[groups.length - 1]
    if (last && last[0] === b.date) last[1].push(b)
    else groups.push([b.date, [b]])
  }

  const list = (
    <div className="inbox-list">
      {view === 'upcoming' && !typeID && !q && summary && <div className="stats-row tight">
        <div className="stat"><div className="stat-l">Today</div><div className="stat-v tnum">{summary.today}</div></div>
        <div className="stat"><div className="stat-l">Next 7 days</div><div className="stat-v tnum">{summary.this_week}</div></div>
        <button type="button" className={`stat stat-btn ${summary.pending ? 'warn' : ''}`} onClick={() => set('view', 'pending')}><div className="stat-l">To confirm</div><div className="stat-v tnum">{summary.pending}</div></button>
      </div>}
      <div className="inbox-filters">
        <div className="row wrap filters">
          <input className="search-in" value={q} onChange={e => setQ(e.target.value)} placeholder="Search by name, email, what they wrote…" aria-label="Search bookings" />
          <Select ariaLabel="Booking page" value={String(typeID || '')} onChange={v => set('type', v)} placeholder="Every page" clearLabel="Every page"
            options={(types || []).map(t => ({ value: String(t.id), label: `${t.emoji ? t.emoji + ' ' : ''}${t.title}`, hint: t.upcoming ? `${t.upcoming} coming` : '' }))} />
        </div>
        <div className="scroll-x"><Segmented size="sm" value={view} onChange={v => set('view', v === 'upcoming' ? '' : v)} options={VIEWS} label="Show" /></div>
      </div>
      {!data ? <Loading /> : data.bookings.length === 0 ? <Empty icon="calendar" title={q ? 'Nothing found' : view === 'pending' ? 'Nothing waits for you' : view === 'upcoming' ? 'Nothing coming up' : 'Nothing here'}>
        {view === 'upcoming' && !q && <>{types && types.length === 0
          ? <><p>Make a booking page, share its link, and the times people book land here.</p><button type="button" className="btn primary mt" onClick={() => openCreate('page')}>Make your first booking page</button></>
          : <><p>Share a booking page, send someone a personal link, or book someone in yourself.</p><div className="row wrap mt" style={{ justifyContent: 'center' }}><button type="button" className="btn primary" onClick={() => openCreate('book')}>Book someone in</button><button type="button" className="btn" onClick={() => openCreate('link')}>Send a booking link</button></div></>}</>}
      </Empty> : <div className="stack">
        {groups.map(([d, bs]) => (
          <section key={d} className="card flush">
            <h3 className="day-h">{dayHeading(d)}<span className="muted"> · {bs.length}</span></h3>
            <ul className="list">{bs.map(b => <BookingRow key={b.id} b={b} active={b.id === openID} onOpen={() => open(b.id)} />)}</ul>
          </section>
        ))}
      </div>}
      {data && data.total > data.bookings.length && <button type="button" className="btn ghost sm" onClick={() => setLimit(l => l + 200)}>Show more ({data.bookings.length} of {data.total})</button>}
    </div>
  )
  const head = <ListHead view={view} count={data?.total} type={cur?.title} />
  if (!wide) {
    if (openID) return <BookingPanel key={openID} id={openID} back={() => navigate(`/${keep}`)} />
    return <div className="page stack loose">{head}{list}</div>
  }
  return (
    <div className="inbox">
      <div className="inbox-left">{head}{list}</div>
      <div className="inbox-right">
        {openID ? <BookingPanel key={openID} id={openID} /> : <div className="inbox-empty muted"><Icon name="calendar" size={28} /><p>Open a booking to see who and what, confirm or move it, or send it on to your other apps.</p></div>}
      </div>
    </div>
  )
}

function ListHead({ view, count, type }: { view: string; count?: number; type?: string }) {
  const label = VIEWS.find(v => v.value === view)?.label || 'Bookings'
  return <div className="page-head"><div className="grow"><h1 className="page-title">{type || (view === 'upcoming' ? 'Coming up' : label)}</h1><p className="muted">{count === undefined ? ' ' : count === 0 ? 'Nothing to show.' : `${count.toLocaleString()} ${count === 1 ? 'booking' : 'bookings'}`}</p></div>
    <button type="button" className="btn" onClick={() => openCreate('link')}><Icon name="link" size={15} />Send a link</button>
  </div>
}

function BookingRow({ b, active, onOpen }: { b: Booking; active: boolean; onOpen: () => void }) {
  const st = STATUS[b.status]
  const who = b.name || b.email || `Booking #${b.id}`
  return (
    <li>
      <button type="button" className={`list-row booking-row ${active ? 'active' : ''} ${b.status === 'pending' ? 'is-new' : ''} ${b.status === 'cancelled' || b.status === 'declined' ? 'is-off' : ''}`} onClick={onOpen}>
        <span className="bk-time tnum"><b>{b.time}</b><span className="muted small">{b.end_time}</span></span>
        <span className="grow row-main">
          <span className="row-title"><span className="ellipsis">{who}</span>{b.status !== 'confirmed' && <span className={`pill sm ${st.cls}`}>{st.label}</span>}</span>
          <span className="muted small ellipsis">{b.emoji} {b.title} · {b.minutes} min{b.price ? ` · ${money(b.price, b.currency)}${b.paid ? ' paid' : ''}` : ''}</span>
        </span>
        <span className="row-side">
          <Icon name={locInfo(b.location_kind).icon} size={14} className="muted" />
          {b.source !== 'page' && b.source !== 'owner' && <span className="muted tiny">{b.source === 'invite' ? 'link' : b.source}</span>}
        </span>
      </button>
    </li>
  )
}

export function useApps() {
  const { data } = useAsync(() => api.get<AppState[]>(`${V1}/apps`).catch(() => [] as AppState[]), [])
  const on = (slug: string) => !!data?.find(a => a.slug === slug)?.connected
  const why = (slug: string) => data?.find(a => a.slug === slug)?.detail || ''
  return { on, why, loaded: !!data }
}

export function BookingPanel({ id, back }: { id: number; back?: () => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const navigate = useNavigate()
  const { on, why } = useApps()
  const [b, setB] = useState<Booking | null>(null)
  const [err, setErr] = useState('')
  const [note, setNote] = useState('')
  const [sheet, setSheet] = useState<'' | 'move' | 'cancel' | 'decline' | 'email' | 'delete' | 'prep'>('')
  const [busy, setBusy] = useState('')
  const load = useCallback(async () => {
    try { const x = await api.get<Booking>(`${V1}/bookings/${id}`); setB(x); setNote(x.owner_note) } catch (e) { setErr(errMsg(e)) }
  }, [id])
  useEffect(() => { load() }, [load])
  useCrumb(back && b ? `${b.name || b.email}` : undefined)
  if (err) return <div className="page"><Empty icon="alert" title="Not found"><p>{err}</p>{back && <button type="button" className="btn mt" onClick={back}>Back</button>}</Empty></div>
  if (!b) return <Loading />
  const st = STATUS[b.status]
  const live = b.status === 'pending' || b.status === 'confirmed'
  const started = new Date(b.start).getTime() <= Date.now()
  const act = async (path: string, body: unknown, msg: string, key = path) => {
    setBusy(key)
    try { const n = await api.post<Booking>(`${V1}/bookings/${b.id}/${path}`, body); setB(x => x && { ...x, ...n }); announceChange(); toast(msg); setSheet('') } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const saveNote = async () => {
    if (note === b.owner_note) return
    try { const n = await api.put<Booking>(`${V1}/bookings/${b.id}`, { owner_note: note }); setB(x => x && { ...x, owner_note: n.owner_note }); toast('Note saved') } catch (e) { toast(errMsg(e), 'err') }
  }
  const make = async (target: string) => {
    setBusy('make-' + target)
    try {
      const m = await api.post<Made>(`${V1}/bookings/${b.id}/make`, { target })
      setB(x => x && { ...x, made: [m, ...(x.made || []).filter(o => !(o.target === m.target && (o.error || o.id === m.id)))] })
      toast(`Done: ${m.title}`)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const copy = (v: string, what: string) => navigator.clipboard?.writeText(v).then(() => toast(`${what} copied`), () => toast(v))
  const makeItems: MenuItem[] = TARGETS.filter(t => (t.target !== 'invoice' && t.target !== 'payment') || b.price > 0)
    .filter(t => t.target !== 'time' || started)
    .map(t => ({ label: t.label, icon: t.icon, disabled: !on(t.app), hint: on(t.app) ? undefined : 'off', onSelect: () => make(t.target) }))
  const more: MenuItem[] = [
    { label: 'Copy their booking link', icon: 'link', onSelect: () => copy(b.manage_url, 'Their link') },
    { label: 'Open their booking page', icon: 'external', onSelect: () => window.open(b.manage_url, '_blank') },
    ...(b.email ? [{ label: 'Send them a booking link for another time', icon: 'calendar-plus', onSelect: () => openCreate('link', b.type_id) }] : []),
    ...(!live || b.past ? ['sep' as const, { label: 'Delete', icon: 'trash', danger: true, onSelect: () => setSheet('delete') }] : []),
  ]
  const aiOK = settings?.ai_help !== false
  return (
    <div className="answer-panel page stack loose">
      <div className="ap-head">
        {back && <button type="button" className="btn ghost icon-only" onClick={back} aria-label="Back"><Icon name="chevron-left" size={18} /></button>}
        <Avatar name={b.name || undefined} email={b.email || String(b.id)} size={42} />
        <div className="grow">
          <h2 className="ap-title">{b.name || b.email || `Booking #${b.id}`}</h2>
          <p className="muted small">{b.emoji} {b.title} · booked {ago(b.created_at)}{b.source === 'invite' ? ' with a personal link' : b.source === 'owner' ? ' by you' : b.source === 'import' ? ' (imported)' : b.source !== 'page' ? ` by ${b.source}` : ''}</p>
        </div>
        <span className={`pill ${st.cls}`}><Icon name={st.icon} size={12} />{st.label}</span>
      </div>

      {b.status === 'pending' && <div className="callout warn"><Icon name="hourglass" size={15} /><div className="grow">
        <b>{b.name || 'They'} asked for this time.</b> It is held for them until you say yes or no.
        <div className="row wrap mt">
          <button type="button" className="btn primary" onClick={() => act('confirm', { tell_guest: true }, 'Confirmed — they get an email', 'confirm')} disabled={!!busy}>{busy === 'confirm' ? <Spinner /> : <Icon name="check" size={15} />}Confirm</button>
          <button type="button" className="btn" onClick={() => setSheet('decline')} disabled={!!busy}>Decline…</button>
        </div>
      </div></div>}

      <section className="card">
        <dl className="kv">
          <div><dt>When</dt><dd><b>{weekday(b.date, true)}, {fmtDate(b.date)}</b> · {b.time}–{b.end_time} <span className="muted">({b.minutes} min)</span>{b.moved_from && <span className="muted small"> · moved</span>}</dd></div>
          {b.guest_tz && settings && b.guest_tz !== settings.tz && <div><dt>Their time zone</dt><dd>{b.guest_tz}</dd></div>}
          {b.email && <div><dt>Email</dt><dd><a href={`mailto:${b.email}`}>{b.email}</a></dd></div>}
          {b.phone && <div><dt>Phone</dt><dd><a href={`tel:${b.phone}`}>{b.phone}</a></dd></div>}
          <div><dt>Where</dt><dd>{b.meeting_url ? <a href={b.meeting_url} target="_blank" rel="noreferrer">{locInfo(b.location_kind).label} <Icon name="external" size={12} /></a> : b.location_kind === 'phone' ? `You call them${b.phone ? '' : ' (no number given)'}` : b.location || locInfo(b.location_kind).label}</dd></div>
          {b.answers.filter(a => a.id !== 'where').map(a => <div key={a.id}><dt>{a.label}</dt><dd className="pre">{a.value}</dd></div>)}
          {b.price > 0 && <div><dt>Price</dt><dd className="row wrap" style={{ gap: 10 }}>{money(b.price, b.currency)}
            <Switch size="sm" checked={b.paid} onChange={v => act('paid', { paid: v }, v ? 'Marked paid' : 'Marked not paid', 'paid')} label="Paid" /><span className="small muted">{b.paid ? `Paid ${b.paid_at ? fmtDate(b.paid_at.slice(0, 10)) : ''}` : 'Not paid yet'}</span></dd></div>}
          {b.cancel_reason && <div><dt>{b.status === 'declined' ? 'Your message' : b.cancelled_by === 'guest' ? 'Their reason' : 'Your message'}</dt><dd className="pre">{b.cancel_reason}</dd></div>}
        </dl>
      </section>

      <div className="row wrap">
        {live && !started && <button type="button" className="btn" onClick={() => setSheet('move')}><Icon name="move" size={15} />Move</button>}
        {live && !started && <button type="button" className="btn" onClick={() => setSheet('cancel')}><Icon name="x" size={15} />Cancel</button>}
        {started && (b.status === 'confirmed' || b.status === 'done' || b.status === 'no_show') && <Segmented size="sm" value={b.status === 'confirmed' ? '' : b.status} onChange={v => act(v === 'done' ? 'done' : 'no-show', {}, v === 'done' ? 'Marked as happened' : 'Marked: did not come', 'mark')}
          options={[{ value: 'done', label: 'Happened' }, { value: 'no_show', label: 'Did not come' }]} label="How it went" />}
        {b.email && <button type="button" className="btn" onClick={() => setSheet('email')} disabled={!on('mail')} title={on('mail') ? 'From your Pocket Mail' : why('mail') || 'Connect Pocket Mail in Settings'}><Icon name="mail" size={15} />Email them</button>}
        <MenuButton label="Send to another app" className="btn" items={makeItems} width={290}><Icon name="share" size={15} />Send to…</MenuButton>
        {aiOK && <button type="button" className="btn" onClick={() => setSheet('prep')}><Icon name="sparkles" size={15} />Get ready</button>}
        <span className="grow" />
        <MenuButton label="More" items={more} align="right" width={280}><Icon name="more" size={18} /></MenuButton>
      </div>

      {(b.made || []).length > 0 && <section className="card">
        <h3 className="card-h">In your other apps</h3>
        <ul className="made-list">{(b.made || []).map(m => <MadeRow key={m.id} m={m} />)}</ul>
      </section>}

      <section className="card">
        <h3 className="card-h">Your note</h3>
        <textarea rows={3} value={note} onChange={e => setNote(e.target.value)} onBlur={saveNote} placeholder="Only you see this. Saved when you leave the box." />
      </section>

      {sheet === 'move' && <BookSheet move={b} onClose={() => setSheet('')} onDone={n => setB(x => x && { ...x, ...n })} />}
      {sheet === 'cancel' && <MessageSheet title="Cancel this booking" button="Cancel the booking" danger help="They get an email with your message and a link to book another time."
        busy={busy === 'cancel'} onClose={() => setSheet('')} onSend={(m, tell) => act('cancel', { message: m, tell_guest: tell }, 'Cancelled', 'cancel')} />}
      {sheet === 'decline' && <MessageSheet title="Decline this time" button="Decline" help="They get an email with your message and a link to pick another time."
        busy={busy === 'decline'} onClose={() => setSheet('')} onSend={(m, tell) => act('decline', { message: m, tell_guest: tell }, 'Declined', 'decline')} />}
      {sheet === 'email' && <EmailSheet b={b} onClose={() => setSheet('')} />}
      {sheet === 'prep' && <PrepSheet b={b} onClose={() => setSheet('')} />}
      {sheet === 'delete' && <Confirm title="Delete this booking?" message="It is gone for good, here only: what was made in your other apps stays." confirmLabel="Delete"
        onCancel={() => setSheet('')} onConfirm={async () => { try { await api.del(`${V1}/bookings/${b.id}`); announceChange(); toast('Deleted'); navigate('/') } catch (e) { toast(errMsg(e), 'err'); setSheet('') } }} />}
    </div>
  )
}

function MadeRow({ m }: { m: Made }) {
  const info = APP_INFO[m.app as keyof typeof APP_INFO]
  return (
    <li className={`made-row ${m.error ? 'bad' : ''}`}>
      <Icon name={info?.icon || 'dot'} size={15} />
      <span className="grow">{m.url && !m.error ? <a href={m.url} target="_blank" rel="noreferrer">{m.title}</a> : <span>{m.title || TARGETS.find(t => t.target === m.target)?.label}</span>}
        {m.error && <span className="small bad block-line">{m.error}</span>}</span>
      <span className="muted small" title={fmtDateTime(m.created_at)}>{m.auto ? 'by itself · ' : ''}{ago(m.created_at)}</span>
    </li>
  )
}

function MessageSheet({ title, button, help, danger, busy, onClose, onSend }: { title: string; button: string; help: string; danger?: boolean; busy: boolean; onClose: () => void; onSend: (m: string, tell: boolean) => void }) {
  const [m, setM] = useState('')
  const [tell, setTell] = useState(true)
  return (
    <Modal title={title} onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Keep it</button><button type="button" className={`btn ${danger ? 'danger' : 'primary'}`} onClick={() => onSend(m, tell)} disabled={busy}>{busy && <Spinner />}{button}</button></>}>
      <div className="stack">
        <textarea rows={4} value={m} onChange={e => setM(e.target.value)} placeholder="A message for them (optional): why, and what you suggest instead." autoFocus />
        <div className="row"><Switch checked={tell} onChange={setTell} label="Email them" /><span className="small muted">{help}</span></div>
      </div>
    </Modal>
  )
}

function EmailSheet({ b, onClose }: { b: Booking; onClose: () => void }) {
  const { toast } = useToast()
  const [subject, setSubject] = useState(`${b.title} — ${b.date}`)
  const [body, setBody] = useState(`Hi ${b.name.split(' ')[0] || ''},\n\n`)
  const [busy, setBusy] = useState(false)
  const send = async () => {
    setBusy(true)
    try { await api.post(`${V1}/bookings/${b.id}/email`, { subject, body }); toast('Sent from your Pocket Mail'); onClose() } catch (e) { toast(errMsg(e), 'err'); setBusy(false) }
  }
  return (
    <Modal title={`Email ${b.name || b.email}`} onClose={onClose} footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={send} disabled={busy || !body.trim()}>{busy && <Spinner />}Send</button></>}>
      <div className="stack">
        <input value={subject} onChange={e => setSubject(e.target.value)} aria-label="Subject" />
        <textarea rows={8} value={body} onChange={e => setBody(e.target.value)} aria-label="Message" autoFocus />
        <p className="muted small">The booking’s time, place and their link to move or cancel go under your message.</p>
      </div>
    </Modal>
  )
}

function PrepSheet({ b, onClose }: { b: Booking; onClose: () => void }) {
  const [text, setText] = useState('')
  const [err, setErr] = useState('')
  useEffect(() => { api.post<{ text: string }>(`${V1}/bookings/${b.id}/ai/prep`).then(r => setText(r.text), e => setErr(errMsg(e))) }, [b.id])
  return (
    <Modal title={`Get ready: ${b.name || b.email}`} onClose={onClose} footer={<button type="button" className="btn" onClick={onClose}>Close</button>}>
      {err ? <p className="bad">{err}</p> : !text ? <Loading label="Pocket AI is reading their answers and earlier bookings…" /> : <div className="notes">{text}</div>}
    </Modal>
  )
}
