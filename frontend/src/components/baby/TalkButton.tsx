import { useCallback, useEffect, useRef, useState } from 'react'
import { Loader2, Mic } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { TalkSession, talkSupported, type TalkState } from '@/lib/talkSession'

interface TalkButtonProps {
  babyUid: string
  disabled: boolean
  // Why the last talk ended early, for the panel to show; null to clear
  onError: (message: string | null) => void
}

// Hold to talk through the camera's speaker, like a walkie-talkie. The
// camera takes a moment to start playing: "Talking" means it's listening.
export default function TalkButton({ babyUid, disabled, onError }: TalkButtonProps) {
  const [state, setState] = useState<TalkState>('idle')
  const session = useRef<TalkSession | null>(null)
  const supported = talkSupported()

  const stop = useCallback(() => {
    session.current?.stop()
    session.current = null
  }, [])

  const start = () => {
    if (session.current || disabled) return
    onError(null)
    const s = new TalkSession(babyUid, setState, (reason) => {
      if (session.current === s) session.current = null
      onError(reason ?? null)
    })
    session.current = s
    void s.start()
  }

  // The camera going offline ends it too
  useEffect(() => {
    if (disabled) stop()
  }, [disabled, stop])

  // Leaving the page, or switching away from it, lets go
  useEffect(() => {
    const onHide = () => {
      if (document.visibilityState === 'hidden') stop()
    }
    document.addEventListener('visibilitychange', onHide)
    return () => {
      document.removeEventListener('visibilitychange', onHide)
      stop()
    }
  }, [stop])

  const label = () => {
    switch (state) {
      case 'starting':
      case 'connecting':
        return (
          <>
            <Loader2 className="animate-spin" />
            Connecting...
          </>
        )
      case 'talking':
        return (
          <>
            <Mic className="animate-pulse" />
            Talking
          </>
        )
      default:
        return (
          <>
            <Mic />
            Hold to Talk
          </>
        )
    }
  }

  return (
    <Button
      variant={state === 'talking' ? 'destructive' : 'outline'}
      className="w-full touch-none select-none [-webkit-touch-callout:none]"
      disabled={disabled || !supported}
      title={supported ? 'Hold to talk through the camera' : 'Talking needs a secure (https) page and a microphone'}
      onPointerDown={(e) => {
        if (e.button !== 0) return
        // Keeps the release coming here even if the finger slides off
        e.currentTarget.setPointerCapture(e.pointerId)
        start()
      }}
      onPointerUp={stop}
      onPointerCancel={stop}
      onLostPointerCapture={stop}
      onKeyDown={(e) => {
        if ((e.key === ' ' || e.key === 'Enter') && !e.repeat) {
          e.preventDefault()
          start()
        }
      }}
      onKeyUp={(e) => {
        if (e.key === ' ' || e.key === 'Enter') stop()
      }}
      onBlur={stop}
      onContextMenu={(e) => e.preventDefault()}
    >
      {label()}
    </Button>
  )
}
