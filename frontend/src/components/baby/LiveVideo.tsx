import { useEffect, useRef, useState } from 'react'
import { Camera, LoaderCircle, Maximize, Pause, Play, Volume1, Volume2, VolumeX } from 'lucide-react'
import { useLivePlayer } from '@/hooks/useLivePlayer'
import { saveSnapshot } from '@/lib/snapshot'
import type { LiveStatus } from '@/lib/liveSession'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'

// The viewer's volume, kept in this browser between visits
const VOLUME_KEY = 'nanit-live-volume'

function savedVolume(): number {
  try {
    const v = Number(localStorage.getItem(VOLUME_KEY))
    return localStorage.getItem(VOLUME_KEY) !== null && v >= 0 && v <= 1 ? v : 1
  } catch {
    return 1
  }
}

const statusText: Record<Exclude<LiveStatus, 'playing'>, string> = {
  connecting: 'Connecting…',
  waiting: 'Waiting for the camera…',
  reconnecting: 'Reconnecting…',
}

interface LiveVideoProps {
  babyUid: string
  // The live view can't play here; HLS takes over
  onFallback: (reason: string) => void
}

// The camera's live view, under a second behind. It starts when the viewer
// presses play.
export default function LiveVideo({ babyUid, onFallback }: LiveVideoProps) {
  const { videoRef, status, failure, start } = useLivePlayer(babyUid)
  const [notice, setNotice] = useState<string | null>(null)
  const noticeTimer = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => {
    if (failure) onFallback(failure)
  }, [failure, onFallback])

  useEffect(() => () => clearTimeout(noticeTimer.current), [])

  const showNotice = (text: string) => {
    setNotice(text)
    clearTimeout(noticeTimer.current)
    noticeTimer.current = setTimeout(() => setNotice(null), 2500)
  }

  const snapshot = async () => {
    if (!videoRef.current) return
    try {
      await saveSnapshot(videoRef.current)
      showNotice('Snapshot saved')
    } catch (err) {
      showNotice(`Snapshot failed: ${err instanceof Error ? err.message : String(err)}`)
    }
  }

  const playing = status === 'playing'

  // The video's own state, for the control bar
  const containerRef = useRef<HTMLDivElement>(null)
  const [paused, setPaused] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(1)
  // iOS keeps the volume with the hardware buttons: a video's volume can't
  // be set there, so it gets no slider
  const [volumeSettable, setVolumeSettable] = useState(false)
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    video.volume = 0.5
    setVolumeSettable(video.volume === 0.5)
    video.volume = savedVolume()
    setVolume(video.volume)

    const sync = () => {
      setPaused(video.paused)
      setMuted(video.muted)
      setVolume(video.volume)
    }
    const events = ['play', 'pause', 'volumechange'] as const
    for (const e of events) video.addEventListener(e, sync)
    return () => {
      for (const e of events) video.removeEventListener(e, sync)
    }
  }, [videoRef])

  const togglePause = () => {
    const video = videoRef.current
    if (!video) return
    // Resuming jumps back to the newest frame (the session keeps up with it)
    if (video.paused) video.play().catch(() => {})
    else video.pause()
  }

  const toggleMute = () => {
    const video = videoRef.current
    if (!video) return
    if (video.muted || video.volume === 0) {
      video.muted = false
      // Unmuting from a volume dragged to nothing brings it back to half
      if (video.volume === 0) changeVolume(0.5)
    } else {
      video.muted = true
    }
  }

  const changeVolume = (v: number) => {
    const video = videoRef.current
    if (!video) return
    video.volume = v
    video.muted = v === 0
    try {
      localStorage.setItem(VOLUME_KEY, String(v))
    } catch {
      // Not saved; it still applies now
    }
  }

  const silent = muted || volume === 0
  const VolumeIcon = silent ? VolumeX : volume < 0.5 ? Volume1 : Volume2

  // The whole box goes fullscreen, controls included. iPhones only let the
  // video itself go fullscreen, with their own controls.
  const fullscreen = () => {
    const video = videoRef.current as (HTMLVideoElement & { webkitEnterFullscreen?: () => void }) | null
    if (document.fullscreenElement) {
      document.exitFullscreen().catch(() => {})
    } else if (document.fullscreenEnabled && containerRef.current) {
      containerRef.current.requestFullscreen().catch(() => {})
    } else {
      video?.webkitEnterFullscreen?.()
    }
  }

  return (
    // No native controls: their seek bar means nothing for a live stream,
    // and redraws as the buffer grows
    <div ref={containerRef} className="group relative aspect-video bg-black rounded-md border overflow-hidden">
      {/* A live camera feed has no captions to offer */}
      {/* oxlint-disable-next-line jsx-a11y/media-has-caption */}
      <video ref={videoRef} className="size-full object-contain" playsInline preload="none" />

      {status === null && (
        <button
          type="button"
          onClick={start}
          aria-label="Play live video"
          className="group/play absolute inset-0 flex items-center justify-center"
        >
          <span className="flex size-16 items-center justify-center rounded-full bg-black/60 text-white ring-1 ring-white/30 transition group-hover/play:scale-105 group-hover/play:bg-black/75">
            <Play className="size-7 translate-x-0.5 fill-current" />
          </span>
        </button>
      )}

      {status !== null && !playing && (
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-3 text-sm text-white/90">
          <LoaderCircle className="size-8 animate-spin" />
          {statusText[status]}
        </div>
      )}

      {playing && (
        <span
          className={`pointer-events-none absolute left-3 top-3 flex items-center gap-1.5 rounded px-2 py-0.5 text-xs font-semibold tracking-wide text-white ${paused ? 'bg-black/60' : 'bg-ctp-red'}`}
        >
          <span className={`size-1.5 rounded-full bg-white ${paused ? '' : 'animate-pulse'}`} />
          {paused ? 'PAUSED' : 'LIVE'}
        </span>
      )}

      {status !== null && (
        // Always shown on touch screens; on others, while the pointer is
        // over the video, or it's paused
        <div
          className={`absolute inset-x-0 bottom-0 flex items-center gap-1 bg-gradient-to-t from-black/70 to-transparent px-2 pb-2 pt-6 transition-opacity ${
            paused ? '' : '[@media(hover:hover)]:opacity-0 group-hover:opacity-100 focus-within:opacity-100'
          }`}
        >
          <ControlButton label={paused ? 'Play' : 'Pause'} onClick={togglePause}>
            {paused ? <Play className="fill-current" /> : <Pause className="fill-current" />}
          </ControlButton>
          <ControlButton label={silent ? 'Unmute' : 'Mute'} onClick={toggleMute}>
            <VolumeIcon />
          </ControlButton>
          {volumeSettable && (
            <Slider
              aria-label="Volume"
              min={0}
              max={100}
              step={1}
              value={[silent ? 0 : Math.round(volume * 100)]}
              onValueChange={([v]) => changeVolume(v / 100)}
              className="ml-1 w-20 sm:w-24 [&_[data-slot=slider-range]]:bg-white [&_[data-slot=slider-track]]:bg-white/30"
            />
          )}
          <div className="flex-1" />
          <ControlButton label="Take snapshot" onClick={snapshot} disabled={!playing}>
            <Camera />
          </ControlButton>
          <ControlButton label="Fullscreen" onClick={fullscreen}>
            <Maximize />
          </ControlButton>
        </div>
      )}

      {notice && (
        <div className="pointer-events-none absolute left-1/2 top-3 -translate-x-1/2 rounded bg-black/70 px-3 py-1 text-xs text-white">
          {notice}
        </div>
      )}
    </div>
  )
}

interface ControlButtonProps {
  label: string
  onClick: () => void
  disabled?: boolean
  children: React.ReactNode
}

function ControlButton({ label, onClick, disabled, children }: ControlButtonProps) {
  return (
    <Button
      size="icon"
      variant="ghost"
      onClick={onClick}
      disabled={disabled}
      title={label}
      aria-label={label}
      className="text-white hover:bg-white/15 hover:text-white"
    >
      {children}
    </Button>
  )
}
