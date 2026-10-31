import useSWR from 'swr'
import { useState } from 'react'
import { CircleAlert, Loader2, Pencil, Plus, Trash2 } from 'lucide-react'
import { cn } from 'cn'
import { api } from '@/lib/api'
import type { SoundSchedule } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import TimePicker, { formatTime } from '@/components/ui/TimePicker'

// "White Noise.wav" -> "White Noise"
const trackLabel = (name: string) => name.replace(/\.[a-z0-9]+$/i, '')

// Sunday first, as the schedule's days count them
const DAYS = [
  { short: 'S', long: 'Sun' },
  { short: 'M', long: 'Mon' },
  { short: 'T', long: 'Tue' },
  { short: 'W', long: 'Wed' },
  { short: 'T', long: 'Thu' },
  { short: 'F', long: 'Fri' },
  { short: 'S', long: 'Sat' },
]

function daysLabel(days: number[]): string {
  if (days.length === 0 || days.length === 7) return 'Every day'
  const sorted = days.toSorted((a, b) => a - b)
  const key = sorted.join()
  if (key === '1,2,3,4,5') return 'Weekdays'
  if (key === '0,6') return 'Weekends'
  return sorted.map((d) => DAYS[d].long).join(', ')
}

// Whether the run ends the next day
const overnight = (s: Pick<SoundSchedule, 'start' | 'end'>) => s.end < s.start

interface EditorProps {
  initial: SoundSchedule
  tracks: string[]
  saving: boolean
  onSave: (s: SoundSchedule) => void
  onCancel: () => void
  onDelete?: () => void
}

function Editor({ initial, tracks, saving, onSave, onCancel, onDelete }: EditorProps) {
  const [draft, setDraft] = useState(initial)
  // Every day is kept as no days; the chips show it as all seven
  const days = draft.days.length === 0 ? [0, 1, 2, 3, 4, 5, 6] : draft.days
  const toggleDay = (d: number) => {
    const next = days.includes(d) ? days.filter((x) => x !== d) : [...days, d]
    setDraft({ ...draft, days: next.length === 7 ? [] : next.toSorted((a, b) => a - b) })
  }
  const options = tracks.includes(draft.track) || !draft.track ? tracks : [draft.track, ...tracks]
  const same = draft.start === draft.end
  const noDays = days.length === 0

  return (
    <div className="space-y-3 rounded-lg border p-3">
      {options.length === 0 && (
        <p className="text-xs text-muted-foreground">
          The camera is offline, so its sounds can’t be listed. Try again once it’s back.
        </p>
      )}
      <Select value={draft.track} onValueChange={(track) => setDraft({ ...draft, track })}>
        <SelectTrigger className="w-full" aria-label="Sound">
          <SelectValue placeholder="Choose a sound" />
        </SelectTrigger>
        <SelectContent>
          {options.map((name) => (
            <SelectItem key={name} value={name}>
              {trackLabel(name)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1">
          <div className="text-xs text-muted-foreground">Starts</div>
          <TimePicker label="Starts" value={draft.start} onChange={(start) => setDraft({ ...draft, start })} className="w-full" />
        </div>
        <div className="space-y-1">
          <div className="text-xs text-muted-foreground">Stops</div>
          <TimePicker label="Stops" value={draft.end} onChange={(end) => setDraft({ ...draft, end })} className="w-full" />
        </div>
      </div>
      {overnight(draft) && <p className="text-xs text-muted-foreground">Stops the next morning.</p>}
      {same && <p className="text-xs text-destructive">Starts and stops at the same time.</p>}

      <div className="space-y-1">
        <div className="text-xs text-muted-foreground">
          {overnight(draft) ? 'Starts on' : 'Days'} · {daysLabel(draft.days)}
        </div>
        <div className="flex gap-1">
          {DAYS.map((d, i) => (
            <button
              key={i}
              type="button"
              aria-label={d.long}
              aria-pressed={days.includes(i)}
              onClick={() => toggleDay(i)}
              className={cn(
                'size-8 rounded-full text-xs font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
                days.includes(i) ? 'bg-primary text-primary-foreground' : 'border text-muted-foreground hover:bg-accent'
              )}
            >
              {d.short}
            </button>
          ))}
        </div>
      </div>

      <div className="flex items-center gap-2">
        {onDelete && (
          <Button variant="ghost" size="sm" className="text-destructive" onClick={onDelete} disabled={saving}>
            <Trash2 />
            Delete
          </Button>
        )}
        <div className="ml-auto flex gap-2">
          <Button variant="outline" size="sm" onClick={onCancel} disabled={saving}>
            Cancel
          </Button>
          <Button size="sm" onClick={() => onSave(draft)} disabled={saving || same || noDays || !draft.track}>
            {saving && <Loader2 className="animate-spin" />}
            Save
          </Button>
        </div>
      </div>
    </div>
  )
}

interface SoundSchedulesProps {
  babyUid: string
  // The camera's sounds; empty while it's offline
  tracks: string[]
}

// Sounds the bridge plays between set times of day. They're the bridge's
// settings, so they can be changed with the camera offline.
export default function SoundSchedules({ babyUid, tracks }: SoundSchedulesProps) {
  const { data, error, mutate } = useSoundSchedules(babyUid)
  // The schedule being edited: its ID, or "new"
  const [editing, setEditing] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string>()

  const schedules = data?.schedules ?? []

  const save = async (next: SoundSchedule[]) => {
    setSaving(true)
    setSaveError(undefined)
    try {
      await mutate(await api.setSoundSchedules(babyUid, next), { revalidate: false })
      setEditing(null)
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Couldn’t save the schedule')
    } finally {
      setSaving(false)
    }
  }

  const replace = (s: SoundSchedule) => schedules.map((x) => (x.id === s.id ? s : x))

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">
          {!data ? '' : schedules.length === 0 ? 'No schedules yet.' : `${schedules.length} schedule${schedules.length === 1 ? '' : 's'}`}
        </p>
        {editing === null && (
          <Button variant="outline" size="sm" onClick={() => setEditing('new')} disabled={!data}>
            <Plus />
            Add schedule
          </Button>
        )}
      </div>

      {(error || saveError) && (
        <p className="flex items-center gap-1.5 text-xs text-destructive">
          <CircleAlert className="size-3.5 shrink-0" aria-hidden="true" />
          {saveError ?? 'Couldn’t read the schedules'}
        </p>
      )}

      {schedules.map((s) =>
        editing === s.id ? (
          <Editor
            key={s.id}
            initial={s}
            tracks={tracks}
            saving={saving}
            onSave={(d) => save(replace(d))}
            onCancel={() => setEditing(null)}
            onDelete={() => save(schedules.filter((x) => x.id !== s.id))}
          />
        ) : (
          <div key={s.id} className="flex items-center gap-3 rounded-lg border px-3 py-2">
            <Switch
              checked={s.enabled}
              aria-label={`${trackLabel(s.track)} schedule on`}
              disabled={saving || editing !== null}
              onCheckedChange={(enabled) => save(replace({ ...s, enabled }))}
            />
            <div className={cn('min-w-0 flex-1', !s.enabled && 'opacity-60')}>
              <div className="truncate text-sm font-medium">{trackLabel(s.track)}</div>
              <div className="truncate text-xs text-muted-foreground tabular-nums">
                {formatTime(s.start)} – {formatTime(s.end)}
                {overnight(s) ? ' (next day)' : ''} · {daysLabel(s.days)}
              </div>
            </div>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label="Edit schedule"
              disabled={saving || editing !== null}
              onClick={() => setEditing(s.id)}
            >
              <Pencil />
            </Button>
          </div>
        )
      )}

      {editing === 'new' && (
        <Editor
          initial={{ id: '', enabled: true, track: tracks.find((t) => /white/i.test(t)) ?? tracks[0] ?? '', start: '19:30', end: '06:30', days: [] }}
          tracks={tracks}
          saving={saving}
          onSave={(d) => save([...schedules, d])}
          onCancel={() => setEditing(null)}
        />
      )}
    </div>
  )
}

// A camera's schedules, shared by everything showing them
export function useSoundSchedules(babyUid: string) {
  return useSWR(`/sound/${babyUid}/schedules`, () => api.getSoundSchedules(babyUid))
}
