// Upkeep: everything that comes back (or is planned once), grouped by how urgent it is; one job with its history.
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import { STATUS_NAME, money, repeatText, useHomeFilter } from '../lib/home'
import type { HomeRecord, Job, JobStatus } from '../lib/types'
import { Card, Confirm, Empty, Loading, Menu, type Anchor, errMsg, rectOf, useAsync, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { DoneModal, JobModal } from '../components/Forms'
import { JobRow, RecordRow, StatusPill } from '../components/Rows'
import { openCreate, useCrumb } from '../components/Layout'
import { HomePicker } from './Overview'

const ORDER: JobStatus[] = ['overdue', 'due', 'soon', 'ok', 'paused', 'done']

export default function Upkeep() {
  const { homes } = useData()
  const [home, setHome] = useHomeFilter(homes)
  const [q, setQ] = useState('')
  const [all, setAll] = useState(false)
  const { data, reload, error } = useAsync(() => api.get<Job[]>(`${V1}/jobs?${new URLSearchParams({ home_id: String(home), q, all: all ? '1' : '' })}`), [home, q, all])
  useOnChange(reload)
  const many = (homes || []).filter(h => !h.archived).length > 1
  return (
    <div className="stack loose">
      <div className="home-head">
        <div className="grow"><h1 className="home-title">Upkeep</h1><p className="muted">What comes back every so often, and what is planned once. Mark it done and the next date is set for you.</p></div>
        <HomePicker value={home} onChange={setHome} />
        <button type="button" className="btn primary" onClick={() => openCreate('job')}><Icon name="plus" size={15} />Add upkeep</button>
      </div>
      <div className="row wrap filters">
        <input className="search-in" value={q} onChange={e => setQ(e.target.value)} placeholder="Find upkeep…" aria-label="Find upkeep" />
        <label className="check-row"><input type="checkbox" checked={all} onChange={e => setAll(e.target.checked)} />Show finished one-offs</label>
      </div>
      {error && <div className="callout danger"><Icon name="alert" size={15} />{error}</div>}
      {!data ? <Loading /> : data.length === 0 ? (
        <Empty title={q ? 'Nothing matches' : 'No upkeep yet'} icon="wrench">{!q && <>Add an appliance and pick from its suggested upkeep, or <button type="button" className="link-btn" onClick={() => openCreate('job')}>add upkeep yourself</button>.</>}</Empty>
      ) : ORDER.map(st => {
        const list = data.filter(j => j.status === st)
        if (!list.length) return null
        return (
          <Card key={st} title={<>{STATUS_NAME[st]} <span className="muted count">{list.length}</span></>} flush>
            <div className="list">{list.map(j => <JobRow key={j.id} job={j} showHome={many && !home} onChanged={reload} />)}</div>
          </Card>
        )
      })}
    </div>
  )
}

interface JobData { job: Job; records: HomeRecord[]; task_url: string; calendar_url: string }

export function JobPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { settings } = useApp()
  const { toast } = useToast()
  const { data, reload, error } = useAsync(() => api.get<JobData>(`${V1}/jobs/${id}`), [id])
  useOnChange(reload)
  const [edit, setEdit] = useState(false)
  const [done, setDone] = useState(false)
  const [del, setDel] = useState(false)
  const [snooze, setSnooze] = useState<Anchor | null>(null)
  useCrumb(data?.job.title)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!data) return <Loading />
  const j = data.job
  const act = async (fn: () => Promise<unknown>, msg: string) => { try { await fn(); toast(msg); announceChange(); reload() } catch (e) { toast(errMsg(e), 'err') } }
  const tasksOn = settings && settings.tasks_mode !== 'off'
  const tasksReady = settings?.tasks_via !== ''
  const spent = data.records.reduce((s, r) => s + r.cost, 0)
  return (
    <div className="stack loose">
      <div className="detail-head">
        <span className={`detail-ico st-${j.status}`}><Icon name="wrench" size={22} /></span>
        <div className="grow">
          <h1 className="home-title">{j.title}</h1>
          <p className="muted">{[j.thing_name && <Link key="t" to={`/things/${j.thing_id}`}>{j.thing_name}</Link>, j.home_name, repeatText(j.every_n, j.every_unit)].filter(Boolean).reduce<React.ReactNode[]>((a, x, i) => i ? [...a, ' · ', x] : [x], [])}</p>
        </div>
        <StatusPill job={j} />
      </div>
      <div className="row wrap toolbar">
        {j.status !== 'done' && <button type="button" className="btn primary" onClick={() => setDone(true)}><Icon name="check" size={15} />Mark done</button>}
        {j.status !== 'done' && <button type="button" className="btn" onClick={e => setSnooze(rectOf(e.currentTarget))}><Icon name="clock" size={15} />Later</button>}
        {j.every_n > 0 && j.status !== 'done' && <button type="button" className="btn" onClick={() => act(() => api.post(`${V1}/jobs/${j.id}/skip`), 'Skipped this time')}><Icon name="repeat" size={15} />Skip this time</button>}
        {tasksOn && j.status !== 'done' && (data.task_url ? <a className="btn" href={data.task_url} target="_blank" rel="noreferrer"><Icon name="check-square" size={15} />In Pocket Tasks</a>
          : <button type="button" className="btn" disabled={!tasksReady} title={tasksReady ? 'Add it to your task list, with its due date' : settings?.tasks_detail}
            onClick={() => act(() => api.post(`${V1}/jobs/${j.id}/task`), 'Added to Pocket Tasks')}><Icon name="check-square" size={15} />Add to my tasks</button>)}
        <span className="grow" />
        <button type="button" className="btn ghost" onClick={() => setEdit(true)}><Icon name="edit" size={15} />Edit</button>
        <button type="button" className="btn ghost danger-text" onClick={() => setDel(true)}><Icon name="trash" size={15} />Delete</button>
      </div>
      {snooze && <Menu anchor={snooze} onClose={() => setSnooze(null)} items={[3, 7, 14, 30].map(d => ({ label: `${d} days later`, onSelect: () => act(() => api.post(`${V1}/jobs/${j.id}/snooze`, { days: d }), `Moved ${d} days`) }))} />}
      <div className="home-grid">
        <Card title="Details">
          <dl className="set-dl">
            <div><dt>Due</dt><dd>{j.status === 'done' ? `Finished ${fmtDate(j.finished_at.slice(0, 10))}` : fmtDate(j.due_on)}</dd></div>
            <div><dt>How often</dt><dd>{repeatText(j.every_n, j.every_unit)}</dd></div>
            <div><dt>Reminder</dt><dd>{j.soon_days ? `${j.soon_days} days before` : 'On the day'}</dd></div>
            {j.last_done && <div><dt>Last done</dt><dd>{fmtDate(j.last_done)} · {j.done_count} {j.done_count === 1 ? 'time' : 'times'} in all</dd></div>}
            {j.pro_name && <div><dt>Who</dt><dd><Link to={`/pros/${j.pro_id}`}>{j.pro_name}</Link></dd></div>}
            {j.est_cost != null && <div><dt>Usual cost</dt><dd>{money(j.est_cost, j.currency)}</dd></div>}
            {settings?.calendar_sync && <div><dt>Calendar</dt><dd>{j.cal_event_id ? (data.calendar_url ? <a href={data.calendar_url} target="_blank" rel="noreferrer">On your calendar</a> : 'On your calendar') : j.paused || j.status === 'done' ? 'Not shown' : 'Added within minutes'}</dd></div>}
          </dl>
          {j.notes && <p className="notes">{j.notes}</p>}
        </Card>
        <Card title="History" sub={data.records.length ? `${data.records.length} times logged${spent ? ` · ${money(spent, j.currency)} in all` : ''}` : undefined} flush>
          {data.records.length === 0 ? <p className="muted pad">Nothing logged for it yet. Mark it done and it appears here.</p>
            : <div className="list">{data.records.map(r => <RecordRow key={r.id} r={r} showThing={false} />)}</div>}
        </Card>
      </div>
      {edit && <JobModal job={j} onClose={() => setEdit(false)} onSaved={reload} />}
      {done && <DoneModal job={j} onClose={() => setDone(false)} onDone={reload} />}
      {del && <Confirm title={`Delete “${j.title}”?`} message="The upkeep and its reminders go; what was logged for it stays in the log." confirmLabel="Delete"
        onCancel={() => setDel(false)} onConfirm={() => { setDel(false); act(() => api.del(`${V1}/jobs/${j.id}`), 'Deleted').then(() => navigate('/upkeep')) }} />}
    </div>
  )
}
