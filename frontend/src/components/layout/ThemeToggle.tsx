import { useEffect, useState } from 'react'
import { Monitor, Moon, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  getThemePreference,
  onThemePreferenceChange,
  setThemePreference,
  type ThemePreference,
} from '@/lib/theme'

const ORDER: ThemePreference[] = ['system', 'light', 'dark']

const THEMES = {
  system: { label: 'System', icon: Monitor },
  light: { label: 'Light', icon: Sun },
  dark: { label: 'Dark', icon: Moon },
} as const

// Cycles System -> Light -> Dark. Sits on the gradient header, so it is
// tinted for that rather than the page (see BabyHeader's buttons).
export default function ThemeToggle() {
  const [preference, setPreference] = useState<ThemePreference>(getThemePreference)

  useEffect(() => onThemePreferenceChange(setPreference), [])

  const next = ORDER[(ORDER.indexOf(preference) + 1) % ORDER.length]
  const { label, icon: Icon } = THEMES[preference]

  return (
    <Button
      variant="ghost"
      size="sm"
      className="text-current hover:bg-ctp-base/20 hover:text-current dark:hover:bg-ctp-base/20"
      onClick={() => setThemePreference(next)}
      aria-label={`Theme: ${label}. Switch to ${THEMES[next].label}`}
      title={`Theme: ${label} (click for ${THEMES[next].label})`}
    >
      <Icon />
      <span className="hidden sm:inline">{label}</span>
    </Button>
  )
}
