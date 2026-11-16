// One booking page: what it is, when people can book it, what it asks,
// and what happens after someone books.
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData } from '../lib/data'
import { ACCENTS, APP_INFO, LANGUAGES, LOCATIONS, QUESTION_TYPES, lengths, typeState } from '../lib/booking'
import type { BookingType, Question, QuestionType, Schedule } from '../lib/types'
import { openCreate, useCrumb } from '../components/Layout'
import { useApps } from './Bookings'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Confirm, CopyField, Field, Loading, type MenuItem, MenuButton, Segmented, SettingRow, Spinner, Switch, errMsg, useAsync, useToast } from '../components/ui'

const NOTICE = [0, 60, 120, 240, 720, 1440, 2880, 10080].map(m => ({ value: String(m), label: m === 0 ? 'Any time before' : m < 1440 ? `${m / 60} ${m === 60 ? 'hour' : 'hours'}` : `${m / 1440} ${m === 1440 ? 'day' : m === 10080 ? 'week' : 'days'}`.replace('7 week', '1 week') }))
const BUFFER = [0, 5, 10, 15, 30, 45, 60].map(m => ({ value: String(m), label: m ? `${m} min` : 'None' }))
const REMIND = [0, 1, 2, 4, 12, 24, 48].map(h => ({ value: String(h), label: h ? `${h} ${h === 1 ? 'hour' : 'hours'} before` : 'No reminder' }))

export default function TypeEditor() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { settings } = useApp()
  const { types } = useData()
  const { on, why } = useApps()
  const [t, setT] = useState<BookingType | null>(null)
  const [saved, setSaved] = useState('')
  const [busy, setBusy] = useState(false)
  const [del, setDel] = useState(false)
  const [newLen, setNewLen] = useState('')
  const { data: scheds } = useAsync(() => api.get<{ schedules: Schedule[] }>(`${V1}/schedules`).then(r => r.schedules), [])
  const { data: places } = useAsync(() => api.get<{ id: string; name: string; address: string }[]>(`${V1}/apps/options/places`).catch(() => []), [])
  useEffect(() => {
    api.get<BookingType>(`${V1}/types/${id}`).then(x => { setT(x); setSaved(JSON.stringify(x)) }, e => { toast(errMsg(e), 'err'); navigate('/types') })
  }, [id]) // eslint-disable-line react-hooks/exhaustive-deps
  useCrumb(t?.title)
  const dirty = !!t && JSON.stringify(t) !== saved
  useEffect(() => {
    const h = (e: BeforeUnloadEvent) => { if (dirty) e.preventDefault() }
    window.addEventListener('beforeunload', h)
    return () => window.removeEventListener('beforeunload', h)
  }, [dirty])
  const up = (p: Partial<BookingType>) => setT(x => x && { ...x, ...p })
  const save = async (extra?: Partial<BookingType>) => {
    if (!t) return
    setBusy(true)
    try {
      const n = await api.put<BookingType>(`${V1}/types/${t.id}`, { ...t, ...extra })
      setT(n); setSaved(JSON.stringify(n)); announceChange(); toast('Saved')
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const more: MenuItem[] = useMemo(() => !t ? [] : [
    { label: 'Book someone in', icon: 'calendar-plus', disabled: t.status === 'draft', onSelect: () => openCreate('book', t.id) },
    { label: 'Send a personal link', icon: 'link', disabled: t.status !== 'open', onSelect: () => openCreate('link', t.id) },
    { label: 'Make a copy', icon: 'copy', onSelect: async () => { try { const n = await api.post<BookingType>(`${V1}/types/${t.id}/duplicate`); announceChange(); navigate(`/types/${n.id}`) } catch (e) { toast(errMsg(e), 'err') } } },
    'sep',
    { label: 'Delete the page', icon: 'trash', danger: true, onSelect: () => setDel(true) },
  ], [t]) // eslint-disable-line react-hooks/exhaustive-deps
  if (!t || !types) return <Loading />
  const st = typeState(t)
  const setQ = (i: number, p: Partial<Question>) => up({ questions: t.questions.map((q, j) => j === i ? { ...q, ...p } : q) })
  const moveQ = (i: number, d: number) => { const qs = [...t.questions]; const [x] = qs.splice(i, 1); qs.splice(i + d, 0, x); up({ questions: qs }) }
  const addLen = () => { const n = Number(newLen); if (n >= 5 && !t.durations.includes(n)) up({ durations: [...t.durations, n].sort((a, b) => a - b) }); setNewLen('') }
  const loc = LOCATIONS.find(l => l.value === t.location_kind)
  return (
    <div className="page stack loose type-editor">
      <div className="page-head">
        <span className="fc-emoji big" aria-hidden="true">{t.emoji || '📅'}</span>
        <div className="grow"><h1 className="page-title">{t.title || 'Booking page'}</h1><p className="muted"><span className={`pill ${st.cls}`}><Icon name={st.icon} size={11} />{st.label}</span> {lengths(t.durations)} · {t.upcoming} coming up · {t.bookings} in all</p></div>
        <a className="btn" href={t.url} target="_blank" rel="noreferrer"><Icon name="external" size={15} />{t.status === 'draft' ? 'Preview' : 'Open'}</a>
        <MenuButton label="More" items={more} align="right" width={240}><Icon name="more" size={18} /></MenuButton>
      </div>

      <section className="card stack">
        <h2 className="card-h">Who can book it</h2>
        <div className="row"><Segmented value={t.status} onChange={v => up({ status: v })} options={[{ value: 'open', label: 'Open' }, { value: 'paused', label: 'Paused' }, { value: 'draft', label: 'Draft' }]} label="Status" /></div>
        <p className="muted small">{t.status === 'open' ? 'Anyone with the link can book a free time.' : t.status === 'paused' ? 'The page says it does not take bookings right now; bookings already made stay.' : 'Only you see it (a preview), until you open it.'}</p>
        <SettingRow title="Only with a personal link" help="The page is not listed, and people need a link you send them."><Switch checked={t.invite_only} onChange={v => up({ invite_only: v })} label="Personal links only" /></SettingRow>
        <Field label="Its address"><div className="input-group"><span className="input-prefix">/b/</span><input value={t.handle} onChange={e => up({ handle: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-') })} /></div></Field>
        {t.status !== 'draft' && <CopyField value={t.url} label="Link to share" />}
      </section>

      <section className="card stack">
        <h2 className="card-h">What it is</h2>
        <div className="two-col tight">
          <Field label="Name"><input value={t.title} onChange={e => up({ title: e.target.value })} /></Field>
          <Field label="Emoji"><input value={t.emoji} onChange={e => up({ emoji: e.target.value })} className="emoji-in" maxLength={8} /></Field>
        </div>
        <Field label="Words on the page" help="What it is, who it is for, what to bring."><textarea rows={3} value={t.description} onChange={e => up({ description: e.target.value })} /></Field>
        <Field label="How long" help="More than one: they pick.">
          <div className="row wrap">
            {t.durations.map(d => <span key={d} className="chip">{lengths([d])}{t.durations.length > 1 && <button type="button" className="chip-x" aria-label={`Remove ${d} minutes`} onClick={() => up({ durations: t.durations.filter(x => x !== d) })}><Icon name="x" size={12} /></button>}</span>)}
            <input className="num-sm" type="number" min={5} step={5} value={newLen} onChange={e => setNewLen(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') addLen() }} placeholder="min" aria-label="Another length in minutes" />
            <button type="button" className="btn sm" onClick={addLen} disabled={!newLen}>Add</button>
          </div>
        </Field>
        <div className="two-col tight">
          <Field label="Where"><Select ariaLabel="Where" value={t.location_kind} onChange={v => up({ location_kind: v as BookingType['location_kind'] })} options={LOCATIONS.map(l => ({ value: l.value, label: l.label, hint: l.help }))} /></Field>
          {t.location_kind === 'place' && <Field label="Address" help={on('places') ? 'Or pick one of your places from Pocket Places.' : undefined}>
            <input value={t.location} onChange={e => up({ location: e.target.value })} placeholder="Street, town" />
          </Field>}
          {t.location_kind === 'link' && <Field label="Your video link"><input value={t.location} onChange={e => up({ location: e.target.value })} placeholder="https://zoom.us/j/…" /></Field>}
          {t.location_kind === 'phone_in' && <Field label="Your phone number" help="Shown to them after they book."><input value={t.location} onChange={e => up({ location: e.target.value })} /></Field>}
        </div>
        {t.location_kind === 'place' && (places || []).length > 0 && <Field label="From Pocket Places"><Select ariaLabel="Place" value={t.place_id} onChange={v => { const p = places?.find(x => x.id === v); up({ place_id: v, location: p ? [p.name, p.address].filter(Boolean).join(', ') : t.location }) }} placeholder="Pick a place" clearLabel="None" options={(places || []).map(p => ({ value: p.id, label: p.name, hint: p.address }))} /></Field>}
        {loc && <p className="muted small row" style={{ gap: 6 }}><Icon name={loc.icon} size={13} />{loc.help}.</p>}
        <div className="two-col tight">
          <Field label="Price" help="Empty: free. Shown on the page and in the email."><div className="input-group"><input type="number" min={0} step="0.01" value={t.price ? t.price / 100 : ''} onChange={e => up({ price: Math.round(Number(e.target.value || 0) * 100) })} placeholder="0" /><input className="cur-in" value={t.currency} onChange={e => up({ currency: e.target.value.toUpperCase().slice(0, 3) })} aria-label="Currency" /></div></Field>
          {t.price > 0 && <Field label="How to pay" help="Bank details, cash on the day…"><textarea rows={2} value={t.pay_note} onChange={e => up({ pay_note: e.target.value })} /></Field>}
        </div>
      </section>

      <section className="card stack">
        <h2 className="card-h">When</h2>
        <SettingRow title="Hours" help={<>Which weekly hours this page uses. <Link to="/hours">Change your hours and days off</Link>.</>}>
          <Select ariaLabel="Hours" value={String(t.schedule_id || '')} onChange={v => up({ schedule_id: Number(v) })} options={(scheds || []).map(s => ({ value: String(s.id), label: s.name, hint: s.is_default ? 'default' : '' }))} />
        </SettingRow>
        <SettingRow title="Not sooner than" help="How much notice you need."><Select ariaLabel="Notice" value={String(t.notice_min)} onChange={v => up({ notice_min: Number(v) })} options={NOTICE.some(n => n.value === String(t.notice_min)) ? NOTICE : [...NOTICE, { value: String(t.notice_min), label: `${t.notice_min} min` }]} /></SettingRow>
        <SettingRow title="Up to" help="How far ahead people can book; 0 = no limit."><div className="row"><input type="number" className="num-sm" min={0} max={730} value={t.days_ahead} onChange={e => up({ days_ahead: Math.max(0, Number(e.target.value) || 0) })} /><span className="muted small">days ahead</span></div></SettingRow>
        <SettingRow title="Times start every" help="Same as the length, or more often."><Select ariaLabel="Times start every" value={String(t.step_min)} onChange={v => up({ step_min: Number(v) })} options={[{ value: '0', label: 'The length' }, ...[15, 20, 30, 60].map(m => ({ value: String(m), label: `${m} min` }))]} /></SettingRow>
        <SettingRow title="Free time before" help="Kept free before each booking (getting ready, travel)."><Select ariaLabel="Before" value={String(t.buffer_before)} onChange={v => up({ buffer_before: Number(v) })} options={BUFFER} /></SettingRow>
        <SettingRow title="Free time after"><Select ariaLabel="After" value={String(t.buffer_after)} onChange={v => up({ buffer_after: Number(v) })} options={BUFFER} /></SettingRow>
        <SettingRow title="At most a day" help="0 = no limit."><input type="number" className="num-sm" min={0} value={t.max_per_day} onChange={e => up({ max_per_day: Math.max(0, Number(e.target.value) || 0) })} /></SettingRow>
        <SettingRow title="People per time" help="More than 1 for a class or a tour: each person books a place."><input type="number" className="num-sm" min={1} max={1000} value={t.capacity} onChange={e => up({ capacity: Math.max(1, Number(e.target.value) || 1) })} /></SettingRow>
        <SettingRow title="Confirm each one yourself" help="Off: a booking is on at once. On: it waits for your yes (and holds the time meanwhile)."><Switch checked={t.confirm === 'manual'} onChange={v => up({ confirm: v ? 'manual' : 'auto' })} label="Confirm by hand" /></SettingRow>
      </section>

      <section className="card stack">
        <h2 className="card-h">What it asks</h2>
        <p className="muted small">Their name and email are always asked.</p>
        <SettingRow title="Phone number"><Segmented size="sm" value={t.ask_phone} onChange={v => up({ ask_phone: v })} options={[{ value: 'no', label: 'Not asked' }, { value: 'optional', label: 'Optional' }, { value: 'required', label: 'Required' }]} /></SettingRow>
        {t.questions.map((q, i) => (
          <div key={q.id || i} className="q-row">
            <div className="row wrap">
              <div className="q-type"><Select ariaLabel="Kind of question" value={q.type} onChange={v => setQ(i, { type: v as QuestionType })} options={QUESTION_TYPES.map(x => ({ value: x.type, label: x.label }))} /></div>
              <input className="grow" value={q.label} onChange={e => setQ(i, { label: e.target.value })} placeholder="The question" aria-label="The question" />
              <Switch size="sm" checked={!!q.required} onChange={v => setQ(i, { required: v })} label="Required" />
              <button type="button" className="btn ghost sm icon-only" aria-label="Up" disabled={i === 0} onClick={() => moveQ(i, -1)}><Icon name="chevron-up" size={15} /></button>
              <button type="button" className="btn ghost sm icon-only" aria-label="Down" disabled={i === t.questions.length - 1} onClick={() => moveQ(i, 1)}><Icon name="chevron-down" size={15} /></button>
              <button type="button" className="btn ghost sm icon-only" aria-label="Remove the question" onClick={() => up({ questions: t.questions.filter((_, j) => j !== i) })}><Icon name="trash" size={15} /></button>
            </div>
            {(q.type === 'choice' || q.type === 'checkboxes') && <textarea rows={3} value={(q.options || []).join('\n')} onChange={e => setQ(i, { options: e.target.value.split('\n') })} placeholder="One choice per line" aria-label="Choices" />}
          </div>
        ))}
        <div><button type="button" className="btn sm" onClick={() => up({ questions: [...t.questions, { id: '', type: 'text', label: '' }] })} disabled={t.questions.length >= 30}><Icon name="plus" size={14} />Add a question</button></div>
      </section>

      <section className="card stack">
        <h2 className="card-h">After someone books</h2>
        <p className="muted small">Made by itself in your other apps when a booking is on (for one that waits: when you confirm it).</p>
        {([['event', 'calendar', 'Put it in Pocket Calendar', 'Moved and removed with the booking'], ['contact', 'contacts', 'Add them to Pocket Contacts', 'Unless they are there already'],
          ['deal', 'crm', 'Add a lead in Pocket CRM', 'With what they wrote'], ['task', 'tasks', 'A task to get ready', 'Due the day before'],
          ['note', 'notes', 'A meeting note in Pocket Notes', 'With who, when and their answers'], ['invoice', 'invoicing', 'A draft invoice in Pocket Invoicing', 'For pages with a price']] as const).map(([k, app, title, help]) => (
          <SettingRow key={k} title={<span className="row" style={{ gap: 6 }}><Icon name={APP_INFO[app].icon} size={14} />{title}</span>} help={on(app) ? help : `${help}. Not connected: ${why(app) || 'Settings → Connected apps'}.`}>
            <Switch checked={!!t.after[k]} onChange={v => up({ after: { ...t.after, [k]: v } })} label={title} disabled={!on(app) && !t.after[k]} />
          </SettingRow>
        ))}
        <SettingRow title="Reminder email" help={on('mail') ? 'To the person who booked, through your Pocket Mail.' : 'Needs Pocket Mail (Settings → Connected apps).'}><Select ariaLabel="Reminder" value={String(t.reminder_h)} onChange={v => up({ reminder_h: Number(v) })} options={REMIND.some(r => r.value === String(t.reminder_h)) ? REMIND : [...REMIND, { value: String(t.reminder_h), label: `${t.reminder_h} hours before` }]} /></SettingRow>
      </section>

      <section className="card stack">
        <h2 className="card-h">Look</h2>
        <SettingRow title="Language of the page" help="The words the page and the emails say themselves."><Select ariaLabel="Language" value={t.language} onChange={v => up({ language: v })} options={LANGUAGES} /></SettingRow>
        <SettingRow title="Colour"><div className="row wrap swatches">{ACCENTS.map(c => <button key={c} type="button" className={`swatch ${(t.accent || settings?.accent) === c ? 'on' : ''}`} style={{ background: c }} aria-label={c} onClick={() => up({ accent: c })} />)}<input type="color" value={t.accent || settings?.accent || '#087f5b'} onChange={e => up({ accent: e.target.value })} aria-label="Another colour" /></div></SettingRow>
      </section>

      <div className="save-bar">
        <span className="grow muted small">{dirty ? 'Changes not saved yet' : 'All saved'}</span>
        <button type="button" className="btn" onClick={() => saved && setT(JSON.parse(saved))} disabled={!dirty || busy}>Undo changes</button>
        <button type="button" className="btn primary" onClick={() => save()} disabled={!dirty || busy}>{busy && <Spinner />}Save</button>
      </div>
      {del && <Confirm title="Delete this booking page?" message="Its link stops working. Bookings already made stay, with their time and who." confirmLabel="Delete"
        onCancel={() => setDel(false)} onConfirm={async () => { try { await api.del(`${V1}/types/${t.id}`); announceChange(); navigate('/types') } catch (e) { toast(errMsg(e), 'err'); setDel(false) } }} />}
    </div>
  )
}
