import { useState } from 'react'
import useSWR from 'swr'
import { AudioLines, CircleAlert, Loader2, Play, Square, Volume1 } from 'lucide-react'
import { api } from '@/lib/api'
import type { Baby, SoundStatus } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

// Playback length, in seconds; -1 keeps playing until stopped (how the Nanit
// app loops a sound)
const DURATIONS = [
  { value: '-1', label: 'Continuous' },
  { value: '1800', label: '30 minutes' },
  { value: '3600', label: '1 hour' },
]

// "White Noise.wav" -> "White Noise"
const trackLabel = (name: string) => name.replace(/\.[a-z0-9]+$/i, '')

// The camera's built-in sounds and speaker volume. It only reads the camera's
// state on its own: nothing plays or changes until one of the controls is used.
export default function SoundPanel({ baby }: { baby: Baby }) {
  const { data, error, isLoading, mutate } = useSWR<SoundStatus>(
    baby.websocket_alive ? `/sound/${baby.uid}` : null,
    () => api.getSound(baby.uid),
    // Each read is three requests to the camera, so keep it occasional
    { refreshInterval: 30000, revalidateOnFocus: true }
  )

  const [track, setTrack] = useState<string>()
  const [duration, setDuration] = useState('-1')
  // Only set while the slider is being dragged; otherwise the camera's value
  const [dragVolume, setDragVolume] = useState<number>()
  const [busy, setBusy] = useState<'play' | 'stop' | 'volume' | null>(null)
  const [actionError, setActionError] = useState<string>()

  const playing = data?.playback?.playing ?? false
  const tracks = data?.tracks ?? []
  const selected = track ?? data?.playback?.track ?? tracks[0]

  const volume = dragVolume ?? data?.volume ?? undefined

  const run = async (kind: 'play' | 'stop' | 'volume', action: () => Promise<SoundStatus>) => {
    setBusy(kind)
    setActionError(undefined)
    try {
      await mutate(await action(), { revalidate: false })
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'The camera did not respond')
    } finally {
      setBusy(null)
    }
  }

  const disabled = !baby.websocket_alive || isLoading || !!error || busy !== null

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">Sound</h3>
        <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
          {playing ? (
            <>
              <AudioLines className="size-3.5 text-ctp-mauve" aria-hidden="true" />
              Playing {data?.playback?.track ? trackLabel(data.playback.track) : ''}
            </>
          ) : (
            'Not playing'
          )}
        </span>
      </div>

      {(error || actionError || data?.errors?.length) && (
        <p className="flex items-center gap-1.5 text-xs text-destructive">
          <CircleAlert className="size-3.5 shrink-0" aria-hidden="true" />
          {actionError ?? (error ? 'Could not read the camera’s sound settings' : data?.errors?.join('; '))}
        </p>
      )}

      <div className="grid grid-cols-2 gap-2">
        <Select value={selected} onValueChange={setTrack} disabled={disabled || tracks.length === 0}>
          <SelectTrigger className="w-full" aria-label="Sound">
            <SelectValue placeholder={isLoading ? 'Loading…' : 'No sounds'} />
          </SelectTrigger>
          <SelectContent>
            {tracks.map((name) => (
              <SelectItem key={name} value={name}>
                {trackLabel(name)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={duration} onValueChange={setDuration} disabled={disabled}>
          <SelectTrigger className="w-full" aria-label="Duration">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {DURATIONS.map((d) => (
              <SelectItem key={d.value} value={d.value}>
                {d.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {playing ? (
        <Button
          variant="outline"
          className="w-full"
          disabled={disabled}
          onClick={() => run('stop', () => api.stopSound(baby.uid))}
        >
          {busy === 'stop' ? <Loader2 className="animate-spin" /> : <Square />}
          Stop
        </Button>
      ) : (
        <Button
          className="w-full"
          disabled={disabled || !selected}
          onClick={() => selected && run('play', () => api.playSound(baby.uid, selected, Number(duration)))}
        >
          {busy === 'play' ? <Loader2 className="animate-spin" /> : <Play />}
          Play {selected ? trackLabel(selected) : ''}
        </Button>
      )}

      <div className="flex items-center gap-3 pt-1">
        <Volume1 className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <Slider
          aria-label="Speaker volume"
          min={0}
          max={100}
          step={1}
          value={volume !== undefined ? [volume] : [0]}
          disabled={disabled || volume === undefined}
          onValueChange={([v]) => setDragVolume(v)}
          // Sent once, on release, not on every step of the drag
          onValueCommit={([v]) => run('volume', () => api.setVolume(baby.uid, v)).finally(() => setDragVolume(undefined))}
        />
        <span className="w-8 text-right text-xs tabular-nums text-muted-foreground">
          {volume ?? '--'}
        </span>
      </div>
    </div>
  )
}
