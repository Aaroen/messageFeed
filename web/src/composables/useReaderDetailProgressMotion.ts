import { computed, type Ref } from 'vue'

type ReaderDetailProgressMotionOptions = {
  surfaceMargin: Ref<number>
  expandedTop: Ref<number>
  visible: Ref<boolean>
  dragging: Ref<boolean>
  readerBackDragging: Ref<boolean>
  readingProgress: Ref<number>
}

function cssNumber(value: number, precision = 2) {
  return (Number.isFinite(value) ? value : 0).toFixed(precision)
}

function cssPx(value: number) {
  return `${cssNumber(value)}px`
}

const railTransition = [
  'top var(--motion-chrome) var(--ease-emphasized)',
  'bottom var(--motion-chrome) var(--ease-emphasized)',
  'opacity var(--motion-quick) var(--ease-standard)',
].join(', ')

export function useReaderDetailProgressMotion(options: ReaderDetailProgressMotionOptions) {
  const railStyle = computed(() => {
    const margin = options.surfaceMargin.value
    const top = Math.max(margin, options.expandedTop.value + margin)
    return {
      top: cssPx(top),
      right: cssPx(Math.max(6, margin * 0.5)),
      bottom: `${margin}px`,
      opacity: options.visible.value ? '1' : '0',
      pointerEvents: options.visible.value ? ('auto' as const) : ('none' as const),
      transition: options.dragging.value || options.readerBackDragging.value ? 'none' : railTransition,
    }
  })

  const fillStyle = computed(() => ({
    height: `${(options.readingProgress.value * 100).toFixed(2)}%`,
    transition: options.dragging.value
      ? 'none'
      : 'height var(--motion-micro) var(--ease-linear)',
  }))

  const thumbStyle = computed(() => {
    const progress = options.readingProgress.value
    return {
      top: `${(progress * 100).toFixed(2)}%`,
      transform: `translate3d(0, ${(-progress * 42).toFixed(2)}px, 0)`,
    }
  })

  return {
    railStyle,
    fillStyle,
    thumbStyle,
  }
}
