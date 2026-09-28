import { useState } from 'react'
import { CircleAlert, Loader2, RefreshCw, Trash2 } from 'lucide-react'
import { api } from '@/lib/api'
import { getTimeRange } from '@/lib/utils'
import { useHistoricalData } from '@/hooks/useHistoricalData'
import { useTemperatureUnit } from '@/hooks/useTemperatureUnit'
import type { Baby } from '@/types/api'
import TemperatureHumidityChart from '@/components/charts/TemperatureHumidityChart'
import DayNightChart from '@/components/charts/DayNightChart'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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

const percent = (value: number | undefined, empty = '--') =>
  value !== undefined ? `${value.toFixed(1)}%` : empty

function SummaryCard({ title, rows }: { title: string; rows: [string, string][] }) {
  return (
    <Card size="sm" className="text-center">
      <CardHeader>
        <CardTitle className="text-sm text-muted-foreground">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-1 text-sm text-muted-foreground">
        {rows.map(([label, value]) => (
          <div key={label}>
            {label}: <span className="font-medium text-foreground">{value}</span>
          </div>
        ))}
      </CardContent>
    </Card>
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

  return (
    <div className="space-y-6">
      <h3 className="text-lg font-semibold">Historical Data</h3>

      {/* Controls */}
      <div className="flex flex-wrap items-center gap-3">
        <Select value={selectedTimeframe} onValueChange={setSelectedTimeframe}>
          <SelectTrigger className="w-44" aria-label="Timeframe">
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

        <Button onClick={refreshAll} disabled={isLoading}>
          {isLoading ? <Loader2 className="animate-spin" /> : <RefreshCw />}
          {isLoading ? 'Loading...' : 'Refresh'}
        </Button>

        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="destructive" disabled={isResetting || isLoading}>
              {isResetting ? <Loader2 className="animate-spin" /> : <Trash2 />}
              {isResetting ? 'Resetting...' : 'Reset Data'}
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

      {/* Error State */}
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

      {/* Charts */}
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Temperature & Humidity</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="h-64">
              <TemperatureHumidityChart
                key={unit}
                data={sensorData}
                range={getTimeRange(selectedTimeframe)}
                isLoading={isLoading}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Day/Night Pattern</CardTitle>
          </CardHeader>
          <CardContent>
            <DayNightChart analytics={analytics || null} isLoading={isLoading} />
          </CardContent>
        </Card>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <SummaryCard
          title="Temperature"
          rows={[
            ['Avg', summary?.avg_temperature ? formatTemperature(summary.avg_temperature) : '--'],
            ['Min', summary?.min_temperature ? formatTemperature(summary.min_temperature) : '--'],
            ['Max', summary?.max_temperature ? formatTemperature(summary.max_temperature) : '--'],
          ]}
        />
        <SummaryCard
          title="Humidity"
          rows={[
            ['Avg', percent(summary?.avg_humidity)],
            ['Min', percent(summary?.min_humidity)],
            ['Max', percent(summary?.max_humidity)],
          ]}
        />
        <SummaryCard
          title="Day/Night"
          // From the timeline's analytics, which also know how much of the
          // window went unrecorded
          rows={[
            ['Day', percent(analytics?.day_mode_percentage, '--%')],
            ['Night', percent(analytics?.night_mode_percentage, '--%')],
            ...(analytics?.unknown_mode_percentage
              ? [['No data', percent(analytics.unknown_mode_percentage)] as [string, string]]
              : []),
            ['Switches', analytics?.mode_transitions !== undefined ? String(analytics.mode_transitions) : '--'],
          ]}
        />
      </div>
    </div>
  )
}
