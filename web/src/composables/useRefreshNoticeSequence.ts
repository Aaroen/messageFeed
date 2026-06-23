type RefreshNotice<TType extends string> = {
  type: TType
  message: string
}

type RefreshNoticeSequenceOptions<TType extends string> = {
  show: (type: TType, message: string) => void
}

export function useRefreshNoticeSequence<TType extends string>(
  options: RefreshNoticeSequenceOptions<TType>,
) {
  let refreshReleased = true
  let queuedNotice: RefreshNotice<TType> | null = null

  function begin() {
    refreshReleased = false
    queuedNotice = null
  }

  function showNow(notice: RefreshNotice<TType>) {
    const message = notice.message.trim()
    if (!message) {
      return
    }
    options.show(notice.type, message)
  }

  function showAfterRefreshReleased(notice: RefreshNotice<TType>) {
    const message = notice.message.trim()
    if (!message) {
      return false
    }

    const nextNotice = { type: notice.type, message }
    if (refreshReleased) {
      showNow(nextNotice)
      return true
    }

    queuedNotice = nextNotice
    return true
  }

  function completeRefreshRelease(afterReleased?: () => void) {
    refreshReleased = true
    afterReleased?.()

    if (queuedNotice) {
      const notice = queuedNotice
      queuedNotice = null
      showNow(notice)
      return
    }
  }

  function cancel() {
    refreshReleased = true
    queuedNotice = null
  }

  return {
    begin,
    showAfterRefreshReleased,
    completeRefreshRelease,
    cancel,
  }
}
