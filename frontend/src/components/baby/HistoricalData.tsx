import { useState } from 'react'
import { ChartLine, CircleAlert, Loader2, RefreshCw, Trash2 } from 'lucide-react'
import { api } from '@/lib/api'
import { getTimeRange } from '@/lib/utils'
import { useHistoricalData } from '@/hooks/useHistoricalData'
import { useTemperatureUnit } from '@/hooks/useTemperatureUnit'
import type { Baby } from '@/types/api'
import TemperatureHumidityChart from '@/components/charts/TemperatureHumidityChart'
import DayNightChart from '@/components/charts/DayNightChart'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

// Import Chart.js setup
import '@/lib/chartSetup'

interface HistoricalDataProps {
  baby: Baby
}

const timeframeOptions = [
  { value: '1h', label: 'Last Hour' },
  { value: '6h', label: 'Last 6 Hours' },
  { value: '24h', label: 'Last 24 Hours' },
  { value: '7d', label: 'Last 7 Days' },
  { value: '30d', label: 'Last 30 Days' },
]

const fixed = (value: number | undefined, suffix = '') =>
  value !== undefined && value !== null ? `${value.toFixed(1)}${suffix}` : '--'

// The window's temperature and humidity at a glance, as a small table
function SummaryTable({ rows }: { rows: [string, string, string, string][] }) {
  return (
    <table className="w-full text-sm tabular-nums">
      <thead>
        <tr className="text-xs text-muted-foreground">
          <th className="py-1 text-left font-medium" scope="col"><span className="sr-only">Reading</span></th>
          <th className="py-1 text-right font-medium" scope="col">Avg</th>
          <th className="py-1 text-right font-medium" scope="col">Min</th>
          <th className="py-1 text-right font-medium" scope="col">Max</th>
        </tr>
      </thead>
      <tbody className="divide-y">
        {rows.map(([label, avg, min, max]) => (
          <tr key={label}>
            <th className="py-1.5 text-left font-normal text-muted-foreground" scope="row">{label}</th>
            <td className="py-1.5 text-right font-medium">{avg}</td>
            <td className="py-1.5 text-right">{min}</td>
            <td className="py-1.5 text-right">{max}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export default function HistoricalData({ baby }: HistoricalDataProps) {
  const [selectedTimeframe, setSelectedTimeframe] = useState('24h')
  const [isResetting, setIsResetting] = useState(false)
  const [resetFailed, setResetFailed] = useState(false)
  const { formatTemperature, unit } = useTemperatureUnit()

  const {
    sensorData,
    summary,
    analytics,
    isLoading,
    isError,
    refreshAll,
  } = useHistoricalData(baby.uid, selectedTimeframe)

  const handleReset = async () => {
    setIsResetting(true)
    setResetFailed(false)
    try {
      await api.resetHistoricalData(baby.uid)
      refreshAll()
    } catch (error) {
      console.error('Failed to reset data:', error)
      setResetFailed(true)
    } finally {
      setIsResetting(false)
    }
  }

  const temperature = (value: number | undefined) => (value ? formatTemperature(value) : '--')

  return (
    // The camera box's second section, with its own GitHub-style header row
    <section className="border-t border-ctp-mauve/25">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-ctp-mauve/25 bg-ctp-mauve/10 px-4 py-1.5">
        <h3 className="flex items-center gap-2 font-semibold">
          <ChartLine className="size-4 text-ctp-mauve" aria-hidden="true" />
          History
        </h3>

        <div className="flex flex-wrap items-center gap-2">
          <Select value={selectedTimeframe} onValueChange={setSelectedTimeframe}>
            <SelectTrigger size="sm" className="w-36 bg-background" aria-label="Timeframe">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {timeframeOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Button variant="outline" size="sm" onClick={refreshAll} disabled={isLoading}>
            {isLoading ? <Loader2 className="animate-spin" /> : <RefreshCw />}
            Refresh
          </Button>

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="destructive" size="sm" disabled={isResetting || isLoading}>
                {isResetting ? <Loader2 className="animate-spin" /> : <Trash2 />}
                {isResetting ? 'Resetting...' : 'Reset'}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Reset historical data?</AlertDialogTitle>
                <AlertDialogDescription>
                  This deletes all recorded temperature, humidity and day/night history for this baby.
                  It can't be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction variant="destructive" onClick={handleReset}>
                  Reset Data
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      <div className="space-y-4 p-4">
        {(isError || resetFailed) && (
          <Alert variant="destructive">
            <CircleAlert />
            <AlertDescription>
              {resetFailed
                ? 'Failed to reset data. Please try again.'
                : 'Failed to load historical data. Please try refreshing.'}
            </AlertDescription>
          </Alert>
        )}

        {/* The chart beside the camera-mode timeline and the summary */}
        <div className="grid gap-4 lg:grid-cols-3">
          <div className="space-y-2 lg:col-span-2">
            <h4 className="text-xs font-semibold text-muted-foreground">Temperature & humidity</h4>
            <div className="h-56">
              <TemperatureHumidityChart
                key={unit}
                data={sensorData}
                range={getTimeRange(selectedTimeframe)}
                isLoading={isLoading}
              />
            </div>
          </div>

          <div className="space-y-4">
            <div className="space-y-2">
              <h4 className="text-xs font-semibold text-muted-foreground">Camera mode</h4>
              <DayNightChart analytics={analytics || null} isLoading={isLoading} />
            </div>

            <SummaryTable
              rows={[
                ['Temperature', temperature(summary?.avg_temperature), temperature(summary?.min_temperature), temperature(summary?.max_temperature)],
                ['Humidity', fixed(summary?.avg_humidity, '%'), fixed(summary?.min_humidity, '%'), fixed(summary?.max_humidity, '%')],
              ]}
            />
          </div>
        </div>
      </div>
    </section>
  )
}
