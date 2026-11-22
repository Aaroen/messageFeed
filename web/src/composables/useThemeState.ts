import { readonly, ref } from 'vue'

const themeStorageKey = 'messagefeed-theme'
const dark = ref(false)

function applyTheme(nextDark: boolean) {
  if (typeof document === 'undefined') {
    return
  }
  if (nextDark) {
    document.body.setAttribute('arco-theme', 'dark')
    return
  }
  document.body.removeAttribute('arco-theme')
}

function saveTheme(nextDark: boolean) {
  if (typeof localStorage === 'undefined') {
    return
  }
  localStorage.setItem(themeStorageKey, nextDark ? 'dark' : 'light')
}

export function useThemeState() {
  function setDark(nextDark: boolean, options: { persist?: boolean } = {}) {
    dark.value = nextDark
    applyTheme(nextDark)
    if (options.persist ?? true) {
      saveTheme(nextDark)
    }
  }

  function load() {
    const storedDark = typeof localStorage !== 'undefined' && localStorage.getItem(themeStorageKey) === 'dark'
    setDark(storedDark, { persist: false })
  }

  function toggle() {
    setDark(!dark.value)
  }

  return {
    dark: readonly(dark),
    load,
    setDark,
    toggle,
  }
}
