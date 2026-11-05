// The plant dialogs: add or edit a plant, its care schedule, "water these",
// and logging care that is not on the schedule.
import React, { useEffect, useRef, useState } from 'react'
import { api, V1 } from '../lib/api'
import { announceChange, useData } from '../lib/data'
import { fmtDate, todayISO } from '../lib/format'
import { UNITS } from '../lib/home'
import { CARE, CARE_OF, GONE, LIGHTS, PLANT_KINDS } from '../lib/plants'
import type { CareKind, CareSuggestion, Job, Plant, Unit } from '../lib/types'
import { Field, Modal, errMsg, useDebounce, useToast } from './ui'
import { Select } from './Select'
import { Icon } from './icons'
import { Actions, HomeSelect } from './Forms'

interface Templates { kind: Plant['kind']; guessed_kind: string; light: Plant['light']; care: CareSuggestion[] }

/** One line per kind of care: on or off, and how often. */
interface CareRow { care: CareKind; on: boolean; every_n: string; every_unit: Unit; soon_days: number; notes: string; job?: Job }

function rowsFrom(suggested: CareSuggestion[], jobs: Job[] = []): CareRow[] {
  return CARE.map(c => {
    const j = jobs.find(x => x.care === c.value)
    const s = suggested.find(x => x.care === c.value)
    if (j) return { care: c.value, on: true, every_n: String(j.every_n), every_unit: j.every_unit, soon_days: j.soon_days, notes: j.notes, job: j }
    return { care: c.value, on: !jobs.length && !!s, every_n: String(s?.every_n || 1), every_unit: s?.every_unit || 'month', soon_days: s?.soon_days ?? 2, notes: s?.notes || '' }
  })
}

function CareEditor({ rows, onChange }: { rows: CareRow[]; onChange: (rows: CareRow[]) => void }) {
  const set = (i: number, p: Partial<CareRow>) => onChange(rows.map((r, k) => k === i ? { ...r, ...p } : r))
  return (
    <div className="care-edit">
      {rows.map((r, i) => (
        <div key={r.care} className={`care-line ${r.on ? '' : 'off'}`}>
          <label className="check-row care-name"><input type="checkbox" checked={r.on} onChange={e => set(i, { on: e.target.checked })} /><Icon name={CARE_OF[r.care].icon} size={14} />{CARE_OF[r.care].label}</label>
          <span className="row every-row">
            <span className="muted small">every</span>
            <input type="number" min={1} max={1000} value={r.every_n} disabled={!r.on} onChange={e => set(i, { every_n: e.target.value })} className="num-sm" aria-label={`${CARE_OF[r.care].label} every`} />
            <Select ariaLabel={`${CARE_OF[r.care].label} unit`} value={r.every_unit} disabled={!r.on} onChange={v => set(i, { every_unit: v as Unit })} options={UNITS.map(u => ({ value: u.value, label: u.label }))} />
          </span>
        </div>
      ))}
    </div>
  )
}

/** The schedule as the API takes it; with `all`, lines switched off are sent as every_n 0 (removed). */
function careBody(rows: CareRow[], all: boolean) {
  return rows.filter(r => r.on || (all && r.job)).map(r => r.on
    ? { care: r.care, every_n: Math.max(1, Number(r.every_n) || 1), every_unit: r.every_unit, ...(r.job ? {} : { soon_days: r.soon_days, notes: r.notes }) }
    : { care: r.care, every_n: 0 })
}

/** One tap: mark a plant's care done now, say so, and reload. */
export function useCareDone() {
  const { toast } = useToast()
  const [busy, setBusy] = useState('')
  const done = async (plantID: number, name: string, care: CareKind) => {
    const key = `${plantID}:${care}`
    if (busy) return
    setBusy(key)
    try {
      const r = await api.post<{ job: Job | null }>(`${V1}/plants/${plantID}/done`, { care })
      toast(`${CARE_OF[care]?.done || 'Done'}: ${name}${r.job ? ` · next ${fmtDate(r.job.due_on, false)}` : ''}`)
      announceChange()
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  return { done, busy }
}

export function PlantModal({ plant, onClose, onSaved }: { plant?: Plant; onClose: () => void; onSaved?: (p: Plant) => void }) {
  const { homes } = useData()
  const { toast } = useToast()
  const [rooms, setRooms] = useState<string[]>([])
  useEffect(() => { api.get<string[]>(`${V1}/rooms`).then(setRooms).catch(() => {}) }, [])
  const [f, setF] = useState({ home_id: plant?.home_id || homes?.find(h => !h.archived)?.id || 0, name: plant?.name || '', species: plant?.species || '', kind: plant?.kind || '' as Plant['kind'] | '',
    room: plant?.room || '', light: plant?.light || 'medium' as Plant['light'], acquired_on: plant?.acquired_on || '', pot: plant?.pot || '', notes: plant?.notes || '',
    archived: !!plant?.archived, gone_reason: (plant?.gone_reason || 'died') as Plant['gone_reason'], last_watered: todayISO() })
  const [care, setCare] = useState<CareRow[]>([])
  const [guess, setGuess] = useState<Plant['kind'] | ''>('')
  const touched = useRef(false)
  const [busy, setBusy] = useState(false)
  const name = useDebounce(`${f.name}|${f.species}`, 350)
  // a new plant: the suggested care for its kind and light (the kind guessed from the name until picked)
  useEffect(() => {
    if (plant) return
    const [n, sp] = name.split('|')
    const q = new URLSearchParams({ light: f.light, name: n, species: sp, ...(f.kind ? { kind: f.kind } : {}) })
    api.get<Templates>(`${V1}/plants/templates?${q}`).then(t => { setGuess(t.guessed_kind as Plant['kind']); if (!touched.current) setCare(rowsFrom(t.care)) }).catch(() => {})
  }, [plant, name, f.kind, f.light])
  const kind = f.kind || guess || 'foliage'
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    const body: Record<string, unknown> = { home_id: f.home_id, name: f.name, species: f.species, kind, room: f.room, light: f.light, acquired_on: f.acquired_on, pot: f.pot, notes: f.notes }
    if (plant) Object.assign(body, { archived: f.archived, gone_reason: f.archived ? f.gone_reason : '' })
    else Object.assign(body, { last_watered: f.last_watered, care: careBody(care, false) })
    try {
      const p = plant ? await api.put<Plant>(`${V1}/plants/${plant.id}`, body) : await api.post<Plant>(`${V1}/plants`, body)
      toast(plant ? 'Saved' : `${p.name} added`); announceChange(); onSaved?.(p); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={plant ? `Edit ${plant.name}` : 'Add a plant'} onClose={onClose} size="lg">
      <form onSubmit={save} className="grid form-grid">
        <div className="two-col tight">
          <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} required maxLength={120} autoFocus placeholder="Monstera" /></Field>
          <Field label="Species or common name (optional)"><input value={f.species} onChange={e => setF({ ...f, species: e.target.value })} maxLength={160} placeholder="Monstera deliciosa" /></Field>
        </div>
        <div className="two-col tight">
          <Field group label="Kind" help={!f.kind && guess ? 'Guessed from the name; change it if it is wrong.' : undefined}>
            <Select ariaLabel="Kind" value={kind} onChange={v => { setF({ ...f, kind: v as Plant['kind'] }) }} options={PLANT_KINDS.map(k => ({ value: k.value, label: k.label }))} /></Field>
          <Field group label="Light"><Select ariaLabel="Light" value={f.light} onChange={v => setF({ ...f, light: v as Plant['light'] })} options={LIGHTS.map(l => ({ value: l.value, label: l.label, hint: l.help }))} /></Field>
        </div>
        <div className="two-col tight">
          <HomeSelect value={f.home_id} onChange={id => setF({ ...f, home_id: id })} />
          <Field label="Room or spot"><input value={f.room} onChange={e => setF({ ...f, room: e.target.value })} list="phm-plant-rooms" placeholder="Kitchen windowsill" maxLength={60} /><datalist id="phm-plant-rooms">{rooms.map(r => <option key={r} value={r} />)}</datalist></Field>
        </div>
        <div className="two-col tight">
          <Field label="Got it on (optional)"><input type="date" value={f.acquired_on} max={todayISO()} onChange={e => setF({ ...f, acquired_on: e.target.value })} /></Field>
          <Field label="Pot (optional)"><input value={f.pot} onChange={e => setF({ ...f, pot: e.target.value })} maxLength={200} placeholder="18 cm terracotta" /></Field>
        </div>
        {!plant && <>
          <Field label="Last watered"><input type="date" value={f.last_watered} max={todayISO()} onChange={e => setF({ ...f, last_watered: e.target.value })} required /></Field>
          <Field group label="Care" help="Suggested for this kind of plant in this light. Untick what you don't do; you can change it later.">
            <CareEditor rows={care} onChange={r => { touched.current = true; setCare(r) }} />
          </Field>
        </>}
        <Field label="Notes"><textarea rows={2} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} placeholder="Where it likes to be, what it doesn't like…" /></Field>
        {plant && <div className="stack tight">
          <label className="check-row"><input type="checkbox" checked={f.archived} onChange={e => setF({ ...f, archived: e.target.checked })} />Gone (its care stops; the history stays)</label>
          {f.archived && <Field group label="What happened"><Select ariaLabel="What happened" value={f.gone_reason} onChange={v => setF({ ...f, gone_reason: v as Plant['gone_reason'] })} options={Object.entries(GONE).map(([value, label]) => ({ value, label }))} /></Field>}
        </div>}
        <Actions onClose={onClose} busy={busy} label={plant ? 'Save' : 'Add the plant'} />
      </form>
    </Modal>
  )
}

export function ScheduleModal({ plant, suggestions, onClose, onSaved }: { plant: Plant; suggestions: CareSuggestion[]; onClose: () => void; onSaved?: () => void }) {
  const { toast } = useToast()
  const [rows, setRows] = useState(() => rowsFrom(suggestions, plant.care))
  const [busy, setBusy] = useState(false)
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    try { await api.put(`${V1}/plants/${plant.id}`, { care: careBody(rows, true) }); toast('Care saved'); announceChange(); onSaved?.(); onClose() } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={`Care for ${plant.name}`} onClose={onClose}>
      <form onSubmit={save} className="grid form-grid">
        <p className="muted small">Each kind of care comes back on its own. A new rhythm counts from the last time you did it.</p>
        <CareEditor rows={rows} onChange={setRows} />
        <Actions onClose={onClose} busy={busy} label="Save" />
      </form>
    </Modal>
  )
}

/** "Water these": the plants with watering on their schedule, those due ticked. */
export function WaterAllModal({ plants, onClose, onDone }: { plants: Plant[]; onClose: () => void; onDone?: () => void }) {
  const { toast } = useToast()
  const list = plants.filter(p => !p.archived && p.care.some(c => c.care === 'water' && c.status !== 'paused'))
    .map(p => ({ p, w: p.care.find(c => c.care === 'water')! }))
    .sort((a, b) => a.w.due_on.localeCompare(b.w.due_on) || a.p.name.localeCompare(b.p.name))
  const [picked, setPicked] = useState<Record<number, boolean>>(() => Object.fromEntries(list.map(x => [x.p.id, x.w.status === 'due' || x.w.status === 'overdue'])))
  const [busy, setBusy] = useState(false)
  const ids = list.filter(x => picked[x.p.id]).map(x => x.p.id)
  const go = async () => {
    setBusy(true)
    try {
      const r = await api.post<{ count: number }>(`${V1}/plants/done`, { care: 'water', plant_ids: ids })
      toast(`${r.count} ${r.count === 1 ? 'plant' : 'plants'} watered`); announceChange(); onDone?.(); onClose()
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="Water plants" onClose={onClose} size="sm" footer={<><span className="grow" /><button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" disabled={busy || ids.length === 0} onClick={go}><Icon name="droplet" size={15} />{ids.length ? `Mark ${ids.length} watered` : 'Mark watered'}</button></>}>
      <div className="suggest-list">
        {list.map(({ p, w }) => (
          <label key={p.id} className="suggest-row">
            <input type="checkbox" checked={!!picked[p.id]} onChange={e => setPicked({ ...picked, [p.id]: e.target.checked })} />
            <span className="grow"><b>{p.name}</b> <span className="muted small">· {p.room || p.home_name}</span>
              <span className="small block-line"><span className={`care-due st-${w.status}`}>{w.days_left < 0 ? `${-w.days_left} ${w.days_left === -1 ? 'day' : 'days'} late` : w.days_left === 0 ? 'Due today' : `Due in ${w.days_left} ${w.days_left === 1 ? 'day' : 'days'}`}</span></span></span>
          </label>
        ))}
        {list.length === 0 && <p className="muted">No plant has watering on its schedule.</p>}
      </div>
    </Modal>
  )
}

/** Care done on another day, with a note, or care that is not on the schedule. */
export function CareLogModal({ plant, care: start = 'water', onClose, onDone }: { plant: Plant; care?: CareKind; onClose: () => void; onDone?: () => void }) {
  const { toast } = useToast()
  const [f, setF] = useState({ care: start as CareKind, date: todayISO(), notes: '' })
  const [busy, setBusy] = useState(false)
  const job = plant.care.find(c => c.care === f.care)
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    try {
      const r = await api.post<{ job: Job | null }>(`${V1}/plants/${plant.id}/done`, f)
      toast(`${CARE_OF[f.care].done}${r.job ? `. Next time: ${fmtDate(r.job.due_on, false)}` : ''}`); announceChange(); onDone?.(); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={`Log care for ${plant.name}`} onClose={onClose} size="sm">
      <form onSubmit={save} className="grid form-grid">
        <div className="two-col tight">
          <Field group label="What"><Select ariaLabel="Care" value={f.care} onChange={v => setF({ ...f, care: v as CareKind })} options={CARE.map(c => ({ value: c.value, label: c.label }))} /></Field>
          <Field label="When"><input type="date" value={f.date} max={todayISO()} onChange={e => setF({ ...f, date: e.target.value })} required /></Field>
        </div>
        <p className="muted small">{job ? 'The next time is counted from this day.' : 'Not on its schedule: it is only written in the history.'}</p>
        <Field label="Notes"><textarea rows={2} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} placeholder="A new leaf, yellow tips, bottom-watered…" autoFocus /></Field>
        <Actions onClose={onClose} busy={busy} label="Log it" />
      </form>
    </Modal>
  )
}
