import { Droplets, Lightbulb, LightbulbOff, Moon, Sun, Thermometer, type LucideIcon } from 'lucide-react'
import { useTemperatureUnit } from '@/hooks/useTemperatureUnit'
import { sensorTooltipConfig } from '@/lib/tooltipSetup'
import type { Baby } from '@/types/api'

interface SensorRowProps {
  icon: LucideIcon
  iconClass: string
  title: string
  value: string
  tooltip?: string
  onClick?: () => void
}

// One reading as a GitHub-style box row: icon, label, value on the right
function SensorRow({ icon: Icon, iconClass, title, value, tooltip, onClick }: SensorRowProps) {
  const Row = onClick ? 'button' : 'div'
  return (
    <Row
      {...(onClick ? { type: 'button' as const, onClick } : {})}
      className={`flex w-full items-center gap-2.5 px-3 py-2 text-left ${onClick ? 'hover:bg-accent cursor-pointer' : ''}`}
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

const formatNightMode = (isNight: boolean | undefined): string =>
  isNight === undefined || isNight === null ? '--' : isNight ? 'Night' : 'Day'

const formatNightLight = (nightLight: boolean | undefined): string =>
  nightLight === undefined || nightLight === null ? '--' : nightLight ? 'On' : 'Off'

interface SensorGridProps {
  baby: Baby
}

export default function SensorGrid({ baby }: SensorGridProps) {
  const { formatTemperature, toggleUnit } = useTemperatureUnit()

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
        icon={baby.night_light ? Lightbulb : LightbulbOff}
        iconClass={baby.night_light ? 'text-ctp-sky-900 dark:text-ctp-sky' : 'text-muted-foreground'}
        title="Night light"
        value={formatNightLight(baby.night_light)}
      />
    </div>
  )
}
