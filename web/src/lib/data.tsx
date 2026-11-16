// The person's booking pages and how many bookings wait, shared by every page.
import React, { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { api, V1 } from './api'
import type { BookingType, Summary, Template } from './types'

const EVENT = 'pbkg:changed'

interface DataState {
  types: BookingType[] | null
  templates: Template[]
  summary: Summary | null
  reload: () => Promise<void>
}
const Ctx = createContext<DataState>({ types: null, templates: [], summary: null, reload: async () => {} })
export function useData() { return useContext(Ctx) }

/** Something changed (a booking confirmed, a page saved): lists and counts reload. */
export function announceChange() { window.dispatchEvent(new CustomEvent(EVENT)) }

export function useOnChange(fn: () => void) {
  useEffect(() => {
    window.addEventListener(EVENT, fn)
    return () => window.removeEventListener(EVENT, fn)
  }, [fn])
}

export function DataProvider({ children }: { children: React.ReactNode }) {
  const [types, setTypes] = useState<BookingType[] | null>(null)
  const [templates, setTemplates] = useState<Template[]>([])
  const [summary, setSummary] = useState<Summary | null>(null)
  const reload = useCallback(async () => {
    try {
      const [t, s] = await Promise.all([api.get<{ types: BookingType[]; templates: Template[] }>(`${V1}/types`), api.get<Summary>(`${V1}/summary`)])
      setTypes(t.types); setTemplates(t.templates); setSummary(s)
    } catch { setTypes(x => x || []) }
  }, [])
  useEffect(() => { reload() }, [reload])
  useOnChange(reload)
  // new bookings arrive while the page is open: look once a minute, quietly
  useEffect(() => {
    const t = window.setInterval(() => { if (document.visibilityState === 'visible') reload() }, 60000)
    return () => window.clearInterval(t)
  }, [reload])
  return <Ctx.Provider value={{ types, templates, summary, reload }}>{children}</Ctx.Provider>
}
