import { readonly, ref } from 'vue'

const themeStorageKey = 'messagefeed-theme'

function applyTheme(dark: boolean) {
  if (typeof document === 'undefined') {
    return
  }
  if (dark) {
    document.body.setAttribute('arco-theme', 'dark')
    return
  }
  document.body.removeAttribute('arco-theme')
}

function saveTheme(dark: boolean) {
  if (typeof localStorage === 'undefined') {
    return
  }
  localStorage.setItem(themeStorageKey, dark ? 'dark' : 'light')
}

export function useThemeState() {
  const dark = ref(false)

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
