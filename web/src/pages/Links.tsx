// Personal booking links: each books once, with a name and email filled in.
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useOnChange } from '../lib/data'
import { ago, fmtDate } from '../lib/format'
import type { Invite } from '../lib/types'
import { openCreate, useCrumb } from '../components/Layout'
import { Icon } from '../components/icons'
import { Empty, Loading, errMsg, useAsync, useToast } from '../components/ui'

export default function Links() {
  const { toast } = useToast()
  const { data, reload } = useAsync(() => api.get<{ invites: Invite[] }>(`${V1}/invites`).then(r => r.invites), [])
  useOnChange(reload)
  useCrumb(undefined)
  const copy = (iv: Invite) => navigator.clipboard?.writeText(iv.url).then(() => toast('Link copied'), () => toast(iv.url))
  const del = async (iv: Invite) => { try { await api.del(`${V1}/invites/${iv.id}`); announceChange(); toast('Removed') } catch (e) { toast(errMsg(e), 'err') } }
  const now = new Date().toISOString()
  return (
    <div className="page stack loose">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Personal links</h1><p className="muted">A link for one person: it opens a booking page with their name and email filled in, and books once. Pocket Contacts and Pocket CRM can make them too.</p></div>
        <button type="button" className="btn primary" onClick={() => openCreate('link')}><Icon name="plus" size={15} />New link</button>
      </div>
      {!data ? <Loading /> : data.length === 0 ? <Empty icon="link" title="No personal links yet"><p>Send one to someone you want to talk to: they pick a time that suits them.</p></Empty>
        : <section className="card flush"><ul className="list">{data.map(iv => {
          const gone = !iv.used && iv.expires_at && iv.expires_at < now
          return (
            <li key={iv.id} className="list-row">
              <span className="list-ico"><Icon name={iv.used ? 'check-circle' : gone ? 'clock' : 'link'} size={16} /></span>
              <span className="grow row-main">
                <span className="row-title ellipsis">{iv.name || iv.email || 'Anyone with the link'}</span>
                <span className="muted small ellipsis">{iv.type}{iv.note ? ` · ${iv.note}` : ''} · made {ago(iv.created_at)}{iv.source !== 'owner' ? ` by ${iv.source}` : ''}</span>
              </span>
              <span className="row-side">
                {iv.used ? <Link className="pill ok" to={`/bookings/${iv.booking_id}`}>Booked</Link> : gone ? <span className="pill">Ran out</span> : <span className="muted small">until {fmtDate(iv.expires_at.slice(0, 10))}</span>}
                {!iv.used && !gone && <button type="button" className="btn ghost sm icon-only" aria-label="Copy the link" title="Copy the link" onClick={() => copy(iv)}><Icon name="copy" size={14} /></button>}
                <button type="button" className="btn ghost sm icon-only" aria-label="Remove" title="Remove" onClick={() => del(iv)}><Icon name="trash" size={14} /></button>
              </span>
            </li>
          )
        })}</ul></section>}
    </div>
  )
}
