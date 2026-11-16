export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = '' | 'core' | 'direct'
export type AppSlug = 'calendar' | 'contacts' | 'mail' | 'crm' | 'invoicing' | 'money' | 'tasks' | 'notes' | 'places' | 'time'

type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  tz: string; tz_set: string; page_handle: string; page_name: string; page_intro: string; page_url: string; public_base: string
  language: string; accent: string; currency: string
  check_calendar: boolean; busy_calendars: number[]; event_calendar: number; tell_guest: boolean; mail_from: string
  money_account: number; money_income_cat: number; task_days: number; ai_help: boolean
  notify_each: boolean; notify_digest: boolean; notify_hour: number; push_phone: boolean
  notify_url: string; notify_format: string; notify_configured: boolean
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string; public_url: string; demo: boolean
}

export type QuestionType = 'text' | 'long' | 'choice' | 'checkboxes' | 'yesno' | 'number' | 'url' | 'date'
export interface Question { id: string; type: QuestionType; label: string; help?: string; required?: boolean; options?: string[] }

export interface After { event: boolean; contact: boolean; deal: boolean; task: boolean; note: boolean; invoice: boolean }

export type TypeStatus = 'draft' | 'open' | 'paused'
export type LocationKind = 'video' | 'link' | 'phone' | 'phone_in' | 'place' | 'ask'

export interface BookingType {
  id: number; handle: string; title: string; description: string; emoji: string; status: TypeStatus; durations: number[]
  location_kind: LocationKind; location: string; place_id: string; schedule_id: number; notice_min: number; days_ahead: number
  buffer_before: number; buffer_after: number; step_min: number; max_per_day: number; capacity: number; confirm: 'auto' | 'manual'
  ask_phone: 'no' | 'optional' | 'required'; questions: Question[]; price: number; currency: string; pay_note: string; reminder_h: number
  language: string; accent: string; invite_only: boolean; after: After; thanks: string; source: string; position: number
  created_at: string; updated_at: string; upcoming: number; bookings: number; views: number; url: string; link: string; minutes: number
}

export interface Template { key: string; title: string; emoji: string; durations: number[]; location_kind: LocationKind; capacity: number; description: string }

export type Span = [string, string]
export interface Override { day: string; hours: Span[]; note?: string }
export interface Schedule { id: number; name: string; tz: string; hours: Record<string, Span[]>; is_default: boolean; overrides: Override[]; pages: number }

export type BookingStatus = 'pending' | 'confirmed' | 'declined' | 'cancelled' | 'done' | 'no_show'
export interface Answer { id: string; label: string; value: string }
export interface Made { id: number; booking_id: number; target: string; app: string; ref: string; title: string; url: string; error?: string; auto: boolean; created_at: string }

export interface Booking {
  id: number; type_id: number; title: string; emoji: string; status: BookingStatus; minutes: number; guest_tz: string
  name: string; email: string; phone: string; answers: Answer[]; location_kind: LocationKind | ''; location: string; meeting_url: string
  price: number; currency: string; paid: boolean; paid_at: string; source: string; invite_id?: number; cancel_reason: string; cancelled_by: string
  moved_from: string; owner_note: string; language: string; created_at: string; updated_at: string
  start: string; end: string; date: string; time: string; end_time: string; url: string; manage_url: string; past: boolean; made?: Made[]
}

export interface Invite { id: number; type_id: number; type: string; name: string; email: string; note: string; url: string; used: boolean; booking_id: number; expires_at: string; source: string; created_at: string }

export interface Summary {
  upcoming: number; pending: number; today: number; this_week: number; types_open: number; types: number
  next: Booking | null; today_list: Booking[]; pending_list: Booking[]; line: string; tz: string
}

export interface SlotDay { date: string; slots: { start: string; end: string; seats: number }[] }
export interface Slots { tz: string; minutes: number; days: SlotDay[]; calendar_warning?: string }

export interface AppState { slug: AppSlug; name: string; reason: string; connected: boolean; detail: string }

export interface Insights {
  weeks: { start: string; booked: number; cancelled: number; minutes: number }[]
  types: { id: number; title: string; emoji: string; count: number }[]
  statuses: Record<string, number>; hours: number[]; paid: number; currency: string; views: number; from: string
}
