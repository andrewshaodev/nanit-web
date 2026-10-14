// Talking through the camera's speaker. The microphone's audio goes to the
// bridge over a WebSocket as 16-bit PCM; the bridge encodes it and has the
// camera play it. Closing the socket ends the talk, and the bridge tells the
// camera to stop.

import type { TalkMessage, TalkStart } from '@/types/api'

export type TalkState = 'idle' | 'starting' | 'connecting' | 'talking'

// Samples per message: about 40 ms at 48 kHz
const CHUNK = 2048

// The worklet turns the microphone's float samples into 16-bit PCM in
// CHUNK-sample messages. Inline, so it needs no file of its own.
const WORKLET = `
class PCMSender extends AudioWorkletProcessor {
  constructor() {
    super()
    this.buf = new Int16Array(${CHUNK})
    this.n = 0
  }
  process(inputs) {
    const ch = inputs[0] && inputs[0][0]
    if (ch) {
      for (let i = 0; i < ch.length; i++) {
        const s = Math.max(-1, Math.min(1, ch[i]))
        this.buf[this.n++] = s < 0 ? s * 0x8000 : s * 0x7fff
        if (this.n === this.buf.length) {
          this.port.postMessage(this.buf.buffer, [this.buf.buffer])
          this.buf = new Int16Array(${CHUNK})
          this.n = 0
        }
      }
    }
    return true
  }
}
registerProcessor('pcm-sender', PCMSender)
`

export function talkSupported(): boolean {
  return (
    window.isSecureContext &&
    !!navigator.mediaDevices?.getUserMedia &&
    typeof AudioWorkletNode !== 'undefined' &&
    typeof WebSocket !== 'undefined'
  )
}

function talkURL(babyUid: string): string {
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${scheme}://${window.location.host}/api/talk/${babyUid}`
}

export class TalkSession {
  private stopped = false
  private ws?: WebSocket
  private ctx?: AudioContext
  private stream?: MediaStream

  constructor(
    private babyUid: string,
    private onState: (state: TalkState) => void,
    // Why it ended, when that wasn't stop()
    private onEnded: (reason?: string) => void,
  ) {}

  // Must be called from the press itself: browsers only start audio, and
  // ask for the microphone, in answer to one
  async start(): Promise<void> {
    this.onState('starting')
    try {
      this.ctx = new AudioContext()
      const ctx = this.ctx
      const [stream] = await Promise.all([
        navigator.mediaDevices.getUserMedia({
          audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
        }),
        (async () => {
          const url = URL.createObjectURL(new Blob([WORKLET], { type: 'application/javascript' }))
          try {
            await ctx.audioWorklet.addModule(url)
          } finally {
            URL.revokeObjectURL(url)
          }
        })(),
      ])
      this.stream = stream
      if (this.stopped) return this.cleanup()
      await ctx.resume()

      const ws = new WebSocket(talkURL(this.babyUid))
      ws.binaryType = 'arraybuffer'
      this.ws = ws
      ws.addEventListener('open', () => {
        const start: TalkStart = { sample_rate: ctx.sampleRate }
        ws.send(JSON.stringify(start))
        const node = new AudioWorkletNode(ctx, 'pcm-sender')
        node.port.addEventListener('message', (e: MessageEvent<ArrayBuffer>) => {
          if (ws.readyState === WebSocket.OPEN) ws.send(e.data)
        })
        node.port.start()
        ctx.createMediaStreamSource(stream).connect(node)
      })
      ws.addEventListener('message', (e) => {
        if (typeof e.data !== 'string') return
        const msg = JSON.parse(e.data) as TalkMessage
        this.onState(msg.type)
      })
      ws.addEventListener('close', (e) => {
        if (this.ws !== ws) return // closed by stop()
        const wasStopped = this.stopped
        this.cleanup()
        if (!wasStopped) this.onEnded(e.reason || 'The connection to the bridge closed')
      })
    } catch (err) {
      const wasStopped = this.stopped
      this.cleanup()
      if (!wasStopped) {
        const name = err instanceof DOMException ? err.name : ''
        this.onEnded(name === 'NotAllowedError' ? 'Microphone access was refused' : String(err))
      }
    }
  }

  stop(): void {
    if (this.stopped) return
    this.stopped = true
    this.cleanup()
  }

  private cleanup(): void {
    this.stopped = true
    const ws = this.ws
    this.ws = undefined
    ws?.close()
    this.stream?.getTracks().forEach((t) => t.stop())
    this.stream = undefined
    void this.ctx?.close().catch(() => {})
    this.ctx = undefined
    this.onState('idle')
  }
}
