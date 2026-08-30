import { ref } from 'vue'

// system -> light -> dark, persisted, applied as data-theme on <html>.
// Light is unstyled for now; the plumbing exists so that landing the light
// tokens is a stylesheet change and nothing else.
const ORDER = ['system', 'light', 'dark']
const KEY = 'cairn.theme'

const theme = ref(read())

function read() {
  try {
    const stored = localStorage.getItem(KEY)
    return ORDER.includes(stored) ? stored : 'system'
  } catch {
    return 'system'
  }
}

function apply() {
  const root = document.documentElement
  if (theme.value === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', theme.value)
  try {
    localStorage.setItem(KEY, theme.value)
  } catch {
    // A browser refusing storage is not a reason to break the page.
  }
}

export function useTheme() {
  return {
    theme,
    apply,
    cycle() {
      theme.value = ORDER[(ORDER.indexOf(theme.value) + 1) % ORDER.length]
      apply()
    },
  }
}
