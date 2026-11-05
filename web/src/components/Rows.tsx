// The rows every list shares: upkeep (with its Done button), log entries, things.
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { CATEGORY_ICON, RECORD_KIND_ICON, RECORD_KIND_NAME, dueText, money, repeatText } from '../lib/home'
import { CARE_OF, careName } from '../lib/plants'
import { useCareDone } from './PlantForms'
import type { CareKind } from '../lib/types'
import { fmtDate } from '../lib/format'
import type { HomeRecord, Job, Thing } from '../lib/types'
import { DoneModal } from './Forms'
import { Icon } from './icons'

export function StatusPill({ job }: { job: Pick<Job, 'status' | 'days_left' | 'due_on'> }) {
  const icon = job.status === 'overdue' ? 'alert' : job.status === 'due' ? 'clock' : job.status === 'soon' ? 'calendar' : job.status === 'paused' ? 'pause' : job.status === 'done' ? 'check' : 'calendar'
  return <span className={`pill st-${job.status}`}><Icon name={icon} size={12} />{dueText(job)}</span>
}

export function JobRow({ job, showHome, showThing = true, onChanged }: { job: Job; showHome?: boolean; showThing?: boolean; onChanged?: () => void }) {
  const [done, setDone] = useState(false)
  if (job.plant_id) return <PlantCareRow job={job} showHome={showHome} />
  const sub = [showThing && job.thing_name, showHome && job.home_name, repeatText(job.every_n, job.every_unit), job.pro_name].filter(Boolean).join(' · ')
  return (
    <div className="list-row job-row">
      <span className={`list-ico st-${job.status}`}><Icon name={job.thing_id ? 'wrench' : 'check-square'} size={16} /></span>
      <Link to={`/upkeep/${job.id}`} className="grow row-main">
        <span className="row-title">{job.title}</span>
        <span className="muted small ellipsis">{sub}</span>
      </Link>
      <StatusPill job={job} />
      {job.status !== 'done' && <button type="button" className="btn sm" onClick={() => setDone(true)} aria-label={`Mark ${job.title} done`}><Icon name="check" size={14} /><span className="hide-xs">Done</span></button>}
      {done && <DoneModal job={job} onClose={() => setDone(false)} onDone={onChanged} />}
    </div>
  )
}

/** Plant care on a due list: "Water Monstera", one tap to mark it done. */
function PlantCareRow({ job, showHome }: { job: Job; showHome?: boolean }) {
  const { done, busy } = useCareDone()
  const c = CARE_OF[job.care]
  const sub = [showHome && job.home_name, repeatText(job.every_n, job.every_unit)].filter(Boolean).join(' · ')
  return (
    <div className="list-row job-row">
      <span className={`list-ico plant-ico st-${job.status}`}><Icon name={c?.icon || 'leaf'} size={16} /></span>
      <Link to={`/plants/${job.plant_id}`} className="grow row-main">
        <span className="row-title">{careName(job)}</span>
        <span className="muted small ellipsis">{sub}</span>
      </Link>
      <StatusPill job={job} />
      {job.status !== 'done' && job.status !== 'paused' && <button type="button" className="btn sm" disabled={!!busy} onClick={() => done(job.plant_id!, job.plant_name, job.care as CareKind)} aria-label={`${c?.done || 'Done'}: ${job.plant_name}`}>
        <Icon name="check" size={14} /><span className="hide-xs">{c?.done || 'Done'}</span></button>}
    </div>
  )
}

export function RecordRow({ r, showThing = true, showHome }: { r: HomeRecord; showThing?: boolean; showHome?: boolean }) {
  const sub = [fmtDate(r.date), RECORD_KIND_NAME[r.kind] || r.kind, showThing && (r.thing_name || (r.plant_name && !r.title.includes(r.plant_name) ? r.plant_name : '')), showHome && r.home_name, r.pro_name].filter(Boolean).join(' · ')
  return (
    <Link to={`/log/${r.id}`} className="list-row">
      <span className={`list-ico rk-${r.kind}`}><Icon name={RECORD_KIND_ICON[r.kind] || 'note'} size={16} /></span>
      <span className="grow row-main"><span className="row-title">{r.title}</span><span className="muted small ellipsis">{sub}</span></span>
      {r.money_tx_id && <Icon name="wallet" size={14} className="muted" title="Booked in Pocket Money" />}
      <span className="tnum row-amount">{r.cost ? money(r.cost, r.currency) : ''}</span>
    </Link>
  )
}

export function ThingRow({ t, showHome }: { t: Thing; showHome?: boolean }) {
  const sub = [t.room, showHome && t.home_name, [t.brand, t.model].filter(Boolean).join(' ')].filter(Boolean).join(' · ')
  return (
    <Link to={`/things/${t.id}`} className={`list-row ${t.archived ? 'is-archived' : ''}`}>
      <span className={`list-ico cat-${t.category}`}><Icon name={CATEGORY_ICON[t.category] || 'package'} size={16} /></span>
      <span className="grow row-main"><span className="row-title">{t.name}{t.archived && <span className="badge">archived</span>}</span><span className="muted small ellipsis">{sub}</span></span>
      {t.overdue > 0 ? <span className="pill st-overdue"><Icon name="alert" size={12} />{t.overdue} overdue</span>
        : t.next_due ? <span className="muted small hide-xs">next {fmtDate(t.next_due, false)}</span> : null}
    </Link>
  )
}
