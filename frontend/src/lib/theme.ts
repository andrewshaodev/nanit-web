// Light/dark theme: the header's System/Light/Dark choice, remembered per
// browser. "system" follows the device's setting, live. The theme itself is a
// .dark class on <html>, which the Catppuccin and shadcn styles key off.
//
// index.html applies the saved choice before first paint with a copy of
// resolve() and apply(); keep the two in step.

export type ThemePreference = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'theme'
const CHANGED_EVENT = 'themechange'
const media = window.matchMedia('(prefers-color-scheme: dark)')

export function getThemePreference(): ThemePreference {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
  } catch {
    // Blocked storage: fall back to the device
  }
  return 'system'
}

function resolve(preference: ThemePreference): 'light' | 'dark' {
  if (preference === 'system') return media.matches ? 'dark' : 'light'
  return preference
}

// Two classes for dark: .dark drives the dark: utilities, and .mocha switches
// Catppuccin's colours. Its own dark block is written as a media-style
// variant around :root, which a class variant can't reach, so its flavour
// class does it instead. Light needs neither: Latte is the default.
function apply(preference: ThemePreference) {
  const dark = resolve(preference) === 'dark'
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.classList.toggle('mocha', dark)
}

export function setThemePreference(preference: ThemePreference) {
  try {
    localStorage.setItem(STORAGE_KEY, preference)
  } catch {
    // Not remembered, but applies until the page reloads
  }
  apply(preference)
  window.dispatchEvent(new CustomEvent(CHANGED_EVENT, { detail: preference }))
}

// Call once at startup. Re-applies on device changes while on "system", and
// on changes made in other tabs.
export function initTheme() {
  apply(getThemePreference())
  media.addEventListener('change', () => {
    if (getThemePreference() === 'system') apply('system')
  })
  window.addEventListener('storage', (e) => {
    if (e.key === STORAGE_KEY) {
      apply(getThemePreference())
      window.dispatchEvent(new CustomEvent(CHANGED_EVENT, { detail: getThemePreference() }))
    }
  })
}

export function onThemePreferenceChange(callback: (preference: ThemePreference) => void): () => void {
  const listener = (e: Event) => callback((e as CustomEvent<ThemePreference>).detail)
  window.addEventListener(CHANGED_EVENT, listener)
  return () => window.removeEventListener(CHANGED_EVENT, listener)
}
