import { useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router'
import useSWR from 'swr'
import { api } from '@/lib/api'
import { errorTooltipConfig } from '@/lib/tooltipSetup'
import type { StatusResponse } from '@/types/api'

interface ConnectionStatusProps {
  isConnected: boolean
  lastUpdate?: Date
}

function ConnectionStatus({ isConnected, lastUpdate }: ConnectionStatusProps) {
  const statusClass = isConnected ? 'online' : 'offline'
  const statusText = isConnected ? 'Connected' : 'Disconnected'
  
  const getTooltipText = () => {
    if (isConnected) {
      return 'Dashboard is connected to the Nanit bridge API. Data is being refreshed every 5 seconds.'
    } else {
      return 'Dashboard cannot reach the Nanit bridge API. Check that the service is running and accessible.'
    }
  }
  
  return (
    <div 
      className="flex items-center gap-2 px-4 py-2 bg-ctp-base/20 rounded-full text-sm cursor-help"
      data-tooltip-id="app-tooltip"
      data-tooltip-content={getTooltipText()}
      data-tooltip-place={errorTooltipConfig.place}
      data-tooltip-delay-show={errorTooltipConfig.delayShow}
    >
      <div className={`status-dot ${statusClass}`} />
      <span className="text-ctp-base">{statusText}</span>
    </div>
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
    <header className="bg-gradient-nanit shadow-lg">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between items-center py-5">
          <div className="flex items-center gap-8">
            <Link to="/">
              <h1 className="text-3xl font-bold text-ctp-base hover:text-ctp-base/90 transition-colors">
                Nanit Dashboard
              </h1>
            </Link>
            
            <nav className="flex gap-6">
              <Link 
                to="/"
                className={`text-ctp-base/90 hover:text-ctp-base transition-colors ${
                  pathname === '/' ? 'font-semibold text-ctp-base' : ''
                }`}
              >
                Dashboard
              </Link>
              <Link 
                to="/settings"
                className={`text-ctp-base/90 hover:text-ctp-base transition-colors ${
                  pathname === '/settings' ? 'font-semibold text-ctp-base' : ''
                }`}
              >
                Settings
              </Link>
            </nav>
          </div>
          
          <div className="flex items-center gap-4">
            <ConnectionStatus 
              isConnected={isConnected} 
              lastUpdate={lastUpdate} 
            />
            
            {lastUpdate && (
              <div className="text-ctp-base/80 text-sm">
                Last updated: {lastUpdate.toLocaleTimeString()}
              </div>
            )}
          </div>
        </div>
      </div>
    </header>
  )
}