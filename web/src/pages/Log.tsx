// The log: what was done on the house and what it cost, month by month; one entry with its booking and papers.
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import { RECORD_KINDS, RECORD_KIND_ICON, RECORD_KIND_NAME, money, useHomeFilter } from '../lib/home'
import type { Attachment, HomeRecord, MoneyResult, SpendRow } from '../lib/types'
import { Card, Confirm, Empty, Loading, errMsg, useAsync, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Bars, monthLabel } from '../components/Bars'
import { AttachModal, RecordModal } from '../components/Forms'
import { RecordRow } from '../components/Rows'
import { openCreate, useCrumb } from '../components/Layout'
import { HomePicker } from './Overview'

interface Spending { from: string; to: string; months: string[]; by_month: SpendRow[]; by_kind: SpendRow[]; by_thing: SpendRow[]; by_year: SpendRow[] }

export default function Log() {
  const { homes } = useData()
  const [home, setHome] = useHomeFilter(homes)
  const [q, setQ] = useState('')
  const [kind, setKind] = useState('')
  const [limit, setLimit] = useState(50)
  const [plantsToo, setPlantsToo] = useState(() => localStorage.getItem('phm-log-plants') === '1')
  const scope = kind || plantsToo ? '' : 'house'
  const { data, reload, error } = useAsync(() => api.get<{ records: HomeRecord[]; total: number }>(`${V1}/records?${new URLSearchParams({ home_id: String(home), q, kind, scope, limit: String(limit) })}`), [home, q, kind, scope, limit])
  const { data: sp, reload: reloadSp } = useAsync(() => api.get<Spending>(`${V1}/spending?months=12${home ? `&home_id=${home}` : ''}`), [home])
  useOnChange(() => { reload(); reloadSp() })
  const many = (homes || []).filter(h => !h.archived).length > 1
  const cur = sp?.by_year[sp.by_year.length - 1]?.currency || homes?.[0]?.currency || 'EUR'
  const total = (rows: SpendRow[] | undefined, key?: string) => (rows || []).filter(r => r.currency === cur && (key === undefined || r.key === key)).reduce((s, r) => s + r.total, 0)
  const year = new Date().getFullYear()
  const kinds = (sp?.by_kind || []).filter(r => r.currency === cur && r.total > 0).sort((a, b) => b.total - a.total)
  const kmax = Math.max(1, ...kinds.map(k => k.total))
  return (
    <div className="stack loose">
      <div className="home-head">
        <div className="grow"><h1 className="home-title">Log</h1><p className="muted">Repairs, services, improvements and purchases, with who did them and what they cost.</p></div>
        <HomePicker value={home} onChange={setHome} />
        <button type="button" className="btn primary" onClick={() => openCreate('record')}><Icon name="plus" size={15} />Log work</button>
      </div>
      {sp && <div className="home-grid">
        <Card title="Spent per month" sub={`The last 12 months · ${money(total(sp.by_month), cur, true)} · ${year}: ${money(total(sp.by_year, String(year)), cur, true)}, ${year - 1}: ${money(total(sp.by_year, String(year - 1)), cur, true)}`}>
          <Bars ariaLabel="Money spent on the house per month" fmt={v => money(v, cur, true)}
            bars={sp.months.map(m => { const v = total(sp.by_month, m); return { key: m, label: monthLabel(m), value: v, tip: `${monthLabel(m, true)}: ${money(v, cur)}` } })} />
        </Card>
        <Card title="By kind" sub="The last 12 months">
          {kinds.length === 0 ? <p className="muted">No costs logged.</p> : <div className="kind-bars">{kinds.map(k => (
            <button key={k.key} type="button" className={`kind-bar ${kind === k.key ? 'active' : ''}`} onClick={() => setKind(kind === k.key ? '' : k.key)}>
              <span className="kb-label"><Icon name={RECORD_KIND_ICON[k.key] || 'note'} size={14} />{RECORD_KIND_NAME[k.key] || k.key}</span>
              <span className="kb-track"><span style={{ width: `${(k.total / kmax) * 100}%` }} /></span>
              <span className="tnum kb-val">{money(k.total, cur, true)}</span>
            </button>))}</div>}
        </Card>
      </div>}
      <div className="row wrap filters">
        <input className="search-in" value={q} onChange={e => setQ(e.target.value)} placeholder="Find in the log…" aria-label="Find in the log" />
        <Select ariaLabel="Kind" value={kind} onChange={setKind} clearLabel="Every kind" placeholder="Every kind" options={RECORD_KINDS.map(k => ({ value: k.value, label: k.label }))} />
        {!kind && <label className="check-row"><input type="checkbox" checked={plantsToo} onChange={e => { setPlantsToo(e.target.checked); localStorage.setItem('phm-log-plants', e.target.checked ? '1' : '') }} />Plant care too</label>}
      </div>
      {error && <div className="callout danger"><Icon name="alert" size={15} />{error}</div>}
      {!data ? <Loading /> : data.records.length === 0 ? <Empty title={q || kind ? 'Nothing matches' : 'Nothing logged yet'} icon="note" /> : (
        <Card flush title={<>Entries <span className="muted count">{data.total}</span></>}>
          <div className="list">{data.records.map(r => <RecordRow key={r.id} r={r} showHome={many && !home} />)}</div>
          {data.total > data.records.length && <div className="pad"><button type="button" className="btn" onClick={() => setLimit(l => l + 100)}>Show more</button></div>}
        </Card>
      )}
    </div>
  )
}

interface RecordData { record: HomeRecord; attachments: Attachment[]; money_url: string }

export function RecordPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { settings } = useApp()
  const { toast } = useToast()
  const { data, reload, error } = useAsync(() => api.get<RecordData>(`${V1}/records/${id}`), [id])
  useOnChange(reload)
  const [modal, setModal] = useState<'' | 'edit' | 'attach' | 'delete'>('')
  const [booking, setBooking] = useState(false)
  useCrumb(data?.record.title)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!data) return <Loading />
  const r = data.record
  const book = async () => {
    setBooking(true)
    try { const m = await api.post<MoneyResult>(`${V1}/records/${r.id}/money`); if (m.ok) toast(`Booked in Pocket Money${m.account ? ` on ${m.account}` : ''}`); else toast(m.error || 'Pocket Money did not answer', 'err'); reload() } catch (e) { toast(errMsg(e), 'err') } finally { setBooking(false) }
  }
  const moneyOn = settings && settings.money_mode !== 'off'
  return (
    <div className="stack loose">
      <div className="detail-head">
        <span className={`detail-ico rk-${r.kind}`}><Icon name={RECORD_KIND_ICON[r.kind] || 'note'} size={22} /></span>
        <div className="grow"><h1 className="home-title">{r.title}</h1><p className="muted">{fmtDate(r.date)} · {RECORD_KIND_NAME[r.kind] || r.kind} · {r.home_name}</p></div>
        {r.cost > 0 && <span className="big-num tnum">{money(r.cost, r.currency)}</span>}
      </div>
      <div className="row wrap toolbar">
        <button type="button" className="btn" onClick={() => setModal('edit')}><Icon name="edit" size={15} />Edit</button>
        <button type="button" className="btn" onClick={() => setModal('attach')}><Icon name="file" size={15} />Keep a paper</button>
        <span className="grow" />
        <button type="button" className="btn ghost danger-text" onClick={() => setModal('delete')}><Icon name="trash" size={15} />Delete</button>
      </div>
      <div className="home-grid">
        <Card title="Details">
          <dl className="set-dl">
            {r.thing_name && <div><dt>Appliance or part</dt><dd><Link to={`/things/${r.thing_id}`}>{r.thing_name}</Link></dd></div>}
            {r.plant_name && <div><dt>Plant</dt><dd><Link to={`/plants/${r.plant_id}`}>{r.plant_name}</Link></dd></div>}
            {r.job_title && <div><dt>{r.plant_id ? 'Care' : 'Upkeep'}</dt><dd><Link to={`/upkeep/${r.job_id}`}>{r.job_title}</Link></dd></div>}
            {r.pro_name && <div><dt>Who did it</dt><dd><Link to={`/pros/${r.pro_id}`}>{r.pro_name}</Link></dd></div>}
            <div><dt>Cost</dt><dd>{r.cost ? money(r.cost, r.currency) : 'Nothing'}</dd></div>
            {moneyOn && r.cost > 0 && <div><dt>Pocket Money</dt><dd>{r.money_tx_id ? (data.money_url ? <a href={data.money_url} target="_blank" rel="noreferrer">Booked <Icon name="external" size={12} /></a> : 'Booked')
              : <button type="button" className="btn sm" onClick={book} disabled={booking || settings?.money_via === ''} title={settings?.money_via === '' ? settings?.money_detail : ''}><Icon name="wallet" size={14} />{booking ? 'Booking…' : 'Book it in Pocket Money'}</button>}</dd></div>}
            {r.source !== 'web' && r.source !== 'demo' && <div><dt>Added by</dt><dd>{r.source === 'api' ? 'The API' : r.source === 'import' ? 'An import' : `Pocket ${r.source[0].toUpperCase()}${r.source.slice(1)}`}</dd></div>}
          </dl>
          {r.notes && <p className="notes">{r.notes}</p>}
        </Card>
        <Card title="Papers and links" flush>
          {data.attachments.length === 0 ? <p className="muted pad">Keep the invoice or a photo of the work: from Pocket Documents, or a web link.</p>
            : <div className="list">{data.attachments.map(a => (
              <div key={a.id} className="list-row">
                <span className="list-ico"><Icon name={a.kind === 'document' ? 'file' : 'link'} size={16} /></span>
                <span className="grow row-main">{a.open_url ? <a href={a.open_url} target="_blank" rel="noreferrer" className="row-title">{a.title}</a> : <span className="row-title">{a.title}</span>}</span>
                <button type="button" className="btn ghost sm icon-only" aria-label={`Remove ${a.title}`} onClick={async () => { await api.del(`${V1}/attachments/${a.id}`).catch(() => {}); reload() }}><Icon name="x" size={14} /></button>
              </div>))}</div>}
        </Card>
      </div>
      {modal === 'edit' && <RecordModal record={r} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'attach' && <AttachModal owner="record" id={r.id} title={r.title} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'delete' && <Confirm title="Delete this entry?" message={r.money_tx_id ? 'Its transaction in Pocket Money is removed too.' : 'It is removed from the log.'} confirmLabel="Delete"
        onCancel={() => setModal('')} onConfirm={async () => { setModal(''); try { await api.del(`${V1}/records/${r.id}`); toast('Deleted'); announceChange(); navigate('/log') } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}
