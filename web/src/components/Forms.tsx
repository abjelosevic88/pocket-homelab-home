// Every add/edit dialog: a log entry, upkeep, "done", a thing, a meter and
// its readings, a pro, a home, and papers kept with them.
import React, { useEffect, useState } from 'react'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData } from '../lib/data'
import { todayISO } from '../lib/format'
import { CATEGORIES, HOME_KINDS, METER_KINDS, RECORD_KINDS, UNITS, money, moneyInput, parseMoney, repeatText } from '../lib/home'
import type { Attachment, ContactItem, DocumentItem, Home, HomeRecord, InventoryItem, Job, Meter, MoneyResult, Picked, Pro, Reading, Suggestion, Thing } from '../lib/types'
import { CoreHint, Field, Loading, Modal, Segmented, errMsg, useDebounce, useToast } from './ui'
import { Select } from './Select'
import { Icon } from './icons'

const opt = (id: number | null | undefined) => id ? String(id) : ''
const idOf = (v: string) => v ? Number(v) : null

function Actions({ onClose, busy, label, danger }: { onClose: () => void; busy: boolean; label: string; danger?: React.ReactNode }) {
  return <div className="form-actions">{danger}<span className="grow" /><button type="button" className="btn" onClick={onClose}>Cancel</button><button className="btn primary" disabled={busy}>{busy ? 'Saving…' : label}</button></div>
}

function HomeSelect({ value, onChange }: { value: number; onChange: (id: number) => void }) {
  const { homes } = useData()
  const first = homes?.find(h => !h.archived)?.id
  useEffect(() => { if (!value && first) onChange(first) }, [value, first]) // eslint-disable-line react-hooks/exhaustive-deps
  if (!homes || homes.length <= 1) return null
  return <Field group label="Home"><Select ariaLabel="Home" value={opt(value)} onChange={v => onChange(Number(v))} options={homes.filter(h => !h.archived || h.id === value).map(h => ({ value: String(h.id), label: h.name }))} /></Field>
}

function ThingSelect({ home, value, onChange, label = 'Appliance or part (optional)' }: { home: number; value: number | null; onChange: (t: Thing | null) => void; label?: string }) {
  const { things } = useData()
  const list = things.filter(t => !home || t.home_id === home)
  return <Field group label={label}><Select ariaLabel="Thing" value={opt(value)} onChange={v => onChange(list.find(t => String(t.id) === v) || null)} clearLabel="None" placeholder="None"
    options={list.map(t => ({ value: String(t.id), label: t.name, hint: t.room, group: t.room || undefined }))} /></Field>
}

function ProSelect({ value, onChange }: { value: number | null; onChange: (id: number | null) => void }) {
  const { pros } = useData()
  return <Field group label="Who did it (optional)"><Select ariaLabel="Pro" value={opt(value)} onChange={v => onChange(idOf(v))} clearLabel="Nobody / myself" placeholder="Nobody / myself"
    options={pros.map(p => ({ value: String(p.id), label: p.name, hint: p.trade }))} /></Field>
}

/** "Create a transaction in Pocket Money": greyed out with why when Money is not reachable. */
export function MoneyTick({ checked, onChange, booked }: { checked: boolean; onChange: (v: boolean) => void; booked?: boolean }) {
  const { settings } = useApp()
  if (!settings) return null
  const coreOn = settings.core_configured && settings.core_registered !== false
  if (settings.money_mode === 'off') return null
  if (booked) return <p className="muted small row" style={{ gap: 6 }}><Icon name="wallet" size={14} />Booked in Pocket Money; changes to the date, cost and title go there too.</p>
  const ready = settings.money_via !== ''
  return (
    <div className="stack tight">
      <label className={`check-row ${ready ? '' : 'disabled'}`} title={ready ? '' : settings.money_detail}>
        <input type="checkbox" checked={ready && checked} disabled={!ready} onChange={e => onChange(e.target.checked)} />
        <span><Icon name="wallet" size={14} /> Create a transaction in Pocket Money</span>
      </label>
      {!ready && <p className="muted small">{settings.money_detail ? `Pocket Money is not connected: ${settings.money_detail}.` : 'Connect Pocket Money in Settings → Connected apps.'}</p>}
      {!ready && !coreOn && settings.money_mode === 'core' && <CoreHint />}
    </div>
  )
}

function moneyToast(toast: ReturnType<typeof useToast>['toast'], m?: MoneyResult) {
  if (!m || m.skipped) return
  if (m.ok) toast(`Booked in Pocket Money${m.account ? ` on ${m.account}` : ''}`)
  else toast(`Saved, but Pocket Money said: ${m.error}`, 'err')
}

// ---------- the log ----------

export function RecordModal({ record, preset, onClose, onSaved }: { record?: HomeRecord; preset?: Partial<HomeRecord>; onClose: () => void; onSaved?: (r: HomeRecord) => void }) {
  const { homes, things } = useData()
  const { settings } = useApp()
  const { toast } = useToast()
  const start = record || preset || {}
  const firstHome = start.home_id || (start.thing_id ? things.find(t => t.id === start.thing_id)?.home_id : 0) || homes?.find(h => !h.archived)?.id || 0
  const [f, setF] = useState({ home_id: firstHome, thing_id: start.thing_id ?? null, pro_id: start.pro_id ?? null, date: start.date || todayISO(), kind: start.kind || 'maintenance',
    title: start.title || '', notes: start.notes || '', cost: moneyInput(start.cost) })
  const [book, setBook] = useState(settings?.money_book ?? true)
  const [busy, setBusy] = useState(false)
  const currency = homes?.find(h => h.id === f.home_id)?.currency || settings?.currency || 'EUR'
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    const cost = parseMoney(f.cost)
    if (f.cost.trim() && cost == null) { toast('The cost is a number like 120 or 89.90', 'err'); return }
    setBusy(true)
    const body = { home_id: f.home_id, thing_id: f.thing_id, pro_id: f.pro_id, date: f.date, kind: f.kind, title: f.title, notes: f.notes, cost: cost || 0, money: record ? undefined : book }
    try {
      const r = record ? await api.put<{ record: HomeRecord; money?: MoneyResult }>(`${V1}/records/${record.id}`, body) : await api.post<{ record: HomeRecord; money?: MoneyResult }>(`${V1}/records`, body)
      toast(record ? 'Saved' : 'Added to the log'); moneyToast(toast, r.money); announceChange(); onSaved?.(r.record); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={record ? 'Edit log entry' : 'Log work'} onClose={onClose}>
      <form onSubmit={save} className="grid form-grid">
        <Field label="What was done"><input value={f.title} onChange={e => setF({ ...f, title: e.target.value })} required maxLength={200} autoFocus placeholder="Replaced the kitchen tap" /></Field>
        <Field group label="Kind"><Select ariaLabel="Kind" value={f.kind} onChange={v => setF({ ...f, kind: v })} options={RECORD_KINDS.map(k => ({ value: k.value, label: k.label }))} /></Field>
        <div className="two-col tight">
          <Field label="Date"><input type="date" value={f.date} onChange={e => setF({ ...f, date: e.target.value })} required /></Field>
          <Field label={`Cost (${currency})`}><input inputMode="decimal" value={f.cost} onChange={e => setF({ ...f, cost: e.target.value })} placeholder="0.00" /></Field>
        </div>
        <HomeSelect value={f.home_id} onChange={id => setF({ ...f, home_id: id, thing_id: null })} />
        <ThingSelect home={f.home_id} value={f.thing_id} onChange={t => setF({ ...f, thing_id: t?.id ?? null })} />
        <ProSelect value={f.pro_id} onChange={id => setF({ ...f, pro_id: id })} />
        <Field label="Notes"><textarea rows={3} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} placeholder="Part numbers, what to watch next time…" /></Field>
        {(parseMoney(f.cost) || 0) > 0 && <MoneyTick checked={book} onChange={setBook} booked={!!record?.money_tx_id} />}
        <Actions onClose={onClose} busy={busy} label={record ? 'Save' : 'Add to the log'} />
      </form>
    </Modal>
  )
}

// ---------- upkeep ----------

export function JobModal({ job, preset, onClose, onSaved }: { job?: Job; preset?: Partial<Job>; onClose: () => void; onSaved?: (j: Job) => void }) {
  const { homes, things } = useData()
  const { toast } = useToast()
  const start = job || preset || {}
  const firstHome = start.home_id || (start.thing_id ? things.find(t => t.id === start.thing_id)?.home_id : 0) || homes?.find(h => !h.archived)?.id || 0
  const [f, setF] = useState({ home_id: firstHome, thing_id: start.thing_id ?? null, pro_id: start.pro_id ?? null, title: start.title || '', notes: start.notes || '',
    once: job ? job.every_n === 0 : false, every_n: String(start.every_n || 1), every_unit: start.every_unit || 'month', due_on: start.due_on || todayISO(),
    soon_days: String(start.soon_days ?? 14), est: moneyInput(start.est_cost), paused: !!start.paused })
  const [busy, setBusy] = useState(false)
  const currency = homes?.find(h => h.id === f.home_id)?.currency || 'EUR'
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    const body = { home_id: f.home_id, thing_id: f.thing_id, pro_id: f.pro_id, title: f.title, notes: f.notes, every_n: f.once ? 0 : Number(f.every_n) || 1, every_unit: f.every_unit,
      due_on: f.due_on, soon_days: Number(f.soon_days) || 0, est_cost: parseMoney(f.est), paused: f.paused }
    try {
      const j = job ? await api.put<Job>(`${V1}/jobs/${job.id}`, body) : await api.post<Job>(`${V1}/jobs`, body)
      toast(job ? 'Saved' : 'Upkeep added'); announceChange(); onSaved?.(j); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={job ? 'Edit upkeep' : 'Add upkeep'} onClose={onClose}>
      <form onSubmit={save} className="grid form-grid">
        <Field label="What to do"><input value={f.title} onChange={e => setF({ ...f, title: e.target.value })} required maxLength={160} autoFocus placeholder="Clean the gutters" /></Field>
        <HomeSelect value={f.home_id} onChange={id => setF({ ...f, home_id: id, thing_id: null })} />
        <ThingSelect home={f.home_id} value={f.thing_id} onChange={t => setF({ ...f, thing_id: t?.id ?? null, home_id: t?.home_id || f.home_id })} />
        <Field group label="How often"><Segmented value={f.once ? 'once' : 'repeat'} onChange={v => setF({ ...f, once: v === 'once' })} options={[{ value: 'repeat', label: 'Comes back' }, { value: 'once', label: 'Once' }]} /></Field>
        {!f.once && <div className="row every-row"><span className="muted">Every</span><input type="number" min={1} max={1000} value={f.every_n} onChange={e => setF({ ...f, every_n: e.target.value })} className="num-sm" aria-label="Every" />
          <Select ariaLabel="Unit" value={f.every_unit} onChange={v => setF({ ...f, every_unit: v as Job['every_unit'] })} options={UNITS.map(u => ({ value: u.value, label: u.label }))} /></div>}
        <div className="two-col tight">
          <Field label={job ? 'Next due' : 'First due'}><input type="date" value={f.due_on} onChange={e => setF({ ...f, due_on: e.target.value })} required /></Field>
          <Field label="Remind me (days before)"><input type="number" min={0} max={365} value={f.soon_days} onChange={e => setF({ ...f, soon_days: e.target.value })} /></Field>
        </div>
        <div className="two-col tight">
          <ProSelect value={f.pro_id} onChange={id => setF({ ...f, pro_id: id })} />
          <Field label={`Usual cost (${currency}, optional)`}><input inputMode="decimal" value={f.est} onChange={e => setF({ ...f, est: e.target.value })} placeholder="0.00" /></Field>
        </div>
        <Field label="Notes"><textarea rows={3} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} placeholder="How to do it, what to buy" /></Field>
        {job && <label className="check-row"><input type="checkbox" checked={f.paused} onChange={e => setF({ ...f, paused: e.target.checked })} />Paused (no reminders, off the calendar)</label>}
        <Actions onClose={onClose} busy={busy} label={job ? 'Save' : 'Add upkeep'} />
      </form>
    </Modal>
  )
}

/** "I did it": the date, what it cost, who did it, a note. */
export function DoneModal({ job, onClose, onDone }: { job: Job; onClose: () => void; onDone?: () => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const [f, setF] = useState({ date: todayISO(), cost: moneyInput(job.est_cost), pro_id: job.pro_id, notes: '', log: true })
  const [book, setBook] = useState(settings?.money_book ?? true)
  const [busy, setBusy] = useState(false)
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    const cost = parseMoney(f.cost)
    if (f.cost.trim() && cost == null) { toast('The cost is a number like 120 or 89.90', 'err'); return }
    setBusy(true)
    try {
      const r = await api.post<{ job: Job; money?: MoneyResult }>(`${V1}/jobs/${job.id}/done`, { date: f.date, cost: cost || 0, pro_id: f.pro_id, notes: f.notes, log: f.log, money: book })
      toast(job.every_n ? `Done. Next time: ${r.job.due_on}` : 'Done'); moneyToast(toast, r.money); announceChange(); onDone?.(); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={`Done: ${job.title}`} onClose={onClose} size="sm">
      <form onSubmit={save} className="grid form-grid">
        <p className="muted small">{job.thing_name ? `${job.thing_name} · ` : ''}{repeatText(job.every_n, job.every_unit)}{job.every_n ? ': the next date is counted from the day you did it.' : '.'}</p>
        <div className="two-col tight">
          <Field label="When"><input type="date" value={f.date} max={todayISO()} onChange={e => setF({ ...f, date: e.target.value })} required /></Field>
          <Field label={`Cost (${job.currency})`}><input inputMode="decimal" value={f.cost} onChange={e => setF({ ...f, cost: e.target.value })} placeholder="0.00" autoFocus /></Field>
        </div>
        <ProSelect value={f.pro_id} onChange={id => setF({ ...f, pro_id: id })} />
        <Field label="Notes"><textarea rows={2} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} placeholder="Anything worth remembering next time" /></Field>
        <label className="check-row"><input type="checkbox" checked={f.log} onChange={e => setF({ ...f, log: e.target.checked })} />Write it in the log</label>
        {f.log && (parseMoney(f.cost) || 0) > 0 && <MoneyTick checked={book} onChange={setBook} />}
        <Actions onClose={onClose} busy={busy} label="Mark done" />
      </form>
    </Modal>
  )
}

/** Built-in suggestions (and, with AI, ones for this model) to add as upkeep. */
export function SuggestModal({ thing, onClose }: { thing: Thing; onClose: () => void }) {
  const { toast } = useToast()
  const [list, setList] = useState<Suggestion[] | null>(null)
  const [picked, setPicked] = useState<Record<number, boolean>>({})
  const [ai, setAI] = useState<'' | 'asking' | 'done'>('')
  const [busy, setBusy] = useState(false)
  useEffect(() => { api.get<Suggestion[]>(`${V1}/templates?thing_id=${thing.id}`).then(l => { setList(l); setPicked(Object.fromEntries(l.map((_, i) => [i, true]))) }).catch(e => toast(errMsg(e), 'err')) }, [thing.id]) // eslint-disable-line react-hooks/exhaustive-deps
  const askAI = async () => {
    setAI('asking')
    try {
      const r = await api.post<{ ok: boolean; error?: string; suggestions?: Suggestion[] }>(`${V1}/things/${thing.id}/suggest`)
      if (!r.ok) { toast(r.error || 'The AI did not answer', 'err'); setAI(''); return }
      const extra = (r.suggestions || []).filter(s => !(list || []).some(x => x.title.toLowerCase() === s.title.toLowerCase()))
      const base = list?.length || 0
      setList([...(list || []), ...extra]); setPicked(p => ({ ...p, ...Object.fromEntries(extra.map((_, i) => [base + i, true])) })); setAI('done')
    } catch (e) { toast(errMsg(e), 'err'); setAI('') }
  }
  const add = async () => {
    const jobs = (list || []).filter((_, i) => picked[i]).map(s => ({ thing_id: thing.id, title: s.title, every_n: s.every_n, every_unit: s.every_unit, soon_days: s.soon_days, notes: s.notes }))
    if (!jobs.length) return
    setBusy(true)
    try { await api.post(`${V1}/jobs/bulk`, { jobs }); toast(`${jobs.length} added to upkeep, first due today — change the dates as you like`); announceChange(); onClose() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const n = Object.values(picked).filter(Boolean).length
  return (
    <Modal title={`Upkeep for ${thing.name}`} onClose={onClose} footer={<>
      <button type="button" className="btn ghost" onClick={askAI} disabled={ai !== ''}><Icon name="sparkles" size={15} />{ai === 'asking' ? 'Asking…' : ai === 'done' ? 'AI suggestions added' : 'Ask AI for this model'}</button>
      <span className="grow" /><button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" disabled={busy || n === 0} onClick={add}>Add {n || ''}</button></>}>
      {!list ? <Loading /> : (
        <div className="suggest-list">
          {list.map((s, i) => (
            <label key={i} className="suggest-row">
              <input type="checkbox" checked={!!picked[i]} onChange={e => setPicked({ ...picked, [i]: e.target.checked })} />
              <span className="grow"><b>{s.title}</b> <span className="muted small">· {repeatText(s.every_n, s.every_unit)}</span>{s.notes && <span className="muted small block-line">{s.notes}</span>}</span>
            </label>
          ))}
          <p className="muted small">General guidance; the manual of your model comes first. Gas, electrics and roofs are work for a qualified person.</p>
        </div>
      )}
    </Modal>
  )
}

// ---------- things ----------

export function ThingModal({ thing, preset, onClose, onSaved }: { thing?: Thing; preset?: Partial<Thing>; onClose: () => void; onSaved?: (t: Thing) => void }) {
  const { homes } = useData()
  const { settings } = useApp()
  const { toast } = useToast()
  const start = thing || preset || {}
  const [rooms, setRooms] = useState<string[]>([])
  useEffect(() => { api.get<string[]>(`${V1}/rooms`).then(setRooms).catch(() => {}) }, [])
  const [f, setF] = useState({ home_id: start.home_id || homes?.find(h => !h.archived)?.id || 0, name: start.name || '', category: start.category || 'appliance', room: start.room || '',
    brand: start.brand || '', model: start.model || '', serial: start.serial || '', installed_on: start.installed_on || '', purchased_on: start.purchased_on || '',
    price: moneyInput(start.price), vendor: start.vendor || '', warranty_until: start.warranty_until || '', manual_url: start.manual_url || '', notes: start.notes || '',
    inventory_id: start.inventory_id ?? null, inventory_name: start.inventory_name || '', archived: !!start.archived })
  const [inv, setInv] = useState(false)
  const [busy, setBusy] = useState(false)
  const currency = homes?.find(h => h.id === f.home_id)?.currency || 'EUR'
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    const body = { ...f, price: parseMoney(f.price) }
    try {
      const t = thing ? await api.put<Thing>(`${V1}/things/${thing.id}`, body) : await api.post<Thing>(`${V1}/things`, body)
      toast(thing ? 'Saved' : `${t.name} added`); announceChange(); onSaved?.(t); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  const invOn = settings && settings.inventory_mode !== 'off'
  return (
    <Modal title={thing ? `Edit ${thing.name}` : 'Add an appliance or part of the house'} onClose={onClose} size="lg">
      <form onSubmit={save} className="grid form-grid">
        <div className="two-col tight">
          <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} required maxLength={120} autoFocus placeholder="Gas boiler" /></Field>
          <Field group label="Kind"><Select ariaLabel="Category" value={f.category} onChange={v => setF({ ...f, category: v })} options={CATEGORIES.map(c => ({ value: c.value, label: c.label }))} /></Field>
        </div>
        <div className="two-col tight">
          <HomeSelect value={f.home_id} onChange={id => setF({ ...f, home_id: id })} />
          <Field label="Room or place"><input value={f.room} onChange={e => setF({ ...f, room: e.target.value })} list="phm-rooms" placeholder="Kitchen" maxLength={60} /><datalist id="phm-rooms">{rooms.map(r => <option key={r} value={r} />)}</datalist></Field>
        </div>
        <div className="three-col tight">
          <Field label="Brand"><input value={f.brand} onChange={e => setF({ ...f, brand: e.target.value })} /></Field>
          <Field label="Model"><input value={f.model} onChange={e => setF({ ...f, model: e.target.value })} /></Field>
          <Field label="Serial number"><input value={f.serial} onChange={e => setF({ ...f, serial: e.target.value })} /></Field>
        </div>
        <div className="three-col tight">
          <Field label="Bought"><input type="date" value={f.purchased_on} onChange={e => setF({ ...f, purchased_on: e.target.value })} /></Field>
          <Field label="Installed"><input type="date" value={f.installed_on} onChange={e => setF({ ...f, installed_on: e.target.value })} /></Field>
          <Field label="Warranty until"><input type="date" value={f.warranty_until} onChange={e => setF({ ...f, warranty_until: e.target.value })} /></Field>
        </div>
        <div className="two-col tight">
          <Field label={`Price (${currency})`}><input inputMode="decimal" value={f.price} onChange={e => setF({ ...f, price: e.target.value })} placeholder="0.00" /></Field>
          <Field label="Bought from"><input value={f.vendor} onChange={e => setF({ ...f, vendor: e.target.value })} /></Field>
        </div>
        <Field label="Manual (web link)" help="Or keep the PDF in Pocket Documents and add it on the appliance's page."><input value={f.manual_url} onChange={e => setF({ ...f, manual_url: e.target.value })} placeholder="https://…" /></Field>
        <Field label="Notes"><textarea rows={3} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} placeholder="Where the valve is, the filter size, the code on the label…" /></Field>
        {invOn && <Field group label="Pocket Inventory">
          <div className="row wrap">{f.inventory_id ? <><span className="chip"><Icon name="package" size={13} />{f.inventory_name || `Item ${f.inventory_id}`}</span><button type="button" className="btn ghost sm" onClick={() => setF({ ...f, inventory_id: null, inventory_name: '' })}>Unlink</button></>
            : <button type="button" className="btn sm" onClick={() => setInv(true)}><Icon name="link" size={14} />Link to an item</button>}</div>
        </Field>}
        {thing && <label className="check-row"><input type="checkbox" checked={f.archived} onChange={e => setF({ ...f, archived: e.target.checked })} />Archived (gone or replaced; its log stays)</label>}
        <Actions onClose={onClose} busy={busy} label={thing ? 'Save' : 'Add'} />
      </form>
      {inv && <PickerModal<InventoryItem> title="Link to a Pocket Inventory item" url={`${V1}/links/inventory`} initial={f.name} onClose={() => setInv(false)}
        row={i => <><span className="grow"><b>{i.name}</b><span className="muted small block-line">{[i.location_name, i.category_name].filter(Boolean).join(' · ')}</span></span></>}
        onPick={i => { setF({ ...f, inventory_id: i.id, inventory_name: i.name }); setInv(false) }} />}
    </Modal>
  )
}

// ---------- pickers over the other apps ----------

export function PickerModal<T extends { id: number }>({ title, url, initial = '', onClose, onPick, row, needQuery }: { title: string; url: string; initial?: string; onClose: () => void; onPick: (t: T) => void; row: (t: T) => React.ReactNode; needQuery?: boolean }) {
  const [q, setQ] = useState(initial)
  const dq = useDebounce(q, 300)
  const [res, setRes] = useState<Picked<T> | null>(null)
  useEffect(() => {
    if (needQuery && !dq.trim()) { setRes({ connected: true, items: [] }); return }
    let live = true
    api.get<Picked<T>>(`${url}?q=${encodeURIComponent(dq)}`).then(r => { if (live) setRes(r) }).catch(e => live && setRes({ connected: false, detail: errMsg(e), items: [] }))
    return () => { live = false }
  }, [dq, url, needQuery])
  return (
    <Modal title={title} onClose={onClose} size="sm">
      <div className="grid" style={{ gap: 10 }}>
        <input value={q} onChange={e => setQ(e.target.value)} placeholder="Search…" autoFocus />
        <div className="pick-list">
          {!res ? <Loading /> : !res.connected || res.detail ? <p className="bad small">{res.detail}</p>
            : res.items.length === 0 ? <p className="muted small">{needQuery && !dq.trim() ? 'Type a name.' : 'Nothing found.'}</p>
            : res.items.map(it => <button key={it.id} type="button" className="pick-row" onClick={() => onPick(it)}>{row(it)}</button>)}
        </div>
      </div>
    </Modal>
  )
}

// ---------- attachments ----------

export function AttachModal({ owner, id, title, onClose, onSaved }: { owner: Attachment['owner_kind']; id: number; title: string; onClose: () => void; onSaved: () => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const docsOn = !!settings && settings.documents_mode !== 'off'
  const [kind, setKind] = useState<'document' | 'url'>(docsOn ? 'document' : 'url')
  const [f, setF] = useState({ title: '', url: '' })
  const add = async (body: Record<string, unknown>) => {
    try { await api.post(`${V1}/attachments`, { owner_kind: owner, owner_id: id, ...body }); toast('Kept with ' + title); onSaved(); onClose() } catch (e) { toast(errMsg(e), 'err') }
  }
  return (
    <Modal title={`Keep a paper with ${title}`} onClose={onClose} size="sm">
      <div className="grid form-grid">
        <Segmented value={kind} onChange={setKind} options={[{ value: 'document', label: 'From Pocket Documents', disabled: !docsOn, title: docsOn ? '' : 'Pocket Documents is switched off in Settings → Connected apps' }, { value: 'url', label: 'A web link' }]} />
        {kind === 'document' ? (
          <DocPicker onPick={d => add({ kind: 'document', ref: String(d.id), title: d.title || d.original_name })} />
        ) : (
          <form onSubmit={e => { e.preventDefault(); add({ kind: 'url', url: f.url, title: f.title }) }} className="grid form-grid">
            <Field label="Link"><input value={f.url} onChange={e => setF({ ...f, url: e.target.value })} placeholder="https://…" required autoFocus /></Field>
            <Field label="Title"><input value={f.title} onChange={e => setF({ ...f, title: e.target.value })} placeholder="User manual" /></Field>
            <Actions onClose={onClose} busy={false} label="Keep it" />
          </form>
        )}
      </div>
    </Modal>
  )
}

function DocPicker({ onPick }: { onPick: (d: DocumentItem) => void }) {
  const [q, setQ] = useState('')
  const dq = useDebounce(q, 300)
  const [res, setRes] = useState<Picked<DocumentItem> | null>(null)
  useEffect(() => {
    let live = true
    api.get<Picked<DocumentItem>>(`${V1}/links/documents?q=${encodeURIComponent(dq)}`).then(r => { if (live) setRes(r) }).catch(e => live && setRes({ connected: false, detail: errMsg(e), items: [] }))
    return () => { live = false }
  }, [dq])
  return (
    <div className="grid" style={{ gap: 10 }}>
      <input value={q} onChange={e => setQ(e.target.value)} placeholder="Search your documents (manual, warranty, invoice…)" autoFocus />
      <div className="pick-list">
        {!res ? <Loading /> : res.detail ? <p className="bad small">{res.detail}</p> : res.items.length === 0 ? <p className="muted small">Nothing found.</p>
          : res.items.map(d => <button key={d.id} type="button" className="pick-row" onClick={() => onPick(d)}><Icon name="file" size={15} /><span className="grow"><b>{d.title || d.original_name}</b><span className="muted small block-line">{[d.document_type_name, d.correspondent_name, (d.created || d.added_at || '').slice(0, 10)].filter(Boolean).join(' · ')}</span></span></button>)}
      </div>
    </div>
  )
}

// ---------- meters ----------

export function MeterModal({ meter, onClose, onSaved }: { meter?: Meter; onClose: () => void; onSaved?: (m: Meter) => void }) {
  const { homes } = useData()
  const { toast } = useToast()
  const [f, setF] = useState({ home_id: meter?.home_id || homes?.find(h => !h.archived)?.id || 0, name: meter?.name || 'Electricity', kind: meter?.kind || 'electricity', unit: meter?.unit || 'kWh',
    price: meter?.price_per_unit != null ? String(meter.price_per_unit) : '', number: meter?.number || '', notes: meter?.notes || '', remind_day: String(meter?.remind_day ?? 1), archived: !!meter?.archived })
  const [busy, setBusy] = useState(false)
  const currency = homes?.find(h => h.id === f.home_id)?.currency || 'EUR'
  const setKind = (k: string) => { const mk = METER_KINDS.find(x => x.value === k)!; setF({ ...f, kind: k, unit: mk.unit, name: METER_KINDS.some(x => x.label === f.name) || !f.name ? mk.label : f.name }) }
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    const price = f.price.trim() ? Number(f.price.replace(',', '.')) : null
    const body = { home_id: f.home_id, name: f.name, kind: f.kind, unit: f.unit, price_per_unit: price, number: f.number, notes: f.notes, remind_day: Number(f.remind_day) || 0, archived: f.archived }
    try {
      const m = meter ? await api.put<Meter>(`${V1}/meters/${meter.id}`, body) : await api.post<Meter>(`${V1}/meters`, body)
      toast(meter ? 'Saved' : 'Meter added'); announceChange(); onSaved?.(m); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={meter ? `Edit ${meter.name}` : 'Add a meter'} onClose={onClose}>
      <form onSubmit={save} className="grid form-grid">
        <Field group label="What it measures"><Select ariaLabel="Kind" value={f.kind} onChange={setKind} options={METER_KINDS.map(k => ({ value: k.value, label: k.label }))} /></Field>
        <div className="two-col tight">
          <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} required maxLength={80} /></Field>
          <Field label="Unit"><input value={f.unit} onChange={e => setF({ ...f, unit: e.target.value })} maxLength={16} placeholder="kWh" /></Field>
        </div>
        <HomeSelect value={f.home_id} onChange={id => setF({ ...f, home_id: id })} />
        <div className="two-col tight">
          <Field label={`Price per ${f.unit || 'unit'} (${currency}, optional)`} help="Turns usage into money on the chart."><input inputMode="decimal" value={f.price} onChange={e => setF({ ...f, price: e.target.value })} placeholder="0.24" /></Field>
          <Field label="Meter number"><input value={f.number} onChange={e => setF({ ...f, number: e.target.value })} /></Field>
        </div>
        <Field label="Remind me to read it on day" help="Of every month (0 = no reminder). Needs notifications in Settings."><input type="number" min={0} max={31} value={f.remind_day} onChange={e => setF({ ...f, remind_day: e.target.value })} className="num-sm" /></Field>
        <Field label="Notes"><textarea rows={2} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} placeholder="Where it is, the tariff…" /></Field>
        {meter && <label className="check-row"><input type="checkbox" checked={f.archived} onChange={e => setF({ ...f, archived: e.target.checked })} />Archived</label>}
        <Actions onClose={onClose} busy={busy} label={meter ? 'Save' : 'Add meter'} />
      </form>
    </Modal>
  )
}

export function ReadingModal({ meter, reading, onClose, onSaved }: { meter: Pick<Meter, 'id' | 'name' | 'unit' | 'last'>; reading?: Reading; onClose: () => void; onSaved?: () => void }) {
  const { toast } = useToast()
  const [f, setF] = useState({ read_on: reading?.read_on || todayISO(), value: reading ? String(reading.value) : '', reset: !!reading?.reset, note: reading?.note || '' })
  const [busy, setBusy] = useState(false)
  const [sure, setSure] = useState(false)
  const remove = async () => {
    if (!reading) return
    if (!sure) { setSure(true); return }
    try { await api.del(`${V1}/readings/${reading.id}`); toast('Reading deleted'); announceChange(); onSaved?.(); onClose() } catch (err) { toast(errMsg(err), 'err') }
  }
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    const value = Number(f.value.replace(',', '.'))
    if (!Number.isFinite(value)) { toast('The reading is a number', 'err'); return }
    setBusy(true)
    try {
      if (reading) await api.put(`${V1}/readings/${reading.id}`, { ...f, value })
      else await api.post(`${V1}/meters/${meter.id}/readings`, { ...f, value })
      toast('Reading saved'); announceChange(); onSaved?.(); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={`${meter.name}: ${reading ? 'edit reading' : 'new reading'}`} onClose={onClose} size="sm">
      <form onSubmit={save} className="grid form-grid">
        {meter.last && !reading && <p className="muted small">Last: {meter.last.value.toLocaleString()} {meter.unit} on {meter.last.read_on}</p>}
        <div className="two-col tight">
          <Field label={`Reading (${meter.unit})`}><input inputMode="decimal" value={f.value} onChange={e => setF({ ...f, value: e.target.value })} required autoFocus /></Field>
          <Field label="Date"><input type="date" value={f.read_on} max={todayISO()} onChange={e => setF({ ...f, read_on: e.target.value })} required /></Field>
        </div>
        <Field label="Note"><input value={f.note} onChange={e => setF({ ...f, note: e.target.value })} placeholder="Estimated, from the bill…" /></Field>
        <label className="check-row"><input type="checkbox" checked={f.reset} onChange={e => setF({ ...f, reset: e.target.checked })} />New meter (it was replaced: no usage counted from the reading before)</label>
        <Actions onClose={onClose} busy={busy} label="Save reading" danger={reading && <button type="button" className="btn ghost danger-text" onClick={remove}><Icon name="trash" size={14} />{sure ? 'Delete it' : 'Delete'}</button>} />
      </form>
    </Modal>
  )
}

// ---------- pros ----------

export function ProModal({ pro, onClose, onSaved }: { pro?: Pro; onClose: () => void; onSaved?: (p: Pro) => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const [f, setF] = useState({ name: pro?.name || '', trade: pro?.trade || '', company: pro?.company || '', phone: pro?.phone || '', email: pro?.email || '', website: pro?.website || '', notes: pro?.notes || '', rating: pro?.rating || 0 })
  const [busy, setBusy] = useState(false)
  const [pick, setPick] = useState(false)
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    try {
      const p = pro ? await api.put<Pro>(`${V1}/pros/${pro.id}`, f) : await api.post<Pro>(`${V1}/pros`, f)
      toast(pro ? 'Saved' : `${p.name} added`); announceChange(); onSaved?.(p); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  const fromContact = async (c: ContactItem) => {
    try { const p = await api.post<Pro>(`${V1}/pros/from-contact`, { contact_id: c.id, trade: f.trade }); toast(`${p.name} added from your contacts`); announceChange(); onSaved?.(p); onClose() } catch (e) { toast(errMsg(e), 'err') }
  }
  const contactsOn = !!settings && settings.contacts_mode !== 'off'
  return (
    <Modal title={pro ? `Edit ${pro.name}` : 'Add a pro'} onClose={onClose}>
      <form onSubmit={save} className="grid form-grid">
        {!pro && contactsOn && <button type="button" className="btn" onClick={() => setPick(true)} disabled={settings?.contacts_via === ''} title={settings?.contacts_via === '' ? settings?.contacts_detail : ''}><Icon name="users" size={15} />Pick from Pocket Contacts</button>}
        <div className="two-col tight">
          <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} required maxLength={120} autoFocus={!!pro} /></Field>
          <Field label="Trade"><input value={f.trade} onChange={e => setF({ ...f, trade: e.target.value })} placeholder="Plumber" list="phm-trades" /><datalist id="phm-trades">{['Plumber', 'Electrician', 'Heating engineer', 'Chimney sweep', 'Roofer', 'Painter', 'Carpenter', 'Gardener', 'Cleaner', 'Locksmith', 'Handyman'].map(t => <option key={t} value={t} />)}</datalist></Field>
        </div>
        <div className="two-col tight">
          <Field label="Phone"><input type="tel" value={f.phone} onChange={e => setF({ ...f, phone: e.target.value })} /></Field>
          <Field label="Email"><input type="email" value={f.email} onChange={e => setF({ ...f, email: e.target.value })} /></Field>
        </div>
        <div className="two-col tight">
          <Field label="Company"><input value={f.company} onChange={e => setF({ ...f, company: e.target.value })} /></Field>
          <Field label="Website"><input value={f.website} onChange={e => setF({ ...f, website: e.target.value })} placeholder="https://…" /></Field>
        </div>
        <Field group label="How happy you were"><Stars value={f.rating} onChange={v => setF({ ...f, rating: v })} /></Field>
        <Field label="Notes"><textarea rows={2} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} placeholder="Comes on Saturdays, cash only…" /></Field>
        <Actions onClose={onClose} busy={busy} label={pro ? 'Save' : 'Add'} />
      </form>
      {pick && <PickerModal<ContactItem> title="Pick from Pocket Contacts" url={`${V1}/links/contacts`} needQuery onClose={() => setPick(false)} onPick={fromContact}
        row={c => <><Icon name="user" size={15} /><span className="grow"><b>{c.fn || c.org}</b><span className="muted small block-line">{[c.org !== c.fn ? c.org : '', c.phones?.[0], c.emails?.[0]].filter(Boolean).join(' · ')}</span></span></>} />}
    </Modal>
  )
}

export function Stars({ value, onChange }: { value: number; onChange?: (v: number) => void }) {
  return (
    <span className="stars" role={onChange ? 'radiogroup' : undefined} aria-label={`${value} of 5`}>
      {[1, 2, 3, 4, 5].map(n => onChange
        ? <button key={n} type="button" className={n <= value ? 'on' : ''} aria-label={`${n} of 5`} onClick={() => onChange(n === value ? 0 : n)}><Icon name="star" size={18} /></button>
        : <Icon key={n} name="star" size={13} className={n <= value ? 'on' : ''} />)}
    </span>
  )
}

// ---------- homes ----------

export function HomeModal({ home, onClose, onSaved }: { home?: Home; onClose: () => void; onSaved?: (h: Home) => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const [f, setF] = useState({ name: home?.name || '', kind: home?.kind || 'house', address: home?.address || '', year_built: home?.year_built ? String(home.year_built) : '',
    area_m2: home?.area_m2 ? String(home.area_m2) : '', currency: home?.currency || settings?.currency || 'EUR', notes: home?.notes || '', money_tag: home?.money_tag || '', archived: !!home?.archived })
  const [busy, setBusy] = useState(false)
  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true)
    const body = { ...f, year_built: f.year_built ? Number(f.year_built) : null, area_m2: f.area_m2 ? Number(f.area_m2.replace(',', '.')) : null }
    try {
      const h = home ? await api.put<Home>(`${V1}/homes/${home.id}`, body) : await api.post<Home>(`${V1}/homes`, body)
      toast(home ? 'Saved' : `${h.name} added`); announceChange(); onSaved?.(h); onClose()
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={home ? `Edit ${home.name}` : 'Add a home'} onClose={onClose}>
      <form onSubmit={save} className="grid form-grid">
        <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} required maxLength={120} autoFocus placeholder="Our house" /></Field>
        <Field group label="Kind"><Segmented value={f.kind} onChange={v => setF({ ...f, kind: v as Home['kind'] })} options={HOME_KINDS.map(k => ({ value: k.value as Home['kind'], label: k.label }))} /></Field>
        <Field label="Address"><input value={f.address} onChange={e => setF({ ...f, address: e.target.value })} /></Field>
        <div className="three-col tight">
          <Field label="Built in"><input type="number" min={1000} max={2100} value={f.year_built} onChange={e => setF({ ...f, year_built: e.target.value })} placeholder="1987" /></Field>
          <Field label="Area (m²)"><input inputMode="decimal" value={f.area_m2} onChange={e => setF({ ...f, area_m2: e.target.value })} /></Field>
          <Field label="Currency"><input value={f.currency} onChange={e => setF({ ...f, currency: e.target.value.toUpperCase() })} maxLength={3} required /></Field>
        </div>
        <Field label="Notes" help="Where the water and gas shut-offs are, the alarm code holder, the landlord…"><textarea rows={3} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} /></Field>
        {settings && settings.money_mode !== 'off' && <Field label="Tag on Pocket Money bookings (optional)"><input value={f.money_tag} onChange={e => setF({ ...f, money_tag: e.target.value })} placeholder="house" maxLength={40} /></Field>}
        {home && <label className="check-row"><input type="checkbox" checked={f.archived} onChange={e => setF({ ...f, archived: e.target.checked })} />Archived (sold or moved out)</label>}
        <Actions onClose={onClose} busy={busy} label={home ? 'Save' : 'Add home'} />
      </form>
    </Modal>
  )
}

export { money }
