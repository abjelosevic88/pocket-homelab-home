// Names, icons and formatting for Pocket Home's own words.
import { useCallback, useEffect, useState } from 'react'
import type { Home, Job, JobStatus, Unit } from './types'
import { fmtDate } from './format'

export const HOME_KINDS: { value: string; label: string; icon: string }[] = [
  { value: 'house', label: 'House', icon: 'home' }, { value: 'apartment', label: 'Apartment', icon: 'grid' },
  { value: 'cottage', label: 'Cottage', icon: 'mountain' }, { value: 'other', label: 'Other', icon: 'pin' },
]

export const CATEGORIES: { value: string; label: string; icon: string }[] = [
  { value: 'heating', label: 'Heating', icon: 'flame' },
  { value: 'cooling', label: 'Cooling', icon: 'snowflake' },
  { value: 'water', label: 'Water', icon: 'droplet' },
  { value: 'electrical', label: 'Electrical', icon: 'zap' },
  { value: 'appliance', label: 'Appliance', icon: 'package' },
  { value: 'kitchen', label: 'Kitchen', icon: 'utensils' },
  { value: 'structure', label: 'House and roof', icon: 'home' },
  { value: 'outdoor', label: 'Garden and outside', icon: 'sun' },
  { value: 'safety', label: 'Safety', icon: 'shield' },
  { value: 'other', label: 'Other', icon: 'more' },
]
export const CATEGORY_NAME: Record<string, string> = Object.fromEntries(CATEGORIES.map(c => [c.value, c.label]))
export const CATEGORY_ICON: Record<string, string> = Object.fromEntries(CATEGORIES.map(c => [c.value, c.icon]))

export const RECORD_KINDS: { value: string; label: string; icon: string }[] = [
  { value: 'maintenance', label: 'Upkeep', icon: 'wrench' },
  { value: 'repair', label: 'Repair', icon: 'alert' },
  { value: 'improvement', label: 'Improvement', icon: 'sparkles' },
  { value: 'inspection', label: 'Check', icon: 'check-circle' },
  { value: 'purchase', label: 'Purchase', icon: 'cart' },
  { value: 'other', label: 'Other', icon: 'note' },
]
export const RECORD_KIND_NAME: Record<string, string> = Object.fromEntries(RECORD_KINDS.map(c => [c.value, c.label]))
export const RECORD_KIND_ICON: Record<string, string> = Object.fromEntries(RECORD_KINDS.map(c => [c.value, c.icon]))

export const METER_KINDS: { value: string; label: string; icon: string; unit: string }[] = [
  { value: 'electricity', label: 'Electricity', icon: 'zap', unit: 'kWh' },
  { value: 'water', label: 'Water', icon: 'droplet', unit: 'm³' },
  { value: 'gas', label: 'Gas', icon: 'flame', unit: 'm³' },
  { value: 'heating', label: 'Heating', icon: 'thermometer', unit: 'kWh' },
  { value: 'solar', label: 'Solar', icon: 'sun', unit: 'kWh' },
  { value: 'oil', label: 'Heating oil', icon: 'droplet', unit: 'L' },
  { value: 'other', label: 'Other', icon: 'gauge', unit: '' },
]
export const METER_ICON: Record<string, string> = Object.fromEntries(METER_KINDS.map(c => [c.value, c.icon]))

export const UNITS: { value: Unit; label: string }[] = [{ value: 'day', label: 'days' }, { value: 'week', label: 'weeks' }, { value: 'month', label: 'months' }, { value: 'year', label: 'years' }]

/** "Every 3 months", "Every year", "Once". */
export function repeatText(n: number, unit: string): string {
  if (!n) return 'Once'
  if (n === 1) return `Every ${unit}`
  return `Every ${n} ${unit}s`
}

export const STATUS_NAME: Record<JobStatus, string> = { overdue: 'Overdue', due: 'Due today', soon: 'Due soon', ok: 'Later', paused: 'Paused', done: 'Done' }

/** "3 days overdue", "today", "in 12 days", "on 4 Mar". */
export function dueText(j: Pick<Job, 'status' | 'days_left' | 'due_on'>): string {
  if (j.status === 'done') return 'Done'
  if (j.status === 'paused') return 'Paused'
  const d = j.days_left
  if (d < 0) return `${-d} ${-d === 1 ? 'day' : 'days'} overdue`
  if (d === 0) return 'Due today'
  if (d === 1) return 'Due tomorrow'
  if (d <= 31) return `In ${d} days`
  return `On ${fmtDate(j.due_on, d > 300)}`
}

/** Minor units → "€1,234.50" in the currency. */
export function money(minor: number | null | undefined, currency = 'EUR', whole = false): string {
  const v = (minor || 0) / 100
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency, minimumFractionDigits: whole ? 0 : 2, maximumFractionDigits: whole ? 0 : 2 }).format(v)
  } catch {
    return `${v.toFixed(whole ? 0 : 2)} ${currency}`
  }
}

/** A typed amount ("12,50" or "12.50") → minor units, null when empty or bad. */
export function parseMoney(s: string): number | null {
  const t = s.trim().replace(/\s/g, '').replace(/,(\d{1,2})$/, '.$1').replace(/,/g, '')
  if (!t) return null
  const n = Number(t)
  if (!Number.isFinite(n) || n < 0) return null
  return Math.round(n * 100)
}
export const moneyInput = (minor: number | null | undefined) => minor == null || minor === 0 ? '' : (minor / 100).toFixed(2)

export function fmtNum(v: number, digits = 1): string {
  return v.toLocaleString(undefined, { maximumFractionDigits: digits })
}

// ---- the home filter (All homes, or one), kept per browser ----

const HOME_KEY = 'phm-home'
export function useHomeFilter(homes: Home[] | null | undefined): [number, (id: number) => void] {
  const [id, setRaw] = useState(() => Number(localStorage.getItem(HOME_KEY)) || 0)
  useEffect(() => {
    const h = () => setRaw(Number(localStorage.getItem(HOME_KEY)) || 0)
    window.addEventListener('phm:home', h)
    return () => window.removeEventListener('phm:home', h)
  }, [])
  const set = useCallback((v: number) => { if (v) localStorage.setItem(HOME_KEY, String(v)); else localStorage.removeItem(HOME_KEY); setRaw(v); window.dispatchEvent(new CustomEvent('phm:home')) }, [])
  const valid = !homes || id === 0 || homes.some(h => h.id === id) ? id : 0
  return [valid, set]
}
