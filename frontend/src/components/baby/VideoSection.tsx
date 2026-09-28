import { useEffect } from 'react'
import { api } from '@/lib/api'
import type { Baby } from '@/types/api'
import { useVideoPlayer } from '@/hooks/useVideoPlayer'
import { CircleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface VideoSectionProps {
  baby: Baby
}

export default function VideoSection({ baby }: VideoSectionProps) {
  // Get HLS URL for this baby
  const hlsUrl = api.getHLSUrl(baby.uid)
  
  // Use Video.js player hook with integrated controls
  const { videoRef, error } = useVideoPlayer({ hlsUrl })

  // Ask the camera to stream while someone is watching. With
  // NANIT_RTMP_AUTO_START on it already is, and the bridge leaves a running
  // stream alone; with it off, this is what starts it. The player waits for
  // the stream either way.
  useEffect(() => {
    if (baby.websocket_alive) {
      api.startStream(baby.uid).catch(() => {})
    }
  }, [baby.uid, baby.websocket_alive])

  return (
    <div className="space-y-2">
      {/* Error display */}
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