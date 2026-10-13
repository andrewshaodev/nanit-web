import { useCallback, useEffect, useRef, useState } from 'react'
import { LiveSession, liveUrl, type LiveStatus } from '@/lib/liveSession'

interface UseLivePlayerReturn {
  videoRef: React.RefObject<HTMLVideoElement | null>
  // null until the viewer presses play
  status: LiveStatus | null
  // Why the live view can't play here, once it has given up
  failure: string | null
  start: () => void
}

// The live view of a camera, started by the viewer (there's no autoplay)
export function useLivePlayer(babyUid: string): UseLivePlayerReturn {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const sessionRef = useRef<LiveSession | null>(null)
  const [status, setStatus] = useState<LiveStatus | null>(null)
  const [failure, setFailure] = useState<string | null>(null)

  const start = useCallback(() => {
    const video = videoRef.current
    if (!video || sessionRef.current) return
    const session = new LiveSession(video, liveUrl(babyUid), {
      onStatus: setStatus,
      onFail: (reason) => {
        console.warn('Live view unavailable, using HLS:', reason)
        sessionRef.current = null
        setFailure(reason)
      },
    })
    sessionRef.current = session
    session.start()
  }, [babyUid])

  // Leaving the page ends the session. (The player is keyed by camera, so
  // another camera is another player.)
  useEffect(() => {
    return () => {
      sessionRef.current?.stop()
      sessionRef.current = null
    }
  }, [])

  return { videoRef, status, failure, start }
}
