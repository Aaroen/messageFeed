import { readonly, ref } from 'vue'

type ViewportSizeOptions = {
  defaultWidth?: number
  defaultHeight?: number
}

function currentWidth(defaultWidth: number) {
  return typeof window === 'undefined' ? defaultWidth : window.innerWidth
}

function currentHeight(defaultHeight: number) {
  return typeof window === 'undefined' ? defaultHeight : window.innerHeight
}

export function useViewportSize(options: ViewportSizeOptions = {}) {
  const defaultWidth = options.defaultWidth ?? 1440
  const defaultHeight = options.defaultHeight ?? 900
  const width = ref(currentWidth(defaultWidth))
  const height = ref(currentHeight(defaultHeight))

  function sync() {
    width.value = currentWidth(defaultWidth)
    height.value = currentHeight(defaultHeight)
  }

  return {
    width: readonly(width),
    height: readonly(height),
    sync,
  }
}
