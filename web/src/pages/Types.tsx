// Your booking pages, and a new one: from a ready-made one, from a sentence
// for Pocket AI, or blank.
import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData } from '../lib/data'
import { lengths, locInfo, money, typeState } from '../lib/booking'
import type { BookingType } from '../lib/types'
import { openCreate, useCrumb } from '../components/Layout'
import { Icon } from '../components/icons'
import { CopyField, Empty, Loading, Spinner, errMsg, useAsync, useToast } from '../components/ui'

export default function Types() {
  const { types } = useData()
  const { settings } = useApp()
  const navigate = useNavigate()
  const { toast } = useToast()
  const copy = (t: BookingType) => { navigator.clipboard?.writeText(t.url).then(() => toast('Link copied'), () => toast(t.url)) }
  return (
    <div className="page stack loose">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Booking pages</h1><p className="muted">Each page is one thing people can book with you. Share its link, put it on your website, or send someone a personal link.</p></div>
        <button type="button" className="btn primary" onClick={() => openCreate('page')}><Icon name="plus" size={15} />New page</button>
      </div>
      {settings?.page_url && <section className="card row wrap your-page">
        <Icon name="globe" size={18} /><div className="grow"><b>Your page with all of them</b><p className="muted small">Lists every open page (not the ones for personal links only).</p></div>
        <div className="your-page-link"><CopyField value={settings.page_url} /></div>
      </section>}
      {!types ? <Loading /> : types.length === 0 ? <Empty icon="list" title="No booking pages yet">
        <p>Start from a ready-made one — an intro call, a consultation, a viewing, a class — or describe what people should book to Pocket AI.</p>
        <button type="button" className="btn primary mt" onClick={() => openCreate('page')}>Make your first booking page</button>
      </Empty> : <div className="form-grid-cards">
        {types.map(t => {
          const st = typeState(t)
          return (
            <article key={t.id} className="form-card" onClick={() => navigate(`/types/${t.id}`)}>
              <div className="fc-head">
                <span className="fc-emoji" aria-hidden="true">{t.emoji || '📅'}</span>
                <span className={`pill ${st.cls}`}><Icon name={st.icon} size={11} />{st.label}</span>
              </div>
              <h3 className="fc-title">{t.title}</h3>
              <p className="muted small fc-sub"><Icon name="clock" size={12} /> {lengths(t.durations)} · <Icon name={locInfo(t.location_kind).icon} size={12} /> {locInfo(t.location_kind).label}{t.price ? ` · ${money(t.price, t.currency)}` : ''}{t.capacity > 1 ? ` · ${t.capacity} people` : ''}</p>
              <div className="fc-nums">
                <button type="button" className="fc-num linkish" onClick={e => { e.stopPropagation(); navigate(`/?type=${t.id}`) }}><b className="tnum">{t.upcoming}</b> coming up</button>
                <span className="fc-num muted"><b className="tnum">{t.bookings}</b> in all</span>
                <span className="fc-num muted"><b className="tnum">{t.views}</b> visits</span>
              </div>
              <div className="fc-foot muted small">
                <span className="grow ellipsis">/b/{t.handle}</span>
                {t.status === 'open' && <button type="button" className="btn ghost sm icon-only" aria-label="Book someone in" title="Book someone in" onClick={e => { e.stopPropagation(); openCreate('book', t.id) }}><Icon name="calendar-plus" size={14} /></button>}
                {t.status === 'open' && <button type="button" className="btn ghost sm icon-only" aria-label="Copy the link" title="Copy the link" onClick={e => { e.stopPropagation(); copy(t) }}><Icon name="link" size={14} /></button>}
                <a className="btn ghost sm icon-only" href={t.url} target="_blank" rel="noreferrer" aria-label="Open the page" title={t.status === 'draft' ? 'Preview' : 'Open the page'} onClick={e => e.stopPropagation()}><Icon name="external" size={14} /></a>
              </div>
            </article>
          )
        })}
      </div>}
    </div>
  )
}

export function NewType() {
  const navigate = useNavigate()
  const [sp] = useSearchParams()
  const { settings } = useApp()
  const { templates } = useData()
  const { toast } = useToast()
  const { data: ai } = useAsync(() => api.get<{ ready: boolean; detail: string }>(`${V1}/ai/status`), [])
  const [busy, setBusy] = useState('')
  const [desc, setDesc] = useState('')
  useCrumb('New page')
  useEffect(() => { if (sp.get('ai')) document.getElementById('ai-desc')?.focus() }, [sp])
  const make = async (body: Record<string, unknown>, key: string) => {
    setBusy(key)
    try { const t = await api.post<BookingType>(`${V1}/types`, body); announceChange(); navigate(`/types/${t.id}`) } catch (e) { toast(errMsg(e), 'err'); setBusy('') }
  }
  const draft = async () => {
    setBusy('ai')
    try {
      const d = await api.post<Record<string, unknown>>(`${V1}/ai/draft`, { prompt: desc })
      await make({ ...d, status: 'draft' }, 'ai')
    } catch (e) { toast(errMsg(e), 'err'); setBusy('') }
  }
  const aiOK = !!ai?.ready && settings?.ai_help !== false
  return (
    <div className="page stack loose">
      <div className="page-head"><div className="grow"><h1 className="page-title">A new booking page</h1><p className="muted">Pick one to start from; the name, length, place, hours and questions can all be changed afterwards.</p></div></div>
      <section className="card ai-start">
        <div className="row" style={{ gap: 10 }}><Icon name="sparkles" size={18} /><h2 className="card-h" style={{ margin: 0 }}>Describe it, and Pocket AI sets it up</h2></div>
        <textarea id="ai-desc" data-gramm="false" rows={2} value={desc} onChange={e => setDesc(e.target.value)} disabled={!aiOK}
          placeholder="e.g. A 45-minute guitar lesson at my flat in Grbavica, beginners welcome, 25 KM, ask their level" />
        <div className="row wrap">
          <button type="button" className="btn primary" onClick={draft} disabled={!aiOK || !desc.trim() || !!busy}>{busy === 'ai' ? <Spinner /> : <Icon name="sparkles" size={15} />}Make the page</button>
          {ai && !aiOK && <span className="muted small">{settings?.ai_help === false ? 'AI help is switched off in Settings → AI.' : `No AI connected${ai.detail ? ` (${ai.detail})` : ''}.`}</span>}
        </div>
      </section>
      <div className="tpl-grid">
        {templates.map(t => (
          <button key={t.key} type="button" className="tpl-card" onClick={() => make({ template: t.key }, t.key)} disabled={!!busy}>
            <span className="tpl-emoji" aria-hidden="true">{busy === t.key ? <Spinner size={20} /> : t.emoji}</span>
            <span className="tpl-text">
              <b>{t.title}</b>
              <span className="muted small">{lengths(t.durations)} · {locInfo(t.location_kind).label}{t.capacity > 1 ? ` · up to ${t.capacity} people` : ''}</span>
              {t.description && <span className="muted small">{t.description}</span>}
            </span>
          </button>
        ))}
        <button type="button" className="tpl-card" onClick={() => make({ title: 'New booking page' }, 'blank')} disabled={!!busy}>
          <span className="tpl-emoji" aria-hidden="true">{busy === 'blank' ? <Spinner size={20} /> : '➕'}</span>
          <span className="tpl-text"><b>Blank</b><span className="muted small">30 minutes, a video call, your working hours.</span></span>
        </button>
      </div>
      <p className="muted small">Moving from Calendly or Cal.com? <button type="button" className="linkish" onClick={() => openCreate('import')}>Bring your past bookings with you</button>.</p>
    </div>
  )
}
