import { computed, ref, type Ref } from 'vue'

type SourceContentMotionOptions = {
  headerSpace: Ref<number>
  headerHeight: Ref<number>
  darkTheme: Ref<boolean>
  underDetail: Ref<boolean>
  revealProgress: Ref<number>
  chromeSettling: Ref<boolean>
  isVisible: () => boolean
  resolveDelay?: (duration: number) => number
}

type SourceContentSettlePhase = 'idle' | 'holding' | 'settling'

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

function cssTranslate3d(x: number, y: number, z = 0) {
  return `translate3d(${cssPx(x)}, ${cssPx(y)}, ${cssPx(z)})`
}

const sourceUnderlayTransition =
  'opacity var(--motion-quick) var(--ease-linear), filter var(--motion-quick) var(--ease-linear)'
const sourceContentShiftTransition = [
  'transform var(--motion-chrome) var(--ease-emphasized)',
  sourceUnderlayTransition,
].join(', ')

export function useSourceContentMotion(options: SourceContentMotionOptions) {
  const settleOffset = ref(0)
  const settlePhase = ref<SourceContentSettlePhase>('idle')
  let settleTimer = 0
  let settleFrame = 0
  let motionToken = 0

  const contentStyle = computed(() => {
    const underlayBaseOpacity = options.darkTheme.value ? 0.74 : 0.54
    const underlayBlur = options.underDetail.value
      ? (1 - options.revealProgress.value) * (options.darkTheme.value ? 5 : 8)
      : 0
    const underlayOpacity = options.underDetail.value
      ? underlayBaseOpacity + options.revealProgress.value * (1 - underlayBaseOpacity)
      : 1
    const contentShift = options.headerSpace.value - options.headerHeight.value
    const transition = settlePhase.value === 'settling'
      ? sourceContentShiftTransition
      : settlePhase.value === 'holding'
        ? 'none'
        : options.chromeSettling.value
          ? sourceContentShiftTransition
          : sourceUnderlayTransition

    return {
      paddingTop: cssPx(options.headerHeight.value + 14),
      opacity: underlayOpacity.toFixed(3),
      filter: `blur(${underlayBlur.toFixed(2)}px)`,
      transform: cssTranslate3d(0, contentShift + settleOffset.value),
      transition,
    }
  })

  function delay(duration: number) {
    return options.resolveDelay?.(duration) ?? duration
  }

  function clearTimer() {
    if (typeof window !== 'undefined' && settleTimer !== 0) {
      window.clearTimeout(settleTimer)
    }
    if (typeof window !== 'undefined' && settleFrame !== 0) {
      window.cancelAnimationFrame(settleFrame)
    }
    settleTimer = 0
    settleFrame = 0
    motionToken += 1
  }

  function reset() {
    clearTimer()
    settleOffset.value = 0
    settlePhase.value = 'idle'
  }

  function settleAfterRefresh(duration: number) {
    if (!options.isVisible()) {
      reset()
      return
    }

    clearTimer()
    const token = motionToken + 1
    motionToken = token
    settlePhase.value = 'holding'
    settleOffset.value = options.headerHeight.value
    settleFrame = window.requestAnimationFrame(() => {
      if (token !== motionToken) {
        return
      }
      settleFrame = window.requestAnimationFrame(() => {
        if (token !== motionToken) {
          return
        }
        settleFrame = 0
        if (!options.isVisible()) {
          reset()
          return
        }
        settlePhase.value = 'settling'
        settleOffset.value = 0
      })
    })
    settleTimer = window.setTimeout(() => {
      if (token !== motionToken) {
        return
      }
      settleTimer = 0
      settlePhase.value = 'idle'
    }, delay(duration))
  }

  return {
    contentStyle,
    clearTimer,
    reset,
    settleAfterRefresh,
  }
}
