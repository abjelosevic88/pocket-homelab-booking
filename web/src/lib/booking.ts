import type { AppSlug, BookingStatus, BookingType, LocationKind, QuestionType } from './types'

export const APP_INFO: Record<AppSlug, { name: string; icon: string }> = {
  calendar: { name: 'Pocket Calendar', icon: 'calendar' },
  contacts: { name: 'Pocket Contacts', icon: 'user' },
  mail: { name: 'Pocket Mail', icon: 'mail' },
  crm: { name: 'Pocket CRM', icon: 'trending-up' },
  invoicing: { name: 'Pocket Invoicing', icon: 'receipt' },
  money: { name: 'Pocket Money', icon: 'wallet' },
  tasks: { name: 'Pocket Tasks', icon: 'check-square' },
  notes: { name: 'Pocket Notes', icon: 'note' },
  places: { name: 'Pocket Places', icon: 'pin' },
  time: { name: 'Pocket Time', icon: 'timer' },
}

/** What can be made from a booking, and in which app. */
export const TARGETS: { target: string; app: AppSlug; label: string; icon: string; help: string }[] = [
  { target: 'event', app: 'calendar', label: 'Put it in my calendar', icon: 'calendar-plus', help: 'An event in Pocket Calendar that moves and goes with the booking' },
  { target: 'contact', app: 'contacts', label: 'Add them to my contacts', icon: 'user', help: 'Name, email and phone in Pocket Contacts' },
  { target: 'deal', app: 'crm', label: 'Add a lead in Pocket CRM', icon: 'trending-up', help: 'A deal with what they wrote' },
  { target: 'task', app: 'tasks', label: 'Make a task', icon: 'check-square', help: 'Get ready before, or follow up after' },
  { target: 'note', app: 'notes', label: 'Start a meeting note', icon: 'note', help: 'Who, when and what they wrote' },
  { target: 'invoice', app: 'invoicing', label: 'Make a draft invoice', icon: 'receipt', help: 'For the price of the booking' },
  { target: 'payment', app: 'money', label: 'Put the payment in Pocket Money', icon: 'wallet', help: 'Income on your account' },
  { target: 'time', app: 'time', label: 'Log the time in Pocket Time', icon: 'timer', help: 'After it happened' },
]

export const STATUS: Record<BookingStatus, { label: string; cls: string; icon: string }> = {
  pending: { label: 'Waits for you', cls: 'warn', icon: 'hourglass' },
  confirmed: { label: 'Booked', cls: 'ok', icon: 'check' },
  declined: { label: 'Declined', cls: '', icon: 'x' },
  cancelled: { label: 'Cancelled', cls: 'bad', icon: 'x-circle' },
  done: { label: 'Happened', cls: '', icon: 'check-circle' },
  no_show: { label: 'Did not come', cls: 'bad', icon: 'eye-off' },
}

export const LOCATIONS: { value: LocationKind; label: string; icon: string; help: string }[] = [
  { value: 'video', label: 'Video call', icon: 'video', help: 'A video call link (Jitsi) is made for each booking' },
  { value: 'link', label: 'My own video link', icon: 'link', help: 'Your Zoom, Meet or Teams room' },
  { value: 'phone', label: 'I call them', icon: 'phone', help: 'They give their number' },
  { value: 'phone_in', label: 'They call me', icon: 'phone', help: 'Your number is shown after booking' },
  { value: 'place', label: 'In person', icon: 'pin', help: 'An address' },
  { value: 'ask', label: 'They say where', icon: 'navigation', help: 'Asked when they book' },
]
export const locInfo = (k: string) => LOCATIONS.find(l => l.value === k) || LOCATIONS[0]

export const QUESTION_TYPES: { type: QuestionType; label: string; icon: string }[] = [
  { type: 'text', label: 'Short answer', icon: 'text' },
  { type: 'long', label: 'Paragraph', icon: 'list' },
  { type: 'choice', label: 'One of a list', icon: 'circle' },
  { type: 'checkboxes', label: 'Several of a list', icon: 'check-square' },
  { type: 'yesno', label: 'Yes or no', icon: 'check-circle' },
  { type: 'number', label: 'Number', icon: 'percent' },
  { type: 'url', label: 'Web address', icon: 'link' },
  { type: 'date', label: 'Date', icon: 'calendar' },
]

export const LANGUAGES = [{ value: 'en', label: 'English' }, { value: 'bs', label: 'Bosanski' }, { value: 'hr', label: 'Hrvatski' }, { value: 'sr', label: 'Srpski' }, { value: 'de', label: 'Deutsch' }]

export const ACCENTS = ['#087f5b', '#3157d5', '#0f766e', '#7048e8', '#b4532a', '#be123c', '#a16207', '#1f2937']

export const typeState = (t: BookingType) => t.status === 'open' ? { label: t.invite_only ? 'Personal links only' : 'Open', cls: 'ok', icon: 'check' }
  : t.status === 'paused' ? { label: 'Paused', cls: 'warn', icon: 'pause' } : { label: 'Draft', cls: '', icon: 'edit' }

export function lengths(ds: number[]): string {
  return ds.map(d => d >= 60 && d % 60 === 0 ? `${d / 60} h` : d > 60 ? `${Math.floor(d / 60)} h ${d % 60} min` : `${d} min`).join(' / ')
}

export function money(cents: number, cur: string): string {
  if (!cents) return ''
  const v = cents / 100
  try { return new Intl.NumberFormat(undefined, { style: 'currency', currency: cur || 'EUR', maximumFractionDigits: v % 1 ? 2 : 0 }).format(v) } catch { return `${v} ${cur}` }
}

export const WEEKDAYS = [{ d: '1', label: 'Monday' }, { d: '2', label: 'Tuesday' }, { d: '3', label: 'Wednesday' }, { d: '4', label: 'Thursday' }, { d: '5', label: 'Friday' }, { d: '6', label: 'Saturday' }, { d: '0', label: 'Sunday' }]

export const ZONES = ['Europe/Sarajevo', 'Europe/Belgrade', 'Europe/Zagreb', 'Europe/Ljubljana', 'Europe/Vienna', 'Europe/Berlin', 'Europe/London', 'Europe/Paris',
  'Europe/Istanbul', 'America/New_York', 'America/Chicago', 'America/Los_Angeles', 'America/Toronto', 'Asia/Dubai', 'Asia/Singapore', 'Asia/Tokyo', 'Australia/Sydney', 'UTC']
