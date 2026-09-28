import { useEffect, useState } from 'react'

export type ColorScheme = 'light' | 'dark'

const query = '(prefers-color-scheme: dark)'

const current = (): ColorScheme =>
  typeof window !== 'undefined' && window.matchMedia(query).matches ? 'dark' : 'light'

// The device's light/dark setting, which picks the Catppuccin flavour (Latte
// or Mocha). Components drawing on a canvas re-render through this, since CSS
// alone cannot restyle them.
export function useColorScheme(): ColorScheme {
  const [scheme, setScheme] = useState<ColorScheme>(current)

  useEffect(() => {
    const media = window.matchMedia(query)
    const onChange = () => setScheme(current())
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [])

  return scheme
}
