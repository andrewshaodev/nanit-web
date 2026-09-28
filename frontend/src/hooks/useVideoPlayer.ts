import { useRef, useEffect, useState, useCallback } from 'react'
import videojs from 'video.js'
import {
  createVideoJSOptions,
  initializeVideoJS,
  disposeVideoJS,
  addCustomControlsToPlayer
} from '@/lib/videojs-setup'

// Define Video.js player type
type VideoJSPlayer = ReturnType<typeof videojs>

interface UseVideoPlayerOptions {
  hlsUrl: string
}

interface UseVideoPlayerReturn {
  videoRef: (element: HTMLVideoElement | null) => void
  error: string | null
}

export function useVideoPlayer({
  hlsUrl,
}: UseVideoPlayerOptions): UseVideoPlayerReturn {
  const playerRef = useRef<VideoJSPlayer | null>(null)
  const streamRetryIntervalRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const retryStartTimeRef = useRef<number | null>(null)

  const [error, setError] = useState<string | null>(null)

  // The callbacks below are declared in dependency order, each listing what
  // it uses, so none keeps a stale copy of another (or of hlsUrl)

  const stopStreamRetry = useCallback(() => {
    if (streamRetryIntervalRef.current) {
      clearTimeout(streamRetryIntervalRef.current)
      streamRetryIntervalRef.current = null
    }
    retryStartTimeRef.current = null
  }, [])

  // Whether the playlist exists yet: the bridge only has one once the camera
  // is streaming to it
  const checkStreamAvailability = useCallback(async (): Promise<boolean> => {
    if (!hlsUrl || !playerRef.current) return false

    try {
      const response = await fetch(hlsUrl, {
        method: 'GET',
        signal: AbortSignal.timeout(5000)
      })

      if (response.ok) {
        const text = await response.text()
        if (text.includes('#EXTM3U')) {
          return true
        }
      }
      return false
    } catch {
      return false
    }
  }, [hlsUrl])

  // Poll for the stream, then load it: every 2.5 s for the first minute,
  // every 30 s after that
  const startStreamRetry = useCallback(() => {
    if (!hlsUrl || !playerRef.current) return

    // Stop any existing retry
    stopStreamRetry()

    retryStartTimeRef.current = Date.now()
    setError(null)

    console.log('Starting smart stream retry...')

    const attemptStreamLoad = async () => {
      const player = playerRef.current
      if (!player) return

      const isAvailable = await checkStreamAvailability()

      if (isAvailable) {
        console.log('Stream is available, loading...')
        player.src({
          src: hlsUrl,
          type: 'application/x-mpegURL'
        })
        player.load()
        player.trigger('streamAvailable')
        stopStreamRetry() // Stop retrying once successful
        return
      }

      // Determine retry interval based on elapsed time
      const elapsedMs = Date.now() - (retryStartTimeRef.current || 0)
      const elapsedMinutes = elapsedMs / (1000 * 60)

      let nextInterval: number
      if (elapsedMinutes < 1) {
        // First minute: check every 2-3 seconds
        nextInterval = 2500
        player.trigger('streamUnavailable', 'Checking for stream...')
      } else {
        // After first minute: check every 30 seconds
        nextInterval = 30000
        player.trigger('streamUnavailable', 'Waiting for stream...')
      }

      console.log(`Stream not ready, retrying in ${nextInterval/1000}s...`)

      streamRetryIntervalRef.current = setTimeout(attemptStreamLoad, nextInterval)
    }

    // Start immediate check
    attemptStreamLoad()
  }, [hlsUrl, checkStreamAvailability, stopStreamRetry])

  // Initialize Video.js player when element is mounted
  const initializePlayer = useCallback((videoElement: HTMLVideoElement) => {
    if (!videoElement || playerRef.current) return

    console.log('Initializing Video.js player')

    // Initialize Video.js
    const vjs = initializeVideoJS()

    // Create player options with HLS URL
    const options = createVideoJSOptions(hlsUrl)

    // Create the player
    const player = vjs(videoElement, options)
    playerRef.current = player

    // Add custom controls to the player
    addCustomControlsToPlayer(player)

    console.log('Video.js player created successfully')

    // Player event handlers
    player.ready(() => {
      console.log('Video.js player is ready')

      // Start smart retry logic for stream availability
      if (hlsUrl) {
        startStreamRetry()
      }
    })

    player.on('loadstart', () => {
      setError(null)
    })

    player.on('error', () => {
      const playerError = player.error()
      setError(playerError ? playerError.message : 'Video playback error')
    })
  }, [hlsUrl, startStreamRetry])

  // Create callback ref for video element
  const videoRef = useCallback((videoElement: HTMLVideoElement | null) => {
    // Cleanup existing player if element is being removed
    if (!videoElement && playerRef.current) {
      console.log('Cleaning up Video.js player')
      stopStreamRetry()
      disposeVideoJS(playerRef.current)
      playerRef.current = null
      return
    }

    // Initialize player if element is being added and no player exists
    if (videoElement && !playerRef.current) {
      // Small delay to ensure React has fully mounted the element
      setTimeout(() => {
        if (videoElement.isConnected && !playerRef.current) {
          initializePlayer(videoElement)
        }
      }, 0)
    }
  }, [initializePlayer, stopStreamRetry])

  // Cleanup player on unmount
  useEffect(() => {
    return () => {
      stopStreamRetry()
      disposeVideoJS(playerRef.current)
      playerRef.current = null
    }
  }, [stopStreamRetry])

  return { videoRef, error }
}
