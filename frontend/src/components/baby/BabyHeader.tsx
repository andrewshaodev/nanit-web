import { useState, useEffect } from 'react'
import { api } from '@/lib/api'
import { errorTooltipConfig } from '@/lib/tooltipSetup'
import type { Baby, StreamStatusResponse, HealthResponse } from '@/types/api'
import { displayName } from '@/lib/utils'
import { useTemperatureUnit } from '@/hooks/useTemperatureUnit'
import { Cctv, ChevronDown, ChevronRight, ChevronUp } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface BabyHeaderProps {
  baby: Baby
  collapsed: boolean
  onToggleCollapsed: () => void
  // Left undefined for the camera already at that end
  onMoveUp?: () => void
  onMoveDown?: () => void
}

// (aria-expanded: shadcn's ghost button highlights an open disclosure, which
// here is every expanded card)
const headerButton = 'aria-expanded:bg-transparent aria-expanded:hover:bg-accent'

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
    // A GitHub-style box header: a subtle row with the title and its actions
    <div className={`bg-ctp-mauve/10 px-2 py-1.5 ${collapsed ? '' : 'border-b border-ctp-mauve/25'}`}>
      <div className="flex justify-between items-center gap-3">
        <div className="flex items-center gap-1 min-w-0">
          <Button
            variant="ghost"
            size="icon-sm"
            className={headerButton}
            onClick={onToggleCollapsed}
            aria-expanded={!collapsed}
            aria-label={`${collapsed ? 'Expand' : 'Collapse'} ${displayName(baby)}`}
            title={collapsed ? 'Expand' : 'Collapse'}
          >
            {collapsed ? <ChevronRight /> : <ChevronDown />}
          </Button>
          <Cctv className="size-4 shrink-0 text-ctp-mauve" aria-hidden="true" />
          <h2 className="ml-1 font-semibold truncate">{displayName(baby)}</h2>
        </div>

        <div className="flex items-center gap-3 shrink-0">
          {/* A collapsed card still shows the readings at a glance */}
          {collapsed && (
            <div className="hidden sm:flex items-center gap-3 text-xs text-muted-foreground tabular-nums">
              <span>{formatTemperature(baby.temperature)}</span>
              {baby.humidity !== undefined && baby.humidity > 0 && <span>{baby.humidity.toFixed(0)}%</span>}
            </div>
          )}

          {/* Single Camera Status */}
          <div
            className="flex items-center gap-1.5 rounded-full border bg-background px-2 py-0.5 text-xs cursor-help"
            data-tooltip-id="app-tooltip"
            data-tooltip-content={cameraStatus.tooltip}
            data-tooltip-place={errorTooltipConfig.place}
            data-tooltip-delay-show={errorTooltipConfig.delayShow}
          >
            <div className={`size-2 rounded-full ${cameraStatus.color}`} />
            <span className="font-medium">{cameraStatus.text}</span>
          </div>

          {canReorder && (
            <div className="flex items-center">
              <Button
                variant="ghost"
                size="icon-sm"
                className={headerButton}
                onClick={onMoveUp}
                disabled={!onMoveUp}
                aria-label={`Move ${displayName(baby)} up`}
                title="Move up"
              >
                <ChevronUp />
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                className={headerButton}
                onClick={onMoveDown}
                disabled={!onMoveDown}
                aria-label={`Move ${displayName(baby)} down`}
                title="Move down"
              >
                <ChevronDown />
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}