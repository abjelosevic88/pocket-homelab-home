// House plants, by room, with one tap for "Watered"; one plant with its care, history and links.
import type React from 'react'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useData, useOnChange } from '../lib/data'
import { fmtDate, todayISO } from '../lib/format'
import { repeatText, useHomeFilter } from '../lib/home'
import { CARE_OF, GONE, LIGHT_NAME, PLANT_KIND_NAME, careDue, dueCare, nextCare, wateredText } from '../lib/plants'
import type { Attachment, CareKind, CareSuggestion, HomeRecord, Job, Plant } from '../lib/types'
import { Card, Confirm, Empty, Loading, errMsg, useAsync, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { AttachModal } from '../components/Forms'
import { CareLogModal, PlantModal, ScheduleModal, WaterAllModal, useCareDone } from '../components/PlantForms'
import { RecordRow, StatusPill } from '../components/Rows'
import { openCreate, useCrumb } from '../components/Layout'
import { HomePicker } from './Overview'

export function PlantRow({ p, showHome }: { p: Plant; showHome?: boolean }) {
  const { done, busy } = useCareDone()
  const now = dueCare(p)
  const next = nextCare(p)
  const water = p.care.find(c => c.care === 'water' && c.status !== 'paused')
  const sub = [p.species || PLANT_KIND_NAME[p.kind], showHome && p.home_name, LIGHT_NAME[p.light], p.last_watered && wateredText(p.last_watered)].filter(Boolean).join(' · ')
  return (
    <div className={`list-row plant-row ${p.archived ? 'is-archived' : ''}`}>
      <span className={`list-ico plant-ico st-${p.archived ? 'paused' : p.status}`}><Icon name="leaf" size={16} /></span>
      <div className="grow row-main">
        <Link to={`/plants/${p.id}`} className="row-link">
          <span className="row-title">{p.name}{p.archived && <span className="badge">{GONE[p.gone_reason] || 'gone'}</span>}</span>
          <span className="muted small ellipsis">{sub}</span>
        </Link>
        {!p.archived && (now.length > 0
          ? <span className="small care-line-due">{now.map(c => c.care === 'water'
            ? <span key={c.id} className={`care-due st-${c.status}`}><Icon name="droplet" size={12} />{careDue(c)}</span>
            : <button key={c.id} type="button" className={`care-chip st-${c.status}`} disabled={!!busy} onClick={() => done(p.id, p.name, c.care as CareKind)}
              aria-label={`${CARE_OF[c.care]?.done}: ${p.name}`} title={`Mark ${p.name} ${CARE_OF[c.care]?.done.toLowerCase()} today`}>
              <Icon name={CARE_OF[c.care]?.icon || 'leaf'} size={12} />{careDue(c)}<span className="care-chip-done"><Icon name="check" size={12} />Done</span></button>)}</span>
          : next && <span className="small care-line-due"><span className="care-due st-ok"><Icon name={CARE_OF[next.care]?.icon || 'leaf'} size={12} />{careDue(next)}</span></span>)}
      </div>
      {!p.archived && water && <button type="button" className="btn sm care-btn" disabled={!!busy} onClick={() => done(p.id, p.name, 'water')} aria-label={`Watered: ${p.name}`} title={`Mark ${p.name} watered today`}>
        <Icon name="droplet" size={14} />Watered</button>}
    </div>
  )
}

export default function Plants() {
  const { homes } = useData()
  const [home, setHome] = useHomeFilter(homes)
  const [q, setQ] = useState('')
  const [gone, setGone] = useState(false)
  const [water, setWater] = useState(false)
  const { data, reload, error } = useAsync(() => api.get<Plant[]>(`${V1}/plants?${new URLSearchParams({ home_id: String(home), q, archived: gone ? '1' : '' })}`), [home, q, gone])
  useOnChange(reload)
  const many = (homes || []).filter(h => !h.archived).length > 1
  const live = (data || []).filter(p => !p.archived)
  const thirsty = live.filter(p => p.care.some(c => c.care === 'water' && (c.status === 'due' || c.status === 'overdue')))
  const groups = new Map<string, Plant[]>()
  for (const p of data || []) {
    const k = p.archived ? 'Gone' : (many && !home ? `${p.home_name} · ` : '') + (p.room || 'No room')
    groups.set(k, [...(groups.get(k) || []), p])
  }
  return (
    <div className="stack loose">
      <div className="home-head">
        <div className="grow"><h1 className="home-title">Plants</h1><p className="muted">Your house plants and when they need water, food or a bigger pot. Tap the button when it's done; the next time is set for you.</p></div>
        <HomePicker value={home} onChange={setHome} />
        <button type="button" className="btn primary" onClick={() => openCreate('plant')}><Icon name="plus" size={15} />Add a plant</button>
      </div>
      {live.length > 0 && <div className={`callout ${thirsty.length ? 'info' : ''} water-bar`}>
        <Icon name="droplet" size={15} />
        <div className="grow">{thirsty.length ? <><b>{thirsty.length === 1 ? '1 plant needs' : `${thirsty.length} plants need`} water today:</b> {thirsty.map(p => p.name).join(', ')}</> : 'Nothing to water today.'}</div>
        <button type="button" className="btn sm" onClick={() => setWater(true)}><Icon name="droplet" size={14} />{thirsty.length > 1 ? 'Water all…' : 'Water…'}</button>
      </div>}
      <div className="row wrap filters">
        <input className="search-in" value={q} onChange={e => setQ(e.target.value)} placeholder="Find a plant…" aria-label="Find a plant" />
        <label className="check-row"><input type="checkbox" checked={gone} onChange={e => setGone(e.target.checked)} />Gone too</label>
      </div>
      {error && <div className="callout danger"><Icon name="alert" size={15} />{error}</div>}
      {!data ? <Loading /> : data.length === 0 ? (
        <Empty title={q ? 'Nothing matches' : 'No plants yet'} icon="leaf">{!q && <>Add your first plant: watering, feeding and repotting are suggested for its kind and light, and it reminds you when they are due.</>}</Empty>
      ) : [...groups.entries()].map(([g, list]) => (
        <Card key={g} title={<>{g} <span className="muted count">{list.length}</span></>} flush>
          <div className="list">{list.map(p => <PlantRow key={p.id} p={p} showHome={false} />)}</div>
        </Card>
      ))}
      {water && <WaterAllModal plants={live} onClose={() => setWater(false)} />}
    </div>
  )
}

interface PlantData { plant: Plant; records: HomeRecord[]; records_total: number; attachments: Attachment[]; suggestions: CareSuggestion[] }

export function PlantPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { homes } = useData()
  const { data, reload, error } = useAsync(() => api.get<PlantData>(`${V1}/plants/${id}`), [id])
  useOnChange(reload)
  const { done, busy } = useCareDone()
  const [modal, setModal] = useState<'' | 'edit' | 'care' | 'log' | 'attach' | 'delete'>('')
  useCrumb(data?.plant.name)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!data) return <Loading />
  const p = data.plant
  const many = (homes || []).filter(h => !h.archived).length > 1
  const care = p.care.filter(c => c.status !== 'done')
  const hasWater = care.some(c => c.care === 'water')
  const detach = async (a: Attachment) => { try { await api.del(`${V1}/attachments/${a.id}`); reload() } catch (e) { toast(errMsg(e), 'err') } }
  const back = async () => { try { await api.put(`${V1}/plants/${p.id}`, { archived: false }); toast(`${p.name} is back`); announceChange() } catch (e) { toast(errMsg(e), 'err') } }
  const rows: [string, React.ReactNode][] = [
    ['Species', p.species], ['Kind', PLANT_KIND_NAME[p.kind]], ['Light', LIGHT_NAME[p.light]], ['Where', [p.room, many && p.home_name].filter(Boolean).join(' · ')],
    ['Got it', p.acquired_on && fmtDate(p.acquired_on)], ['Pot', p.pot], ['Last watered', p.last_watered && fmtDate(p.last_watered)],
  ]
  return (
    <div className="stack loose">
      <div className="detail-head">
        <span className={`detail-ico plant-ico st-${p.archived ? 'paused' : p.status}`}><Icon name="leaf" size={22} /></span>
        <div className="grow">
          <h1 className="home-title">{p.name}{p.archived && <span className="badge">{GONE[p.gone_reason] || 'gone'}</span>}</h1>
          <p className="muted">{[p.species || PLANT_KIND_NAME[p.kind], p.room, LIGHT_NAME[p.light]].filter(Boolean).join(' · ')}</p>
        </div>
      </div>
      <div className="row wrap toolbar">
        {!p.archived && hasWater && <button type="button" className="btn primary" disabled={!!busy} onClick={() => done(p.id, p.name, 'water')}><Icon name="droplet" size={15} />Watered</button>}
        {!p.archived && <button type="button" className="btn" onClick={() => setModal('log')}><Icon name="note" size={15} />Log care</button>}
        <button type="button" className="btn" onClick={() => setModal('care')}><Icon name="repeat" size={15} />Care schedule</button>
        <span className="grow" />
        <button type="button" className="btn ghost" onClick={() => setModal('edit')}><Icon name="edit" size={15} />Edit</button>
        <button type="button" className="btn ghost danger-text" onClick={() => setModal('delete')}><Icon name="trash" size={15} />Delete</button>
      </div>
      {p.archived && <div className="callout"><Icon name="leaf" size={15} /><div className="grow">{GONE[p.gone_reason] || 'Gone'}{p.gone_on ? ` on ${fmtDate(p.gone_on)}` : ''}. Its care is paused; the history stays.</div>
        <button type="button" className="btn sm" onClick={back}>It's back</button></div>}
      <div className="home-grid">
        <div className="stack">
          <Card title="Care" flush actions={<button type="button" className="btn ghost sm" onClick={() => setModal('care')}><Icon name="edit" size={14} />Change</button>}>
            {care.length === 0 ? <div className="pad stack"><p className="muted">No care planned for it.</p>
              <div className="row"><button type="button" className="btn sm" onClick={() => setModal('care')}><Icon name="plus" size={14} />Plan its care</button></div></div>
              : <div className="list">{care.map(c => (
                <div key={c.id} className="list-row job-row">
                  <span className={`list-ico plant-ico st-${c.status}`}><Icon name={CARE_OF[c.care]?.icon || 'leaf'} size={16} /></span>
                  <Link to={`/upkeep/${c.id}`} className="grow row-main">
                    <span className="row-title">{c.title}</span>
                    <span className="muted small ellipsis">{[repeatText(c.every_n, c.every_unit), c.last_done && `last ${fmtDate(c.last_done, c.last_done.slice(0, 4) !== todayISO().slice(0, 4))}`].filter(Boolean).join(' · ')}</span>
                  </Link>
                  <StatusPill job={c} />
                  {!p.archived && <button type="button" className="btn sm" disabled={!!busy} onClick={() => done(p.id, p.name, c.care as CareKind)} aria-label={`${CARE_OF[c.care]?.done}: ${p.name}`}><Icon name="check" size={14} /><span className="hide-xs">Done</span></button>}
                </div>))}</div>}
          </Card>
          <Card title="History" sub={data.records_total ? `${data.records_total} ${data.records_total === 1 ? 'entry' : 'entries'}` : undefined} flush>
            {data.records.length === 0 ? <p className="muted pad">Nothing logged yet. Tap Watered or Done and it appears here.</p>
              : <div className="list">{data.records.map(r => <RecordRow key={r.id} r={r} showThing={false} />)}</div>}
          </Card>
        </div>
        <div className="stack">
          <Card title="Details">
            <dl className="set-dl">{rows.filter(([, v]) => v).map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>
            {p.notes && <p className="notes">{p.notes}</p>}
          </Card>
          <Card title="Links" sub="A care guide, where you bought it, a photo" flush actions={<button type="button" className="btn ghost sm" onClick={() => setModal('attach')}><Icon name="plus" size={14} />Add</button>}>
            {data.attachments.length === 0 ? <p className="muted pad">Keep a web link or a document with it.</p>
              : <div className="list">{data.attachments.map(a => (
                <div key={a.id} className="list-row">
                  <span className="list-ico"><Icon name={a.kind === 'document' ? 'file' : 'link'} size={16} /></span>
                  <span className="grow row-main">{a.open_url ? <a href={a.open_url} target="_blank" rel="noreferrer" className="row-title">{a.title}</a> : <span className="row-title">{a.title}</span>}</span>
                  <button type="button" className="btn ghost sm icon-only" onClick={() => detach(a)} aria-label={`Remove ${a.title}`} title="Remove"><Icon name="x" size={14} /></button>
                </div>))}</div>}
          </Card>
        </div>
      </div>
      {modal === 'edit' && <PlantModal plant={p} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'care' && <ScheduleModal plant={p} suggestions={data.suggestions} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'log' && <CareLogModal plant={p} onClose={() => setModal('')} onDone={reload} />}
      {modal === 'attach' && <AttachModal owner="plant" id={p.id} title={p.name} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'delete' && <Confirm title={`Delete ${p.name}?`} message={<>Its care and its care history go with it. To keep the history, mark it <b>gone</b> under Edit instead.</>} confirmLabel="Delete"
        onCancel={() => setModal('')} onConfirm={async () => { setModal(''); try { await api.del(`${V1}/plants/${p.id}`); toast('Deleted'); announceChange(); navigate('/plants') } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}
