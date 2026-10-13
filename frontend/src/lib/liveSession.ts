// The dashboard's live view: the bridge sends the camera's stream over a
// WebSocket as fragmented MP4 (a fragment per video frame), and Media Source
// Extensions play it, well under a second behind the camera. iOS has MSE as
// ManagedMediaSource (iOS 17.1+); elsewhere it's MediaSource. Where neither
// works, or the WebSocket can't connect (a proxy that doesn't pass them),
// the dashboard falls back to HLS.
import type { LiveMessage } from '@/types/api'

export type LiveStatus = 'connecting' | 'waiting' | 'playing' | 'reconnecting'

// How far behind the newest frame playback may drift before it jumps
// forward, and between which it plays slightly fast to catch up (seconds)
const MAX_LATENCY = 1.5
const CATCH_UP_FROM = 1.0
const CATCH_UP_UNTIL = 0.5
const CATCH_UP_RATE = 1.1
// Played video kept in the buffer; older is removed so it doesn't fill up
const KEEP_BEHIND = 20
// Without a frame for this long the connection is assumed stuck (ms)
const STALL_TIMEOUT = 10_000
const RECONNECT_DELAY = 1_000
// The bridge's close code when the camera didn't start streaming in time
const NO_STREAM = 4001
const NO_STREAM_RETRY = 5_000

// A typical H.264 + AAC type, to check MSE can play the camera's stream
// before trying
const PROBE_TYPE = 'video/mp4; codecs="avc1.4d4028,mp4a.40.2"'

type MediaSourceClass = typeof MediaSource

function mediaSourceClass(): MediaSourceClass | undefined {
  if (typeof window === 'undefined') return undefined
  const w = window as Window & { ManagedMediaSource?: MediaSourceClass }
  return w.ManagedMediaSource ?? plainMediaSource()
}

function plainMediaSource(): MediaSourceClass | undefined {
  return typeof MediaSource === 'undefined' ? undefined : MediaSource
}

export function liveSupported(): boolean {
  const MS = mediaSourceClass()
  return !!MS && typeof WebSocket !== 'undefined' && MS.isTypeSupported(PROBE_TYPE)
}

export function liveUrl(babyUid: string): string {
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${scheme}://${window.location.host}/api/stream/live/${babyUid}`
}

// Marks where a reconnection's stream begins in the append queue: its
// timestamps start from zero again, so they're moved to follow what's
// already buffered
const RESTART = Symbol('restart')
type Queued = ArrayBuffer | typeof RESTART

interface LiveCallbacks {
  onStatus: (status: LiveStatus) => void
  // The live view can't work here; the caller falls back to HLS
  onFail: (reason: string) => void
}

export class LiveSession {
  private readonly video: HTMLVideoElement
  private readonly url: string
  private readonly cb: LiveCallbacks

  private ws?: WebSocket
  private ms?: MediaSource
  private sb?: SourceBuffer
  private objectUrl?: string
  private mime?: string
  private queue: Queued[] = []

  private tickTimer?: number
  private reconnectTimer?: number
  private lastData = 0
  private everReceived = false
  private stopped = false

  constructor(video: HTMLVideoElement, url: string, cb: LiveCallbacks) {
    this.video = video
    this.url = url
    this.cb = cb
  }

  // Call from the click that starts playback: play() has to be called in
  // the user's gesture for the sound to be allowed, so it's called here,
  // before there is anything to play
  start() {
    this.video.addEventListener('playing', this.onPlaying)
    this.video.addEventListener('error', this.onVideoError)
    this.attach()
    this.play()
    this.connect()
    this.tickTimer = window.setInterval(() => this.tick(), 500)
  }

  stop() {
    this.stopped = true
    window.clearInterval(this.tickTimer)
    window.clearTimeout(this.reconnectTimer)
    this.video.removeEventListener('playing', this.onPlaying)
    this.video.removeEventListener('error', this.onVideoError)
    const ws = this.ws
    this.ws = undefined
    ws?.close()
    this.detach()
  }

  private play() {
    this.video.play().catch(() => {
      // Paused by the user, or the media changed underneath: the controls
      // are there to play again
    })
  }

  private onPlaying = () => this.cb.onStatus('playing')

  private onVideoError = () => {
    if (this.stopped) return
    const err = this.video.error
    this.fail(`The browser couldn't play the stream${err?.message ? `: ${err.message}` : ''}`)
  }

  // A new MediaSource for the video element
  private attach() {
    this.detach()
    const MS = mediaSourceClass()
    if (!MS) {
      this.fail('Media Source Extensions are unavailable')
      return
    }
    const ms = new MS()
    this.ms = ms
    // ManagedMediaSource only plays with AirPlay turned off (or given an
    // alternative source for it)
    if (MS !== plainMediaSource()) {
      this.video.disableRemotePlayback = true
    }
    this.objectUrl = URL.createObjectURL(ms)
    this.video.src = this.objectUrl
    ms.addEventListener('sourceopen', () => this.setup(), { once: true })
  }

  private detach() {
    this.sb = undefined
    this.ms = undefined
    this.queue = []
    if (this.objectUrl) {
      this.video.removeAttribute('src')
      this.video.load()
      URL.revokeObjectURL(this.objectUrl)
      this.objectUrl = undefined
    }
  }

  private connect() {
    if (this.stopped) return
    this.cb.onStatus(this.everReceived ? 'reconnecting' : 'connecting')

    const ws = new WebSocket(this.url)
    ws.binaryType = 'arraybuffer'
    this.ws = ws
    let opened = false
    ws.addEventListener('open', () => {
      opened = true
    })
    ws.addEventListener('message', (event: MessageEvent<string | ArrayBuffer>) => {
      if (this.ws !== ws) return
      if (typeof event.data === 'string') {
        this.onMessage(JSON.parse(event.data) as LiveMessage)
        return
      }
      this.everReceived = true
      this.lastData = Date.now()
      this.queue.push(event.data)
      this.flush()
    })
    ws.addEventListener('close', (event) => {
      if (this.ws !== ws || this.stopped) return
      this.ws = undefined
      // Never connected at all: WebSockets aren't getting through
      if (!opened && !this.everReceived) {
        this.fail('The live view could not connect')
        return
      }
      const delay = event.code === NO_STREAM ? NO_STREAM_RETRY : RECONNECT_DELAY
      this.reconnectTimer = window.setTimeout(() => this.connect(), delay)
    })
  }

  private onMessage(msg: LiveMessage) {
    if (msg.type === 'waiting') {
      // Nothing is sent while waiting, which isn't a stall
      this.lastData = 0
      this.cb.onStatus('waiting')
      return
    }
    const mime = msg.mime ?? ''
    this.lastData = Date.now()
    if (this.sb && mime === this.mime) {
      // The same stream again, after a reconnection: carry on in the same
      // buffer
      this.queue.push(RESTART)
      this.flush()
      return
    }
    if (this.sb) {
      // The camera's stream changed (its resolution, say): start afresh
      this.attach()
      this.play()
    }
    this.mime = mime
    this.setup()
  }

  // The source buffer, once the MediaSource is open and the type is known
  private setup() {
    const ms = this.ms
    if (!ms || ms.readyState !== 'open' || !this.mime || this.sb) return
    if (!mediaSourceClass()?.isTypeSupported(this.mime)) {
      this.fail(`This browser can't play ${this.mime}`)
      return
    }
    try {
      // A live stream has no end; browsers show their live controls
      ms.duration = Infinity
    } catch {
      // Not every browser takes it, and playback doesn't need it
    }
    try {
      this.sb = ms.addSourceBuffer(this.mime)
    } catch (err) {
      this.fail(`The browser couldn't set up the stream: ${String(err)}`)
      return
    }
    this.sb.addEventListener('updateend', () => this.flush())
    this.flush()
  }

  // Appends the next queued fragment, one at a time
  private flush() {
    const sb = this.sb
    if (!sb || sb.updating || this.queue.length === 0) return
    const next = this.queue.shift()!
    try {
      if (next === RESTART) {
        sb.abort()
        const buffered = sb.buffered
        if (buffered.length > 0) {
          sb.timestampOffset = buffered.end(buffered.length - 1)
        }
        this.flush()
        return
      }
      sb.appendBuffer(next)
    } catch (err) {
      if (err instanceof DOMException && err.name === 'QuotaExceededError') {
        // Full: drop this fragment and clear out what's been played. A
        // gap is better than stopping; playback jumps over it.
        this.trim(2)
        return
      }
      this.fail(`The browser rejected the stream: ${String(err)}`)
    }
  }

  // Removes what was played more than keep seconds ago
  private trim(keep: number) {
    const sb = this.sb
    if (!sb || sb.updating || sb.buffered.length === 0) return
    const before = this.video.currentTime - keep
    if (before > sb.buffered.start(0)) {
      sb.remove(0, before)
    }
  }

  // Keeps playback near the newest frame, the buffer small, and the
  // connection alive
  private tick() {
    const video = this.video
    const buffered = video.buffered
    if (buffered.length > 0) {
      const end = buffered.end(buffered.length - 1)
      const latency = end - video.currentTime
      if (!video.paused) {
        if (latency > MAX_LATENCY || video.currentTime < buffered.start(0)) {
          video.currentTime = Math.max(end - 0.3, buffered.start(0))
          video.playbackRate = 1
        } else {
          if (latency > CATCH_UP_FROM) {
            video.playbackRate = CATCH_UP_RATE
          } else if (latency < CATCH_UP_UNTIL) {
            video.playbackRate = 1
          }
        }
      }
      if (video.currentTime - buffered.start(0) > KEEP_BEHIND * 1.5) {
        this.trim(KEEP_BEHIND)
      }
    }

    // The bridge sends frames continuously once it's streaming. Silence
    // means a stuck connection; closing it reconnects.
    if (this.ws && this.lastData && Date.now() - this.lastData > STALL_TIMEOUT) {
      this.lastData = 0
      this.ws.close()
    }
  }

  private fail(reason: string) {
    if (this.stopped) return
    this.stop()
    this.cb.onFail(reason)
  }
}
