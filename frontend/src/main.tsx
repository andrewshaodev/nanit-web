import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import '@fontsource-variable/inter'
import './index.css'
import ClientTooltip from '@/components/ui/ClientTooltip'
import Dashboard from '@/pages/Dashboard'
import SettingsPage from '@/pages/SettingsPage'
import SetupPage from '@/pages/Setup'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/setup" element={<SetupPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ClientTooltip />
    </BrowserRouter>
  </StrictMode>,
)
