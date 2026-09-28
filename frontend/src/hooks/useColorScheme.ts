import { useEffect, useState } from 'react'

export type ColorScheme = 'light' | 'dark'

const current = (): ColorScheme =>
  document.documentElement.classList.contains('dark') ? 'dark' : 'light'

// The theme in effect (Latte or Mocha), from the .dark class that
// src/lib/theme.ts sets on <html>. Components drawing on a canvas re-render
// through this, since CSS alone cannot restyle them.
export function useColorScheme(): ColorScheme {
  const [scheme, setScheme] = useState<ColorScheme>(current)

  useEffect(() => {
    const observer = new MutationObserver(() => setScheme(current()))
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
    return () => observer.disconnect()
  }, [])

  return scheme
}
