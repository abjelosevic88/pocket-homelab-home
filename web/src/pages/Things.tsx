// Appliances and parts of the house, by room; one thing with its upkeep, history and papers.
import type React from 'react'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import { CATEGORIES, CATEGORY_ICON, CATEGORY_NAME, money, useHomeFilter } from '../lib/home'
import type { Attachment, HomeRecord, Job, Suggestion, Thing } from '../lib/types'
import { Card, Confirm, Empty, Loading, Segmented, errMsg, useAsync, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { AttachModal, JobModal, RecordModal, SuggestModal, ThingModal } from '../components/Forms'
import { JobRow, RecordRow, ThingRow } from '../components/Rows'
import { Select } from '../components/Select'
import { openCreate, useCrumb } from '../components/Layout'
import { HomePicker } from './Overview'

export default function Things() {
  const { homes } = useData()
  const [home, setHome] = useHomeFilter(homes)
  const [q, setQ] = useState('')
  const [cat, setCat] = useState('')
  const [by, setBy] = useState<'room' | 'category'>(() => (localStorage.getItem('phm-things-by') as 'room' | 'category') || 'room')
  const [arch, setArch] = useState(false)
  const { data, reload, error } = useAsync(() => api.get<Thing[]>(`${V1}/things?${new URLSearchParams({ home_id: String(home), q, category: cat, archived: arch ? '1' : '' })}`), [home, q, cat, arch])
  useOnChange(reload)
  const many = (homes || []).filter(h => !h.archived).length > 1
  const groups = new Map<string, Thing[]>()
  for (const t of data || []) {
    const k = by === 'room' ? (many && !home ? `${t.home_name} · ` : '') + (t.room || 'No room') : CATEGORY_NAME[t.category] || t.category
    groups.set(k, [...(groups.get(k) || []), t])
  }
  const setGrouping = (v: 'room' | 'category') => { setBy(v); localStorage.setItem('phm-things-by', v) }
  return (
    <div className="stack loose">
      <div className="home-head">
        <div className="grow"><h1 className="home-title">Appliances and parts</h1><p className="muted">Everything that needs looking after, with its model, warranty, manual and history.</p></div>
        <HomePicker value={home} onChange={setHome} />
        <button type="button" className="btn primary" onClick={() => openCreate('thing')}><Icon name="plus" size={15} />Add</button>
      </div>
      <div className="row wrap filters">
        <input className="search-in" value={q} onChange={e => setQ(e.target.value)} placeholder="Find by name, brand, model…" aria-label="Find" />
        <Select ariaLabel="Kind" value={cat} onChange={setCat} clearLabel="Every kind" placeholder="Every kind" options={CATEGORIES.map(c => ({ value: c.value, label: c.label }))} />
        <Segmented size="sm" value={by} onChange={setGrouping} options={[{ value: 'room', label: 'By room' }, { value: 'category', label: 'By kind' }]} label="Group by" />
        <label className="check-row"><input type="checkbox" checked={arch} onChange={e => setArch(e.target.checked)} />Archived too</label>
      </div>
      {error && <div className="callout danger"><Icon name="alert" size={15} />{error}</div>}
      {!data ? <Loading /> : data.length === 0 ? (
        <Empty title={q || cat ? 'Nothing matches' : 'Nothing here yet'} icon="package">{!q && !cat && <>Start with what breaks or needs a service: the boiler, the water heater, the washing machine, the roof.</>}</Empty>
      ) : [...groups.entries()].map(([g, list]) => (
        <Card key={g} title={<>{g} <span className="muted count">{list.length}</span></>} flush>
          <div className="list">{list.map(t => <ThingRow key={t.id} t={t} showHome={many && !home && by === 'category'} />)}</div>
        </Card>
      ))}
    </div>
  )
}

interface ThingData { thing: Thing; jobs: Job[]; records: HomeRecord[]; attachments: Attachment[]; warranty: '' | 'active' | 'ending' | 'ended'; suggestions: Suggestion[] }

export function ThingPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { settings } = useApp()
  const { toast } = useToast()
  const { data, reload, error } = useAsync(() => api.get<ThingData>(`${V1}/things/${id}`), [id])
  useOnChange(reload)
  const [modal, setModal] = useState<'' | 'edit' | 'job' | 'record' | 'suggest' | 'attach' | 'delete'>('')
  useCrumb(data?.thing.name)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!data) return <Loading />
  const t = data.thing
  const open = data.jobs.filter(j => j.status !== 'done')
  const detach = async (a: Attachment) => { try { await api.del(`${V1}/attachments/${a.id}`); reload() } catch (e) { toast(errMsg(e), 'err') } }
  const rows: [string, React.ReactNode][] = [
    ['Kind', CATEGORY_NAME[t.category] || t.category], ['Where', [t.room, t.home_name].filter(Boolean).join(' · ')],
    ['Brand', t.brand], ['Model', t.model], ['Serial number', t.serial && <code key="s">{t.serial}</code>],
    ['Bought', [t.purchased_on && fmtDate(t.purchased_on), t.vendor].filter(Boolean).join(' · ')], ['Installed', t.installed_on && fmtDate(t.installed_on)],
    ['Price', t.price != null ? money(t.price, t.currency) : ''],
    ['Warranty', t.warranty_until && <span key="w" className={data.warranty === 'ended' ? 'muted' : data.warranty === 'ending' ? 'warn-text' : ''}>{data.warranty === 'ended' ? 'Ended' : 'Until'} {fmtDate(t.warranty_until)}</span>],
    ['Manual', t.manual_url && <a key="m" href={t.manual_url} target="_blank" rel="noreferrer">Open the manual <Icon name="external" size={12} /></a>],
    ['Pocket Inventory', t.inventory_id ? <span key="i" className="row" style={{ gap: 6 }}><Icon name="package" size={13} />{t.inventory_name || `Item ${t.inventory_id}`}</span> : ''],
    ['Spent on it', t.spent ? money(t.spent, t.currency) : ''],
  ]
  return (
    <div className="stack loose">
      <div className="detail-head">
        <span className={`detail-ico cat-${t.category}`}><Icon name={CATEGORY_ICON[t.category] || 'package'} size={22} /></span>
        <div className="grow">
          <h1 className="home-title">{t.name}{t.archived && <span className="badge">archived</span>}</h1>
          <p className="muted">{[t.brand, t.model].filter(Boolean).join(' ') || CATEGORY_NAME[t.category]}{t.room ? ` · ${t.room}` : ''}</p>
        </div>
      </div>
      <div className="row wrap toolbar">
        <button type="button" className="btn primary" onClick={() => setModal('record')}><Icon name="note" size={15} />Log work</button>
        <button type="button" className="btn" onClick={() => setModal('suggest')}><Icon name="sparkles" size={15} />Suggested upkeep</button>
        <button type="button" className="btn" onClick={() => setModal('job')}><Icon name="plus" size={15} />Add upkeep</button>
        <span className="grow" />
        <button type="button" className="btn ghost" onClick={() => setModal('edit')}><Icon name="edit" size={15} />Edit</button>
        <button type="button" className="btn ghost danger-text" onClick={() => setModal('delete')}><Icon name="trash" size={15} />Delete</button>
      </div>
      {data.warranty === 'ending' && <div className="callout warn"><Icon name="shield" size={15} /><div>The warranty ends on {fmtDate(t.warranty_until)}. Anything to claim, claim it before then.</div></div>}
      <div className="home-grid">
        <div className="stack">
          <Card title="Upkeep" flush actions={open.length > 0 && <button type="button" className="btn ghost sm" onClick={() => setModal('job')}><Icon name="plus" size={14} />Add</button>}>
            {open.length === 0 ? (
              <div className="pad stack">
                <p className="muted">No upkeep planned for it yet.</p>
                {data.suggestions.length > 0 && <div className="row wrap"><button type="button" className="btn sm" onClick={() => setModal('suggest')}><Icon name="sparkles" size={14} />See {data.suggestions.length} suggestions</button><span className="muted small">e.g. {data.suggestions[0].title.toLowerCase()}</span></div>}
              </div>
            ) : <div className="list">{open.map(j => <JobRow key={j.id} job={j} showThing={false} onChanged={reload} />)}</div>}
          </Card>
          <Card title="History" sub={data.records.length ? `${data.records.length} entries` : undefined} flush>
            {data.records.length === 0 ? <p className="muted pad">Nothing logged yet.</p> : <div className="list">{data.records.map(r => <RecordRow key={r.id} r={r} showThing={false} />)}</div>}
          </Card>
        </div>
        <div className="stack">
          <Card title="Details">
            <dl className="set-dl">{rows.filter(([, v]) => v).map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>
            {t.notes && <p className="notes">{t.notes}</p>}
          </Card>
          <Card title="Papers and links" sub="Manual, warranty card, receipt" flush actions={<button type="button" className="btn ghost sm" onClick={() => setModal('attach')}><Icon name="plus" size={14} />Add</button>}>
            {data.attachments.length === 0 ? <p className="muted pad">{settings?.documents_mode !== 'off' ? 'Keep a manual or an invoice from Pocket Documents, or a web link.' : 'Keep a web link to the manual or the receipt.'}</p>
              : <div className="list">{data.attachments.map(a => (
                <div key={a.id} className="list-row">
                  <span className="list-ico"><Icon name={a.kind === 'document' ? 'file' : 'link'} size={16} /></span>
                  <span className="grow row-main">{a.open_url ? <a href={a.open_url} target="_blank" rel="noreferrer" className="row-title">{a.title}</a> : <span className="row-title">{a.title}</span>}
                    <span className="muted small">{a.kind === 'document' ? 'Pocket Documents' : (a.url.replace(/^https?:\/\//, '').split('/')[0])}</span></span>
                  <button type="button" className="btn ghost sm icon-only" onClick={() => detach(a)} aria-label={`Remove ${a.title}`} title="Remove"><Icon name="x" size={14} /></button>
                </div>))}</div>}
          </Card>
        </div>
      </div>
      {modal === 'edit' && <ThingModal thing={t} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'job' && <JobModal preset={{ home_id: t.home_id, thing_id: t.id }} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'record' && <RecordModal preset={{ home_id: t.home_id, thing_id: t.id }} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'suggest' && <SuggestModal thing={t} onClose={() => { setModal(''); reload() }} />}
      {modal === 'attach' && <AttachModal owner="thing" id={t.id} title={t.name} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'delete' && <Confirm title={`Delete ${t.name}?`} message={<>Its upkeep goes with it. What was logged stays in the log, without the appliance. To keep it out of the way instead, <b>archive</b> it under Edit.</>} confirmLabel="Delete"
        onCancel={() => setModal('')} onConfirm={async () => { setModal(''); try { await api.del(`${V1}/things/${t.id}`); toast('Deleted'); announceChange(); navigate('/things') } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}
