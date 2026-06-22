import { readonly, shallowRef } from 'vue'

export type TimedNotice<TType extends string> = {
  type: TType
  message: string
}

type TimedNoticeOptions<TType extends string> = {
  duration: (type: TType) => number
  canShow?: () => boolean
  setNotice?: (notice: TimedNotice<TType> | null) => void
}

export function useTimedNotice<TType extends string>(options: TimedNoticeOptions<TType>) {
  const notice = shallowRef<TimedNotice<TType> | null>(null)
  let noticeTimer = 0
  let noticeTimerToken = 0
  let disposed = false

  function canShowNotice() {
    return !disposed && (options.canShow?.() ?? true)
  }

  function clearTimer() {
    noticeTimerToken += 1
    if (typeof window !== 'undefined' && noticeTimer !== 0) {
      window.clearTimeout(noticeTimer)
    }
    noticeTimer = 0
  }

  function clear() {
    clearTimer()
    setNotice(null)
  }

  function setNotice(nextNotice: TimedNotice<TType> | null) {
    notice.value = nextNotice
    options.setNotice?.(nextNotice)
  }

  function show(type: TType, message: string, durationMS?: number, delayMS = 0) {
    if (!canShowNotice()) {
      clear()
      return
    }

    const normalized = message.trim()
    if (!normalized) {
      clear()
      return
    }

    clearTimer()
    const token = noticeTimerToken
    const showNow = () => {
      if (token !== noticeTimerToken || !canShowNotice()) {
        return
      }
      setNotice({ type, message: normalized })
      const duration = durationMS ?? options.duration(type)
      if (duration > 0 && typeof window !== 'undefined') {
        noticeTimer = window.setTimeout(() => {
          if (token !== noticeTimerToken) {
            return
          }
          noticeTimer = 0
          setNotice(null)
        }, duration)
      }
    }

    if (delayMS > 0 && typeof window !== 'undefined') {
      noticeTimer = window.setTimeout(() => {
        if (token !== noticeTimerToken) {
          return
        }
        noticeTimer = 0
        showNow()
      }, delayMS)
      return
    }

    showNow()
  }

  function dispose() {
    disposed = true
    clear()
  }

  return {
    notice: readonly(notice),
    show,
    clear,
    clearTimer,
    dispose,
  }
}
