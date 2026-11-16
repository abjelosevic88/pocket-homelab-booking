// Left column: book someone in (and the new-thing menu), the bookings and
// the rest of the app, then your booking pages with what is coming up.
import { useState } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { useApp } from '../lib/app-context'
import { useData } from '../lib/data'
import { Icon } from './icons'
import { createItems, type CreateKind } from './Layout'
import { type Anchor, Menu, rectOf } from './ui'

export function Sidebar({ onCreate, onNavigate }: { onCreate: (k: CreateKind) => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const { types, summary } = useData()
  const loc = useLocation()
  const [menu, setMenu] = useState<Anchor | null>(null)
  const title = settings?.title || status?.title || 'Pocket Booking'
  const sp = new URLSearchParams(loc.search)
  const onList = loc.pathname === '/' || loc.pathname.startsWith('/bookings')
  const view = onList ? sp.get('view') || 'upcoming' : ''
  const nav: { to: string; label: string; icon: string; active: boolean; count?: number; hint?: string; warn?: boolean }[] = [
    { to: '/', label: 'Upcoming', icon: 'calendar', active: onList && view === 'upcoming' && !sp.get('type'), count: summary?.today || 0, hint: 'Today' },
    { to: '/?view=pending', label: 'To confirm', icon: 'hourglass', active: onList && view === 'pending', count: summary?.pending || 0, hint: 'Waiting for your yes', warn: true },
    { to: '/?view=past', label: 'Past', icon: 'archive', active: onList && view === 'past' },
    { to: '/types', label: 'Booking pages', icon: 'list', active: loc.pathname.startsWith('/types') },
    { to: '/hours', label: 'Hours & days off', icon: 'clock', active: loc.pathname.startsWith('/hours') },
    { to: '/links', label: 'Personal links', icon: 'link', active: loc.pathname.startsWith('/links') },
    { to: '/insights', label: 'Insights', icon: 'chart', active: loc.pathname.startsWith('/insights') },
  ]
  const live = (types || []).filter(t => t.status !== 'draft').slice(0, 12)
  return (
    <aside className="sidebar" aria-label="Pocket Booking">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Booking' ? title : <>Pocket <b>Booking</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <div className="create-split">
        <button type="button" className="btn primary create-btn" onClick={() => onCreate('book')} title="Book someone in (B)">
          <Icon name="plus" size={15} strokeWidth={2.2} />Book someone in
        </button>
        <button type="button" className="btn primary create-more" aria-label="More ways to start" aria-haspopup="menu" aria-expanded={!!menu} onClick={e => setMenu(menu ? null : rectOf(e.currentTarget))}>
          <Icon name="chevron-down" size={15} />
        </button>
      </div>
      {menu && <Menu anchor={menu} items={createItems(onCreate)} onClose={() => setMenu(null)} width={290} />}

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(n => (
          <NavLink key={n.to} to={n.to} onClick={onNavigate} className={() => `sb-nav-item ${n.active ? 'active' : ''}`}>
            <Icon name={n.icon} size={16} />{n.label}
            {!!n.count && <span className={`sb-count ${n.warn ? 'warn' : ''}`} title={n.hint}>{n.count}</span>}
          </NavLink>
        ))}
      </nav>

      {live.length > 0 && <section className="sb-sec">
        <h4 className="sb-h">Your pages</h4>
        {live.map(t => (
          <Link key={t.id} to={`/?type=${t.id}`} className={`sb-row sb-list ${onList && Number(sp.get('type')) === t.id ? 'active' : ''}`} onClick={onNavigate} title={t.status === 'paused' ? 'Paused' : `${t.upcoming} coming up`}>
            <span className="sb-list-ico sb-emoji" aria-hidden="true">{t.emoji || '📅'}</span>
            <span className="grow ellipsis">{t.title}</span>
            {t.status === 'paused' ? <Icon name="pause" size={12} className="muted" /> : <span className="muted small tnum">{t.upcoming || ''}</span>}
          </Link>
        ))}
      </section>}
      {types && types.length === 0 && <section className="sb-sec">
        <Link to="/types/new" className="sb-row sb-list" onClick={onNavigate}><span className="sb-list-ico" aria-hidden="true"><Icon name="plus" size={14} /></span><span className="grow">Make your first booking page</span></Link>
      </section>}

      <div className="sb-foot">
        <Link to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</Link>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('pbkg:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}
