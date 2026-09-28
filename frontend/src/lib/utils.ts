// shadcn's class merger (clsx + tailwind-merge in one), which its components
// import from here
export { cn } from 'cn'

// Nanit leaves the name empty for a baby that was never named, and its app
// shows "Baby" for it, so do the same
export function displayName(baby: { name?: string | null }): string {
  return baby.name?.trim() || 'Baby'
}

// The backend's baby.StreamState values, as /api/status sends them. The
// streaming panel used to compare against the string 'streaming', which never
// matched, so every camera read as offline.
export const StreamState = { Unknown: 0, Unhealthy: 1, Alive: 2 } as const

export function streamStatus(baby: { stream_state?: number }): { streaming: boolean; label: string } {
  switch (baby.stream_state) {
    case StreamState.Alive:
      return { streaming: true, label: 'Streaming' }
    case StreamState.Unhealthy:
      return { streaming: false, label: 'Stream lost' }
    default:
      return { streaming: false, label: 'Not streaming' }
  }
}

export function formatRelativeTime(date: Date): string {
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const diffMins = Math.floor(diffSecs / 60)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffSecs < 60) {
    return `${diffSecs}s ago`
  } else if (diffMins < 60) {
    return `${diffMins}m ago`
  } else if (diffHours < 24) {
    return `${diffHours}h ago`
  } else if (diffDays < 7) {
    return `${diffDays}d ago`
  } else {
    return date.toLocaleDateString()
  }
}

export function getTimeRange(timeframe: string): { start: number; end: number } {
  const end = Date.now()
  let start: number

  switch (timeframe) {
    case '1h':
      start = end - (1 * 60 * 60 * 1000)
      break
    case '6h':
      start = end - (6 * 60 * 60 * 1000)
      break
    case '24h':
      start = end - (24 * 60 * 60 * 1000)
      break
    case '7d':
      start = end - (7 * 24 * 60 * 60 * 1000)
      break
    case '30d':
      start = end - (30 * 24 * 60 * 60 * 1000)
      break
    default:
      start = end - (24 * 60 * 60 * 1000)
  }

  return {
    start: Math.floor(start / 1000),
    end: Math.floor(end / 1000)
  }
}

export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
    } else {
      // Fallback for older browsers or non-HTTPS
      const textArea = document.createElement('textarea')
      textArea.value = text
      textArea.style.position = 'fixed'
      textArea.style.left = '-999999px'
      textArea.style.top = '-999999px'
      document.body.appendChild(textArea)
      textArea.focus()
      textArea.select()
      document.execCommand('copy')
      textArea.remove()
    }
    return true
  } catch (err) {
    console.error('Failed to copy text:', err)
    return false
  }
}

export function debounce<T extends (...args: any[]) => any>(
  func: T,
  delay: number
): (...args: Parameters<T>) => void {
  let timeoutId: NodeJS.Timeout
  return (...args: Parameters<T>) => {
    clearTimeout(timeoutId)
    timeoutId = setTimeout(() => func(...args), delay)
  }
}