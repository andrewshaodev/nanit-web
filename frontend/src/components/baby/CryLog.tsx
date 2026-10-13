import { useState } from 'react'
import useSWR from 'swr'
import { Baby as BabyIcon, ChevronLeft, ChevronRight, CircleAlert, Play, X } from 'lucide-react'
import { api } from '@/lib/api'
import { useSectionCollapsed } from '@/hooks/useSectionCollapsed'
import type { Baby, CriesResponse, Cry } from '@/types/api'
import { Button } from '@/components/ui/button'
import CollapseButton from './CollapseButton'
import CryRecording from './CryRecording'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const timeframes = [
  { value: '24', label: 'Last 24 Hours' },
  { value: '168', label: 'Last 7 Days' },
  { value: '720', label: 'Last 30 Days' },
]

// "45 s", "3 min 20 s", "1 h 5 min". Frigate gives a single cry the same
// start and end.
function formatDuration(seconds: number): string {
  const s = Math.round(seconds)
  if (s < 1) return 'one cry'
  if (s < 60) return `${s} s`
  const m = Math.floor(s / 60)
  if (m < 60) return s % 60 ? `${m} min ${s % 60} s` : `${m} min`
  return m % 60 ? `${Math.floor(m / 60)} h ${m % 60} min` : `${Math.floor(m / 60)} h`
}

const timeOfDay = (date: Date) => date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

// "Today", "Yesterday", or the date
function dayLabel(date: Date): string {
  const day = new Date(date).setHours(0, 0, 0, 0)
  const today = new Date().setHours(0, 0, 0, 0)
  if (day === today) return 'Today'
  if (day === today - 24 * 60 * 60 * 1000) return 'Yesterday'
  return date.toLocaleDateString([], { weekday: 'short', day: 'numeric', month: 'short' })
}

// Spells grouped by the day they started, latest first
function byDay(cries: Cry[]): [string, Cry[]][] {
  const days = new Map<string, Cry[]>()
  for (const cry of cries) {
    const label = dayLabel(new Date(cry.start * 1000))
    days.set(label, [...(days.get(label) ?? []), cry])
  }
  return [...days]
}

const crying = (cry: Cry) => cry.end === null

const PAGE_SIZE = 10

// The page numbers to offer: the first, the last, and the ones either side
// of the current one, with null for a gap
function pageNumbers(current: number, count: number): (number | null)[] {
  const shown = [...new Set([0, current - 1, current, current + 1, count - 1])]
    .filter((p) => p >= 0 && p < count)
    .toSorted((a, b) => a - b)
  return shown.flatMap((p, i) => (i > 0 && p - shown[i - 1] > 1 ? [null, p] : [p]))
}

// Previous, the page numbers, and next
function Pager({ page, count, onChange }: { page: number; count: number; onChange: (page: number) => void }) {
  return (
    <nav className="flex items-center justify-center gap-1" aria-label="Crying log pages">
      <Button size="sm" variant="ghost" onClick={() => onChange(page - 1)} disabled={page === 0} aria-label="Previous page">
        <ChevronLeft />
      </Button>
      {pageNumbers(page, count).map((p, i) =>
        p === null ? (
          <span key={`gap-${i}`} className="px-1 text-muted-foreground" aria-hidden="true">
            …
          </span>
        ) : (
          <Button
            key={p}
            size="sm"
            variant={p === page ? 'secondary' : 'ghost'}
            className="min-w-8 tabular-nums"
            onClick={() => onChange(p)}
            aria-label={`Page ${p + 1}`}
            aria-current={p === page ? 'page' : undefined}
          >
            {p + 1}
          </Button>
        ),
      )}
      <Button size="sm" variant="ghost" onClick={() => onChange(page + 1)} disabled={page === count - 1} aria-label="Next page">
        <ChevronRight />
      </Button>
    </nav>
  )
}

// The crying spells Frigate heard, with their recordings. Hidden when the
// bridge isn't set up with Frigate.
export default function CryLog({ baby }: { baby: Baby }) {
  const [hours, setHours] = useState('24')
  const [playing, setPlaying] = useState<string | null>(null)
  const [requestedPage, setPage] = useState(0)
  const [collapsed, toggleCollapsed] = useSectionCollapsed('cries', baby.uid)

  const { data, error } = useSWR<CriesResponse>(
    `/cries/${baby.uid}?hours=${hours}`,
    () => {
      const end = Math.floor(Date.now() / 1000)
      return api.getCries(baby.uid, end - Number(hours) * 60 * 60, end)
    },
    // Quicker while crying, so the end shows up soon after it's heard
    { refreshInterval: (latest) => (latest?.cries.some(crying) ? 5000 : 15000), revalidateOnFocus: true },
  )

  if (!data?.enabled) return null

  const cries = data.cries
  const now = cries.find(crying)
  // As of the last fetch, which is every few seconds while crying
  const asOf = data.end_time
  const total = cries.reduce((sum, c) => sum + ((c.end ?? asOf) - c.start), 0)
  // Latest first, 10 to a page. Kept in range as spells come and go.
  const pageCount = Math.max(1, Math.ceil(cries.length / PAGE_SIZE))
  const page = Math.min(requestedPage, pageCount - 1)
  const shown = cries.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)
  const range = timeframes.find((t) => t.value === hours)?.label.toLowerCase() ?? ''

  return (
    <section className="border-t border-ctp-peach/25">
      <div
        className={`flex flex-wrap items-center justify-between gap-2 bg-ctp-peach/10 py-1.5 pr-4 pl-2 ${collapsed ? '' : 'border-b border-ctp-peach/25'}`}
      >
        <h3 className="flex items-center gap-2 font-semibold">
          <CollapseButton collapsed={collapsed} onToggle={toggleCollapsed} label="crying log" />
          <BabyIcon className="size-4 text-ctp-peach" aria-hidden="true" />
          Crying
          {now && (
            <span className="flex items-center gap-1.5 rounded-full bg-ctp-peach/20 px-2 py-0.5 text-xs font-medium text-ctp-peach">
              <span className="size-1.5 animate-pulse rounded-full bg-ctp-peach" aria-hidden="true" />
              Crying now
            </span>
          )}
        </h3>
        {collapsed ? (
          // Still polled, so the badge and this line stay current
          <span className="text-xs text-muted-foreground tabular-nums">
            {cries.length === 0 ? 'None' : `${cries.length} ${cries.length === 1 ? 'spell' : 'spells'}, ${formatDuration(total)}`}
            {' in the '}
            {range}
          </span>
        ) : (
          <Select
            value={hours}
            onValueChange={(value) => {
              setHours(value)
              setPage(0)
            }}
          >
            <SelectTrigger size="sm" className="w-36 bg-background" aria-label="Timeframe">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {timeframes.map((t) => (
                <SelectItem key={t.value} value={t.value}>
                  {t.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>

      {!collapsed && (
        <div className="space-y-3 p-4">
          {error && (
            <p className="flex items-center gap-1.5 text-sm text-destructive">
              <CircleAlert className="size-4 shrink-0" aria-hidden="true" />
              Couldn&apos;t load the crying log
            </p>
          )}

          <p className="text-sm text-muted-foreground">
            {cries.length === 0
              ? `No crying heard in the ${range}.`
              : `${cries.length} ${cries.length === 1 ? 'spell' : 'spells'}, ${formatDuration(total)} in all, in the ${range}.`}
          </p>

          {byDay(shown).map(([day, dayCries]) => (
            <div key={day} className="space-y-1">
              <h4 className="text-xs font-semibold text-muted-foreground">{day}</h4>
              <ul className="divide-y rounded-md border">
                {dayCries.map((cry) => {
                  const start = new Date(cry.start * 1000)
                  const open = playing === cry.review_id
                  return (
                    <li key={cry.review_id}>
                      <div className="flex items-center justify-between gap-2 px-3 py-1.5">
                        <span className="text-sm tabular-nums">
                          <span className="font-medium">{timeOfDay(start)}</span>
                          <span className="text-muted-foreground">
                            {' · '}
                            {crying(cry)
                              ? `crying for ${formatDuration(asOf - cry.start)} so far`
                              : formatDuration(cry.end! - cry.start)}
                          </span>
                        </span>
                        <Button
                          size="sm"
                          variant={open ? 'secondary' : 'ghost'}
                          onClick={() => setPlaying(open ? null : cry.review_id)}
                          aria-expanded={open}
                        >
                          {open ? <X /> : <Play />}
                          {open ? 'Close' : 'Recording'}
                        </Button>
                      </div>
                      {open && (
                        // The recording starts 30 s before the first cry, and
                        // one still going stops at the moment it was opened
                        <CryRecording src={api.cryRecordingUrl(baby.uid, cry.review_id, Date.now() / 1000)} />
                      )}
                    </li>
                  )
                })}
              </ul>
            </div>
          ))}

          {pageCount > 1 && <Pager page={page} count={pageCount} onChange={setPage} />}
        </div>
      )}
    </section>
  )
}
