// Meters: the last reading of each; one meter with its usage per month and every reading.
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useData, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import { METER_ICON, fmtNum, money, useHomeFilter } from '../lib/home'
import type { Meter, Reading, UsageMonth, UsageSummary } from '../lib/types'
import { Card, Confirm, Empty, Loading, errMsg, useAsync, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { Bars, monthLabel } from '../components/Bars'
import { MeterModal, ReadingModal } from '../components/Forms'
import { openCreate, useCrumb } from '../components/Layout'
import { HomePicker } from './Overview'

export default function Meters() {
  const { homes } = useData()
  const [home, setHome] = useHomeFilter(homes)
  const { data, reload, error } = useAsync(() => api.get<Meter[]>(`${V1}/meters${home ? `?home_id=${home}` : ''}`), [home])
  useOnChange(reload)
  const [read, setRead] = useState<Meter | null>(null)
  const many = (homes || []).filter(h => !h.archived).length > 1
  return (
    <div className="stack loose">
      <div className="home-head">
        <div className="grow"><h1 className="home-title">Meters</h1><p className="muted">Electricity, water and gas: write down the numbers, see what you use each month.</p></div>
        <HomePicker value={home} onChange={setHome} />
        <button type="button" className="btn primary" onClick={() => openCreate('meter')}><Icon name="plus" size={15} />Add a meter</button>
      </div>
      {error && <div className="callout danger"><Icon name="alert" size={15} />{error}</div>}
      {!data ? <Loading /> : data.length === 0 ? <Empty title="No meters yet" icon="gauge">Add the electricity, water or gas meter and read it once a month.</Empty> : (
        <div className="meter-grid">
          {data.map(m => (
            <div key={m.id} className={`card meter-card ${m.archived ? 'is-archived' : ''}`}>
              <Link to={`/meters/${m.id}`} className="meter-top">
                <span className={`list-ico mk-${m.kind}`}><Icon name={METER_ICON[m.kind] || 'gauge'} size={16} /></span>
                <span className="grow"><b>{m.name}</b>{many && !home && <span className="muted small block-line">{m.home_name}</span>}</span>
                <Icon name="chevron-right" size={15} className="muted" />
              </Link>
              <div className="meter-val tnum">{m.last ? <>{fmtNum(m.last.value)} <small>{m.unit}</small></> : <span className="muted">No reading yet</span>}</div>
              <div className="muted small">{m.last ? `${fmtDate(m.last.read_on)}${m.last.usage != null ? ` · +${fmtNum(m.last.usage)} ${m.unit} in ${m.last.days} days` : ''}` : ' '}</div>
              {!m.archived && <button type="button" className="btn sm mt" onClick={() => setRead(m)}><Icon name="plus" size={14} />Reading</button>}
            </div>
          ))}
        </div>
      )}
      {read && <ReadingModal meter={read} onClose={() => setRead(null)} />}
    </div>
  )
}

interface MeterData { meter: Meter; readings: Reading[]; usage: UsageMonth[]; summary: UsageSummary }

export function MeterPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { data, reload, error } = useAsync(() => api.get<MeterData>(`${V1}/meters/${id}`), [id])
  useOnChange(reload)
  const [modal, setModal] = useState<'' | 'edit' | 'read' | 'delete'>('')
  const [editR, setEditR] = useState<Reading | null>(null)
  const [show, setShow] = useState<'usage' | 'cost'>('usage')
  useCrumb(data?.meter.name)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!data) return <Loading />
  const m = data.meter
  const s = data.summary
  const hasCost = data.usage.some(u => u.cost != null)
  const change = s.per_day && s.per_day_before ? (s.per_day / s.per_day_before - 1) * 100 : null
  return (
    <div className="stack loose">
      <div className="detail-head">
        <span className={`detail-ico mk-${m.kind}`}><Icon name={METER_ICON[m.kind] || 'gauge'} size={22} /></span>
        <div className="grow"><h1 className="home-title">{m.name}</h1><p className="muted">{[m.home_name, m.number && `No. ${m.number}`, m.price_per_unit != null && `${money(Math.round(m.price_per_unit * 10000) / 100, m.currency)} per ${m.unit}`].filter(Boolean).join(' · ')}</p></div>
      </div>
      <div className="row wrap toolbar">
        {!m.archived && <button type="button" className="btn primary" onClick={() => setModal('read')}><Icon name="plus" size={15} />New reading</button>}
        <span className="grow" />
        <button type="button" className="btn ghost" onClick={() => setModal('edit')}><Icon name="edit" size={15} />Edit</button>
        <button type="button" className="btn ghost danger-text" onClick={() => setModal('delete')}><Icon name="trash" size={15} />Delete</button>
      </div>
      <div className="stats-row">
        <div className="stat"><span className="stat-l">Last reading</span><span className="stat-v tnum">{m.last ? `${fmtNum(m.last.value)} ${m.unit}` : '—'}</span><span className="stat-s small">{m.last ? fmtDate(m.last.read_on) : ''}</span></div>
        <div className="stat"><span className="stat-l">Per day</span><span className="stat-v tnum">{s.per_day != null ? `${fmtNum(s.per_day, 2)} ${m.unit}` : '—'}</span><span className="stat-s small">over the last year</span></div>
        <div className="stat"><span className="stat-l">Per year</span><span className="stat-v tnum">{s.per_year != null ? `${fmtNum(s.per_year, 0)} ${m.unit}` : '—'}</span><span className="stat-s small">{s.cost_per_year != null ? `about ${money(Math.round(s.cost_per_year * 100), m.currency, true)}` : ''}</span></div>
        <div className="stat"><span className="stat-l">Against the year before</span><span className={`stat-v tnum ${change == null ? '' : change > 3 ? 'bad' : change < -3 ? 'ok' : ''}`}>{change == null ? '—' : `${change > 0 ? '+' : ''}${fmtNum(change, 0)} %`}</span><span className="stat-s small">use per day</span></div>
      </div>
      <Card title="Use per month" sub="Readings that span months are spread evenly over the days in between" actions={hasCost && <div className="segmented sm" role="radiogroup">
        {(['usage', 'cost'] as const).map(v => <button key={v} type="button" role="radio" aria-checked={show === v} className={show === v ? 'active' : ''} onClick={() => setShow(v)}>{v === 'usage' ? m.unit || 'Units' : m.currency}</button>)}</div>}>
        {data.readings.length < 2 ? <p className="muted">Two readings make the first month of use.</p> :
          <Bars ariaLabel={`${m.name} use per month`} fmt={v => show === 'cost' ? money(Math.round(v * 100), m.currency, true) : `${fmtNum(v, 0)}`}
            bars={data.usage.map(u => ({ key: u.month, label: monthLabel(u.month), value: show === 'cost' ? (u.cost || 0) : u.usage,
              tip: `${monthLabel(u.month, true)}: ${fmtNum(u.usage)} ${m.unit}${u.cost != null ? ` · ${money(Math.round(u.cost * 100), m.currency)}` : ''}$` }))} />}
      </Card>
      <Card title={<>Readings <span className="muted count">{data.readings.length}</span></>} flush>
        {data.readings.length === 0 ? <p className="muted pad">No readings yet.</p> : (
          <table className="table compact readings">
            <thead><tr><th>Date</th><th className="right">Reading</th><th className="right">Used</th><th className="hide-xs">Note</th><th /></tr></thead>
            <tbody>{data.readings.map(r => (
              <tr key={r.id}>
                <td>{fmtDate(r.read_on)}</td>
                <td className="right tnum">{fmtNum(r.value)} {m.unit}</td>
                <td className="right tnum muted">{r.reset ? 'new meter' : r.usage != null ? `+${fmtNum(r.usage)} in ${r.days} d` : ''}</td>
                <td className="muted hide-xs">{r.note}</td>
                <td className="right"><button type="button" className="btn ghost sm" onClick={() => setEditR(r)}>Edit</button></td>
              </tr>))}</tbody>
          </table>
        )}
      </Card>
      {m.notes && <Card title="Notes"><p className="notes">{m.notes}</p></Card>}
      {modal === 'edit' && <MeterModal meter={m} onClose={() => setModal('')} onSaved={reload} />}
      {modal === 'read' && <ReadingModal meter={m} onClose={() => setModal('')} onSaved={reload} />}
      {editR && <ReadingModal meter={m} reading={editR} onClose={() => setEditR(null)} onSaved={reload} />}
      {modal === 'delete' && <Confirm title={`Delete the ${m.name.toLowerCase()} meter?`} message="With all its readings." confirmLabel="Delete"
        onCancel={() => setModal('')} onConfirm={async () => { setModal(''); try { await api.del(`${V1}/meters/${m.id}`); toast('Deleted'); announceChange(); navigate('/meters') } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}
