import { useSyncExternalStore } from 'react'

// Whether the stream password's key is shown on the Streaming page. One
// switch for the page: the key section's eye button shows it in every
// address too. Hidden on every visit.
let revealed = false
const listeners = new Set<() => void>()

export function setKeyRevealed(next: boolean) {
  revealed = next
  for (const l of listeners) l()
}

export function useKeyRevealed(): boolean {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l)
      return () => listeners.delete(l)
    },
    () => revealed,
  )
}

// An address for display: the key masked unless shown. Copying uses the
// real address.
export function maskKey(text: string, key: string | undefined, show: boolean): string {
  if (!key || show) return text
  return text.split(key).join('••••••')
}
