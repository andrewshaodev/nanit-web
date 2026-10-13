import { useEffect, useState } from 'react'
import { Droplets, Lightbulb, LightbulbOff, Loader2, Moon, Sun, Thermometer, type LucideIcon } from 'lucide-react'
import { useTemperatureUnit } from '@/hooks/useTemperatureUnit'
import { api } from '@/lib/api'
import { sensorTooltipConfig } from '@/lib/tooltipSetup'
import type { Baby } from '@/types/api'

interface SensorRowProps {
  icon: LucideIcon
  iconClass: string
  title: string
  value: string
  tooltip?: string
  onClick?: () => void
  disabled?: boolean
}

// One reading as a GitHub-style box row: icon, label, value on the right
function SensorRow({ icon: Icon, iconClass, title, value, tooltip, onClick, disabled }: SensorRowProps) {
  const Row = onClick ? 'button' : 'div'
  return (
    <Row
      {...(onClick ? { type: 'button' as const, onClick, disabled } : {})}
      className={`flex w-full items-center gap-2.5 px-3 py-2 text-left ${onClick ? 'enabled:hover:bg-accent enabled:cursor-pointer disabled:opacity-60' : ''}`}
      data-tooltip-id={tooltip ? 'app-tooltip' : undefined}
      data-tooltip-content={tooltip}
      data-tooltip-place={sensorTooltipConfig.place}
      data-tooltip-delay-show={sensorTooltipConfig.delayShow}
    >
      <Icon className={`size-4 shrink-0 ${iconClass}`} aria-hidden="true" />
      <span className="text-muted-foreground">{title}</span>
      <span className="ml-auto font-semibold tabular-nums">{value}</span>
    </Row>
  )
}

const formatHumidity = (humidity: number | undefined): string =>
  humidity === undefined || humidity === null || humidity <= 0 ? '--%' : `${humidity.toFixed(1)}%`

const formatNightMode = (isNight: boolean | null | undefined): string =>
  isNight === undefined || isNight === null ? '--' : isNight ? 'Night' : 'Day'

const formatNightLight = (nightLight: boolean | undefined): string =>
  nightLight === undefined || nightLight === null ? '--' : nightLight ? 'On' : 'Off'

// How long to show "Turning on…" before giving up on the camera reporting it
const NIGHT_LIGHT_WAIT_MS = 15000

// The night light row is its switch: a click asks the camera to turn it the
// other way, and the row says so until the camera reports the change
function useNightLight(baby: Baby) {
  const [target, setTarget] = useState<boolean | null>(null)
  const [failed, setFailed] = useState(false)

  // Done once the camera reports it, or after a while if it never does
  const pending = target !== null && baby.night_light !== target
  useEffect(() => {
    if (target === null) return
    const timer = setTimeout(() => setTarget(null), pending ? NIGHT_LIGHT_WAIT_MS : 0)
    return () => clearTimeout(timer)
  }, [target, pending])

  useEffect(() => {
    if (!failed) return
    const timer = setTimeout(() => setFailed(false), 3000)
    return () => clearTimeout(timer)
  }, [failed])

  const toggle = async () => {
    setFailed(false)
    setTarget(!baby.night_light)
    try {
      await api.toggleNightLight(baby.uid)
    } catch (error) {
      console.error('Night light toggle failed:', error)
      setTarget(null)
      setFailed(true)
    }
  }

  const value = failed ? 'Failed' : !pending ? formatNightLight(baby.night_light) : target ? 'Turning on…' : 'Turning off…'
  return { toggle, pending, value }
}

interface SensorGridProps {
  baby: Baby
}

export default function SensorGrid({ baby }: SensorGridProps) {
  const { formatTemperature, toggleUnit } = useTemperatureUnit()
  const nightLight = useNightLight(baby)

  // Safety check: if baby object is completely undefined
  if (!baby) {
    console.error('SensorGrid: baby object is undefined!')
    return (
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="text-ctp-red dark:text-ctp-red text-center col-span-full">
          Error: Baby data not available
        </div>
      </div>
    )
  }


  return (
    <div className="divide-y rounded-md border bg-card">
      <SensorRow
        icon={Thermometer}
        iconClass="text-ctp-peach-900 dark:text-ctp-peach"
        title="Temperature"
        value={formatTemperature(baby.temperature)}
        onClick={toggleUnit}
        tooltip="Click to toggle °C/°F"
      />
      <SensorRow
        icon={Droplets}
        iconClass="text-ctp-teal-900 dark:text-ctp-teal"
        title="Humidity"
        value={formatHumidity(baby.humidity)}
      />
      <SensorRow
        icon={baby.is_night ? Moon : Sun}
        iconClass={baby.is_night ? 'text-ctp-mauve' : 'text-ctp-yellow-900 dark:text-ctp-yellow'}
        title="Camera mode"
        value={formatNightMode(baby.is_night)}
        tooltip="Whether the camera is in day mode or has switched to night vision"
      />
      <SensorRow
        icon={nightLight.pending ? Loader2 : baby.night_light ? Lightbulb : LightbulbOff}
        iconClass={`${nightLight.pending ? 'animate-spin ' : ''}${baby.night_light ? 'text-ctp-sky-900 dark:text-ctp-sky' : 'text-muted-foreground'}`}
        title="Night light"
        value={nightLight.value}
        onClick={nightLight.toggle}
        disabled={!baby.websocket_alive || nightLight.pending}
        tooltip={baby.websocket_alive ? `Click to turn the night light ${baby.night_light ? 'off' : 'on'}` : 'The camera is offline'}
      />
    </div>
  )
}
