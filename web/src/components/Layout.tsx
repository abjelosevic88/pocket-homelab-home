// The app shell: sidebar, top bar, search palette, shortcuts — the Pocket Calendar layout.
import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { Outlet, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { DataProvider, useData } from '../lib/data'
import type { Meter } from '../lib/types'
import { Icon } from './icons'
import { Sidebar } from './Sidebar'
import { CommandPalette, type PaletteCommand } from './CommandPalette'
import { ShortcutsSheet } from './ShortcutsSheet'
import { HomeModal, JobModal, MeterModal, ProModal, ReadingModal, RecordModal, ThingModal } from './Forms'
import { PlantModal } from './PlantForms'
import { Avatar, type Anchor, type MenuItem, Menu, MenuButton, modalOpen, rectOf, useIsMobile, useMediaQuery, useToast } from './ui'
import { AskButton } from './AskButton'

export { announceChange, useOnChange } from '../lib/data'

const SIDEBAR_KEY = 'phm-sidebar'

/** The page's own name for the top bar breadcrumb. */
const CrumbCtx = createContext<(t: string) => void>(() => {})
export function useCrumb(title: string | undefined) {
  const set = useContext(CrumbCtx)
  useEffect(() => { set(title || ''); return () => set('') }, [title, set])
}

export type CreateKind = 'record' | 'job' | 'thing' | 'plant' | 'reading' | 'meter' | 'pro' | 'home'

/** Open a create dialog from anywhere (a page's button). */
export const openCreate = (k: CreateKind) => window.dispatchEvent(new CustomEvent('phm:create', { detail: { kind: k } }))

/** The add menu: the sidebar's "Add" button and the phone's + button. */
export function createItems(onCreate: (k: CreateKind) => void): MenuItem[] {
  return [
    { label: 'Log work', icon: 'note', hint: 'N', onSelect: () => onCreate('record') },
    { label: 'Add upkeep', icon: 'wrench', hint: 'A', onSelect: () => onCreate('job') },
    { label: 'Read a meter', icon: 'gauge', hint: 'R', onSelect: () => onCreate('reading') },
    'sep',
    { label: 'An appliance or part', icon: 'package', onSelect: () => onCreate('thing') },
    { label: 'A plant', icon: 'leaf', onSelect: () => onCreate('plant') },
    { label: 'A pro', icon: 'user', onSelect: () => onCreate('pro') },
    { label: 'A meter', icon: 'zap', onSelect: () => onCreate('meter') },
    { label: 'A home', icon: 'home', onSelect: () => onCreate('home') },
  ]
}

export default function Layout() {
  return <DataProvider><Shell /></DataProvider>
}

function Shell() {
  const navigate = useNavigate()
  const loc = useLocation()
  const [sp, setSp] = useSearchParams()
  const { toast } = useToast()
  const { homes } = useData()
  const mobile = useIsMobile()
  const narrow = useMediaQuery('(max-width: 1099px)')
  const drawer = mobile || narrow
  const [sidebarOpen, setSidebarOpen] = useState(() => !drawer && localStorage.getItem(SIDEBAR_KEY) !== '0')
  useEffect(() => { setSidebarOpen(!drawer && localStorage.getItem(SIDEBAR_KEY) !== '0') }, [drawer])
  const drawerOpenedAt = useRef(0)
  const toggleSidebar = () => setSidebarOpen(o => { drawerOpenedAt.current = performance.now(); const n = !o; if (!drawer) localStorage.setItem(SIDEBAR_KEY, n ? '1' : '0'); return n })
  const closeDrawer = () => { if (drawer) setSidebarOpen(false) }
  useEffect(() => { document.querySelector('.content')?.scrollTo({ top: 0 }) }, [loc.pathname])

  const [crumb, setCrumbRaw] = useState('')
  const setCrumb = useCallback((t: string) => setCrumbRaw(t), [])

  const [palette, setPalette] = useState(false)
  const [shortcuts, setShortcuts] = useState(false)
  const [modal, setModal] = useState<{ kind: CreateKind; meter?: Meter } | null>(null)
  const [fabMenu, setFabMenu] = useState<Anchor | null>(null)

  const create = async (k: CreateKind) => {
    closeDrawer()
    if ((k === 'record' || k === 'job' || k === 'thing' || k === 'plant' || k === 'reading' || k === 'meter') && homes && homes.length === 0) {
      toast('Add your home first'); setModal({ kind: 'home' }); return
    }
    if (k === 'reading') {
      const ms = (await api.get<Meter[]>(`${V1}/meters`).catch(() => [] as Meter[])).filter(m => !m.archived)
      if (ms.length === 1) { setModal({ kind: 'reading', meter: ms[0] }); return }
      if (ms.length === 0) { toast('Add a meter first'); setModal({ kind: 'meter' }); return }
      navigate('/meters'); return
    }
    setModal({ kind: k })
  }
  const createRef = useRef(create)
  createRef.current = create

  // /?add=record (the PWA shortcut) opens a dialog
  useEffect(() => { const a = sp.get('add'); if (a) { createRef.current((a === '1' ? 'record' : a) as CreateKind); setSp(new URLSearchParams(), { replace: true }) } }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const open = () => setShortcuts(true)
    const make = (e: Event) => { const d = (e as CustomEvent<{ kind: CreateKind }>).detail; createRef.current(d.kind) }
    window.addEventListener('phm:shortcuts', open)
    window.addEventListener('phm:create', make)
    return () => { window.removeEventListener('phm:shortcuts', open); window.removeEventListener('phm:create', make) }
  }, [])
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); setPalette(p => !p); return }
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (t.closest('input, textarea, select, [contenteditable="true"]') || modalOpen() || document.querySelector('.menu, .popover, .sel-pop')) return
      const go = (to: string) => { e.preventDefault(); navigate(to) }
      const mk = (k: CreateKind) => { e.preventDefault(); createRef.current(k) }
      switch (e.key) {
        case '/': e.preventDefault(); setPalette(true); break
        case '?': e.preventDefault(); setShortcuts(true); break
        case 'n': mk('record'); break
        case 'a': mk('job'); break
        case 'r': mk('reading'); break
        case 'h': go('/'); break
        case 'u': go('/upkeep'); break
        case 't': go('/things'); break
        case 'g': go('/plants'); break
        case 'l': go('/log'); break
        case 'm': go('/meters'); break
        case 'p': go('/pros'); break
        case ',': go('/settings'); break
        case '[': e.preventDefault(); toggleSidebar(); break
      }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }) // eslint-disable-line react-hooks/exhaustive-deps

  const commands: PaletteCommand[] = [
    { id: 'record', label: 'Log work', icon: 'note', hint: 'N', keywords: 'repair add record cost', run: () => create('record') },
    { id: 'job', label: 'Add upkeep', icon: 'wrench', hint: 'A', keywords: 'maintenance recurring reminder task', run: () => create('job') },
    { id: 'reading', label: 'Read a meter', icon: 'gauge', hint: 'R', keywords: 'electricity water gas reading', run: () => create('reading') },
    { id: 'thing', label: 'Add an appliance or part', icon: 'package', keywords: 'boiler fridge new thing', run: () => create('thing') },
    { id: 'plant', label: 'Add a plant', icon: 'leaf', keywords: 'house plant flower herb', run: () => create('plant') },
    { id: 'pro', label: 'Add a pro', icon: 'user', keywords: 'plumber electrician contractor', run: () => create('pro') },
    { id: 'home', label: 'Overview', icon: 'home', hint: 'H', run: () => navigate('/') },
    { id: 'upkeep', label: 'Upkeep', icon: 'wrench', hint: 'U', keywords: 'due overdue maintenance', run: () => navigate('/upkeep') },
    { id: 'things', label: 'Appliances and parts', icon: 'package', hint: 'T', keywords: 'things warranty manuals', run: () => navigate('/things') },
    { id: 'plants', label: 'Plants', icon: 'leaf', hint: 'G', keywords: 'house plants water watering green', run: () => navigate('/plants') },
    { id: 'log', label: 'Log', icon: 'note', hint: 'L', keywords: 'history repairs costs spending', run: () => navigate('/log') },
    { id: 'meters', label: 'Meters', icon: 'gauge', hint: 'M', keywords: 'electricity water gas usage', run: () => navigate('/meters') },
    { id: 'pros', label: 'Pros', icon: 'users', hint: 'P', keywords: 'plumber electrician people', run: () => navigate('/pros') },
    { id: 'homes', label: 'My homes', icon: 'home', run: () => navigate('/homes') },
    { id: 'settings', label: 'Settings', icon: 'settings', hint: ',', run: () => navigate('/settings') },
    { id: 'apps', label: 'Settings: connected apps', icon: 'layers', keywords: 'money calendar tasks contacts documents inventory core', run: () => navigate('/settings/apps') },
    { id: 'notify', label: 'Settings: notifications', icon: 'bell', keywords: 'reminders push', run: () => navigate('/settings/notifications') },
    { id: 'shortcuts', label: 'Keyboard shortcuts', icon: 'keyboard', hint: '?', run: () => setShortcuts(true) },
  ]

  const close = () => setModal(null)
  return (
    <CrumbCtx.Provider value={setCrumb}>
      <div className={`app ${sidebarOpen ? 'with-sidebar' : 'no-sidebar'} ${mobile ? 'is-mobile' : ''} ${drawer ? 'drawer-mode' : ''}`}>
        {drawer && sidebarOpen && <div className="drawer-backdrop" onClick={e => { if (e.timeStamp - drawerOpenedAt.current > 400) setSidebarOpen(false) }} />}
        <div className={`sidebar-wrap ${sidebarOpen ? 'open' : ''}`}>
          <Sidebar onCreate={create} onNavigate={closeDrawer} />
        </div>
        <div className="main">
          <TopBar onMenu={toggleSidebar} onPalette={() => setPalette(true)} mobile={mobile} compact={narrow} sidebarOpen={sidebarOpen} crumb={crumb} />
          <main className="content"><div className="content-inner"><Outlet /></div></main>
        </div>
        {mobile && <button type="button" className="fab" aria-label="Add" title="Add" aria-haspopup="menu" aria-expanded={!!fabMenu} onClick={e => setFabMenu(fabMenu ? null : rectOf(e.currentTarget))}><Icon name="plus" size={24} strokeWidth={2.2} /></button>}
        {fabMenu && <Menu anchor={fabMenu} align="right" items={createItems(create)} onClose={() => setFabMenu(null)} width={250} />}
        {palette && <CommandPalette onClose={() => setPalette(false)} commands={commands} />}
        {shortcuts && <ShortcutsSheet onClose={() => setShortcuts(false)} />}
        {modal?.kind === 'record' && <RecordModal onClose={close} onSaved={r => navigate(`/log/${r.id}`)} />}
        {modal?.kind === 'job' && <JobModal onClose={close} onSaved={j => navigate(`/upkeep/${j.id}`)} />}
        {modal?.kind === 'thing' && <ThingModal onClose={close} onSaved={t => navigate(`/things/${t.id}`)} />}
        {modal?.kind === 'plant' && <PlantModal onClose={close} onSaved={p => navigate(`/plants/${p.id}`)} />}
        {modal?.kind === 'meter' && <MeterModal onClose={close} onSaved={m => navigate(`/meters/${m.id}`)} />}
        {modal?.kind === 'reading' && modal.meter && <ReadingModal meter={modal.meter} onClose={close} />}
        {modal?.kind === 'pro' && <ProModal onClose={close} onSaved={p => navigate(`/pros/${p.id}`)} />}
        {modal?.kind === 'home' && <HomeModal onClose={close} onSaved={h => navigate(`/homes/${h.id}`)} />}
      </div>
    </CrumbCtx.Provider>
  )
}

const SECTION: Record<string, [string, string]> = { '': ['Overview', '/'], upkeep: ['Upkeep', '/upkeep'], things: ['Appliances and parts', '/things'], plants: ['Plants', '/plants'], log: ['Log', '/log'], meters: ['Meters', '/meters'], pros: ['Pros', '/pros'], homes: ['My homes', '/homes'] }

function TopBar({ onMenu, onPalette, mobile, compact, sidebarOpen, crumb }: { onMenu: () => void; onPalette: () => void; mobile: boolean; compact: boolean; sidebarOpen: boolean; crumb: string }) {
  const { user, themeMode, setThemeMode, status, setUser } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [section, sectionTo] = SECTION[loc.pathname.split('/')[1] || ''] || ['', '/']
  const signOut = async () => { await api.post(`${V1}/auth/logout`).catch(() => {}); setUser(null); navigate('/login') }
  const avatarMenu = (): MenuItem[] => [
    { heading: user?.name || user?.email || '' },
    { label: 'Profile', icon: 'user', onSelect: () => navigate('/settings/profile') },
    { label: 'Settings', icon: 'settings', onSelect: () => navigate('/settings'), hint: ',' },
    { label: 'Keyboard shortcuts', icon: 'keyboard', onSelect: () => window.dispatchEvent(new CustomEvent('phm:shortcuts')), hint: '?' },
    'sep',
    { heading: 'Appearance' },
    { label: 'System', checked: themeMode === 'system', onSelect: () => setThemeMode('system') },
    { label: 'Light', checked: themeMode === 'light', onSelect: () => setThemeMode('light') },
    { label: 'Dark', checked: themeMode === 'dark', onSelect: () => setThemeMode('dark') },
    ...(status?.auth_mode === 'local' ? ['sep' as const, { label: 'Sign out', icon: 'log-out', onSelect: signOut }] : []),
  ]
  return (
    <header className="topbar">
      <button type="button" className="btn ghost icon-only" onClick={onMenu} aria-label={sidebarOpen ? 'Hide the sidebar' : 'Show the sidebar'} title="Menu ([)"><Icon name={mobile || compact ? 'menu' : 'sidebar'} size={18} /></button>
      <h1 className="range-title">
        {crumb ? <><button type="button" className="crumb" onClick={() => navigate(sectionTo)}>{section}</button><Icon name="chevron-right" size={16} className="crumb-sep" /><span className="ellipsis">{crumb}</span></> : section}
      </h1>
      <span className="spacer" />
      <button type="button" className="btn ghost icon-only" onClick={onPalette} aria-label="Search" title="Search (⌘K or /)"><Icon name="search" size={18} /></button>
      <AskButton />
      <MenuButton label="Account" className="avatar-btn" align="right" width={220} items={avatarMenu}>
        <Avatar name={user?.name} email={user?.email} size={30} />
      </MenuButton>
    </header>
  )
}
