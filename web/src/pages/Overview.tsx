// Overview: what needs doing now, what is coming, what was done lately, what it cost, warranties and meters.
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useData, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import { METER_ICON, fmtNum, money, useHomeFilter } from '../lib/home'
import type { Overview as OverviewT } from '../lib/types'
import { Card, Empty, Loading, useAsync } from '../components/ui'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { JobRow, RecordRow } from '../components/Rows'
import { openCreate } from '../components/Layout'

export function HomePicker({ value, onChange }: { value: number; onChange: (id: number) => void }) {
  const { homes } = useData()
  const live = (homes || []).filter(h => !h.archived)
  if (live.length < 2) return null
  return <div className="home-pick"><Select ariaLabel="Home" value={String(value)} onChange={v => onChange(Number(v))} options={[{ value: '0', label: 'All homes' }, ...live.map(h => ({ value: String(h.id), label: h.name }))]} /></div>
}

export default function Overview() {
  const { user } = useApp()
  const { homes } = useData()
  const [home, setHome] = useHomeFilter(homes)
  const { data: o, reload, error } = useAsync(() => api.get<OverviewT>(`${V1}/overview${home ? `?home_id=${home}` : ''}`), [home])
  useOnChange(reload)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!o) return <Loading />
  const first = (user?.name || '').split(' ')[0]
  if (o.homes.length === 0) return (
    <Card className="welcome">
      <div className="welcome-body">
        <img src="/favicon.svg" alt="" width={56} height={56} />
        <div className="grow">
          <h2>Welcome to Pocket Home</h2>
          <p className="muted">Start with your home, then add the things that need looking after — the boiler, the roof, the washing machine. Pocket Home suggests their upkeep, reminds you before it is due and keeps a log of what was done and what it cost.</p>
          <div className="row wrap mt"><button type="button" className="btn primary" onClick={() => openCreate('home')}><Icon name="plus" size={15} />Add your home</button></div>
        </div>
      </div>
    </Card>
  )
  const n = o.counts.overdue + o.counts.due
  const cur = o.spent_year[0]?.currency || o.homes[0]?.currency || 'EUR'
  const spent = o.spent_year.filter(r => r.currency === cur).reduce((s, r) => s + r.total, 0)
  const before = o.spent_last_year_to_date.filter(r => r.currency === cur).reduce((s, r) => s + r.total, 0)
  return (
    <div className="stack loose">
      <div className="home-head">
        <div className="grow">
          <h1 className="home-title">{first && first !== 'Admin' ? `Hello, ${first}` : 'Your home'}</h1>
          <p className="muted">{n > 0 ? `${n} ${n === 1 ? 'thing needs' : 'things need'} doing${o.counts.soon ? `, ${o.counts.soon} more soon` : ''}.` : o.counts.soon ? `Nothing due today; ${o.counts.soon} coming up soon.` : 'Nothing needs doing right now.'}</p>
        </div>
        <HomePicker value={home} onChange={setHome} />
      </div>

      <div className="tiles">
        <Link to="/upkeep" className={`tile ${o.counts.overdue ? 'tile-bad' : ''}`}>
          <div className="tile-head"><span className="tile-ico"><Icon name="alert" size={14} /></span>Overdue</div>
          <div className="tile-val">{o.counts.overdue}</div>
        </Link>
        <Link to="/upkeep" className="tile">
          <div className="tile-head"><span className="tile-ico"><Icon name="clock" size={14} /></span>Due today or soon</div>
          <div className="tile-val">{o.counts.due + o.counts.soon}</div>
        </Link>
        <Link to="/log" className="tile">
          <div className="tile-head"><span className="tile-ico"><Icon name="wallet" size={14} /></span>Spent this year</div>
          <div className="tile-val">{money(spent, cur, true)}</div>
          <div className="muted small">{before ? `${money(before, cur, true)} by this time last year` : 'nothing logged last year'}</div>
        </Link>
        <Link to="/things" className="tile">
          <div className="tile-head"><span className="tile-ico"><Icon name="package" size={14} /></span>Appliances and parts</div>
          <div className="tile-val">{o.things}</div>
        </Link>
      </div>

      <div className="home-grid">
        <Card title="Needs doing" flush actions={<Link to="/upkeep" className="btn ghost sm">All upkeep</Link>}>
          {o.attention.length === 0 ? <p className="muted pad">All done. Nothing is overdue or due in the next days.</p>
            : <div className="list">{o.attention.map(j => <JobRow key={j.id} job={j} showHome={!home && o.homes.length > 1} onChanged={reload} />)}</div>}
        </Card>
        <Card title="Coming up" sub="The next two months" flush>
          {o.upcoming.length === 0 ? <p className="muted pad">Nothing planned in the next two months.</p>
            : <div className="list">{o.upcoming.map(j => <JobRow key={j.id} job={j} showHome={!home && o.homes.length > 1} onChanged={reload} />)}</div>}
        </Card>
      </div>

      <div className="home-grid">
        <Card title="Recently done" flush actions={<button type="button" className="btn ghost sm" onClick={() => openCreate('record')}><Icon name="plus" size={14} />Log work</button>}>
          {o.recent.length === 0 ? <Empty title="Nothing logged yet" icon="note">Repairs, services and improvements, with what they cost.</Empty>
            : <div className="list">{o.recent.map(r => <RecordRow key={r.id} r={r} showHome={!home && o.homes.length > 1} />)}</div>}
        </Card>
        <div className="stack">
          {o.meters.length > 0 && <Card title="Meters" flush actions={<Link to="/meters" className="btn ghost sm">Usage</Link>}>
            <div className="list">{o.meters.map(m => (
              <Link key={m.id} to={`/meters/${m.id}`} className="list-row">
                <span className={`list-ico mk-${m.kind}`}><Icon name={METER_ICON[m.kind] || 'gauge'} size={16} /></span>
                <span className="grow row-main"><span className="row-title">{m.name}{!home && o.homes.length > 1 ? <span className="muted"> · {m.home_name}</span> : null}</span>
                  <span className="muted small">{m.last ? `${fmtNum(m.last.value)} ${m.unit} on ${fmtDate(m.last.read_on, false)}` : 'No readings yet'}</span></span>
                {m.needs_reading && <span className="pill st-soon"><Icon name="clock" size={12} />Read it</span>}
              </Link>))}</div>
          </Card>}
          {o.warranties.length > 0 && <Card title="Warranties" flush>
            <div className="list">{o.warranties.map(w => (
              <Link key={w.thing_id} to={`/things/${w.thing_id}`} className="list-row">
                <span className="list-ico"><Icon name="shield" size={16} /></span>
                <span className="grow row-main"><span className="row-title">{w.name}</span><span className="muted small">{w.state === 'ended' ? `Ended on ${fmtDate(w.until)}` : `Ends on ${fmtDate(w.until)}`}</span></span>
                <span className={`pill ${w.state === 'ended' ? 'st-paused' : 'st-soon'}`}>{w.state === 'ended' ? 'Ended' : `${w.days} days left`}</span>
              </Link>))}</div>
          </Card>}
        </div>
      </div>
    </div>
  )
}
