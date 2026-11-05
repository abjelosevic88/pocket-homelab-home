// Left column: the add button, sections with what needs doing, your homes — the Pocket Calendar sidebar.
import { useState } from 'react'
import { Link, NavLink } from 'react-router-dom'
import { useApp } from '../lib/app-context'
import { useData } from '../lib/data'
import { HOME_KINDS } from '../lib/home'
import { Icon } from './icons'
import { createItems, type CreateKind } from './Layout'
import { type Anchor, Menu, rectOf } from './ui'

export function Sidebar({ onCreate, onNavigate }: { onCreate: (k: CreateKind) => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const { homes, things, plants } = useData()
  const [menu, setMenu] = useState<Anchor | null>(null)
  const title = settings?.title || status?.title || 'Pocket Home'
  const live = (homes || []).filter(h => !h.archived)
  const overdue = live.reduce((n, h) => n + h.overdue, 0)
  const soon = live.reduce((n, h) => n + h.due_soon, 0)
  const thirsty = plants.filter(p => p.status === 'due' || p.status === 'overdue').length
  const late = plants.some(p => p.status === 'overdue')
  const nav: { to: string; label: string; icon: string; end?: boolean; count?: string; warn?: boolean }[] = [
    { to: '/', label: 'Overview', icon: 'home', end: true },
    { to: '/upkeep', label: 'Upkeep', icon: 'wrench', count: overdue ? String(overdue) : soon ? String(soon) : undefined, warn: overdue > 0 },
    { to: '/things', label: 'Appliances', icon: 'package', count: things.length ? String(things.length) : undefined },
    { to: '/plants', label: 'Plants', icon: 'leaf', count: thirsty ? String(thirsty) : undefined, warn: late },
    { to: '/log', label: 'Log', icon: 'note' },
    { to: '/meters', label: 'Meters', icon: 'gauge' },
    { to: '/pros', label: 'Pros', icon: 'users' },
  ]
  return (
    <aside className="sidebar" aria-label="Pocket Home">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Home' ? title : <>Pocket <b>Home</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <button type="button" className="btn primary create-btn" aria-haspopup="menu" aria-expanded={!!menu} onClick={e => setMenu(menu ? null : rectOf(e.currentTarget))}>
        <Icon name="plus" size={15} strokeWidth={2.2} />Add<Icon name="chevron-down" size={15} className="create-caret" />
      </button>
      {menu && <Menu anchor={menu} items={createItems(onCreate)} onClose={() => setMenu(null)} width={248} />}

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(n => (
          <NavLink key={n.to} to={n.to} end={n.end} onClick={onNavigate} className={({ isActive }) => `sb-nav-item ${isActive ? 'active' : ''}`}>
            <Icon name={n.icon} size={16} />{n.label}
            {!!n.count && <span className={`sb-count ${n.warn ? 'warn' : ''}`} title={n.warn ? 'Overdue' : undefined}>{n.count}</span>}
          </NavLink>
        ))}
      </nav>

      <section className="sb-sec">
        <h4 className="sb-h"><Link to="/homes" className="sb-h-btn" onClick={onNavigate}>My homes</Link></h4>
        {homes && live.length === 0 && <button type="button" className="sb-row sb-empty" onClick={() => onCreate('home')}><Icon name="plus" size={14} />Add your home</button>}
        {live.map(h => (
          <NavLink key={h.id} to={`/homes/${h.id}`} onClick={onNavigate} className={({ isActive }) => `sb-row sb-home ${isActive ? 'active' : ''}`}>
            <Icon name={HOME_KINDS.find(k => k.value === h.kind)?.icon || 'home'} size={15} className="sb-home-ico" />
            <span className="grow ellipsis">{h.name}</span>
            {h.overdue > 0 ? <span className="sb-dot bad" title={`${h.overdue} overdue`}>{h.overdue}</span> : h.due_soon > 0 ? <span className="sb-dot warn" title={`${h.due_soon} due soon`}>{h.due_soon}</span> : null}
          </NavLink>
        ))}
      </section>

      <div className="sb-foot">
        <Link to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</Link>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('phm:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}
