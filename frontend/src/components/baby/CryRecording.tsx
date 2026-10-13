import { useEffect, useRef } from 'react'
import videojs from 'video.js'
import 'video.js/dist/video-js.css'

// A crying spell's recording, as HLS from Frigate. Safari and iOS play it
// themselves; elsewhere video.js does.
export default function CryRecording({ src }: { src: string }) {
  const containerRef = useRef<HTMLDivElement>(null)

  // Opened once: a spell still going keeps the end it had when opened
  const srcRef = useRef(src)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    // video.js takes over the element it's given, so it gets one React
    // doesn't manage
    const element = document.createElement('video-js')
    element.classList.add('vjs-fill')
    container.appendChild(element)
    const player = videojs(element, {
      controls: true,
      autoplay: true,
      playsinline: true,
      preload: 'auto',
      html5: { vhs: { overrideNative: false } },
      sources: [{ src: srcRef.current, type: 'application/x-mpegURL' }],
    })
    return () => {
      if (!player.isDisposed()) player.dispose()
    }
  }, [])

  return <div ref={containerRef} className="aspect-video max-h-[60vh] w-full bg-black" />
}
