// My homes: each place you look after; one home with its notes, rooms, meters and papers.
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useData, useOnChange } from '../lib/data'
import { HOME_KINDS, fmtNum, useHomeFilter } from '../lib/home'
import type { Attachment, Home, Thing } from '../lib/types'
import { Card, Confirm, Empty, Loading, errMsg, useAsync, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { AttachModal, HomeModal } from '../components/Forms'
import { ThingRow } from '../components/Rows'
import { openCreate, useCrumb } from '../components/Layout'

const kindIcon = (k: string) => HOME_KINDS.find(x => x.value === k)?.icon || 'home'

export default function Homes() {
  const { homes } = useData()
  return (
    <div className="stack loose">
      <div className="home-head">
        <div className="grow"><h1 className="home-title">My homes</h1><p className="muted">A house, a flat, a weekend cottage: each with its own appliances, upkeep, log and meters.</p></div>
        <button type="button" className="btn primary" onClick={() => openCreate('home')}><Icon name="plus" size={15} />Add a home</button>
      </div>
      {!homes ? <Loading /> : homes.length === 0 ? <Empty title="No home yet" icon="home" /> : (
        <div className="meter-grid">{homes.map(h => (
          <Link key={h.id} to={`/homes/${h.id}`} className={`card home-card ${h.archived ? 'is-archived' : ''}`}>
            <span className="detail-ico"><Icon name={kindIcon(h.kind)} size={20} /></span>
            <b>{h.name}{h.archived && <span className="badge">archived</span>}</b>
            <span className="muted small">{h.address || HOME_KINDS.find(k => k.value === h.kind)?.label}</span>
            <span className="small">{h.things} appliances · {h.open_jobs} upkeep{h.overdue ? <span className="bad"> · {h.overdue} overdue</span> : ''}</span>
          </Link>))}</div>
      )}
    </div>
  )
}

export function HomePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { homes } = useData()
  const [, setFilter] = useHomeFilter(homes)
  const { data, reload, error } = useAsync(() => api.get<{ home: Home; attachments: Attachment[] }>(`${V1}/homes/${id}`), [id])
  const { data: things, reload: reloadThings } = useAsync(() => api.get<Thing[]>(`${V1}/things?home_id=${id}`), [id])
  useOnChange(() => { reload(); reloadThings() })
  const [modal, setModal] = useState<'' | 'edit' | 'attach' | 'delete'>('')
  useCrumb(data?.home.name)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!data) return <Loading />
  const h = data.home
  const focus = (to: string) => { setFilter(h.id); navigate(to) }
  return (
    <div className="stack loose">
      <div className="detail-head">
        <span className="detail-ico"><Icon name={kindIcon(h.kind)} size={22} /></span>
        <div className="grow"><h1 className="home-title">{h.name}{h.archived && <span className="badge">archived</span>}</h1>
          <p className="muted">{[h.address, h.year_built && `built ${h.year_built}`, h.area_m2 && `${fmtNum(h.area_m2, 0)} m²`].filter(Boolean).join(' · ')}</p></div>
      </div>
      <div className="row wrap toolbar">
        <button type="button" className="btn" onClick={() => focus('/upkeep')}><Icon name="wrench" size={15} />Upkeep{h.overdue ? <span className="badge warn">{h.overdue}</span> : null}</button>
        <button type="button" className="btn" onClick={() => focus('/log')}><Icon name="note" size={15} />Log</button>
        <button type="button" className="btn" onClick={() => focus('/meters')}><Icon name="gauge" size={15} />Meters</button>
        <span className="grow" />
        <button type="button" className="btn ghost" onClick={() => setModal('edit')}><Icon name="edit" size={15} />Edit</button>
        <button type="button" className="btn ghost danger-text" onClick={() => setModal('delete')}><Icon name="trash" size={15} />Delete</button>
      </div>
      <div className="home-grid">
        <Card title="Appliances and parts" flush actions={<button type="button" className="btn ghost sm" onClick={() => openCreate('thing')}><Icon name="plus" size={14} />Add</button>}>
          {!things ? <Loading /> : things.length === 0 ? <p className="muted pad">Nothing added yet.</p> : <div className="list">{things.map(t => <ThingRow key={t.id} t={t} />)}</div>}
        </Card>
        <div className="stack">
          <Card title="Notes">{h.notes ? <p className="notes">{h.notes}</p> : <p className="muted">Where the water and gas shut-offs are, who has a spare key… <button type="button" className="link-btn" onClick={() => setModal('edit')}>Add notes</button></p>}</Card>
          <Card title="Papers and links" sub="Deeds, insurance, the floor plan" flush actions={<button type="button" className="btn ghost sm" onClick={() => setModal('attach')}><Icon name="plus" size={14} />Add</button>}>
            {data.attachments.length === 0 ? <p className="muted pad">Nothing kept here yet.</p> : <div className="list">{data.attachments.map(a => (
              <div key={a.id} className="list-row">
                <span className="list-ico"><Icon name={a.kind === 'document' ? 'file' : 'link'} size={16} /></span>
                <span className="grow row-main">{a.open_url ? <a href={a.open_url} target="_blank" rel="noreferrer" className="row-title">{a.title}</a> : <span className="row-title">{a.title}</span>}</span>
                <button type="button" className="btn ghost sm icon-only" aria-label={`Remove ${a.title}`} onClick={async () => { await api.del(`${V1}/attachments/${a.id}`).catch(() => {}); reload() }}><Icon name="x" size={14} /></button>
              </div>))}</div>}
          </Card>
        </div>
      </div>
      {modal === 'edit' && <HomeModal home={h} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'attach' && <AttachModal owner="home" id={h.id} title={h.name} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'delete' && <Confirm title={`Delete ${h.name}?`} message="Everything in it goes: appliances, upkeep, the log and the meters. Bookings already in Pocket Money stay there. To keep the history, archive the home under Edit instead." confirmLabel="Delete everything"
        onCancel={() => setModal('')} onConfirm={async () => { setModal(''); try { await api.del(`${V1}/homes/${h.id}`); toast('Deleted'); announceChange(); navigate('/homes') } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}
