// ⌘K / "/" — find an appliance, upkeep, a log entry or a pro, jump to a page, or run a command from the keyboard.
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { HomeRecord, Job, Meter, Pro, Thing } from '../lib/types'
import { useDebounce } from './ui'
import { Icon } from './icons'

export interface PaletteCommand { id: string; label: string; icon: string; hint?: string; keywords?: string; run: () => void }

interface Found { things: Thing[]; jobs: Job[]; records: HomeRecord[]; pros: Pro[]; meters: Meter[] }

export function CommandPalette({ onClose, commands }: { onClose: () => void; commands: PaletteCommand[] }) {
  const navigate = useNavigate()
  const [q, setQ] = useState('')
  const [hi, setHi] = useState(0)
  const list = useRef<HTMLDivElement>(null)
  const dq = useDebounce(q.trim(), 200)
  const [found, setFound] = useState<Found | null>(null)
  useEffect(() => {
    if (dq.length < 2) { setFound(null); return }
    let live = true
    api.get<Found>(`${V1}/search?q=${encodeURIComponent(dq)}&limit=6`).then(f => { if (live) setFound(f) }).catch(() => {})
    return () => { live = false }
  }, [dq])

  type Row = { key: string; group: string; icon: React.ReactNode; label: React.ReactNode; sub?: React.ReactNode; hint?: string; run: () => void }
  const rows = useMemo<Row[]>(() => {
    const out: Row[] = []
    const n = q.trim().toLowerCase()
    const words = n.split(/\s+/).filter(Boolean)
    const hit = (text: string) => words.every(w => text.toLowerCase().includes(w))
    const cmds = commands.filter(c => !n || hit(`${c.label} ${c.keywords || ''}`)).map(c => ({ key: c.id, group: 'Commands', icon: <Icon name={c.icon} size={16} />, label: c.label, hint: c.hint, run: c.run }))
    const f = n.length >= 2 ? found : null
    const things = (f?.things || []).map(t => ({ key: `t${t.id}`, group: 'Appliances and parts', icon: <Icon name="package" size={16} />, label: t.name, sub: [t.room, t.brand, t.model].filter(Boolean).join(' · '), run: () => navigate(`/things/${t.id}`) }))
    const jobs = (f?.jobs || []).map(j => ({ key: `j${j.id}`, group: 'Upkeep', icon: <Icon name="wrench" size={16} />, label: j.title, sub: [j.thing_name, j.due_on].filter(Boolean).join(' · '), run: () => navigate(`/upkeep/${j.id}`) }))
    const recs = (f?.records || []).slice(0, 6).map(r => ({ key: `r${r.id}`, group: 'Log', icon: <Icon name="note" size={16} />, label: r.title, sub: [r.date, r.thing_name].filter(Boolean).join(' · '), run: () => navigate(`/log/${r.id}`) }))
    const pros = (f?.pros || []).map(x => ({ key: `p${x.id}`, group: 'Pros', icon: <Icon name="user" size={16} />, label: x.name, sub: x.trade, run: () => navigate(`/pros/${x.id}`) }))
    const meters = (f?.meters || []).map(m => ({ key: `m${m.id}`, group: 'Meters', icon: <Icon name="gauge" size={16} />, label: m.name, sub: m.home_name, run: () => navigate(`/meters/${m.id}`) }))
    if (n) out.push(...things, ...jobs, ...recs, ...pros, ...meters, ...cmds)
    else out.push(...cmds)
    return out
  }, [q, found, commands, navigate])
  useEffect(() => setHi(0), [q])
  useEffect(() => { list.current?.querySelector(`[data-i="${hi}"]`)?.scrollIntoView({ block: 'nearest' }) }, [hi])

  const run = (r: Row | undefined) => { if (!r) return; onClose(); r.run() }
  let last = ''
  return createPortal(
    <div className="palette-backdrop" onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}>
      <div className="palette-box" role="dialog" aria-label="Search and commands">
        <div className="pal-input">
          <Icon name="search" size={18} />
          <input autoFocus value={q} onChange={e => setQ(e.target.value)} placeholder="Find an appliance, upkeep, a log entry, or run a command…" aria-label="Search"
            onKeyDown={e => {
              if (e.key === 'ArrowDown') { e.preventDefault(); setHi(h => Math.min(rows.length - 1, h + 1)) }
              else if (e.key === 'ArrowUp') { e.preventDefault(); setHi(h => Math.max(0, h - 1)) }
              else if (e.key === 'Enter') { e.preventDefault(); run(rows[hi]) }
              else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); onClose() }
            }} />
          <kbd>esc</kbd>
        </div>
        <div className="pal-list" ref={list} role="listbox">
          {rows.length === 0 && <div className="pal-empty">Nothing matches.</div>}
          {rows.map((r, i) => {
            const head = r.group !== last ? <div className="pal-group" key={'g' + r.group}>{r.group}</div> : null
            last = r.group
            return (
              <React.Fragment key={r.key}>
                {head}
                <div data-i={i} role="option" aria-selected={i === hi} className={`pal-row ${i === hi ? 'hi' : ''}`} onMouseEnter={() => setHi(i)} onClick={() => run(r)}>
                  <span className="pal-ico">{r.icon}</span>
                  <span className="grow"><span className="pal-label">{r.label}</span>{r.sub && <span className="pal-sub">{r.sub}</span>}</span>
                  {r.hint && <kbd>{r.hint}</kbd>}
                  {i === hi && <Icon name="chevron-right" size={14} className="muted" />}
                </div>
              </React.Fragment>
            )
          })}
        </div>
      </div>
    </div>, document.body)
}
