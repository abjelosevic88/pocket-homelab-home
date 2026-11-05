// Names and icons for house plants and their care.
import { daysBetween, todayISO } from './format'
import type { CareKind, Job, Light, Plant, PlantKind } from './types'

export const CARE: { value: CareKind; label: string; done: string; icon: string }[] = [
  { value: 'water', label: 'Water', done: 'Watered', icon: 'droplet' },
  { value: 'fertilise', label: 'Fertilise', done: 'Fertilised', icon: 'sparkles' },
  { value: 'mist', label: 'Mist', done: 'Misted', icon: 'cloud-rain' },
  { value: 'rotate', label: 'Rotate', done: 'Rotated', icon: 'refresh' },
  { value: 'prune', label: 'Prune', done: 'Pruned', icon: 'scissors' },
  { value: 'pests', label: 'Check for pests', done: 'Checked', icon: 'bug' },
  { value: 'repot', label: 'Repot', done: 'Repotted', icon: 'sprout' },
]
export const CARE_OF: Record<string, (typeof CARE)[number]> = Object.fromEntries(CARE.map(c => [c.value, c]))

export const PLANT_KINDS: { value: PlantKind; label: string }[] = [
  { value: 'foliage', label: 'Leafy (foliage)' }, { value: 'flowering', label: 'Flowering' }, { value: 'succulent', label: 'Succulent' },
  { value: 'cactus', label: 'Cactus' }, { value: 'herb', label: 'Herb' }, { value: 'fern', label: 'Fern' }, { value: 'orchid', label: 'Orchid' },
  { value: 'palm', label: 'Palm' }, { value: 'other', label: 'Other' },
]
export const PLANT_KIND_NAME: Record<string, string> = Object.fromEntries(PLANT_KINDS.map(k => [k.value, k.label]))

export const LIGHTS: { value: Light; label: string; help: string }[] = [
  { value: 'low', label: 'Low light', help: 'Away from windows, a dim corner' },
  { value: 'medium', label: 'Medium', help: 'A few metres from a window' },
  { value: 'bright', label: 'Bright, indirect', help: 'Near a window, no sun on the leaves' },
  { value: 'direct', label: 'Direct sun', help: 'On a sunny sill' },
]
export const LIGHT_NAME: Record<string, string> = Object.fromEntries(LIGHTS.map(l => [l.value, l.label]))

export const GONE: Record<string, string> = { died: 'Died', given: 'Given away', other: 'Gone' }

/** "Water today", "Water: 2 days late", "Water in 3 days", "Repot on 4 Mar". */
export function careDue(j: Pick<Job, 'status' | 'days_left' | 'title'>): string {
  const d = j.days_left
  if (j.status === 'paused') return `${j.title}: paused`
  if (d < 0) return `${j.title}: ${-d} ${-d === 1 ? 'day' : 'days'} late`
  if (d === 0) return `${j.title} today`
  if (d === 1) return `${j.title} tomorrow`
  return `${j.title} in ${d} days`
}

/** "Water Monstera", "Check Ficus for pests". */
export function careName(j: Pick<Job, 'care' | 'title' | 'plant_name'>): string {
  if (!j.plant_name) return j.title
  if (j.care === 'pests' && j.title === CARE_OF.pests.label) return `Check ${j.plant_name} for pests`
  return `${j.title} ${j.plant_name}`
}

/** The care that needs doing now (due or overdue), most urgent first. */
export const dueCare = (p: Plant) => p.care.filter(c => c.status === 'overdue' || c.status === 'due')

/** A plant's most pressing care, else its next. */
export const nextCare = (p: Plant): Job | undefined => p.care.find(c => c.status !== 'paused' && c.status !== 'done')

/** "watered today", "watered yesterday", "watered 4 days ago". */
export function wateredText(date: string): string {
  const d = daysBetween(date, todayISO())
  if (d <= 0) return 'watered today'
  if (d === 1) return 'watered yesterday'
  return `watered ${d} days ago`
}
