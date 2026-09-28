import { useState, useEffect } from 'react'
import { api } from '@/lib/api'
import { errorTooltipConfig } from '@/lib/tooltipSetup'
import type { Baby, StreamStatusResponse, HealthResponse } from '@/types/api'
import { displayName } from '@/lib/utils'
import { useTemperatureUnit } from '@/hooks/useTemperatureUnit'

interface BabyHeaderProps {
  baby: Baby
  collapsed: boolean
  onToggleCollapsed: () => void
  // Left undefined for the camera already at that end
  onMoveUp?: () => void
  onMoveDown?: () => void
}

const headerButton =
  'p-2 rounded-full hover:bg-ctp-base/20 focus-visible:outline-2 focus-visible:outline-ctp-base disabled:opacity-30 disabled:hover:bg-transparent transition-colors'

function Chevron({ direction }: { direction: 'up' | 'down' | 'right' }) {
  const rotate = { up: 'rotate-180', down: '', right: '-rotate-90' }[direction]
  return (
    <svg className={`w-5 h-5 transition-transform ${rotate}`} viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
      <path fillRule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.17l3.71-3.94a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clipRule="evenodd" />
    </svg>
  )
}

export default function BabyHeader({ baby, collapsed, onToggleCollapsed, onMoveUp, onMoveDown }: BabyHeaderProps) {
  const { formatTemperature } = useTemperatureUnit()
  const canReorder = onMoveUp !== undefined || onMoveDown !== undefined
  const [streamStatus, setStreamStatus] = useState<StreamStatusResponse | null>(null)
  const [health, setHealth] = useState<HealthResponse | null>(null)
  
  useEffect(() => {
    let pollInterval: NodeJS.Timeout
    
    const pollHealth = async () => {
      try {
        const healthStatus = await api.getHealth(baby.uid)
        setHealth(healthStatus)
      } catch (error) {
        // Health status not available
        setHealth(null)
      }
    }
    
    const pollStreamStatus = async () => {
      try {
        const status = await api.getStreamStatus(baby.uid)
        setStreamStatus(status)
      } catch (error) {
        // Stream status not available
        setStreamStatus(null)
      }
    }
    
    // Always poll health, regardless of websocket status
    pollHealth() // Initial poll
    pollInterval = setInterval(pollHealth, 15000) // Poll every 15 seconds
    
    // Also poll stream status if websocket is alive for backward compatibility
    if (baby.websocket_alive) {
      pollStreamStatus()
    }
    
    return () => {
      if (pollInterval) {
        clearInterval(pollInterval)
      }
    }
  }, [baby.uid, baby.websocket_alive])
  
  const getCameraStatusInfo = () => {
    if (!health) {
      return { text: 'Checking...', color: 'bg-ctp-overlay1', tooltip: 'Loading camera status...' }
    }
    
    const details = health.details
    const isStreaming = streamStatus?.status === 'running'
    
    // Determine primary status based on health and streaming state
    switch (health.overall_health) {
      case 'healthy':
        if (isStreaming) {
          return { 
            text: 'Online & Streaming', 
            color: 'bg-ctp-green-900 dark:bg-ctp-green', 
            tooltip: 'Camera online and actively streaming video'
          }
        }
        return { 
          text: 'Camera Online', 
          color: 'bg-ctp-green-900 dark:bg-ctp-green', 
          tooltip: 'Camera connected and ready to stream'
        }
      case 'degraded':
        return { 
          text: 'Camera Issues', 
          color: 'bg-ctp-yellow-900 dark:bg-ctp-yellow', 
          tooltip: 'Camera connected but has warnings. Click Settings → Devices for details.'
        }
      case 'starting':
        return { 
          text: 'Camera Starting', 
          color: 'bg-ctp-blue-700 dark:bg-ctp-blue', 
          tooltip: 'Camera initializing, please wait...'
        }
      case 'unhealthy':
      default:
        if (!baby.websocket_alive) {
          return { 
            text: 'Camera Offline', 
            color: 'bg-ctp-red dark:bg-ctp-red', 
            tooltip: 'Camera disconnected from Nanit servers'
          }
        }
        return { 
          text: 'Camera Error', 
          color: 'bg-ctp-red dark:bg-ctp-red', 
          tooltip: 'Camera has critical issues. Check Settings → Devices for troubleshooting.'
        }
    }
  }

  const cameraStatus = getCameraStatusInfo()
  
  return (
    <div className={`bg-gradient-nanit text-ctp-base ${collapsed ? 'px-6 py-3' : 'p-6'}`}>
      <div className="flex justify-between items-center gap-4">
        <div className="flex items-center gap-2 min-w-0">
          <button
            type="button"
            className={headerButton}
            onClick={onToggleCollapsed}
            aria-expanded={!collapsed}
            aria-label={`${collapsed ? 'Expand' : 'Collapse'} ${displayName(baby)}`}
            title={collapsed ? 'Expand' : 'Collapse'}
          >
            <Chevron direction={collapsed ? 'right' : 'down'} />
          </button>
          <h2 className={`font-bold truncate ${collapsed ? 'text-xl' : 'text-2xl'}`}>{displayName(baby)}</h2>
        </div>

        <div className="flex items-center gap-3 shrink-0">
          {/* A collapsed card still shows the readings at a glance */}
          {collapsed && (
            <div className="hidden sm:flex items-center gap-3 text-sm text-ctp-base/90">
              <span>{formatTemperature(baby.temperature)}</span>
              {baby.humidity !== undefined && baby.humidity > 0 && <span>{baby.humidity.toFixed(0)}%</span>}
            </div>
          )}

          {/* Single Camera Status */}
          <div 
            className="flex items-center gap-2 bg-ctp-base/20 px-4 py-2 rounded-full text-sm cursor-help"
            data-tooltip-id="app-tooltip"
            data-tooltip-content={cameraStatus.tooltip}
            data-tooltip-place={errorTooltipConfig.place}
            data-tooltip-delay-show={errorTooltipConfig.delayShow}
          >
            <div className={`w-2.5 h-2.5 rounded-full ${cameraStatus.color}`} />
            <span className="font-medium">{cameraStatus.text}</span>
          </div>

          {canReorder && (
            <div className="flex items-center">
              <button
                type="button"
                className={headerButton}
                onClick={onMoveUp}
                disabled={!onMoveUp}
                aria-label={`Move ${displayName(baby)} up`}
                title="Move up"
              >
                <Chevron direction="up" />
              </button>
              <button
                type="button"
                className={headerButton}
                onClick={onMoveDown}
                disabled={!onMoveDown}
                aria-label={`Move ${displayName(baby)} down`}
                title="Move down"
              >
                <Chevron direction="down" />
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}