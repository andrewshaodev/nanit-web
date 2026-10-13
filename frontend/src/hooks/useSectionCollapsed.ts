import { useState } from 'react'

// Which of a camera card's sections are collapsed, per browser: section name
// to the baby uids it's collapsed for
const STORAGE_KEY = 'sectionCollapsed'

type Collapsed = Record<string, string[]>

function load(): Collapsed {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}')
    const out: Collapsed = {}
    if (parsed && typeof parsed === 'object') {
      for (const [section, uids] of Object.entries(parsed)) {
        if (Array.isArray(uids)) out[section] = uids.filter((uid: unknown) => typeof uid === 'string')
      }
    }
    return out
  } catch {
    // Private windows and blocked storage throw
    return {}
  }
}

function save(collapsed: Collapsed) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(collapsed))
  } catch {
    // Not remembered, but it stays as set until the page reloads
  }
}

// Whether one section of one camera's card is collapsed, and a toggle that
// remembers it
export function useSectionCollapsed(section: string, babyUid: string): [boolean, () => void] {
  const [collapsed, setCollapsed] = useState(() => load()[section]?.includes(babyUid) ?? false)

  const toggle = () => {
    // Read again: other sections and cameras may have changed it meanwhile
    const all = load()
    const others = (all[section] ?? []).filter((uid) => uid !== babyUid)
    save({ ...all, [section]: collapsed ? others : [...others, babyUid] })
    setCollapsed(!collapsed)
  }

  return [collapsed, toggle]
}
