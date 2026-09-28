import { timelineTooltipConfig } from '@/lib/tooltipSetup'
import type { DayNightAnalytics, DayNightPeriod } from '@/types/api'

interface DayNightChartProps {
  analytics: DayNightAnalytics | null
  isLoading?: boolean
}

// The camera's day/night mode over the selected window: day when it sees
// enough light, night when it has switched to night vision
const MODES = {
  day: { label: 'Day', fill: 'bg-linear-to-r from-ctp-yellow to-ctp-peach' },
  night: { label: 'Night', fill: 'bg-linear-to-r from-ctp-lavender to-ctp-mauve' },
  unknown: {
    label: 'No data',
    // Striped, so "not recorded" can't be mistaken for a mode
    fill: 'bg-ctp-surface0 bg-[repeating-linear-gradient(135deg,transparent_0_6px,var(--color-ctp-surface1)_6px_12px)]',
  },
} as const

// Windows longer than a day need the date as well as the time to make sense
function timeFormatter(start: number, end: number) {
  const multiDay = end - start > 20 * 60 * 60
  const options: Intl.DateTimeFormatOptions = multiDay
    ? { weekday: 'short', hour: 'numeric', minute: '2-digit' }
    : { hour: 'numeric', minute: '2-digit' }
  return (timestamp: number) => new Date(timestamp * 1000).toLocaleString(undefined, options)
}

function formatDuration(minutes: number): string {
  const hours = Math.floor(minutes / 60)
  const mins = Math.round(minutes % 60)

  if (hours === 0) return `${mins} min`
  if (mins === 0) return `${hours}h`
  return `${hours}h ${mins}m`
}

function tooltip(period: DayNightPeriod, format: (t: number) => string): string {
  return `${MODES[period.mode].label}
Start: ${format(period.start)}
End: ${format(period.end)}
Duration: ${formatDuration((period.end - period.start) / 60)}`
}

export default function DayNightChart({ analytics, isLoading }: DayNightChartProps) {
  if (isLoading) {
    return (
      <div className="h-16 bg-muted rounded-md flex items-center justify-center">
        <div className="text-muted-foreground text-sm">Loading day/night data...</div>
      </div>
    )
  }

  const periods = analytics?.periods ?? []
  if (!analytics || periods.length === 0) {
    return (
      <div className="h-16 bg-muted rounded-md flex items-center justify-center">
        <div className="text-muted-foreground text-sm">No day/night pattern data available</div>
      </div>
    )
  }

  const format = timeFormatter(analytics.start_time, analytics.end_time)
  const midpoint = Math.round((analytics.start_time + analytics.end_time) / 2)
  const legend = [
    { mode: 'day' as const, minutes: analytics.day_mode_minutes },
    { mode: 'night' as const, minutes: analytics.night_mode_minutes },
    { mode: 'unknown' as const, minutes: analytics.unknown_mode_minutes },
  ].filter(({ minutes }) => minutes > 0)

  return (
    <div className="space-y-3">
      {/* Timeline: each period's width is its share of the window */}
      <div className="flex h-8 rounded-md border overflow-hidden">
        {periods.map((period) => (
          <div
            key={period.start}
            className={`h-full min-w-0.5 transition-[filter] hover:brightness-110 ${MODES[period.mode].fill}`}
            style={{ flex: `${period.end - period.start} 0 0` }}
            data-tooltip-id="app-tooltip"
            data-tooltip-content={tooltip(period, format)}
            data-tooltip-place={timelineTooltipConfig.place}
            data-tooltip-delay-show={timelineTooltipConfig.delayShow}
          />
        ))}
      </div>

      {/* Time axis */}
      <div className="flex justify-between text-xs text-muted-foreground px-1">
        <span>{format(analytics.start_time)}</span>
        <span>{format(midpoint)}</span>
        <span>{format(analytics.end_time)}</span>
      </div>

      {/* Legend, with the time spent in each */}
      <div className="flex flex-wrap gap-x-5 gap-y-1 text-sm">
        {legend.map(({ mode, minutes }) => (
          <div key={mode} className="flex items-center gap-2">
            <span className={`inline-block size-3 rounded-sm border ${MODES[mode].fill}`} aria-hidden="true" />
            <span className="text-muted-foreground">
              {MODES[mode].label} <span className="font-medium text-foreground">{formatDuration(minutes)}</span>
            </span>
          </div>
        ))}
        {analytics.mode_transitions > 0 && (
          <span className="text-muted-foreground">
            {analytics.mode_transitions} {analytics.mode_transitions === 1 ? 'switch' : 'switches'}
          </span>
        )}
      </div>
    </div>
  )
}
