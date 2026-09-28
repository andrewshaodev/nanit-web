import { useState } from 'react'
import { Baby } from 'lucide-react'
import { Link, useLocation } from 'react-router'
import ThemeToggle from '@/components/layout/ThemeToggle'
import useSWR from 'swr'
import { api } from '@/lib/api'
import { errorTooltipConfig } from '@/lib/tooltipSetup'
import type { StatusResponse } from '@/types/api'

interface ConnectionStatusProps {
  isConnected: boolean
  lastUpdate?: Date
}

function ConnectionStatus({ isConnected, lastUpdate }: ConnectionStatusProps) {
  const tooltip = isConnected
    ? 'Dashboard is connected to the Nanit bridge API. Data is being refreshed every 5 seconds.'
    : 'Dashboard cannot reach the Nanit bridge API. Check that the service is running and accessible.'

  // A GitHub-style label: outlined, with a status dot, and the last refresh
  return (
    <div
      className="flex items-center gap-2 rounded-full border px-2.5 py-0.5 text-xs text-muted-foreground cursor-help"
      data-tooltip-id="app-tooltip"
      data-tooltip-content={tooltip}
      data-tooltip-place={errorTooltipConfig.place}
      data-tooltip-delay-show={errorTooltipConfig.delayShow}
    >
      <span className={`status-dot inline-block ${isConnected ? 'online' : 'offline'}`} />
      <span className="font-medium text-foreground">{isConnected ? 'Connected' : 'Disconnected'}</span>
      {lastUpdate && <span className="hidden sm:inline tabular-nums">{lastUpdate.toLocaleTimeString()}</span>}
    </div>
  )
}

// GitHub-style underline tabs: the active one is marked in mauve along the
// header's bottom edge
function NavLink({ to, active, children }: { to: string; active: boolean; children: React.ReactNode }) {
  return (
    <Link
      to={to}
      aria-current={active ? 'page' : undefined}
      className={`relative flex h-12 items-center px-1 text-sm transition-colors ${
        active ? 'font-semibold text-foreground' : 'text-muted-foreground hover:text-foreground'
      }`}
    >
      <span className="rounded-md px-2 py-1 hover:bg-accent">{children}</span>
      {active && <span className="absolute inset-x-1 bottom-0 h-0.5 rounded-full bg-ctp-mauve" aria-hidden="true" />}
    </Link>
  )
}

export default function Header() {
  const [lastUpdate, setLastUpdate] = useState<Date>()
  // Links may arrive as /settings/ (Next's old trailing-slash URLs)
  const pathname = useLocation().pathname.replace(/\/+$/, '') || '/'
  
  const { data, error, isLoading } = useSWR<StatusResponse>(
    '/status',
    () => api.getStatus(),
    {
      refreshInterval: 5000,
      onSuccess: () => setLastUpdate(new Date())
    }
  )

  const isConnected = !error && !isLoading && !!data

  return (
    <header className="border-b bg-ctp-crust">
      <div className="mx-auto flex h-12 max-w-[1280px] items-center justify-between gap-4 px-4 md:px-6">
        <div className="flex items-center gap-4">
          <Link to="/" className="flex items-center gap-2 font-semibold text-foreground">
            <Baby className="size-5 text-ctp-mauve" />
            <span>Nanit</span>
          </Link>

          <nav className="flex items-center self-stretch">
            <NavLink to="/" active={pathname === '/'}>Dashboard</NavLink>
            <NavLink to="/settings" active={pathname === '/settings'}>Settings</NavLink>
          </nav>
        </div>

        <div className="flex items-center gap-2">
          <ConnectionStatus isConnected={isConnected} lastUpdate={lastUpdate} />
          <ThemeToggle />
        </div>
      </div>
    </header>
  )
}
