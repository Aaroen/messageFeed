import type { RectSnapshot } from '@/composables/useReaderSession'

export function snapshotRect(rect?: DOMRect): RectSnapshot | null {
  if (!rect || rect.width <= 0 || rect.height <= 0) {
    return null
  }
  return {
    left: Math.max(0, rect.left),
    top: Math.max(0, rect.top),
    width: Math.max(1, rect.width),
    height: Math.max(1, rect.height),
  }
}

export function snapshotElementRect(element: Element | null) {
  return element instanceof HTMLElement ? snapshotRect(element.getBoundingClientRect()) : null
}
