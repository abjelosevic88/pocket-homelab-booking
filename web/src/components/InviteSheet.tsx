// A personal booking link: books once, with their name and email filled in.
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useData } from '../lib/data'
import type { Invite } from '../lib/types'
import { Select } from './Select'
import { CopyField, Field, Modal, Segmented, Spinner, errMsg, useToast } from './ui'

export function InviteSheet({ typeID, name: name0 = '', email: email0 = '', onClose }: { typeID?: number; name?: string; email?: string; onClose: () => void }) {
  const { types } = useData()
  const { toast } = useToast()
  const navigate = useNavigate()
  const usable = (types || []).filter(t => t.status === 'open')
  const [tid, setTid] = useState(typeID || usable[0]?.id || 0)
  const [name, setName] = useState(name0)
  const [email, setEmail] = useState(email0)
  const [note, setNote] = useState('')
  const [days, setDays] = useState(14)
  const [busy, setBusy] = useState(false)
  const [made, setMade] = useState<Invite | null>(null)
  const make = async () => {
    setBusy(true)
    try { const iv = await api.post<Invite>(`${V1}/invites`, { type_id: tid, name, email, note, days }); setMade(iv); announceChange() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const mailto = made ? `mailto:${encodeURIComponent(made.email)}?subject=${encodeURIComponent(`Book a time: ${made.type}`)}&body=${encodeURIComponent(`Hi${made.name ? ' ' + made.name.split(' ')[0] : ''},\n\npick a time that suits you here:\n${made.url}\n`)}` : ''
  if (usable.length === 0) return (
    <Modal title="Send a booking link" onClose={onClose} size="sm">
      <p className="muted">A personal link opens one of your booking pages. Open a page first.</p>
      <div className="row mt"><button type="button" className="btn primary" onClick={() => { onClose(); navigate('/types') }}>Your booking pages</button></div>
    </Modal>
  )
  return (
    <Modal title="Send a booking link" onClose={onClose} size="sm" footer={made ? <button type="button" className="btn primary" onClick={onClose}>Done</button> : <>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={make} disabled={busy || !tid}>{busy && <Spinner />}Make the link</button>
    </>}>
      {made ? <div className="stack">
        <p>This link books <b>{made.type}</b> once{made.name ? <> for <b>{made.name}</b></> : ''}, until {made.expires_at.slice(0, 10)}.</p>
        <CopyField value={made.url} label="Link" />
        <div className="row wrap">
          {made.email && <a className="btn" href={mailto}>Email it</a>}
          <button type="button" className="btn ghost" onClick={() => { onClose(); navigate('/links') }}>All personal links</button>
        </div>
      </div> : <div className="stack">
        <p className="muted small">They pick a time on the page with their name and email already filled in. The link works once.</p>
        <Field label="Booking page"><Select ariaLabel="Booking page" value={String(tid)} onChange={v => setTid(Number(v))} options={usable.map(t => ({ value: String(t.id), label: `${t.emoji ? t.emoji + ' ' : ''}${t.title}` }))} /></Field>
        <Field label="Their name"><input value={name} onChange={e => setName(e.target.value)} /></Field>
        <Field label="Their email"><input type="email" value={email} onChange={e => setEmail(e.target.value)} /></Field>
        <Field label="A note for you" help="Why you sent it — only you see it."><input value={note} onChange={e => setNote(e.target.value)} /></Field>
        <Field label="Works for"><Segmented size="sm" value={days} onChange={setDays} options={[{ value: 7, label: '1 week' }, { value: 14, label: '2 weeks' }, { value: 30, label: '1 month' }, { value: 90, label: '3 months' }]} /></Field>
      </div>}
    </Modal>
  )
}
