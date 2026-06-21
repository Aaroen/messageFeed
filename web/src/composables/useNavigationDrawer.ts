import { computed, readonly, ref, type Ref } from 'vue'

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

function clampProgress(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

export function useNavigationDrawer(options: NavigationDrawerOptions) {
  const motionTimings = useMotionTimings()
  const open = ref(false)
  const progress = ref(0)
  const settling = ref(false)
  let timer = 0

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
    if (typeof window !== 'undefined') {
      window.clearTimeout(timer)
    }
  }

  function settle(nextOpen: boolean) {
    clearTimer()
    settling.value = true
    open.value = nextOpen
    progress.value = nextOpen ? 1 : 0
    timer = window.setTimeout(() => {
      settling.value = false
      if (!nextOpen) {
        progress.value = 0
      }
    }, delay(settleDuration()))
  }

  function openPanel() {
    clearTimer()
    open.value = true
    settling.value = true
    progress.value = 0
    requestAnimationFrame(() => {
      progress.value = 1
    })
    timer = window.setTimeout(() => {
      settling.value = false
    }, delay(settleDuration()))
  }

  function closePanel() {
    if (!visible.value) {
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
