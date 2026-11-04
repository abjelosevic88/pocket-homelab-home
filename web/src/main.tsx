import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import './styles/base.css'
import './styles/home.css'
import './styles/app.css'
import './styles/settings.css'
import { AppProvider, useApp } from './lib/app-context'
import { Loading, ToastProvider } from './components/ui'
import Layout from './components/Layout'
import { ErrorBoundary } from './components/ErrorBoundary'
import Login from './pages/Login'
import Overview from './pages/Overview'
import Upkeep, { JobPage } from './pages/Upkeep'
import Things, { ThingPage } from './pages/Things'
import Log, { RecordPage } from './pages/Log'
import Meters, { MeterPage } from './pages/Meters'
import Pros, { ProPage } from './pages/Pros'
import Homes, { HomePage } from './pages/Homes'
import Settings from './pages/Settings'

function Gate({ children }: { children: React.ReactNode }) {
  const { status, user } = useApp()
  const loc = useLocation()
  if (!status) return <div className="boot"><img src="/favicon.svg" alt="" /><Loading /></div>
  if (status.needs_setup) return <Navigate to="/setup" replace />
  if (!user) return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />
  return <>{children}</>
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login mode="login" />} />
      <Route path="/setup" element={<Login mode="setup" />} />
      <Route path="/settings/:tab?" element={<Gate><ErrorBoundary where="settings"><Settings /></ErrorBoundary></Gate>} />
      <Route element={<Gate><ErrorBoundary where="home"><Layout /></ErrorBoundary></Gate>}>
        <Route path="/" element={<Overview />} />
        <Route path="/upkeep" element={<Upkeep />} />
        <Route path="/upkeep/:id" element={<JobPage />} />
        <Route path="/things" element={<Things />} />
        <Route path="/things/:id" element={<ThingPage />} />
        <Route path="/log" element={<Log />} />
        <Route path="/log/:id" element={<RecordPage />} />
        <Route path="/meters" element={<Meters />} />
        <Route path="/meters/:id" element={<MeterPage />} />
        <Route path="/pros" element={<Pros />} />
        <Route path="/pros/:id" element={<ProPage />} />
        <Route path="/homes" element={<Homes />} />
        <Route path="/homes/:id" element={<HomePage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ErrorBoundary where="app"><ToastProvider><AppProvider><App /></AppProvider></ToastProvider></ErrorBoundary>
    </BrowserRouter>
  </React.StrictMode>,
)
