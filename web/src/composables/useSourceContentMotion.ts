import { computed, ref, type Ref } from 'vue'

type SourceContentMotionOptions = {
  headerSpace: Ref<number>
  headerHeight: Ref<number>
  isVisible: () => boolean
  resolveDelay?: (duration: number) => number
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

function cssTranslate3d(x: number, y: number, z = 0) {
  return `translate3d(${cssPx(x)}, ${cssPx(y)}, ${cssPx(z)})`
}

export function useSourceContentMotion(options: SourceContentMotionOptions) {
  const settleOffset = ref(0)
  const settling = ref(false)
  let settleTimer = 0
  let settleFrame = 0

  const contentStyle = computed(() => ({
    paddingTop: cssPx(options.headerSpace.value + 14),
    transform: cssTranslate3d(0, settleOffset.value),
    transition: settling.value
      ? 'padding-top var(--motion-chrome) var(--ease-emphasized), transform var(--motion-chrome) var(--ease-emphasized)'
      : settleOffset.value > 0
        ? 'none'
        : undefined,
  }))

  function delay(duration: number) {
    return options.resolveDelay?.(duration) ?? duration
  }

  function clearTimer() {
    if (typeof window !== 'undefined') {
      window.clearTimeout(settleTimer)
      window.cancelAnimationFrame(settleFrame)
    }
    settleFrame = 0
  }

  function reset() {
    clearTimer()
    settleOffset.value = 0
    settling.value = false
  }

  function settleAfterRefresh(duration: number) {
    if (!options.isVisible()) {
      reset()
      return
    }

    clearTimer()
    settling.value = false
    settleOffset.value = options.headerHeight.value
    settleFrame = window.requestAnimationFrame(() => {
      settleFrame = window.requestAnimationFrame(() => {
        settleFrame = 0
        if (!options.isVisible()) {
          reset()
          return
        }
        settling.value = true
        settleOffset.value = 0
      })
    })
    settleTimer = window.setTimeout(() => {
      settling.value = false
    }, delay(duration))
  }

  return {
    contentStyle,
    clearTimer,
    reset,
    settleAfterRefresh,
  }
}
