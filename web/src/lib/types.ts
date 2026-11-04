export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = '' | 'core' | 'direct'
export type AppSlug = 'money' | 'calendar' | 'tasks' | 'contacts' | 'documents' | 'inventory'

type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number; currency: string; area_unit: 'm2' | 'ft2'
  notify_upkeep: boolean; notify_warranty: boolean; notify_meters: boolean; notify_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  money_book: boolean; money_account_id: number; money_category_id: number
  calendar_sync: boolean; calendar_id: number
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string
}

export type HomeKind = 'house' | 'apartment' | 'cottage' | 'other'
export interface Home {
  id: number; name: string; kind: HomeKind; address: string; year_built: number | null; area_m2: number | null; currency: string; notes: string; money_tag: string
  archived: boolean; sort: number; created_at: string; updated_at: string; things: number; open_jobs: number; overdue: number; due_soon: number
}

export interface Thing {
  id: number; home_id: number; name: string; category: string; room: string; brand: string; model: string; serial: string
  installed_on: string; purchased_on: string; price: number | null; vendor: string; warranty_until: string; manual_url: string
  inventory_id: number | null; inventory_name: string; notes: string; archived: boolean; created_at: string; updated_at: string
  home_name: string; currency: string; open_jobs: number; overdue: number; next_due: string; last_done: string; spent: number
}

export interface Pro {
  id: number; name: string; trade: string; company: string; phone: string; email: string; website: string; notes: string; rating: number
  contact_id: number | null; created_at: string; updated_at: string; jobs: number; last_work: string; spent: number
}

export type JobStatus = 'overdue' | 'due' | 'soon' | 'ok' | 'paused' | 'done'
export type Unit = 'day' | 'week' | 'month' | 'year'
export interface Job {
  id: number; home_id: number; thing_id: number | null; pro_id: number | null; title: string; notes: string; every_n: number; every_unit: Unit
  due_on: string; soon_days: number; est_cost: number | null; last_done: string; done_count: number; paused: boolean; finished_at: string
  cal_event_id: number | null; task_id: number | null; source: string; created_at: string; updated_at: string
  home_name: string; thing_name: string; pro_name: string; currency: string; status: JobStatus; days_left: number
}

export interface HomeRecord {
  id: number; home_id: number; thing_id: number | null; job_id: number | null; pro_id: number | null; date: string; kind: string; title: string; notes: string
  cost: number; currency: string; money_tx_id: number | null; money_user_id: number | null; source: string; external_id: string; created_at: string; updated_at: string
  home_name: string; thing_name: string; pro_name: string; job_title: string
}

export interface Reading { id: number; meter_id: number; read_on: string; value: number; reset: boolean; note: string; source: string; created_at: string; usage: number | null; days: number }
export interface Meter {
  id: number; home_id: number; name: string; kind: string; unit: string; price_per_unit: number | null; number: string; notes: string; remind_day: number
  archived: boolean; created_at: string; updated_at: string; home_name: string; currency: string; last: Reading | null; readings: number
}
export interface UsageMonth { month: string; usage: number; cost: number | null; estimated: boolean }
export interface UsageSummary { unit: string; per_day?: number; per_year?: number; cost_per_year?: number; per_day_before?: number }

export interface Attachment { id: number; owner_kind: 'home' | 'thing' | 'record'; owner_id: number; kind: 'document' | 'url'; ref: string; title: string; url: string; created_at: string; open_url: string }

export interface Suggestion { title: string; every_n: number; every_unit: Unit; soon_days: number; notes: string }

export interface SpendRow { key: string; currency: string; total: number; count: number }

export interface Overview {
  today: string; homes: Home[]; counts: { overdue: number; due: number; soon: number }; attention: Job[]; upcoming: Job[]; recent: HomeRecord[]
  warranties: { thing_id: number; name: string; until: string; state: 'ending' | 'ended'; days: number }[]
  meters: { id: number; name: string; kind: string; unit: string; home_name: string; last: Reading | null; readings: number; needs_reading: boolean }[]
  things: number; spent_year: SpendRow[]; spent_last_year_to_date: SpendRow[]
}

export interface MoneyResult { ok: boolean; tx_id?: number; account?: string; url?: string; via?: string; error?: string; skipped?: boolean }

export interface Picked<T> { connected: boolean; detail?: string; items: T[] }
export interface MoneyAccount { id: number; name: string; kind: string; currency: string; everyday: boolean }
export interface MoneyCategory { id: number; name: string; full_name: string }
export interface CalendarItem { id: number; name: string; color: string; writable: boolean; is_default: boolean }
export interface ContactItem { id: number; fn: string; org: string; title: string; emails: string[] | null; phones: string[] | null }
export interface DocumentItem { id: number; title: string; original_name: string; created: string; added_at: string; correspondent_name: string; document_type_name: string }
export interface InventoryItem { id: number; name: string; quantity: number; unit: string; location_name: string; category_name: string }
