import { useEffect, useRef, useState } from 'react'
import { Clock } from 'lucide-react'
import { cn } from 'cn'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

// Whether this browser's locale writes times with AM/PM
export const uses12Hour = (() => {
  try {
    const cycle = new Intl.DateTimeFormat(undefined, { hour: 'numeric' }).resolvedOptions().hourCycle
    return cycle === 'h11' || cycle === 'h12'
  } catch {
    return true
  }
})()

// "19:30" -> "7:30 PM", or "19:30" in a 24-hour locale
export function formatTime(value: string): string {
  const [h, m] = value.split(':').map(Number)
  if (!uses12Hour) return value
  return `${h % 12 || 12}:${String(m).padStart(2, '0')} ${h < 12 ? 'AM' : 'PM'}`
}

const pad = (n: number) => String(n).padStart(2, '0')

interface ColumnProps<T> {
  label: string
  options: T[]
  value: T
  render: (v: T) => string
  onChange: (v: T) => void
}

// One wheel: a scrolling list with the chosen value kept in view
function Column<T extends string | number>({ label, options, value, render, onChange }: ColumnProps<T>) {
  const list = useRef<HTMLFieldSetElement>(null)
  const index = options.indexOf(value)

  useEffect(() => {
    // Scrolls only the list: scrollIntoView would move the page too
    const box = list.current
    const el = box?.querySelectorAll<HTMLElement>('button')[index]
    if (box && el) box.scrollTop = el.offsetTop - box.clientHeight / 2 + el.clientHeight / 2
  }, [index])

  const step = (e: React.KeyboardEvent, i: number) => {
    const next = e.key === 'ArrowDown' ? i + 1 : e.key === 'ArrowUp' ? i - 1 : -1
    if (next < 0 || next >= options.length) return
    e.preventDefault()
    onChange(options[next])
    list.current?.querySelectorAll<HTMLElement>('button')[next]?.focus()
  }

  return (
    <fieldset
      ref={list}
      className="relative flex max-h-52 min-w-0 snap-y flex-col gap-0.5 overflow-y-auto overscroll-contain px-0.5 [scrollbar-width:none]"
    >
      <legend className="sr-only">{label}</legend>
      {options.map((o, i) => (
        <button
          key={o}
          type="button"
          aria-pressed={o === value}
          tabIndex={o === value ? 0 : -1}
          onClick={() => onChange(o)}
          onKeyDown={(e) => step(e, i)}
          className={cn(
            'min-w-12 shrink-0 snap-center rounded-md px-2 py-1.5 text-center text-sm tabular-nums transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
            o === value ? 'bg-primary font-medium text-primary-foreground' : 'hover:bg-accent hover:text-accent-foreground'
          )}
        >
          {render(o)}
        </button>
      ))}
    </fieldset>
  )
}

interface TimePickerProps {
  // "HH:MM", 24-hour
  value: string
  onChange: (value: string) => void
  label: string
  disabled?: boolean
  className?: string
}

const MINUTES = Array.from({ length: 12 }, (_, i) => i * 5)

// A time of day: hour, minute (in fives) and AM/PM wheels in a popover,
// following the browser's 12- or 24-hour clock
export default function TimePicker({ value, onChange, label, disabled, className }: TimePickerProps) {
  const [open, setOpen] = useState(false)
  const [h, m] = value.split(':').map(Number)
  const pm = h >= 12

  const set = (hour: number, minute: number) => onChange(`${pad(hour)}:${pad(minute)}`)
  // A time set elsewhere may be off the five-minute steps; it stays listed
  const minutes = MINUTES.includes(m) ? MINUTES : [...MINUTES, m].toSorted((a, b) => a - b)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          disabled={disabled}
          aria-label={`${label}: ${formatTime(value)}`}
          className={cn('justify-start tabular-nums', className)}
        >
          <Clock className="text-muted-foreground" />
          {formatTime(value)}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-2" align="start">
        <div className="mb-1.5 px-1 text-xs font-medium text-muted-foreground">{label}</div>
        <div className="flex gap-1">
          {uses12Hour ? (
            <>
              <Column
                label="Hour"
                options={[12, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]}
                value={h % 12 || 12}
                render={String}
                onChange={(v) => set((v % 12) + (pm ? 12 : 0), m)}
              />
              <Column label="Minute" options={minutes} value={m} render={pad} onChange={(v) => set(h, v)} />
              <Column
                label="AM or PM"
                options={['AM', 'PM']}
                value={pm ? 'PM' : 'AM'}
                render={String}
                onChange={(v) => set((h % 12) + (v === 'PM' ? 12 : 0), m)}
              />
            </>
          ) : (
            <>
              <Column
                label="Hour"
                options={Array.from({ length: 24 }, (_, i) => i)}
                value={h}
                render={pad}
                onChange={(v) => set(v, m)}
              />
              <Column label="Minute" options={minutes} value={m} render={pad} onChange={(v) => set(h, v)} />
            </>
          )}
        </div>
        <Button size="sm" className="mt-2 w-full" onClick={() => setOpen(false)}>
          Done
        </Button>
      </PopoverContent>
    </Popover>
  )
}
