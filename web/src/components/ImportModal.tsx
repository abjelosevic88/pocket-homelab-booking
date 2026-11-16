// Bring bookings in: a Pocket Booking export, or past bookings from
// Calendly, Cal.com or a spreadsheet as CSV.
import { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { upload, V1 } from '../lib/api'
import { announceChange } from '../lib/data'
import { Icon } from './icons'
import { Modal, Spinner, errMsg, useToast } from './ui'

type Result = { added: { bookings?: number; types?: number; schedules?: number }; skipped: number }

export function ImportModal({ onClose }: { onClose: () => void }) {
  const { toast } = useToast()
  const navigate = useNavigate()
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [res, setRes] = useState<Result | null>(null)
  const pick = async (file: File) => {
    setBusy(true)
    try { setRes(await upload<Result>(`${V1}/import`, 'file', file)); announceChange() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const n = (v = 0, one: string, many: string) => `${v} ${v === 1 ? one : many}`
  return (
    <Modal title="Bring bookings here" onClose={onClose} footer={res ? <>
      <button type="button" className="btn" onClick={onClose}>Close</button>
      <button type="button" className="btn primary" onClick={() => { onClose(); navigate('/?view=all') }}>See the bookings</button>
    </> : undefined}>
      {res ? <div className="stack">
        <div className="callout ok"><Icon name="check-circle" size={15} /><div>
          Brought {n(res.added.bookings, 'booking', 'bookings')}{res.added.types ? `, ${n(res.added.types, 'booking page', 'booking pages')}` : ''}{res.added.schedules ? `, ${n(res.added.schedules, 'set of hours', 'sets of hours')}` : ''}{res.skipped ? `; ${res.skipped} were here already or could not be read` : ''}.
        </div></div>
        {!!res.added.types && <p className="muted small">Each kind of booking in the file became a booking page, as a draft: check its hours and open it when you are ready.</p>}
      </div> : <div className="stack">
        <p className="muted">Bringing the same file again adds nothing twice. Past bookings are marked as happened (or cancelled); nobody gets an email.</p>
        <div className="import-ways">
          <div className="radio-row"><Icon name="file" size={18} /><div><b>Bookings as a spreadsheet (CSV)</b><p className="muted small">Calendly (Scheduled events → Export), Cal.com (Bookings → Export) or any sheet with a start time, and a name, email and kind of booking.</p></div></div>
          <div className="radio-row"><Icon name="archive" size={18} /><div><b>A Pocket Booking export</b><p className="muted small">The JSON file from Settings → Backup & data, from this box or another one.</p></div></div>
        </div>
        <input ref={input} type="file" accept=".csv,.json,text/csv,application/json" hidden onChange={e => { const f = e.target.files?.[0]; if (f) pick(f); e.target.value = '' }} />
        <button type="button" className="btn primary" onClick={() => input.current?.click()} disabled={busy}>{busy ? <Spinner /> : <Icon name="upload" size={15} />}Choose a file…</button>
      </div>}
    </Modal>
  )
}
