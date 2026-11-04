// Pros: the people who work on the house, with what they did and what it cost.
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import { money } from '../lib/home'
import type { HomeRecord, Pro } from '../lib/types'
import { Avatar, Card, Confirm, Empty, Loading, errMsg, useAsync, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { ProModal, Stars } from '../components/Forms'
import { RecordRow } from '../components/Rows'
import { openCreate, useCrumb } from '../components/Layout'

export default function Pros() {
  const [q, setQ] = useState('')
  const { data, reload, error } = useAsync(() => api.get<Pro[]>(`${V1}/pros?q=${encodeURIComponent(q)}`), [q])
  useOnChange(reload)
  const cur = 'EUR'
  return (
    <div className="stack loose">
      <div className="home-head">
        <div className="grow"><h1 className="home-title">Pros</h1><p className="muted">Plumbers, electricians, the chimney sweep: who to call, and how it went last time.</p></div>
        <button type="button" className="btn primary" onClick={() => openCreate('pro')}><Icon name="plus" size={15} />Add a pro</button>
      </div>
      <div className="row wrap filters"><input className="search-in" value={q} onChange={e => setQ(e.target.value)} placeholder="Find by name or trade…" aria-label="Find a pro" /></div>
      {error && <div className="callout danger"><Icon name="alert" size={15} />{error}</div>}
      {!data ? <Loading /> : data.length === 0 ? <Empty title={q ? 'Nobody matches' : 'No pros yet'} icon="users">Add the people you call when something breaks, or pick them from Pocket Contacts.</Empty> : (
        <Card flush>
          <div className="list">{data.map(p => (
            <Link key={p.id} to={`/pros/${p.id}`} className="list-row">
              <Avatar name={p.name} size={32} />
              <span className="grow row-main"><span className="row-title">{p.name}</span><span className="muted small ellipsis">{[p.trade, p.company !== p.name && p.company, p.jobs ? `${p.jobs} ${p.jobs === 1 ? 'job' : 'jobs'}, last ${fmtDate(p.last_work, false)}` : ''].filter(Boolean).join(' · ')}</span></span>
              {p.rating > 0 && <Stars value={p.rating} />}
              <span className="tnum row-amount hide-xs">{p.spent ? money(p.spent, cur, true) : ''}</span>
            </Link>))}</div>
        </Card>
      )}
    </div>
  )
}

export function ProPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { data, reload, error } = useAsync(() => api.get<{ pro: Pro; records: HomeRecord[]; contact_url: string }>(`${V1}/pros/${id}`), [id])
  useOnChange(reload)
  const [modal, setModal] = useState<'' | 'edit' | 'delete'>('')
  useCrumb(data?.pro.name)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!data) return <Loading />
  const p = data.pro
  const cur = data.records[0]?.currency || 'EUR'
  return (
    <div className="stack loose">
      <div className="detail-head">
        <Avatar name={p.name} size={46} />
        <div className="grow"><h1 className="home-title">{p.name}</h1><p className="muted">{[p.trade, p.company !== p.name && p.company].filter(Boolean).join(' · ')}</p></div>
        {p.rating > 0 && <Stars value={p.rating} />}
      </div>
      <div className="row wrap toolbar">
        {p.phone && <a className="btn primary" href={`tel:${p.phone.replace(/\s/g, '')}`}><Icon name="phone" size={15} />Call</a>}
        {p.email && <a className="btn" href={`mailto:${p.email}`}><Icon name="mail" size={15} />Email</a>}
        {p.website && <a className="btn" href={p.website} target="_blank" rel="noreferrer"><Icon name="globe" size={15} />Website</a>}
        {data.contact_url && <a className="btn" href={data.contact_url} target="_blank" rel="noreferrer"><Icon name="users" size={15} />In Pocket Contacts</a>}
        <span className="grow" />
        <button type="button" className="btn ghost" onClick={() => setModal('edit')}><Icon name="edit" size={15} />Edit</button>
        <button type="button" className="btn ghost danger-text" onClick={() => setModal('delete')}><Icon name="trash" size={15} />Delete</button>
      </div>
      <div className="home-grid">
        <Card title="Work done" sub={data.records.length ? `${data.records.length} entries · ${money(p.spent, cur)} in all` : undefined} flush>
          {data.records.length === 0 ? <p className="muted pad">Nothing logged with {p.name} yet.</p> : <div className="list">{data.records.map(r => <RecordRow key={r.id} r={r} />)}</div>}
        </Card>
        <Card title="Details">
          <dl className="set-dl">
            {p.phone && <div><dt>Phone</dt><dd><a href={`tel:${p.phone.replace(/\s/g, '')}`}>{p.phone}</a></dd></div>}
            {p.email && <div><dt>Email</dt><dd><a href={`mailto:${p.email}`}>{p.email}</a></dd></div>}
            {p.website && <div><dt>Website</dt><dd><a href={p.website} target="_blank" rel="noreferrer">{p.website.replace(/^https?:\/\//, '')}</a></dd></div>}
          </dl>
          {p.notes && <p className="notes">{p.notes}</p>}
        </Card>
      </div>
      {modal === 'edit' && <ProModal pro={p} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'delete' && <Confirm title={`Delete ${p.name}?`} message="The work they did stays in the log." confirmLabel="Delete"
        onCancel={() => setModal('')} onConfirm={async () => { setModal(''); try { await api.del(`${V1}/pros/${p.id}`); toast('Deleted'); announceChange(); navigate('/pros') } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}
