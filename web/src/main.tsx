import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import './styles/base.css'
import './styles/booking.css'
import './styles/app.css'
import './styles/settings.css'
import { AppProvider, useApp } from './lib/app-context'
import { Loading, ToastProvider } from './components/ui'
import Layout from './components/Layout'
import { ErrorBoundary } from './components/ErrorBoundary'
import Login from './pages/Login'
import Bookings from './pages/Bookings'
import Types, { NewType } from './pages/Types'
import TypeEditor from './pages/TypeEditor'
import Hours from './pages/Hours'
import Links from './pages/Links'
import Insights from './pages/Insights'
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
      <Route element={<Gate><ErrorBoundary where="booking"><Layout /></ErrorBoundary></Gate>}>
        <Route path="/" element={<Bookings />} />
        <Route path="/bookings" element={<Bookings />} />
        <Route path="/bookings/:id" element={<Bookings />} />
        <Route path="/types" element={<Types />} />
        <Route path="/types/new" element={<NewType />} />
        <Route path="/types/:id" element={<TypeEditor />} />
        <Route path="/hours" element={<Hours />} />
        <Route path="/links" element={<Links />} />
        <Route path="/insights" element={<Insights />} />
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
