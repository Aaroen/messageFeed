import { computed, readonly, ref, type Ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'
import { useMotionTimings } from '@/composables/useMotionTimings'

type NavigationDrawerOptions = {
  windowWidth: Ref<number>
  resolveDelay?: (duration: number) => number
  settleDuration?: number
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

function cssTranslate3d(x: number, y: number, z = 0) {
  return `translate3d(${cssPx(x)}, ${cssPx(y)}, ${cssPx(z)})`
}

export function useNavigationDrawer(options: NavigationDrawerOptions) {
  const motionTimings = useMotionTimings()
  const open = ref(false)
  const progress = ref(0)
  const settling = ref(false)
  let timer = 0
  let frame = 0
  let motionToken = 0

  const width = computed(() => {
    if (options.windowWidth.value <= 420) {
      return Math.round(options.windowWidth.value * 0.8)
    }
    return Math.round(
      Math.min(Math.max(304, options.windowWidth.value * 0.32), Math.min(440, options.windowWidth.value * 0.8)),
    )
  })

  const visible = computed(() => open.value || progress.value > 0 || settling.value)

  const panelStyle = computed(() => ({
    width: `${width.value}px`,
    transform: cssTranslate3d((progress.value - 1) * (width.value + 28), 0),
    transition: settling.value ? 'transform var(--motion-short) var(--ease-standard)' : undefined,
  }))

  const scrimStyle = computed(() => ({
    opacity: progress.value,
    pointerEvents: progress.value > 0.2 ? ('auto' as const) : ('none' as const),
  }))

  function delay(duration: number) {
    return options.resolveDelay?.(duration) ?? duration
  }

  function settleDuration() {
    return options.settleDuration ?? motionTimings.navigationDrawerSettleDuration
  }

  function clearTimer() {
    if (typeof window !== 'undefined' && timer !== 0) {
      window.clearTimeout(timer)
    }
    if (typeof window !== 'undefined' && frame !== 0) {
      window.cancelAnimationFrame(frame)
    }
    timer = 0
    frame = 0
    motionToken += 1
  }

  function settle(nextOpen: boolean) {
    clearTimer()
    const token = motionToken + 1
    motionToken = token
    settling.value = true
    open.value = nextOpen
    progress.value = nextOpen ? 1 : 0
    timer = window.setTimeout(() => {
      if (token !== motionToken) {
        return
      }
      timer = 0
      settling.value = false
      if (!nextOpen) {
        progress.value = 0
      }
    }, delay(settleDuration()))
  }

  function openPanel() {
    clearTimer()
    const token = motionToken + 1
    motionToken = token
    open.value = true
    settling.value = true
    progress.value = 0
    frame = window.requestAnimationFrame(() => {
      if (token !== motionToken) {
        return
      }
      frame = 0
      if (!open.value) {
        return
      }
      progress.value = 1
    })
    timer = window.setTimeout(() => {
      if (token !== motionToken) {
        return
      }
      timer = 0
      settling.value = false
    }, delay(settleDuration()))
  }

  function closePanel() {
    if (!visible.value) {
      clearTimer()
      open.value = false
      progress.value = 0
      settling.value = false
      return
    }
    settle(false)
  }

  function beginDrag() {
    clearTimer()
    settling.value = false
  }

  function updateOpeningDrag(deltaX: number) {
    progress.value = clampProgress(deltaX / width.value)
  }

  function updateClosingDrag(startProgress: number, deltaX: number) {
    progress.value = clampProgress(startProgress + deltaX / width.value)
  }

  return {
    open: readonly(open),
    progress: readonly(progress),
    settling: readonly(settling),
    width,
    visible,
    panelStyle,
    scrimStyle,
    clearTimer,
    settle,
    openPanel,
    closePanel,
    beginDrag,
    updateOpeningDrag,
    updateClosingDrag,
  }
}
