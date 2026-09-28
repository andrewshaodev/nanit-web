import { useCallback, useEffect, useState } from 'react'
import type { Baby } from '@/types/api'

// Per-browser dashboard layout: the order cameras are shown in, and which
// ones are collapsed to their header. Cameras are identified by baby uid.
interface CameraLayout {
  order: string[]
  collapsed: string[]
}

const STORAGE_KEY = 'cameraLayout'
const CHANGED_EVENT = 'cameraLayoutChanged'

function load(): CameraLayout {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}')
    return {
      order: Array.isArray(parsed.order) ? parsed.order.filter((uid: unknown) => typeof uid === 'string') : [],
      collapsed: Array.isArray(parsed.collapsed) ? parsed.collapsed.filter((uid: unknown) => typeof uid === 'string') : [],
    }
  } catch {
    // Private windows and blocked storage throw; fall back to Nanit's order
    return { order: [], collapsed: [] }
  }
}

function save(layout: CameraLayout) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(layout))
  } catch {
    // Not persisted, but the layout still applies until the page reloads
  }
  window.dispatchEvent(new CustomEvent(CHANGED_EVENT, { detail: layout }))
}

export function useCameraLayout() {
  const [layout, setLayout] = useState<CameraLayout>(load)

  // Stay in sync with other tabs, and with other components in this one
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) setLayout(load())
    }
    const onChanged = (e: Event) => setLayout((e as CustomEvent<CameraLayout>).detail)

    window.addEventListener('storage', onStorage)
    window.addEventListener(CHANGED_EVENT, onChanged)
    return () => {
      window.removeEventListener('storage', onStorage)
      window.removeEventListener(CHANGED_EVENT, onChanged)
    }
  }, [])

  const update = useCallback((next: CameraLayout) => {
    setLayout(next)
    save(next)
  }, [])

  // Saved order first; cameras not in it yet (new ones) follow in Nanit's order
  const sortBabies = useCallback((babies: Baby[]): Baby[] => {
    const rank = (uid: string) => {
      const i = layout.order.indexOf(uid)
      return i === -1 ? Number.MAX_SAFE_INTEGER : i
    }
    return babies
      .map((baby, nanitIndex) => ({ baby, nanitIndex }))
      .toSorted((a, b) => rank(a.baby.uid) - rank(b.baby.uid) || a.nanitIndex - b.nanitIndex)
      .map(({ baby }) => baby)
  }, [layout.order])

  const isCollapsed = useCallback((uid: string) => layout.collapsed.includes(uid), [layout.collapsed])

  const toggleCollapsed = useCallback((uid: string) => {
    const collapsed = layout.collapsed.includes(uid)
      ? layout.collapsed.filter((u) => u !== uid)
      : [...layout.collapsed, uid]
    update({ ...layout, collapsed })
  }, [layout, update])

  // Moves a camera one place within the list as currently shown
  const move = useCallback((sorted: Baby[], uid: string, direction: -1 | 1) => {
    const order = sorted.map((baby) => baby.uid)
    const from = order.indexOf(uid)
    const to = from + direction
    if (from === -1 || to < 0 || to >= order.length) return

    ;[order[from], order[to]] = [order[to], order[from]]
    update({ ...layout, order })
  }, [layout, update])

  return { sortBabies, isCollapsed, toggleCollapsed, move }
}
