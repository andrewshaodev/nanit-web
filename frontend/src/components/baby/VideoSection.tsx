import { useCallback, useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { liveSupported } from '@/lib/liveSession'
import type { Baby } from '@/types/api'
import { useVideoPlayer } from '@/hooks/useVideoPlayer'
import { CircleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import LiveVideo from './LiveVideo'

interface VideoSectionProps {
  baby: Baby
}

export default function VideoSection({ baby }: VideoSectionProps) {
  // The live view where the browser can play it, HLS otherwise or if it
  // fails. Another camera gets a fresh try.
  const [hlsReason, setHlsReason] = useState<string | null>(() =>
    liveSupported() ? null : 'unsupported',
  )
  const [hlsFor, setHlsFor] = useState(baby.uid)
  if (hlsFor !== baby.uid) {
    setHlsFor(baby.uid)
    setHlsReason(liveSupported() ? null : 'unsupported')
  }
  const fallBack = useCallback((reason: string) => setHlsReason(reason), [])

  // Ask the camera to stream while someone is watching. With
  // NANIT_RTMP_AUTO_START on it already is, and the bridge leaves a running
  // stream alone; with it off, this is what starts it. The player waits for
  // the stream either way.
  useEffect(() => {
    if (baby.websocket_alive) {
      api.startStream(baby.uid).catch(() => {})
    }
  }, [baby.uid, baby.websocket_alive])

  if (hlsReason === null) {
    return <LiveVideo key={baby.uid} babyUid={baby.uid} onFallback={fallBack} />
  }
  return (
    <div className="space-y-1">
      <HLSVideo key={baby.uid} babyUid={baby.uid} />
      {hlsReason !== 'unsupported' && (
        <p className="text-xs text-muted-foreground">Live view unavailable ({hlsReason}); showing HLS.</p>
      )}
    </div>
  )
}

// The video.js HLS player, a few seconds behind live
function HLSVideo({ babyUid }: { babyUid: string }) {
  const hlsUrl = api.getHLSUrl(babyUid)
  const { videoRef, error } = useVideoPlayer({ hlsUrl })

  return (
    <div className="space-y-2">
      {error && (
        <Alert variant="destructive">
          <CircleAlert />
          <AlertDescription>Error: {error}</AlertDescription>
        </Alert>
      )}

      {/* Video.js Player with integrated controls. useVideoPlayer sets it up;
          there must be no data-setup attribute, or video.js's own auto-setup
          can reach the element first and create the player without our
          options (fluid/fill), leaving it small and off-centre */}
      {/* 16:9 box; the player fills it (fill: true in its options) */}
      <div className="aspect-video bg-black rounded-md border overflow-hidden">
        {/* A live camera feed has no captions to offer */}
        {/* oxlint-disable-next-line jsx-a11y/media-has-caption */}
        <video
          ref={videoRef}
          className="video-js vjs-default-skin w-full h-full"
          controls
          preload="none"
        >
          <p className="vjs-no-js">
            To view this video please enable JavaScript, and consider upgrading to a web browser that
            <a href="https://videojs.com/html5-video-support/" target="_blank" rel="noopener noreferrer">
              supports HTML5 video
            </a>.
          </p>
        </video>
      </div>
    </div>
  )
}
