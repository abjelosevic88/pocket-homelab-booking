import { AskButton } from '../components/AskButton'
import React, { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import type { APIToken, AppSlug, ConnMode, Settings as SettingsT, User } from '../lib/types'
import { DATE_FORMATS, TIME_FORMATS, fmtBytes, fmtDateTime, type DateFormat, type TimeFormat } from '../lib/format'
import { Avatar, Confirm, CoreHint, Field, Loading, Modal, Segmented, SettingRow, Switch, Tabs, useAsync, useIsMobile, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { ShortcutsSheet } from '../components/ShortcutsSheet'
import { PocketModelSelect } from '../components/PocketModelSelect'
import { ImportModal } from '../components/ImportModal'
import { APP_INFO, LANGUAGES, ZONES } from '../lib/booking'

type TabId = 'general' | 'calendar' | 'notifications' | 'apps' | 'ai' | 'profile' | 'people' | 'tokens' | 'data' | 'about'

const TABS: { id: TabId; label: string; icon: string; sub: string }[] = [
  { id: 'general', label: 'General', icon: 'home', sub: 'Your time zone, your page with all your booking pages, and how new pages start.' },
  { id: 'calendar', label: 'Calendar & emails', icon: 'calendar', sub: 'Busy times from Pocket Calendar, where bookings go, and the emails people get.' },
  { id: 'notifications', label: 'Notifications', icon: 'bell', sub: 'A push for each booking and cancellation, and a morning note about the day.' },
  { id: 'apps', label: 'Connected apps', icon: 'layers', sub: 'Your calendar, contacts, mail, leads, invoices, money, tasks, notes, places and time.' },
  { id: 'ai', label: 'AI', icon: 'sparkles', sub: 'Who helps you: Pocket AI through Core, or your own OpenAI key.' },
  { id: 'profile', label: 'Profile', icon: 'user', sub: 'Your name, email and password.' },
  { id: 'people', label: 'People', icon: 'users', sub: 'Everyone who has an account on this box. Each person has their own pages and bookings.' },
  { id: 'tokens', label: 'API & shortcuts', icon: 'key', sub: 'Personal tokens for Siri Shortcuts, scripts and other tools.' },
  { id: 'data', label: 'Backup & data', icon: 'database', sub: 'Bring bookings from Calendly or Cal.com, export everything, download the database.' },
  { id: 'about', label: 'About', icon: 'info', sub: 'Version, numbers and the server.' },
]

export default function Settings() {
  const { tab: raw } = useParams()
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const tab: TabId = TABS.find(t => t.id === raw)?.id || 'general'
  const current = TABS.find(t => t.id === tab)!
  const go = (id: TabId) => navigate(`/settings/${id}`, { replace: true })
  useEffect(() => { document.querySelector('.set-content')?.scrollTo({ top: 0 }) }, [tab])
  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape' && !document.querySelector('.modal-backdrop, .menu, .sel-pop')) navigate('/') }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [navigate])
  return (
    <div className="set-page">
      <header className="set-header">
        <button type="button" className="btn ghost icon-only" aria-label="Back" title="Back (esc)" onClick={() => navigate('/')}><Icon name="chevron-left" size={18} /></button>
        <img src="/favicon.svg" alt="" width={24} height={24} className="set-logo" />
        <h1>Settings</h1>
        <span style={{ flex: 1 }} />
        <AskButton />
      </header>
      {mobile && <div className="set-tabs-mobile"><Tabs tabs={TABS.map(t => ({ id: t.id, label: t.label, icon: t.icon }))} value={tab} onChange={go} /></div>}
      <div className="set-body">
        {!mobile && (
          <nav className="set-nav" aria-label="Settings sections">
            {TABS.map(t => (
              <button key={t.id} type="button" className={`set-nav-item ${t.id === tab ? 'active' : ''}`} aria-current={t.id === tab ? 'page' : undefined} onClick={() => go(t.id)}>
                <Icon name={t.icon} size={16} />{t.label}
              </button>
            ))}
          </nav>
        )}
        <main className="set-content">
          <div className="set-inner" key={tab}>
            <div className="set-title"><h2>{current.label}</h2><p>{current.sub}</p></div>
            {tab === 'general' && <General />}
            {tab === 'calendar' && <CalendarTab />}
            {tab === 'notifications' && <Notifications />}
            {tab === 'apps' && <Integrations />}
            {tab === 'ai' && <AITab />}
            {tab === 'profile' && <Profile />}
            {tab === 'people' && <Users />}
            {tab === 'tokens' && <Tokens />}
            {tab === 'data' && <Data />}
            {tab === 'about' && <About />}
          </div>
        </main>
      </div>
    </div>
  )
}

function Section({ title, sub, icon, actions, children }: { title: React.ReactNode; sub?: React.ReactNode; icon?: string; actions?: React.ReactNode; children?: React.ReactNode }) {
  return (
    <section className="set-section">
      <div className="set-section-head">
        {icon && <span className="set-app-ico"><Icon name={icon} size={17} /></span>}
        <div className="grow"><h3>{title}</h3>{sub && <p>{sub}</p>}</div>
        {actions && <div className="row">{actions}</div>}
      </div>
      {children && <div className="set-section-body">{children}</div>}
    </section>
  )
}

function useSettingsForm() {
  const { settings, setSettings, user } = useApp()
  const { toast } = useToast()
  const [f, setF] = useState<SettingsT | null>(settings)
  const [busy, setBusy] = useState(false)
  useEffect(() => { if (!f && settings) setF(settings) }, [settings]) // eslint-disable-line react-hooks/exhaustive-deps
  const save = async (patch?: Partial<SettingsT>) => {
    if (!f) return
    setBusy(true)
    try { const body = { ...f, ...patch } as Record<string, unknown>; body.tz = body.tz_set; const s = await api.put<SettingsT>(`${V1}/settings`, body); setSettings(s); setF(s); toast('Saved') } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return { f, setF, save, busy, isAdmin: user?.role === 'admin' }
}

function useTester() {
  const [test, setTest] = useState<Record<string, string>>({})
  const run = async (which: string, url: string, key: string, extra: Record<string, string> = {}) => {
    setTest(t => ({ ...t, [which]: 'testing…' }))
    try { const r = await api.post<{ ok: boolean; message?: string; error?: string }>(`${V1}/integrations/test`, { which, url, key, ...extra }); setTest(t => ({ ...t, [which]: r.ok ? `✓ ${r.message}` : `✕ ${r.error}` })) } catch (e) { setTest(t => ({ ...t, [which]: `✕ ${(e as Error).message}` })) }
  }
  const status = (which: string) => <span className={`small ${test[which]?.startsWith('✓') ? 'ok' : test[which]?.startsWith('✕') ? 'bad' : 'muted'}`}>{test[which]}</span>
  return { run, status }
}

const HOURS = Array.from({ length: 24 }, (_, h) => ({ value: String(h), label: `${String(h).padStart(2, '0')}:00` }))

function General() {
  const { f, setF, save, busy, isAdmin } = useSettingsForm()
  if (!f) return <Loading />
  return (
    <div className="stack loose">
      <Section title="Dates" icon="calendar">
        <SettingRow title="Week starts on"><Segmented size="sm" value={f.week_start} onChange={v => setF({ ...f, week_start: v })} options={[{ value: 1, label: 'Monday' }, { value: 6, label: 'Saturday' }, { value: 7, label: 'Sunday' }]} /></SettingRow>
        <SettingRow title="Date format"><Select ariaLabel="Date format" value={f.date_format} onChange={v => setF({ ...f, date_format: v as DateFormat })} options={DATE_FORMATS.map(d => ({ value: d.id, label: d.label }))} /></SettingRow>
        <SettingRow title="Time format"><Select ariaLabel="Time format" value={f.time_format} onChange={v => setF({ ...f, time_format: v as TimeFormat })} options={TIME_FORMATS.map(d => ({ value: d.id, label: d.label }))} /></SettingRow>
        {isAdmin && <SettingRow title="App title" help="Shown in the sidebar for everyone on this box."><input value={f.title} onChange={e => setF({ ...f, title: e.target.value })} /></SettingRow>}
      </Section>
      <Section title="Time zone" icon="globe" sub="Your hours, bookings and emails are in this zone. People who book see the times in theirs if they pick it.">
        <SettingRow title="Your time zone" help={f.tz_set ? undefined : `The box's zone (${f.tz}).`}><Select ariaLabel="Time zone" value={f.tz_set || f.tz} onChange={v => setF({ ...f, tz_set: v, tz: v })} options={[...new Set([f.tz, ...ZONES])].map(z => ({ value: z, label: z }))} /></SettingRow>
      </Section>
      <Section title="Your page" icon="link" sub="One address that lists all your open booking pages — for your email signature or website.">
        <SettingRow title="Address" help={f.page_url ? <a href={f.page_url} target="_blank" rel="noreferrer">{f.page_url}</a> : 'Empty: no such page.'}><div className="input-group"><span className="input-prefix">/p/</span><input value={f.page_handle} onChange={e => setF({ ...f, page_handle: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-') })} placeholder="your-name" /></div></SettingRow>
        <SettingRow title="Your name on the pages" help="Shown above each page and in the emails; empty: your profile name."><input value={f.page_name} onChange={e => setF({ ...f, page_name: e.target.value })} /></SettingRow>
        <SettingRow title="A few words" stack><textarea rows={2} value={f.page_intro} onChange={e => setF({ ...f, page_intro: e.target.value })} placeholder="What you do, and how booking works." /></SettingRow>
      </Section>
      <Section title="New pages start with" icon="list" sub="Each page can change these.">
        <SettingRow title="Language of the pages" help="The words the page and the emails say themselves."><Select ariaLabel="Language" value={f.language} onChange={v => setF({ ...f, language: v })} options={LANGUAGES} /></SettingRow>
        <SettingRow title="Colour"><input type="color" value={f.accent || '#087f5b'} onChange={e => setF({ ...f, accent: e.target.value })} aria-label="Colour" /></SettingRow>
        <SettingRow title="Currency" help="For pages with a price."><input className="cur-in" value={f.currency} onChange={e => setF({ ...f, currency: e.target.value.toUpperCase().slice(0, 3) })} /></SettingRow>
      </Section>
      <div className="row"><button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button></div>
    </div>
  )
}

function CalendarTab() {
  const { f, setF, save, busy } = useSettingsForm()
  const calOn = !!f && f.calendar_mode !== 'off' && f.calendar_via !== ''
  const { data: cals } = useAsync(() => calOn ? api.get<{ id: number; name: string; color: string; role: string }[]>(`${V1}/apps/options/calendars`).catch(() => []) : Promise.resolve([]), [calOn])
  const moneyOn = !!f && f.money_mode !== 'off' && f.money_via !== ''
  const { data: accts } = useAsync(() => moneyOn ? api.get<{ id: number; name: string; currency: string }[]>(`${V1}/apps/options/accounts`).catch(() => []) : Promise.resolve([]), [moneyOn])
  if (!f) return <Loading />
  const mailOn = f.mail_mode !== 'off' && f.mail_via !== ''
  const busyIDs = f.busy_calendars || []
  return (
    <div className="stack loose">
      <Section title="Busy times" icon="calendar" sub="Times you are busy in Pocket Calendar cannot be booked (events marked free are left out).">
        <SettingRow title="Check Pocket Calendar" help={calOn ? 'Looked at each time someone opens a page or books.' : `Pocket Calendar is not connected: ${f.calendar_detail || 'Connected apps'}.`}><Switch checked={f.check_calendar} onChange={v => setF({ ...f, check_calendar: v })} label="Check Pocket Calendar" /></SettingRow>
        {calOn && f.check_calendar && (cals || []).length > 0 && <SettingRow title="Which calendars" help="None picked: all of them." stack>
          <div className="check-list">{(cals || []).map(c => <label key={c.id} className="check-row"><input type="checkbox" checked={busyIDs.includes(c.id)} onChange={e => setF({ ...f, busy_calendars: e.target.checked ? [...busyIDs, c.id] : busyIDs.filter(x => x !== c.id) })} /><span className="cal-dot" style={{ background: c.color || 'var(--accent)' }} />{c.name}</label>)}</div>
        </SettingRow>}
      </Section>
      <Section title="Bookings in your calendar" icon="calendar-plus" sub="Pages that put bookings in Pocket Calendar (After someone books) use this calendar.">
        <SettingRow title="Calendar" help={calOn ? undefined : 'Connect Pocket Calendar first (Connected apps).'}>
          <Select ariaLabel="Calendar" value={String(f.event_calendar || '')} onChange={v => setF({ ...f, event_calendar: Number(v) })} placeholder="Calendar’s default" clearLabel="Calendar’s default" disabled={!calOn} options={(cals || []).filter(c => c.role !== 'viewer' && c.role !== 'read').map(c => ({ value: String(c.id), label: c.name }))} />
        </SettingRow>
      </Section>
      <Section title="Emails to the people who book" icon="mail" sub="A confirmation with a calendar file, a reminder, and a note when something changes — from your own mailbox through Pocket Mail.">
        <SettingRow title="Send them emails" help={mailOn ? 'Off: nobody gets an email; they still see their booking page.' : `Pocket Mail is not connected: ${f.mail_detail || 'Connected apps'}.`}><Switch checked={f.tell_guest} onChange={v => setF({ ...f, tell_guest: v })} label="Emails" /></SettingRow>
        <SettingRow title="From the account" help={mailOn ? 'The address in Pocket Mail; empty: its default account.' : 'Connect Pocket Mail first.'}><input value={f.mail_from} onChange={e => setF({ ...f, mail_from: e.target.value })} placeholder="Default account" disabled={!mailOn} /></SettingRow>
      </Section>
      <Section title="Payments" icon="wallet" sub="“Put the payment in Pocket Money” books paid bookings as income here.">
        <SettingRow title="Account" help={moneyOn ? 'Empty: your everyday account.' : 'Connect Pocket Money first.'}>
          <Select ariaLabel="Account" value={String(f.money_account || '')} onChange={v => setF({ ...f, money_account: Number(v) })} placeholder="Everyday account" clearLabel="Everyday account" disabled={!moneyOn} options={(accts || []).map(a => ({ value: String(a.id), label: a.name, hint: a.currency }))} />
        </SettingRow>
      </Section>
      <Section title="Follow-up tasks" icon="check-square">
        <SettingRow title="Due after" help="Days after a booking that happened; 0 = no due day."><input type="number" className="num-sm" min={0} max={365} value={f.task_days} onChange={e => setF({ ...f, task_days: Math.max(0, Number(e.target.value) || 0) })} /></SettingRow>
      </Section>
      <div className="row"><button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button></div>
    </div>
  )
}

function Notifications() {
  const { f, setF, save, busy } = useSettingsForm()
  const { toast } = useToast()
  const { run, status } = useTester()
  if (!f) return <Loading />
  const now = async () => { try { const r = await api.post<{ sent: boolean; text: string }>(`${V1}/notify/check`); toast(r.sent ? 'Sent' : r.text ? `Nothing sent (no push address): ${r.text.split('\n')[0]}` : 'Nothing to tell: no bookings today and none waiting') } catch (e) { toast((e as Error).message, 'err') } }
  return (
    <div className="stack loose">
      <Section title="Push" icon="bell" actions={<span className={`badge ${f.notify_configured ? 'ok' : ''}`}>{f.notify_configured ? 'on' : 'not set'}</span>}
        sub={<>Where messages go: an Apprise gateway (<code>…/notify/&lt;key&gt;</code>), an ntfy topic URL, or any webhook that takes JSON.</>}>
        <SettingRow title="URL" stack><input value={f.notify_url} onChange={e => setF({ ...f, notify_url: e.target.value })} placeholder="http://apprise:8000/notify/booking" /></SettingRow>
        <SettingRow title="Format"><Segmented value={f.notify_format} onChange={v => setF({ ...f, notify_format: v })} options={[{ value: 'apprise', label: 'Apprise' }, { value: 'ntfy', label: 'ntfy' }, { value: 'json', label: 'JSON' }]} size="sm" /></SettingRow>
      </Section>
      <Section title="What to tell you" icon="clock">
        <SettingRow title="Each booking" help="Who booked what and when; also when someone cancels or moves a booking."><Switch checked={f.notify_each} onChange={v => setF({ ...f, notify_each: v })} label="Each booking" /></SettingRow>
        <SettingRow title="Morning note" help="The day's bookings, and how many wait for your yes."><Switch checked={f.notify_digest} onChange={v => setF({ ...f, notify_digest: v })} label="Morning note" /></SettingRow>
        <SettingRow title="Morning message at" help="Your time zone."><Select ariaLabel="Hour" value={String(f.notify_hour)} onChange={v => setF({ ...f, notify_hour: Number(v) })} options={HOURS} /></SettingRow>
        <SettingRow title="Also to my phones" help={f.core_configured ? 'Through Pocket Core to the phones with Pocket Hub.' : 'Needs Pocket Core.'}><Switch checked={f.push_phone} onChange={v => setF({ ...f, push_phone: v })} label="Phones" disabled={!f.core_configured} /></SettingRow>
      </Section>
      <div className="row wrap">
        <button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button>
        <button type="button" className="btn" onClick={() => run('notify', f.notify_url, '', { format: f.notify_format })} disabled={!f.notify_url}><Icon name="mail" size={15} />Send a test</button>
        <button type="button" className="btn" onClick={now}><Icon name="refresh" size={15} />Check now</button>
        {status('notify')}
      </div>
    </div>
  )
}

const APPS: { slug: AppSlug; title: string; sub: string; token: string; port: string }[] = [
  { slug: 'calendar', title: 'Pocket Calendar — busy times and bookings', sub: 'Times you are busy there cannot be booked, and each booking goes into your calendar — moved and removed with it.', token: 'Pocket Calendar → Settings → API tokens', port: '8190' },
  { slug: 'mail', title: 'Pocket Mail — emails to people who book', sub: 'Their confirmation with a calendar file, a reminder, and any change, from your own mailbox (it lands in your Sent folder).', token: 'Pocket Mail → Settings → API tokens', port: '8208' },
  { slug: 'contacts', title: 'Pocket Contacts — the people who book', sub: 'Add whoever booked to your contacts (name, email, phone), and see when they are there already.', token: 'Pocket Contacts → Settings → API tokens', port: '8191' },
  { slug: 'crm', title: 'Pocket CRM — bookings become leads', sub: 'Turn an intro call into a deal, with what they wrote, by hand or by itself for each booking.', token: 'Pocket CRM → Settings → API & shortcuts', port: '8249' },
  { slug: 'invoicing', title: 'Pocket Invoicing — paid bookings', sub: 'A draft invoice for a booking with a price, for the person who booked.', token: 'Pocket Invoicing → Settings → API tokens', port: '8168' },
  { slug: 'money', title: 'Pocket Money — payments', sub: 'When a booking is paid, put the payment in Pocket Money as income.', token: 'Pocket Money → Settings → API tokens', port: '8178' },
  { slug: 'tasks', title: 'Pocket Tasks — get ready, follow up', sub: 'A task the day before, or to follow up after.', token: 'Pocket Tasks → Settings → API tokens', port: '8184' },
  { slug: 'notes', title: 'Pocket Notes — meeting notes', sub: 'Start a note with who booked and what they wrote.', token: 'Pocket Notes → Settings → API tokens', port: '8182' },
  { slug: 'places', title: 'Pocket Places — where to meet', sub: 'Pick one of your places as where a booking happens.', token: 'Pocket Places → Settings → API tokens', port: '8236' },
  { slug: 'time', title: 'Pocket Time — time worked', sub: 'Log a booking that happened as time in Pocket Time.', token: 'Pocket Time → Settings → API tokens', port: '8187' },
]

function Integrations() {
  const { f, setF, save, busy } = useSettingsForm()
  const { run, status } = useTester()
  if (!f) return <Loading />
  const coreOn = f.core_configured && f.core_registered !== false
  const eff = (m: ConnMode): ConnMode => !coreOn && m === 'core' ? 'off' : m
  const set = (k: string, v: string) => setF({ ...f, [k]: v } as SettingsT)
  return (
    <div className="stack loose">
      <div className="callout info"><Icon name="info" size={15} /><div>These connections are yours. Booking only reads your busy times and adds things in them when you ask (or when a page is set to do it by itself). Through <b>Pocket Core</b> Booking acts as you in your own Pocket apps with nothing to paste (switch it off for yourself in Pocket Core → Settings → Apps acting for me); <b>Direct</b> uses a URL and an API token you enter.{!coreOn && <CoreHint />}</div></div>
      {APPS.map(a => {
        const mode = f[`${a.slug}_mode`]
        const via = f[`${a.slug}_via`]
        const name = APP_INFO[a.slug].name
        return (
          <Section key={a.slug} title={a.title} icon={APP_INFO[a.slug].icon} actions={connBadge(eff(mode), via)} sub={a.sub}>
            <Connection app={name} mode={mode} via={via} detail={f[`${a.slug}_detail`]} core={coreOn} hasToken={!!f[`${a.slug}_token`]}
              onMode={m => save({ [`${a.slug}_mode`]: m } as Partial<SettingsT>)} onDisconnect={() => save({ [`${a.slug}_url`]: '', [`${a.slug}_token`]: '' } as Partial<SettingsT>)}
              test={<><button type="button" className="btn sm" onClick={() => run(a.slug, f[`${a.slug}_url`], f[`${a.slug}_token`])}>Test connection</button>{status(a.slug)}</>}
              direct={<div className="two-col tight">
                <Field label="URL" help="Inside Docker use http://host.docker.internal:PORT or the service name."><input value={f[`${a.slug}_url`]} onChange={e => set(`${a.slug}_url`, e.target.value)} placeholder={`http://host.docker.internal:${a.port}`} /></Field>
                <Field label="API token" help={a.token}><input value={f[`${a.slug}_token`]} onChange={e => set(`${a.slug}_token`, e.target.value)} autoComplete="off" /></Field>
              </div>} />
          </Section>
        )
      })}
      <Section title="Other Pocket apps" icon="layers" sub="Booking offers your bookings to the rest of the box through Pocket Core: Pocket Day shows the day's bookings, Pocket Search finds them, Pocket Contacts and Pocket CRM show a person's bookings and send them a personal link, Pocket AI can tell you what is coming and book or move a time when you ask, and Pocket Flows can start a flow from every booking. Switched off until you allow them in Pocket Core → Links.">
        <dl className="set-dl">
          <div><dt>booking:read</dt><dd className="muted">Your bookings and booking pages, and their free times.</dd></div>
          <div><dt>booking:write</dt><dd className="muted">Book someone into a free time, confirm, move or cancel a booking, make a personal link, start a page as a draft.</dd></div>
          <div><dt>events:read</dt><dd className="muted"><code>booking.booked</code>, <code>booking.confirmed</code>, <code>booking.cancelled</code>, <code>booking.moved</code>, <code>booking.declined</code>, <code>booking.done</code>, <code>booking.no_show</code> — for Pocket Flows.</dd></div>
        </dl>
      </Section>
      <div className="row"><button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button></div>
    </div>
  )
}

function AITab() {
  const { f, setF, save, busy } = useSettingsForm()
  const { run, status } = useTester()
  const { data: ai } = useAsync(() => api.get<{ ready: boolean; provider: string; detail: string }>(`${V1}/ai/status`), [])
  if (!f) return <Loading />
  const coreOn = f.core_configured && f.core_registered !== false
  const shown = !coreOn && f.ai_provider === 'pocket' ? 'openai' : (f.ai_provider || 'auto')
  return (
    <div className="stack loose">
      <Section title="What the AI does" icon="sparkles" sub="Two things, each only when you press its button: a booking page set up from a sentence, and “Get ready” — a short brief before a meeting from what they wrote and their earlier bookings. Only that page or booking is sent. Questions about your bookings go to Pocket AI through the Ask button at the top.">
        <SettingRow title="Pocket AI buttons" help="Off: the buttons are gone and nothing is sent to an AI."><Switch checked={f.ai_help} onChange={v => save({ ai_help: v })} label="AI buttons" /></SettingRow>
        {ai && (ai.ready ? <p className="muted small row" style={{ gap: 6 }}><Icon name="sparkles" size={13} />{ai.provider === 'pocket' ? 'Pocket AI' : 'Your OpenAI key'} is ready.</p>
          : <div className="callout info"><Icon name="info" size={15} /><div>No AI connected{ai.detail ? ` (${ai.detail})` : ''}.{!coreOn && <CoreHint />}</div></div>)}
      </Section>
      <Section title="Provider" icon="sparkles">
        <div className="grid" style={{ gap: 12 }}>
          <Field group label="Who writes" help="Automatic prefers your enabled Pocket AI connection; without one it uses your OpenAI settings.">
            <Select ariaLabel="AI provider" value={shown} onChange={v => setF({ ...f, ai_provider: v as SettingsT['ai_provider'] })} options={[{ value: 'auto', label: coreOn ? 'Automatic — prefer Pocket AI' : 'Automatic' }, { value: 'pocket', label: 'Pocket AI', disabled: !coreOn, hint: coreOn ? undefined : 'needs Pocket Core' }, { value: 'openai', label: 'OpenAI / compatible API' }]} />
          </Field>
          {!coreOn && <CoreHint />}
          {coreOn && shown !== 'openai' && <PocketModelSelect value={f.ai_pocket_model || ''} onChange={v => setF({ ...f, ai_pocket_model: v })} />}
          {(shown === 'openai' || (!coreOn && shown === 'auto')) && <>
            <Field label="OpenAI API key" help="platform.openai.com → API keys. Stored on the server; shown masked."><input value={f.openai_api_key} onChange={e => setF({ ...f, openai_api_key: e.target.value })} autoComplete="off" placeholder="sk-…" /></Field>
            <div className="two-col tight">
              <Field label="Model" help="Default gpt-4o-mini."><input value={f.openai_model} onChange={e => setF({ ...f, openai_model: e.target.value })} placeholder="gpt-4o-mini" /></Field>
              <Field label="Base URL (optional)" help="Ollama, LM Studio, OpenRouter…"><input value={f.openai_base_url} onChange={e => setF({ ...f, openai_base_url: e.target.value })} placeholder="https://api.openai.com/v1" /></Field>
            </div>
          </>}
        </div>
      </Section>
      <div className="row wrap">
        <button type="button" className="btn primary" onClick={() => save()} disabled={busy}>Save changes</button>
        {(shown === 'openai' || (!coreOn && shown === 'auto')) && <><button type="button" className="btn" onClick={() => run('openai', f.openai_base_url, f.openai_api_key)} disabled={!f.openai_api_key}>Test the OpenAI key</button>{status('openai')}</>}
      </div>
    </div>
  )
}

function Connection({ app, mode: saved, via, detail, core, onMode, onDisconnect, hasToken, test, direct }: {
  app: string; mode: ConnMode; via: string; detail: string; core: boolean; onMode: (m: ConnMode) => void; onDisconnect: () => void
  hasToken: boolean; test: React.ReactNode; direct: React.ReactNode
}) {
  const [advanced, setAdvanced] = useState(saved === 'direct')
  const mode: ConnMode = !core && saved === 'core' ? 'off' : saved
  const ok = via !== '' && mode !== 'off'
  const showDirect = !core || mode === 'direct' || advanced
  const line = mode === 'off' ? `Off: Booking does not reach ${app}.`
    : mode === 'core' ? (ok ? `Connected through Pocket Core, as you${detail ? ` (${detail})` : ''}. Nothing to fill in; switch it off for yourself in Pocket Core → Settings → Apps acting for me.` : `Not connected: ${detail}.`)
    : (ok ? `Direct connection with your own ${app} URL and token; use Test connection to check it.` : `Not connected: ${detail || 'add the URL and a token'}.`)
  return (
    <div className="conn-block">
      <div className="conn-mode">
        <Segmented size="sm" value={mode} onChange={m => onMode(m as ConnMode)} options={[
          { value: 'core', label: 'Pocket Core', title: core ? 'Through Pocket Core: per person, nothing to paste' : 'Needs Pocket Core: there is none on this box', disabled: !core },
          ...(showDirect ? [{ value: 'direct', label: 'Direct', title: 'Your own URL and API token' }] : []),
          { value: 'off', label: 'Off' },
        ]} />
        {!showDirect && <button type="button" className="conn-advanced" onClick={() => setAdvanced(true)}>Advanced: connect directly with a URL and key</button>}
      </div>
      <div className={`conn-status ${mode === 'off' ? '' : ok ? 'ok' : 'bad'}`}><span className="dot" />{line}</div>
      {mode === 'direct' && <>
        {direct}
        <div className="row wrap">{test}{hasToken && <button type="button" className="btn sm ghost danger-text" onClick={onDisconnect} title="Remove the saved URL and token">Disconnect</button>}</div>
      </>}
      {mode === 'core' && <div className="row wrap">{test}</div>}
    </div>
  )
}

const connBadge = (mode: ConnMode, via: string) => mode === 'off' ? <span className="badge">off</span>
  : via ? <span className="badge ok">{via === 'core' ? 'Pocket Core' : 'direct'}</span>
  : <span className="badge warn">not connected</span>

function Users() {
  const { user, users, reloadMeta, status } = useApp()
  const { toast } = useToast()
  const [edit, setEdit] = useState<Partial<User> | null>(null)
  const [del, setDel] = useState<User | null>(null)
  const isAdmin = user?.role === 'admin'
  const run = async (fn: () => Promise<unknown>, msg: string) => { try { await fn(); await reloadMeta(); toast(msg) } catch (e) { toast((e as Error).message, 'err') } }
  return (
    <Section title="People on this box" actions={isAdmin && <button type="button" className="btn primary sm" onClick={() => setEdit({ role: 'member' })}><Icon name="plus" size={14} />Add person</button>}
      sub="Everyone signs in with their own account and has their own booking pages, bookings, hours, settings and connections. Admins manage accounts; they do not see anyone else’s bookings.">
      <div className="set-people">
        {users.map(u => (
          <div key={u.id} className="set-person">
            <Avatar name={u.name} email={u.email} size={34} />
            <div className="grow">
              <div className="set-person-name">{u.name || <span className="muted">—</span>}{u.id === user?.id && <span className="badge">you</span>}{u.role === 'admin' && <span className="badge accent">admin</span>}</div>
              <div className="muted small ellipsis">{u.email}</div>
            </div>
            {isAdmin && <span className="row" style={{ justifyContent: 'flex-end' }}><button type="button" className="btn ghost sm" onClick={() => setEdit(u)}>Edit</button>{u.id !== user?.id && <button type="button" className="btn ghost sm danger-text" onClick={() => setDel(u)}>Remove</button>}</span>}
          </div>
        ))}
      </div>
      {status?.auth_mode !== 'local' && <p className="muted small mt">Auth mode is <code>{status?.auth_mode}</code>: users are created automatically on first visit; passwords are not used here.</p>}
      {edit && <UserModal user={edit} local={status?.auth_mode === 'local'} onClose={() => setEdit(null)} onSaved={() => { setEdit(null); reloadMeta() }} />}
      {del && <Confirm confirmLabel="Remove" title={`Remove ${del.name || del.email}?`} message="Their account is deleted together with their pages and bookings; their booking links stop working." onCancel={() => setDel(null)} onConfirm={() => { const d = del; setDel(null); run(() => api.del(`${V1}/users/${d.id}`), 'User removed') }} />}
    </Section>
  )
}

function UserModal({ user, local, onClose, onSaved }: { user: Partial<User>; local: boolean; onClose: () => void; onSaved: () => void }) {
  const { toast } = useToast()
  const [f, setF] = useState({ name: user.name || '', email: user.email || '', password: '', role: user.role || 'member' })
  const [busy, setBusy] = useState(false)
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    try { if (user.id) await api.put(`${V1}/users/${user.id}`, f); else await api.post(`${V1}/users`, f); toast(user.id ? 'User saved' : 'User added'); onSaved() } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={user.id ? 'Edit person' : 'Add a person'} onClose={onClose} size="sm">
      <form onSubmit={save} className="grid" style={{ gap: 14 }}>
        <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} autoFocus required maxLength={60} /></Field>
        <Field label="Email" help="Used to sign in."><input type="email" value={f.email} onChange={e => setF({ ...f, email: e.target.value })} required /></Field>
        {local && <Field label={user.id ? 'New password (leave empty to keep)' : 'Password'} help="At least 8 characters."><input type="password" value={f.password} onChange={e => setF({ ...f, password: e.target.value })} minLength={user.id ? 0 : 8} required={!user.id} autoComplete="new-password" /></Field>}
        <Field group label="Role"><Segmented value={f.role} onChange={v => setF({ ...f, role: v })} options={[{ value: 'member', label: 'Member' }, { value: 'admin', label: 'Admin' }]} /></Field>
        <div className="form-actions"><button type="button" className="btn" onClick={onClose}>Cancel</button><button className="btn primary" disabled={busy}>{user.id ? 'Save' : 'Add person'}</button></div>
      </form>
    </Modal>
  )
}

function Profile() {
  const { user, setUser, status } = useApp()
  const { toast } = useToast()
  const [f, setF] = useState({ name: user?.name || '', email: user?.email || '', current_password: '', new_password: '' })
  const [busy, setBusy] = useState(false)
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    try { setUser(await api.put(`${V1}/auth/me`, f)); setF(x => ({ ...x, current_password: '', new_password: '' })); toast('Profile saved') } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <form onSubmit={save} className="stack loose">
      <div className="set-profile">
        <Avatar name={user?.name} email={user?.email} size={56} />
        <div className="grow"><h3>{user?.name || user?.email}</h3><p className="muted small">{user?.email} · {user?.role}</p></div>
      </div>
      <Section title="You">
        <div className="stack">
          <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} /></Field>
          <Field label="Email"><input type="email" value={f.email} onChange={e => setF({ ...f, email: e.target.value })} /></Field>
        </div>
      </Section>
      {status?.auth_mode === 'local' && <Section title="Password" sub="Leave empty to keep the current one."><div className="stack">
        <Field label="Current password"><input type="password" value={f.current_password} onChange={e => setF({ ...f, current_password: e.target.value })} autoComplete="current-password" /></Field>
        <Field label="New password" help="At least 8 characters."><input type="password" value={f.new_password} onChange={e => setF({ ...f, new_password: e.target.value })} autoComplete="new-password" /></Field>
      </div></Section>}
      <div className="row"><button className="btn primary" disabled={busy}>Save changes</button></div>
    </form>
  )
}

function Tokens() {
  const { toast } = useToast()
  const { settings } = useApp()
  const { data, reload } = useAsync(() => api.get<APIToken[]>(`${V1}/tokens`), [])
  const [name, setName] = useState('')
  const [fresh, setFresh] = useState('')
  const [del, setDel] = useState<APIToken | null>(null)
  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    try { const r = await api.post<{ token: string }>(`${V1}/tokens`, { name: name || 'token' }); setFresh(r.token); setName(''); reload() } catch (e) { toast((e as Error).message, 'err') }
  }
  const base = settings?.base_url || window.location.origin
  return (
    <div className="stack loose">
      <Section title="Personal tokens" icon="key" sub={<>Use as <code>Authorization: Bearer &lt;token&gt;</code>. A token acts as you: it can read and change everything, feed a Homepage widget (<code>GET /api/v1/summary</code>) or send a recording from a Siri Shortcut. See <code>docs/api.md</code>.</>}>
        <form onSubmit={create} className="row mb"><input value={name} onChange={e => setName(e.target.value)} placeholder="Token name (e.g. iPhone Shortcut)" /><button className="btn primary"><Icon name="plus" size={15} />Create</button></form>
        {fresh && <div className="set-fresh mb"><div className="row"><Icon name="check-circle" size={16} /><b>Copy it now — it is shown once.</b></div><pre className="mono">{fresh}</pre></div>}
        {data && data.length > 0 ? (
          <table className="table compact">
            <thead><tr><th>Name</th><th>Prefix</th><th>Last used</th><th>Created</th><th /></tr></thead>
            <tbody>{data.map(t => <tr key={t.id}><td>{t.name}</td><td className="mono">{t.prefix}…</td><td className="muted">{t.last_used_at ? fmtDateTime(t.last_used_at) : 'never'}</td><td className="muted">{fmtDateTime(t.created_at)}</td><td className="right"><button type="button" className="btn ghost sm danger-text" onClick={() => setDel(t)}>Revoke</button></td></tr>)}</tbody>
          </table>
        ) : <p className="muted">No tokens yet.</p>}
        {del && <Confirm confirmLabel="Revoke" title={`Revoke “${del.name}”?`} message="Anything using this token stops working immediately." onCancel={() => setDel(null)} onConfirm={async () => { await api.del(`${V1}/tokens/${del.id}`); setDel(null); reload() }} />}
      </Section>
      <Section title="Endpoints" icon="code" sub={<>At <code>{base}/api/v1</code>.</>}>
        <dl className="set-dl">
          <div><dt><code>GET /bookings?view=upcoming</code></dt><dd className="muted">Your bookings, with <code>view</code> (upcoming, pending, past, cancelled), <code>from</code>/<code>to</code>, <code>q</code>, <code>email</code> and <code>type_id</code>.</dd></div>
          <div><dt><code>GET /types/&lt;id&gt;/slots?days=7</code></dt><dd className="muted">The free times of a page; <code>POST /bookings</code> books one.</dd></div>
          <div><dt><code>GET /summary</code></dt><dd className="muted">Today, waiting for you, this week and the next booking — for Homepage.</dd></div>
          <div><dt><code>GET /day/YYYY-MM-DD</code></dt><dd className="muted">The bookings of a day — what Pocket Day reads.</dd></div>
          <div><dt><code>GET /lookup?email=…</code></dt><dd className="muted">One person’s bookings with you.</dd></div>
        </dl>
      </Section>
    </div>
  )
}

function Data() {
  const { user } = useApp()
  const [importing, setImporting] = useState(false)
  return (
    <div className="stack loose">
      <Section title="Bring bookings here" icon="upload" sub="Past bookings from Calendly or Cal.com (CSV), or a Pocket Booking export. Importing the same file twice adds nothing twice.">
        <button type="button" className="btn primary" onClick={() => setImporting(true)}><Icon name="upload" size={15} />Bring bookings here…</button>
      </Section>
      <Section title="Export" icon="archive" sub="Everything of yours: your hours, every booking page and every booking, as one JSON file. It comes back in with “Bring bookings here”, also on another box.">
        <a className="btn" href={`${V1}/export.json`}><Icon name="download" size={15} />Export JSON</a>
      </Section>
      {user?.role === 'admin' && <Section title="Database backup" icon="database" sub="The whole box, every account — admins only.">
        <p className="muted">A consistent copy of the SQLite database (<code>VACUUM INTO</code>). Restore by stopping the app and replacing <code>pocket-booking.db</code> in the data folder.</p>
        <a className="btn" href={`${V1}/backup`}><Icon name="database" size={15} />Download database</a>
      </Section>}
      {importing && <ImportModal onClose={() => setImporting(false)} />}
    </div>
  )
}

function About() {
  const { data, error } = useAsync(() => api.get<Record<string, unknown>>(`${V1}/settings/system`), [])
  const [keys, setKeys] = useState(false)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} /><span>{error}</span></div>
  if (!data) return <Loading />
  const counts = (data.counts || {}) as Record<string, number>
  const core = (data.core || {}) as { configured?: boolean; registered?: boolean; url?: string; error?: string }
  const tiles: [string, number | undefined, string][] = [['Booking pages', counts.types, 'list'], ['Bookings', counts.bookings, 'calendar'], ['Personal links', counts.invites, 'link'], ['Sent on to apps', counts.made, 'layers']]
  const rows: [string, React.ReactNode][] = [
    ['Version', <code key="v">{String(data.version)}</code>],
    ['Uptime', String(data.uptime)],
    ['Runtime', `${String(data.go)} · ${String(data.os)}/${String(data.arch)}`],
    ['Time zone', String(data.timezone)],
    ['Sign-in', String(data.auth_mode)],
    ['Pocket Core', core.configured ? (core.registered ? <span key="c" className="ok">connected · {core.url}</span> : <span key="c" className="bad">{core.error || 'not registered yet'}</span>) : <span key="c" className="muted">not configured</span>],
    ['Database', <span key="db"><code>{String(data.db_path)}</code> · {fmtBytes(Number(data.db_bytes))}</span>],
    ['People', `${counts.users ?? 0} accounts on this box`],
  ]
  return (
    <div className="stack loose">
      <div className="set-about-hero">
        <img src="/favicon.svg" alt="" width={48} height={48} />
        <div>
          <h3>Pocket Booking</h3>
          <p className="muted small">Booking pages people use to pick a free time with you, without an account or emails back and forth. Your hours and Pocket Calendar decide what is free; bookings go on to your calendar, contacts, mail, leads, invoices, money, tasks and notes. One binary, one SQLite file, one volume.</p>
        </div>
      </div>
      <div className="set-stats">
        {tiles.map(([label, n, icon]) => (
          <div key={label} className="set-stat"><Icon name={icon} size={16} /><span className="set-stat-n tnum">{(n ?? 0).toLocaleString()}</span><span className="set-stat-l">{label}</span></div>
        ))}
      </div>
      <Section title="This server">
        <dl className="set-dl">
          {rows.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}
        </dl>
      </Section>
      <div className="row wrap">
        <button type="button" className="btn" onClick={() => setKeys(true)}><Icon name="keyboard" size={15} />Keyboard shortcuts</button>
        <a className="btn" href="https://github.com/abjelosevic88/pocket-homelab-booking" target="_blank" rel="noreferrer"><Icon name="external" size={15} />Project on GitHub</a>
      </div>
      <p className="muted small">Open source (MIT). Part of the Pocket apps: Pocket Core connects them — Contacts, Projects, Tasks, Calendar, Mail, Sign, Flows, Day, Pocket AI and more.</p>
      {keys && <ShortcutsSheet onClose={() => setKeys(false)} />}
    </div>
  )
}

